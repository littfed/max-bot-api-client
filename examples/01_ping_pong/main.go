package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func main() {
	tokenFlag := flag.String("token", "", "MAX Bot API Access Token (or set MAX_BOT_TOKEN env variable)")
	flag.Parse()

	// Load token from flag, env, or .env file
	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("MAX_BOT_TOKEN"))
	}
	if token == "" {
		token = loadTokenFromDotEnv()
	}

	if token == "" {
		fmt.Println("================================================================================")
		fmt.Println("❌ ОШИБКА: Токен бота MAX не найден!")
		fmt.Println("================================================================================")
		fmt.Println("Пожалуйста, укажите токен одним из следующих способов:")
		fmt.Println("  1. Флаг командной строки:   go run ./examples/01_ping_pong -token <ВАШ_ТОКЕН>")
		fmt.Println("  2. Переменная окружения:    set MAX_BOT_TOKEN=<ВАШ_ТОКЕН> (Windows)")
		fmt.Println("                              export MAX_BOT_TOKEN=<ВАШ_ТОКЕН> (Linux/macOS)")
		fmt.Println("  3. Файл .env:               создайте файл .env со строкой MAX_BOT_TOKEN=<ВАШ_ТОКЕН>")
		fmt.Println("")
		fmt.Println("Токен бота можно получить на портале https://business.max.ru/")
		fmt.Println("================================================================================")
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("запуск MAX Ping-Pong бота...")

	// Create bot with embedded Russian Root CA certificates
	b, err := bot.New(token,
		bot.WithLogger(logger),
		bot.WithPollingTimeout(30),
	)
	if err != nil {
		logger.Error("ошибка инициализации клиента MAX", slog.Any("error", err))
		os.Exit(1)
	}

	// Attach standard middlewares
	b.Use(bot.Recovery(logger))
	b.Use(bot.Logger(logger))

	// 1. Handler for /start and bot_started event
	b.OnStart(func(c *bot.Context) error {
		senderName := "друг"
		if sender := c.Sender(); sender != nil && sender.FirstName != "" {
			senderName = sender.FirstName
		}

		payload := c.StartPayload()
		greeting := fmt.Sprintf("👋 Привет, %s! Я проверочный Ping-Pong бот в **MAX Messenger**.\n\n", senderName)
		if payload != "" {
			greeting += fmt.Sprintf("📍 Обнаружен параметр перехода: `%s`\n\n", payload)
		}
		greeting += "Отправь мне команду `/ping` или просто напиши `ping`, и я отвечу `pong! 🏓`."

		// Create inline keyboard with interactive ping button
		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewCallbackButton("🏓 Нажми меня (Ping)", "btn_ping"),
			)

		_, err := c.ReplyKeyboard(greeting, kb)
		return err
	})

	// 2. Handler for command /ping
	b.OnCommand("ping", func(c *bot.Context) error {
		start := time.Now()
		_, err := c.Reply(fmt.Sprintf("pong! 🏓 *(обработано за %v)*", time.Since(start).Round(time.Millisecond)))
		return err
	})

	// 3. Handler for plain text "ping" (case-insensitive)
	b.OnText("ping", func(c *bot.Context) error {
		_, err := c.Reply("pong! 🏓")
		return err
	})

	// 4. Handler for button click callback
	b.OnCallbackPayload("btn_ping", func(c *bot.Context) error {
		_ = c.AnswerNotification("Понг! 🏓")
		_, err := c.Reply("🏓 Вы нажали интерактивную кнопку: **pong!**")
		return err
	})

	// 5. Handler for /help
	b.OnCommand("help", func(c *bot.Context) error {
		helpText := "📖 **Справка по боту:**\n\n" +
			"• `/ping` — проверка соединения (ответ `pong! 🏓`)\n" +
			"• `ping` — текстовый пинг\n" +
			"• `/start` — главное приветствие с кнопкой\n" +
			"• `/help` — эта справка"
		_, err := c.Reply(helpText)
		return err
	})

	// Setup graceful shutdown on interrupt signal
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("начинаем Long Polling прослушивание...")
	if err := b.StartLongPolling(ctx); err != nil && err != context.Canceled {
		logger.Error("ошибка выполнения Long Polling", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("бот успешно остановлен")
}

// loadTokenFromDotEnv attempts to read MAX_BOT_TOKEN from .env or ../.env
func loadTokenFromDotEnv() string {
	paths := []string{".env", "../../.env", "../.env"}
	for _, p := range paths {
		file, err := os.Open(p)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if key == "MAX_BOT_TOKEN" && val != "" {
				return val
			}
		}
	}
	return ""
}
