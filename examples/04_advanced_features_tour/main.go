package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/guard"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/payments"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
)

func main() {
	token := os.Getenv("MAX_BOT_TOKEN")
	if token == "" {
		log.Println("MAX_BOT_TOKEN is not set; running in local demonstration mode")
	}

	client, err := maxbot.NewClient(token)
	if err != nil && token != "" {
		log.Fatalf("failed to create maxbot client: %v", err)
	}

	var b *bot.Bot
	if client != nil {
		b = bot.NewWithClient(client)
	} else {
		b = bot.NewWithClient(nil)
	}

	// 1. Initialize FSM Storage
	fsmStorage := fsm.NewMemoryStorage(30 * time.Minute)
	defer fsmStorage.Close()
	b.Use(fsm.Middleware(fsmStorage))

	// 2. Initialize Payment Router
	paymentRouter := payments.NewRouter()
	paymentRouter.RegisterGateway(payments.NewTBankGateway("Terminal01", "SecretPass"))
	paymentRouter.RegisterGateway(payments.NewYooKassaGateway("YooSecret"))

	paymentRouter.OnSuccess(func(ctx context.Context, p *payments.Payment) error {
		log.Printf("💰 Payment Confirmed: ID=%s Amount=%.2f %s UserID=%d", p.ID, p.Amount, p.Currency, p.UserID)
		if client != nil && p.ChatID != 0 {
			_, _ = client.SendTextMessage(ctx, maxbot.ToChat(p.ChatID),
				fmt.Sprintf("🎉 Оплата получена! Ваш заказ #%s успешно оформлен.", p.ID))
		}
		return nil
	})

	// 3. Initialize Guard Captcha Gate
	captchaGate := guard.NewCaptchaGate(client,
		guard.WithTimeout(60*time.Second),
		guard.WithOnVerified(func(ctx context.Context, chatID, userID int64) {
			log.Printf("🛡️ Member %d successfully verified in chat %d", userID, chatID)
		}),
		guard.WithOnKicked(func(ctx context.Context, chatID, userID int64) {
			log.Printf("🚫 Unverified member %d auto-kicked from chat %d", userID, chatID)
		}),
	)
	captchaGate.Register(b.Router())

	// 4. Initialize Multi-Step E-Commerce Wizard
	orderWizard := fsm.NewWizard("order_wizard",
		fsm.WithTimeout(10*time.Minute),
		fsm.WithOnCancel(func(c *bot.Context) error {
			_, err := c.Reply("❌ Оформление заказа отменено.")
			return err
		}),
		fsm.WithOnTimeout(func(c *bot.Context) error {
			_, err := c.Reply("⌛ Время оформления заказа истекло.")
			return err
		}),
	)

	// Step 1: Product Selection
	orderWizard.Step("STEP_SELECT_PRODUCT", func(c *bot.Context) error {
		product := c.Text()
		_ = fsm.FromContext(c).Set("product", product)

		_, _ = c.Reply(fmt.Sprintf("Вы выбрали: *%s*\n\nТеперь введите ваш *номер телефона* или *адрес доставки*:\n(или напишите 'назад' для выбора другого товара)", product))
		return orderWizard.Next(c)
	})

	// Step 2: Shipping details
	orderWizard.Step("STEP_SHIPPING", func(c *bot.Context) error {
		address := c.Text()
		_ = fsm.FromContext(c).Set("address", address)

		data, _ := fsm.FromContext(c).GetData()
		product := data["product"]

		// SBP dynamic QR payment link
		sbpLink := payments.GenerateSBPLink("ORDER_DEMO_99", payments.SBPOptions{
			Type:   payments.SBPTypeDynamic,
			Amount: 1990.00,
		})

		// Mini App button
		appBtn, _ := webapp.OpenAppButton("📱 Открыть в Mini App", "catalog_bot", "order_99")

		kb := maxbot.NewInlineKeyboard().
			Row(payments.PaymentButton("💳 Оплатить через СБП (1 990 ₽)", sbpLink)).
			Row(appBtn).
			Row(maxbot.NewCallbackButton("↩️ Отмена", "wizard:cancel"))

		_, _ = c.ReplyKeyboard(fmt.Sprintf("📦 *Подтверждение заказа*\n\n• Товар: %v\n• Доставка: %s\n• К оплате: 1 990 ₽\n\nВыберите способ оплаты:", product, address), kb)
		return orderWizard.Next(c)
	})

	// Step 3: Checkout pending
	orderWizard.Step("STEP_AWAITING_PAYMENT", func(c *bot.Context) error {
		_, err := c.Reply("Ожидаем подтверждения оплаты. Нажмите кнопку выше для перехода в платежный шлюз.")
		return err
	})

	orderWizard.Register(b.Router())

	// Start command: launches catalog
	b.OnStart(func(c *bot.Context) error {
		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🛍️ Начать заказ", "catalog:start")).
			Row(maxbot.NewCallbackButton("ℹ️ О боте", "bot:info"))

		_, err := c.ReplyKeyboard("👋 Добро пожаловать в демонстрационный магазин MAX Bot API!\n\nНажмите кнопку ниже, чтобы начать покупку.", kb)
		return err
	})

	b.OnCallbackPayload("catalog:start", func(c *bot.Context) error {
		_ = orderWizard.Start(c)
		_, err := c.Reply("📦 Шаг 1 из 3:\n\nВведите название интересующего товара (например: *Кофемашина* или *Наушники*):")
		return err
	})

	b.OnCallbackPayload("bot:info", func(c *bot.Context) error {
		return c.AnswerNotification("Демонстрационный бот MAX Bot API v1.0")
	})

	// HTTP Webhook server for Payments and WebApps
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/payments/tbank", paymentRouter.HTTPHandler("tbank"))
	mux.HandleFunc("/webhook/payments/yookassa", paymentRouter.HTTPHandler("yookassa"))

	bridge := webapp.NewBridge(token)
	bridge.OnAction("cart_sync", func(ctx context.Context, data *webapp.InitData, payload json.RawMessage) (any, error) {
		log.Printf("WebApp Cart Sync from user %v", data.User)
		return map[string]string{"status": "ok"}, nil
	})
	mux.HandleFunc("/api/webapp/action", bridge.HTTPHandler())

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("🌐 Payment & WebApp Webhook Server listening on http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("🤖 MAX Advanced Tour Bot is ready.")
	if client != nil {
		if err := b.StartLongPolling(ctx); err != nil && err != context.Canceled {
			log.Printf("Bot stopped with error: %v", err)
		}
	} else {
		<-ctx.Done()
	}

	_ = server.Shutdown(context.Background())
	log.Println("Shutdown complete.")
}
