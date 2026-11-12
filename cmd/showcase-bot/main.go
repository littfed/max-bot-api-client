package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/payments"
	"github.com/littfed/max-bot-api-client/pkg/telemetry"
)

func main() {
	loadDotEnv()

	tokenFlag := flag.String("token", "", "MAX Bot API Token")
	portFlag := flag.String("port", "8080", "HTTP Server Port for Webhooks and Mini App")
	flag.Parse()

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("MAX_BOT_TOKEN"))
	}

	_ = os.MkdirAll("logs", 0755)
	var logOutput io.Writer = os.Stdout
	if logFile, err := os.OpenFile("logs/showcase-bot.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		defer logFile.Close()
		logOutput = io.MultiWriter(os.Stdout, logFile)
	}

	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("запуск Флагманского бота-витрины MAX Bot API...", slog.String("port", *portFlag))

	// 1. Initialize Under-The-Hood engine
	hood := NewHoodEngine()
	registerBaseTechCards(hood)

	// 2. Initialize FSM Storage (Session persistence)
	fsmStorage := fsm.NewMemoryStorage(1 * time.Hour)
	defer fsmStorage.Close()

	// 3. Initialize Telemetry Collector & Prometheus Exporter
	promExporter := telemetry.NewPrometheusExporter()
	collector := telemetry.NewCollector(
		telemetry.WithBufferSize(500),
		telemetry.WithSink(promExporter),
	)

	// 4. Initialize Payment Router & MAX Bot Client
	paymentRouter := payments.NewRouter()
	paymentRouter.RegisterGateway(payments.NewTBankGateway("TerminalLive", "SecretPass123"))
	paymentRouter.RegisterGateway(payments.NewYooKassaGateway("YooSecretKey"))

	var client *maxbot.Client
	var err error
	if token != "" {
		client, err = maxbot.NewClient(token)
		if err != nil {
			logger.Error("ошибка инициализации клиента MAX Bot API", slog.Any("error", err))
		} else {
			logger.Info("клиент MAX Bot API успешно подключен с сертификатами Минцифры")
		}
	} else {
		logger.Warn("MAX_BOT_TOKEN не задан; бот запущен в режиме оффлайн-сервера")
	}

	b := bot.NewWithClient(client, bot.WithLogger(logger))
	b.Use(bot.Recovery(logger))
	b.Use(bot.Logger(logger))
	b.Use(fsm.Middleware(fsmStorage))
	b.Use(telemetry.Middleware(collector))

	// 5. Setup AI Model Gateway Client (OpenAI-compatible)
	aiGatewayURL := strings.TrimSpace(os.Getenv("AI_GATEWAY_URL"))
	if aiGatewayURL == "" {
		aiGatewayURL = strings.TrimSpace(os.Getenv("GMA_BASE_URL"))
	}
	if aiGatewayURL != "" && !strings.HasSuffix(aiGatewayURL, "/v1") {
		aiGatewayURL = aiGatewayURL + "/v1"
	}
	if aiGatewayURL == "" {
		aiGatewayURL = "http://localhost:8080/v1"
	}

	aiGatewayKey := strings.TrimSpace(os.Getenv("AI_GATEWAY_KEY"))
	if aiGatewayKey == "" {
		aiGatewayKey = strings.TrimSpace(os.Getenv("GMA_API_KEY"))
	}

	chatModel := strings.TrimSpace(os.Getenv("AI_CHAT_MODEL"))
	if chatModel == "" {
		chatModel = strings.TrimSpace(os.Getenv("GMA_CHAT_MODEL"))
	}
	if chatModel == "" {
		chatModel = "gemini-2.5-flash"
	}

	imageModel := strings.TrimSpace(os.Getenv("AI_IMAGE_MODEL"))
	if imageModel == "" {
		imageModel = strings.TrimSpace(os.Getenv("GMA_IMAGE_MODEL"))
	}
	if imageModel == "" {
		imageModel = "gemini-2.5-flash-image"
	}

	voiceModel := strings.TrimSpace(os.Getenv("AI_VOICE_MODEL"))
	if voiceModel == "" {
		voiceModel = strings.TrimSpace(os.Getenv("GMA_VOICE_MODEL"))
	}
	if voiceModel == "" {
		voiceModel = "scout-audio"
	}

	aiClient := ai.NewClient(aiGatewayURL, aiGatewayKey, ai.WithTimeout(30*time.Second))

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := aiClient.Ping(pingCtx); err == nil {
		hood.SetAIAvailable(true)
		logger.Info("AI Gateway успешно подключен", slog.String("url", aiGatewayURL), slog.String("chat_model", chatModel))
	} else {
		logger.Info("AI Gateway оффлайн или не запущен при старте (AI раздел скрыт из меню)", slog.String("url", aiGatewayURL), slog.Any("error", err))
	}
	pingCancel()

	// 6. Setup HTTP Server (Webhooks, Mini App, Metrics)
	httpMux := http.NewServeMux()
	httpMux.Handle("/metrics", promExporter.Handler())
	httpMux.HandleFunc("/webhook/payments/tbank", paymentRouter.HTTPHandler("tbank"))
	httpMux.HandleFunc("/webhook/payments/yookassa", paymentRouter.HTTPHandler("yookassa"))

	// 7. Register Core Routes, Menu & Scenarios
	hood.RegisterRoutes(b.Router())
	RegisterMenuRoutes(b, hood, aiClient)
	RegisterEcomScenario(b, hood, paymentRouter)
	RegisterBookingScenario(b, hood)
	RegisterQuizScenario(b, hood)
	RegisterWebappScenario(b, hood, httpMux, token, *portFlag)
	RegisterGuardScenario(b, hood, client)
	RegisterAIScenario(b, hood, aiClient, AIConfig{
		ChatModel:  chatModel,
		ImageModel: imageModel,
		VoiceModel: voiceModel,
	})

	// Telemetry stats command
	b.OnCommand("stats", func(c *bot.Context) error {
		events := collector.Events()
		text := fmt.Sprintf("📊 **Телеметрия активной сессии**\n\n"+
			"• Зафиксировано событий в буфере: `%d`\n"+
			"• OpenMetrics эндпоинт: `http://localhost:%s/metrics`\n\n"+
			"Все клики по кнопкам замеряют Time-to-Click (TTK) и обогащаются UTM-метками.", len(events), *portFlag)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("◀️ В меню", "nav:menu")).
			Row(hood.Button("telemetry_collector"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	b.OnCallbackPayload("nav:stats", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		events := collector.Events()
		text := fmt.Sprintf("📊 **Телеметрия активной сессии**\n\n"+
			"• Зафиксировано событий в буфере: `%d`\n"+
			"• OpenMetrics эндпоинт: `http://localhost:%s/metrics`\n\n"+
			"Все клики по кнопкам замеряют Time-to-Click (TTK) и обогащаются UTM-метками.", len(events), *portFlag)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("◀️ В меню", "nav:menu")).
			Row(hood.Button("telemetry_collector"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// 7. Start HTTP Server
	server := &http.Server{
		Addr:    ":" + *portFlag,
		Handler: httpMux,
	}

	go func() {
		logger.Info("веб-сервер запущен", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("ошибка веб-сервера", slog.Any("error", err))
		}
	}()

	// 7. Register Bot Commands in MAX Menu
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if client != nil {
		_ = client.SetCommands(ctx, []maxbot.BotCommand{
			{Name: "start", Description: "Главное меню витрины возможностей"},
			{Name: "menu", Description: "Вызов интерактивного меню"},
			{Name: "dev", Description: "Переключить режим разработчика (Под капотом)"},
			{Name: "stats", Description: "Телеметрия и воронка кликов"},
			{Name: "reset", Description: "Сбросить текущий сценарий и сессию"},
		})

		logger.Info("начинаем Long Polling прослушивание...")
		if err := b.StartLongPolling(ctx); err != nil && err != context.Canceled {
			logger.Error("Long Polling остановлен с ошибкой", slog.Any("error", err))
		}
	} else {
		<-ctx.Done()
	}

	_ = server.Shutdown(context.Background())
	logger.Info("бот-витрина успешно остановлен")
}

func registerBaseTechCards(h *HoodEngine) {
	h.Register("main_menu", TechCard{
		Title:       "Главное меню и Роутер команд",
		Endpoint:    "/messages",
		Method:      "POST",
		Description: "Роутер bot.Router обрабатывает события bot_started и команду /start, возвращая двумерную матрицу инлайн-клавиатуры maxbot.InlineKeyboard. Кнопки используют кастомные payload для навигации.",
		DocsLink:    "https://dev.max.ru/docs/buttons",
	})

	h.Register("bot_architecture", TechCard{
		Title:       "Архитектура SDK и Zero-Config TLS",
		Endpoint:    "platform-api2.max.ru",
		Method:      "HTTPS",
		Description: "Клиент автоматически подключает встроенные корневые сертификаты НУЦ Минцифры РФ через certs.NewCertPool(), обеспечивая защищенное соединение без ошибок x509 на любых платформах.",
		DocsLink:    "https://github.com/littfed/max-bot-api-client",
	})

	h.Register("telemetry_collector", TechCard{
		Title:       "Сквозная телеметрия и Prometheus",
		Endpoint:    "/metrics",
		Method:      "GET",
		Description: "Модуль pkg/telemetry обогащает inline-кнопки метками времени и рассчитывает Time-to-Click (TTK) при нажатии. События собираются в кольцевой буфер и экспортируются в Prometheus/OpenMetrics.",
		DocsLink:    "https://prometheus.io/docs/instrumenting/exposition_formats/",
	})
}

func loadDotEnv() {
	paths := []string{".env", "../../.env", "../.env"}
	for _, p := range paths {
		file, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(key) == "" && val != "" {
				_ = os.Setenv(key, val)
			}
		}
		_ = file.Close()
	}
}
