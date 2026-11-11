package main

import (
	"bufio"
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func main() {
	tokenFlag := flag.String("token", "", "MAX Bot API Token")
	gmaURLFlag := flag.String("gma-url", "", "Go Model API Base URL (default: https://gma.littfed.ru)")
	gmaKeyFlag := flag.String("gma-key", "", "Go Model API Key")
	flag.Parse()

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("MAX_BOT_TOKEN"))
	}
	if token == "" {
		token = loadTokenFromDotEnv()
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	gmaURL := strings.TrimSpace(*gmaURLFlag)
	if gmaURL == "" {
		gmaURL = strings.TrimSpace(os.Getenv("GMA_BASE_URL"))
	}
	if gmaURL == "" {
		gmaURL = gma.DefaultBaseURL
	}

	gmaKey := strings.TrimSpace(*gmaKeyFlag)
	if gmaKey == "" {
		gmaKey = strings.TrimSpace(os.Getenv("GMA_API_KEY"))
	}

	logger.Info("запуск AI Ассистента MAX Bot API...",
		slog.String("gma_url", gmaURL),
		slog.Bool("has_gma_key", gmaKey != ""),
		slog.Bool("has_bot_token", token != ""),
	)

	// 1. Initialize GMA Client & Session Manager
	gmaClient := gma.NewClient(gmaURL, gmaKey, gma.WithTimeout(120*time.Second))
	sessionManager := gma.NewSessionManager(12, 30*time.Minute)

	// 2. Initialize MAX Bot Client
	var client *maxbot.Client
	if token != "" {
		var err error
		client, err = maxbot.NewClient(token)
		if err != nil {
			logger.Error("ошибка инициализации MAX Bot Client", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		logger.Warn("ВНИМАНИЕ: MAX_BOT_TOKEN не задан! Задайте переменную окружения MAX_BOT_TOKEN для запуска бота.")
	}

	// 3. Initialize Bot Engine
	b := bot.NewWithClient(client, bot.WithLogger(logger))
	b.Use(bot.Recovery(logger))

	// 4. Register Route Handlers
	RegisterMenuHandlers(b, sessionManager)
	RegisterImageHandlers(b, gmaClient, sessionManager, logger)
	RegisterVisionHandlers(b, gmaClient, sessionManager, logger)
	RegisterChatHandlers(b, gmaClient, sessionManager, logger)

	// 5. Graceful Shutdown & Polling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if client != nil {
		_ = client.SetCommands(ctx, []maxbot.BotCommand{
			{Name: "start", Description: "Запуск бота и главное меню"},
			{Name: "help", Description: "Справка по возможностям и командам"},
			{Name: "model", Description: "Выбор AI-моделей (текст, vision, картинки)"},
			{Name: "image", Description: "Сгенерировать картинку (/image <промпт>)"},
			{Name: "draw", Description: "Нарисовать картинку (/draw <промпт>)"},
			{Name: "clear", Description: "Очистить память диалога"},
		})

		logger.Info("запуск Long Polling цикла обработки сообщений...")
		if err := b.StartLongPolling(ctx); err != nil && err != context.Canceled {
			logger.Error("фатальная ошибка Long Polling", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		logger.Info("ожидание сигнала завершения (dry-run без токена)...")
		<-ctx.Done()
	}

	logger.Info("AI Ассистент остановлен корректно")
}

// loadTokenFromDotEnv reads MAX_BOT_TOKEN from local .env files.
func loadTokenFromDotEnv() string {
	paths := []string{".env", ".env.local", ".env.ai-bot", "../../.env", "../.env"}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "MAX_BOT_TOKEN" {
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if val != "" {
					_ = f.Close()
					return val
				}
			}
		}
		_ = f.Close()
	}
	return ""
}
