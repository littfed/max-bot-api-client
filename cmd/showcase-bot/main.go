package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/littfed/max-bot-api-client/internal/admin"
	"github.com/littfed/max-bot-api-client/internal/api"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/chat"
	"github.com/littfed/max-bot-api-client/internal/modules/reception"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/internal/modules/support"
	"github.com/littfed/max-bot-api-client/internal/platform/max"
	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/logger"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/payments"
	"github.com/littfed/max-bot-api-client/pkg/store"
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

	logger := logger.InitGlobal()

	logger.Info("запуск Флагманского бота-витрины MAX Bot API...", slog.String("port", *portFlag))

	// 1. Initialize Under-The-Hood engine
	hood := NewHoodEngine()
	registerBaseTechCards(hood)

	// 2. Initialize FSM Storage (Distributed Redis or In-Memory fallback)
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	fsmStorage := fsm.NewStorageFromEnv(redisURL, 1*time.Hour)
	defer fsmStorage.Close()
	if _, isRedis := fsmStorage.(*fsm.RedisStorage); isRedis {
		logger.Info("FSM подключен к распределенному Redis хранилищу сессий", slog.String("redis_url", redisURL))
	} else {
		logger.Info("FSM запущен в режиме In-Memory хранилища сессий (1h TTL)")
	}

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
	if testChatID, _ := strconv.ParseInt(os.Getenv("TEST_CHAT_ID"), 10, 64); testChatID != 0 {
		testUserID, _ := strconv.ParseInt(os.Getenv("TEST_USER_ID"), 10, 64)
		b.RecordActiveChat(testChatID, testUserID)
	}
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
		chatModel = "auto"
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

	aiClient := ai.NewClient(
		aiGatewayURL,
		aiGatewayKey,
		ai.WithDefaultChatModel(chatModel),
		ai.WithTimeout(120*time.Second),
	)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 25*time.Second)
	if err := aiClient.Ping(pingCtx); err == nil {
		hood.SetAIAvailable(true)
		logger.Info("AI Gateway успешно подключен", slog.String("url", aiGatewayURL), slog.String("chat_model", chatModel))
	} else {
		logger.Info("AI Gateway оффлайн или не запущен при старте (AI раздел временно скрыт, запущен фоновый ретрай)", slog.String("url", aiGatewayURL), slog.Any("error", err))
		// Background retry: re-probe periodically so when gateway warms up, AI becomes available automatically
		go func() {
			for i := 0; i < 12; i++ {
				time.Sleep(15 * time.Second)
				retryCtx, retryCancel := context.WithTimeout(context.Background(), 20*time.Second)
				if err := aiClient.Ping(retryCtx); err == nil {
					hood.SetAIAvailable(true)
					logger.Info("AI Gateway успешно подключен после повторной проверки", slog.String("url", aiGatewayURL))
					retryCancel()
					break
				}
				retryCancel()
			}
		}()
	}
	pingCancel()

	// 6. Setup HTTP Server (Webhooks, Mini App, Metrics)
	httpMux := http.NewServeMux()
	httpMux.Handle("/metrics", promExporter.Handler())
	httpMux.HandleFunc("/webhook/payments/tbank", paymentRouter.HTTPHandler("tbank"))
	httpMux.HandleFunc("/webhook/payments/yookassa", paymentRouter.HTTPHandler("yookassa"))

	// 7. Initialize Persistent Store
	appStore, storeErr := store.NewFromEnv(os.Getenv("DATABASE_URL"), "data/bot_store.json")
	if storeErr != nil {
		logger.Warn("не удалось инициализировать bot_store, работаем с in-memory", slog.Any("error", storeErr))
		appStore, _ = store.New("")
	}
	defer appStore.Close()

	if appStore != nil && len(appStore.ListAllOrders()) == 0 {
		_ = appStore.CreateOrder(store.Order{
			ID:           "WEB-48201",
			UserID:       396393759,
			UserName:     "Федор В",
			Items: []store.OrderItem{
				{ID: "barista_coffee", Title: "Премиум Кофемашина Barista 15bar", Quantity: 1, Price: 4990},
				{ID: "gopher_hoodie", Title: "Худи Go Gopher Edition", Quantity: 1, Price: 3490},
			},
			Total:        8480,
			DeliveryType: "delivery",
			Address:      "г. Москва, ул. Тверская, д. 7",
			Status:       "paid",
			Verified:     true,
			CreatedAt:    time.Now().Add(-2 * time.Hour),
		})
		_ = appStore.CreateOrder(store.Order{
			ID:           "BOK-10942",
			UserID:       396393759,
			UserName:     "Федор В",
			Items: []store.OrderItem{
				{ID: "srv_audit", Title: "Архитектурный аудит Go / MAX", Quantity: 1, Price: 0},
			},
			Total:        0,
			DeliveryType: "booking",
			Address:      "Специалист: Lead Go Архитектор | Время: Завтра, 11:00",
			Status:       "accepted",
			Verified:     true,
			CreatedAt:    time.Now().Add(-4 * time.Hour),
		})
	}

	// 7.1. Initialize Persistent SQL Store for Bot Hub
	var sqlStore *store.SQLStore
	if appStore != nil && appStore.SQL() != nil {
		sqlStore = appStore.SQL()
	} else {
		dbConn, err := sql.Open("sqlite", ":memory:")
		if err == nil {
			sqlStore, err = store.NewSQLStore(dbConn, "sqlite")
			if err != nil {
				logger.Warn("не удалось инициализировать in-memory SQLStore", slog.Any("error", err))
			} else {
				logger.Info("SQLStore инициализирован в режиме in-memory SQLite с миграциями")
			}
		}
	}

	// 7.2. Initialize Bot Hub Core Components, Adapters & Virtual Modules
	var redisClient core.RedisClient
	if rStorage, ok := fsmStorage.(*fsm.RedisStorage); ok && rStorage != nil {
		redisClient = rStorage.Client()
	}
	compositeFSM := core.NewCompositeFSM(sqlStore, redisClient)
	lockManager := core.NewLockManager(redisClient)
	maintenanceGuard := core.NewMaintenanceGuard(sqlStore, redisClient)
	orchestrator := core.NewOrchestrator(sqlStore, compositeFSM, lockManager, maintenanceGuard)

	var botClient max.BotClient = client
	maxAdapter := max.NewAdapter(botClient)
	adminService := admin.NewService(sqlStore, maintenanceGuard, maxAdapter)

	shopMod := shop.New(sqlStore)
	receptionMod := reception.New(sqlStore, lockManager)
	supportMod := support.New(sqlStore)
	chatMod := chat.New(sqlStore, aiClient, chatModel)

	orchestrator.RegisterModule(shopMod)
	orchestrator.RegisterModule(receptionMod)
	orchestrator.RegisterModule(supportMod)
	orchestrator.RegisterModule(chatMod)

	// 7.3. Mount /api/v1/ REST API routes
	apiRouter := api.NewRouter(api.RouterConfig{
		Store:           sqlStore,
		Guard:           maintenanceGuard,
		FSM:             compositeFSM,
		Orchestrator:    orchestrator,
		AdminService:    adminService,
		ShopModule:      shopMod,
		ReceptionModule: receptionMod,
		SupportModule:   supportMod,
		ChatModule:      chatMod,
		BotToken:        token,
		JWTSecret:       os.Getenv("JWT_SECRET"),
		PlatformAdapter: maxAdapter,
		AllowDevHeaders: os.Getenv("ALLOW_DEV_HEADERS") == "true" || os.Getenv("DEV_MODE") == "true",
	})
	httpMux.Handle("/api/v1/", apiRouter)

	// 7.4. Mount Webhook endpoint and wire bot updates to Core Orchestrator
	webhookSecret := strings.TrimSpace(os.Getenv("MAX_WEBHOOK_SECRET"))
	httpMux.HandleFunc("/webhook", b.WebhookHandler(webhookSecret))
	wireBotToOrchestrator(b, orchestrator, logger)

	// 8. Register Core Routes, Menu & Scenarios
	hood.RegisterRoutes(b.Router())
	RegisterMenuRoutes(b, hood, aiClient)
	RegisterEcomScenario(b, hood, paymentRouter, appStore)
	RegisterBookingScenario(b, hood, appStore)
	RegisterQuizScenario(b, hood)
	RegisterWebappScenario(b, hood, httpMux, token, *portFlag, appStore, client, aiClient, chatModel, true)
	if err := RegisterWebSPA(httpMux); err != nil {
		logger.Error("ошибка регистрации embedded SPA", slog.Any("error", err))
	} else {
		logger.Info("встроенный React Mini App SPA успешно подключен на /")
	}
	RegisterGuardScenario(b, hood, client)
	RegisterChannelsScenario(b, hood, ChannelScenarioConfig{
		Client:     client,
		Store:      appStore,
		AIClient:   aiClient,
		ChatModel:  chatModel,
		MiniAppURL: "https://combot.littfed.ru/maxapp",
	})
	RegisterAIScenario(b, hood, aiClient, AIConfig{
		ChatModel:  chatModel,
		ImageModel: imageModel,
		VoiceModel: voiceModel,
		Store:      appStore,
	})

	// Telemetry stats command
	b.OnCommand("stats", func(c *bot.Context) error {
		events := collector.Events()
		text := fmt.Sprintf("📊 **Телеметрия активной сессии**\n\n"+
			"• Зафиксировано событий в буфере: `%d`\n"+
			"• OpenMetrics эндпоинт: `http://localhost:%s/metrics`\n\n"+
			"Все клики по кнопкам замеряют Time-to-Click (TTK) и обогащаются UTM-метками.", len(events), *portFlag)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("◀️ В меню", "nav:menu"))

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
			Row(maxbot.NewCallbackButton("◀️ В меню", "nav:menu").WithIntent(maxbot.IntentPrimary))

		_, err := c.EditOrReplyKeyboard(text, kb)
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
			{Name: "start", Description: "Приветствие и возможности бота"},
			{Name: "hub", Description: "Главный экран Bot Hub"},
			{Name: "mode", Description: "Выбор активного режима (Магазин, Ресепшен, Поддержка, Чат)"},
			{Name: "lead", Description: "Оставить заявку «Хочу такого бота»"},
			{Name: "app", Description: "Открыть Mini App (витрина, AI, заказы)"},
			{Name: "operator", Description: "Позвать человека / менеджера (HITL)"},
			{Name: "resume", Description: "Вернуться к боту (возобновить AI)"},
			{Name: "reset", Description: "Новый диалог (очистить контекст ИИ)"},
			{Name: "new", Description: "Новый диалог с чистого листа"},
			{Name: "model", Description: "Выбор активной модели AI (LLM)"},
			{Name: "img", Description: "Сгенерировать картинку нейросетью"},
			{Name: "orders", Description: "Мои заказы и бронирования"},
			{Name: "menu", Description: "Витрина возможностей MAX (Магазин, Бронь, Mini App)"},
			{Name: "dev", Description: "Режим разработчика (Под капотом)"},
			{Name: "stats", Description: "Телеметрия активной сессии"},
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
