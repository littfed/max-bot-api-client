package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/telemetry"
)

func main() {
	flagToken := flag.String("token", "", "MAX Bot API Token")
	flagPort := flag.String("metrics-port", "9090", "Prometheus metrics HTTP server port")
	flag.Parse()

	token := *flagToken
	if token == "" {
		token = os.Getenv("MAX_BOT_TOKEN")
	}
	if token == "" {
		// Read from .env if present
		if envBytes, err := os.ReadFile(".env"); err == nil {
			for _, line := range strings.Split(string(envBytes), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "MAX_BOT_TOKEN=") {
					token = strings.Trim(strings.TrimPrefix(line, "MAX_BOT_TOKEN="), "\"'")
					break
				}
			}
		}
	}

	if token == "" {
		log.Fatal("❌ Укажите токен через флаг -token или переменную окружения MAX_BOT_TOKEN")
	}

	// 1. Инициализация Prometheus экспортера и коллектора телеметрии
	promExporter := telemetry.NewPrometheusExporter()
	collector := telemetry.NewCollector(
		telemetry.WithBufferSize(500),
		telemetry.WithSink(promExporter),
		telemetry.WithSink(telemetry.NewJSONLogSink(os.Stdout)),
	)
	defer collector.Close()

	// 2. Запуск HTTP-сервера метрик Prometheus
	metricsAddr := ":" + *flagPort
	if err := promExporter.StartServer(metricsAddr); err != nil {
		log.Printf("⚠️ Не удалось запустить сервер метрик на %s: %v", metricsAddr, err)
	} else {
		fmt.Printf("📊 Prometheus метрики доступны: http://localhost:%s/metrics\n", *flagPort)
	}
	defer promExporter.Close()

	// 3. Создание бота и подключение middleware телеметрии
	b, err := bot.New(token)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	b.Use(bot.Recovery(nil))
	b.Use(telemetry.Middleware(collector))

	// Приветствие и трекаемая клавиатура воронки продаж
	b.OnStart(func(c *bot.Context) error {
		tk := telemetry.NewContextTrackedKeyboard(c, "onboarding")
		tk.Callback("💎 Тариф Pro (490 ₽)", "select_pro", maxbot.IntentPositive).
			Callback("⭐ Тариф VIP (1490 ₽)", "select_vip", maxbot.IntentPrimary).
			Row().
			Callback("ℹ️ О сервисе", "about").
			Link("🌐 Портал dev.max.ru (с UTM)", "https://dev.max.ru/docs/").
			Row()

		text := "👋 **Добро пожаловать в демонстрационный бот с телеметрией!**\n\n" +
			"Каждая кнопка и команда автоматически обогащается трекинг-метаданными и транслируется в Prometheus.\n\n" +
			"Выберите тариф для прохождения воронки:"

		_, err := c.ReplyKeyboard(text, tk.Build())
		return err
	})

	// Обработка выбора тарифа Pro
	b.OnAction("select_pro", func(c *bot.Context) error {
		_ = c.AnswerNotification("Выбран тариф Pro! 💎")

		// Фиксация бизнес-события воронки
		telemetry.Track(c, "funnel_tariff_selected", map[string]any{
			"tariff": "pro",
			"price":  490,
		})

		tk := telemetry.NewContextTrackedKeyboard(c, "checkout_pro")
		tk.Callback("💳 Оплатить Pro (490 ₽)", "checkout_pro", maxbot.IntentPositive).
			Callback("⬅️ Назад", "back_start", maxbot.IntentDefault).
			Row()

		_, err := c.ReplyKeyboard("💎 **Тариф Pro: Оформление заказа**\n\nСтоимость: `490 ₽/мес`.\nНажмите кнопку для оплаты:", tk.Build())
		return err
	})

	// Обработка выбора тарифа VIP
	b.OnAction("select_vip", func(c *bot.Context) error {
		_ = c.AnswerNotification("Выбран тариф VIP! ⭐")

		telemetry.Track(c, "funnel_tariff_selected", map[string]any{
			"tariff": "vip",
			"price":  1490,
		})

		tk := telemetry.NewContextTrackedKeyboard(c, "checkout_vip")
		tk.Callback("💳 Оплатить VIP (1490 ₽)", "checkout_vip", maxbot.IntentPositive).
			Callback("⬅️ Назад", "back_start", maxbot.IntentDefault).
			Row()

		_, err := c.ReplyKeyboard("⭐ **Тариф VIP: Оформление заказа**\n\nСтоимость: `1490 ₽/мес`.\nНажмите кнопку для оплаты:", tk.Build())
		return err
	})

	// Обработка подтверждения оплаты
	b.OnAction("checkout_pro", func(c *bot.Context) error {
		_ = c.AnswerNotification("Оплата тарифа Pro успешно принята!")
		telemetry.Track(c, "order_paid_success", map[string]any{"tariff": "pro", "price": 490})
		_, err := c.Reply("🎉 **Поздравляем!** Тариф Pro успешно активирован.")
		return err
	})

	b.OnAction("checkout_vip", func(c *bot.Context) error {
		_ = c.AnswerNotification("Оплата тарифа VIP успешно принята!")
		telemetry.Track(c, "order_paid_success", map[string]any{"tariff": "vip", "price": 1490})
		_, err := c.Reply("🎉 **Поздравляем!** Тариф VIP успешно активирован.")
		return err
	})

	b.OnAction("about", func(c *bot.Context) error {
		_ = c.AnswerNotification("MAX Bot Telemetry Engine v1.0")
		_, err := c.Reply("ℹ️ **О сервисе**\n\nБот работает на базе пакета `pkg/telemetry` и собирает:\n- CTR кнопок;\n- Время ответа (TTK);\n- Время работы хэндлеров;\n- Активных пользователей.")
		return err
	})

	b.OnAction("back_start", func(c *bot.Context) error {
		tk := telemetry.NewContextTrackedKeyboard(c, "onboarding")
		tk.Callback("💎 Тариф Pro (490 ₽)", "select_pro", maxbot.IntentPositive).
			Callback("⭐ Тариф VIP (1490 ₽)", "select_vip", maxbot.IntentPrimary).
			Row()
		_, err := c.ReplyKeyboard("Выберите тариф:", tk.Build())
		return err
	})

	// Просмотр текущих метрик прямо в чате
	b.OnCommand("metrics", func(c *bot.Context) error {
		events := collector.Last(5)
		summary := fmt.Sprintf("📊 **Текущие метрики бота:**\n\n- Всего событий в памяти: `%d`\n- Последние события:\n", len(collector.Events()))
		for _, e := range events {
			summary += fmt.Sprintf("  • `%s` (%s) user=%d\n", e.Action, e.Type, e.UserID)
		}
		summary += fmt.Sprintf("\n🌐 Prometheus endpoint: `http://localhost:%s/metrics`", *flagPort)
		_, err := c.Reply(summary)
		return err
	})

	// Ping-Pong
	b.OnCommand("ping", func(c *bot.Context) error {
		_, err := c.Reply("pong! 🏓")
		return err
	})

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Println("🚀 Telemetry Tour Bot запущен в режиме Long Polling...")
	if err := b.StartLongPolling(ctx); err != nil && err != context.Canceled && err != http.ErrServerClosed {
		log.Fatalf("Ошибка работы бота: %v", err)
	}
}
