package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func main() {
	tokenFlag := flag.String("token", "", "MAX Bot API Access Token (or set MAX_BOT_TOKEN env variable)")
	flag.Parse()

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
		fmt.Println("Укажите токен через флаг -token или переменную окружения MAX_BOT_TOKEN")
		fmt.Println("================================================================================")
		os.Exit(1)
	}

	// 1. Setup logging both to console (stdout) and file (logs/bot.log)
	_ = os.MkdirAll("logs", 0755)
	logFile, err := os.OpenFile(filepath.Join("logs", "bot.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	var logWriter io.Writer = os.Stdout
	if err == nil {
		defer logFile.Close()
		logWriter = io.MultiWriter(os.Stdout, logFile)
	}

	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	logger.Info("запуск интерактивного Feature Tour бота MAX...",
		slog.String("log_file", "logs/bot.log"),
	)

	b, err := bot.New(token,
		bot.WithLogger(logger),
		bot.WithPollingTimeout(30),
	)
	if err != nil {
		logger.Error("ошибка инициализации", slog.Any("error", err))
		os.Exit(1)
	}

	b.Use(bot.Recovery(logger))
	b.Use(bot.Logger(logger))

	// Register bot commands menu so the MAX client displays the Menu button
	go func() {
		err := b.Client().SetCommands(context.Background(), []maxbot.BotCommand{
			{Name: "start", Description: "Запустить бота"},
			{Name: "menu", Description: "Главное меню функций"},
			{Name: "help", Description: "Справка по боту"},
		})
		if err != nil {
			logger.Warn("не удалось обновить меню команд", slog.Any("error", err))
		} else {
			logger.Info("меню команд бота успешно обновлено (start, menu, help)")
		}
	}()

	// Track owner/last user ID to notify them when bot is added to a channel
	var lastPrivateUserID int64

	// Helper to send the main menu
	sendMainMenu := func(c *bot.Context) error {
		if sender := c.Sender(); sender != nil && sender.UserID != 0 {
			lastPrivateUserID = sender.UserID
		}

		menuText := "🚀 **MAX Feature Tour: Интерактивная отработка возможностей**\n\n" +
			"Этот бот позволяет вживую протестировать и отработать все возможности платформы MAX Messenger в диалоге с ботом.\n\n" +
			"Выберите раздел для проверки:"

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewIntentCallbackButton("🎨 Все типы кнопок", "menu_buttons", maxbot.IntentPrimary),
				maxbot.NewIntentCallbackButton("💬 Редактирование и удаление", "menu_edit_delete", maxbot.IntentDefault),
			).
			Row(
				maxbot.NewIntentCallbackButton("⚡ Колбэк-тост + замена", "menu_toast", maxbot.IntentPositive),
				maxbot.NewIntentCallbackButton("⌨️ Индикатор набора (Typing)", "menu_typing", maxbot.IntentDefault),
			).
			Row(
				maxbot.NewIntentCallbackButton("📸 Загрузка медиа", "menu_media", maxbot.IntentDefault),
				maxbot.NewIntentCallbackButton("🔗 Рефералы и диплинки", "menu_deeplink", maxbot.IntentDefault),
			).
			Row(
				maxbot.NewIntentCallbackButton("📢 Добавить в канал / группу", "menu_channel", maxbot.IntentNegative),
			)

		_, err := c.ReplyKeyboard(menuText, kb)
		return err
	}

	// 1. /start and bot_started handling
	b.OnStart(func(c *bot.Context) error {
		payload := c.StartPayload()
		if payload != "" {
			_, _ = c.Reply(fmt.Sprintf("🎉 **Успешный переход по диплинку!**\nПереданный параметр: `%s`", payload))
		}
		return sendMainMenu(c)
	})

	b.OnCommand("menu", sendMainMenu)
	b.OnCallbackPayload("menu_main", func(c *bot.Context) error {
		_ = c.AnswerNotification("Главное меню")
		return sendMainMenu(c)
	})

	// 2. Module 1: Buttons and intents
	b.OnCallbackPayload("menu_buttons", func(c *bot.Context) error {
		_ = c.AnswerNotification("Раздел: Кнопки")
		text := "🎨 **Демонстрация всех типов кнопок в MAX:**\n\n" +
			"• **Цвета (intent):** Зелёная (positive), Красная (negative), Синяя (primary).\n" +
			"• **Clipboard:** Копирует промокод в буфер обмена телефона в 1 клик.\n" +
			"• **Link:** Открывает внешний сайт.\n" +
			"• **Запрос данных:** Кнопка телефона (`request_contact`) и GPS (`request_geo_location`)."

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewIntentCallbackButton("✅ Positive", "act_pos", maxbot.IntentPositive),
				maxbot.NewIntentCallbackButton("❌ Negative", "act_neg", maxbot.IntentNegative),
				maxbot.NewIntentCallbackButton("⭐ Primary", "act_prim", maxbot.IntentPrimary),
			).
			Row(
				maxbot.NewClipboardButton("📋 Скопировать промокод", "MAX-2026-VIP"),
				maxbot.NewLinkButton("🌐 Портал dev.max.ru", "https://dev.max.ru/docs/"),
			).
			Row(
				maxbot.NewContactButton("📱 Поделиться телефоном"),
				maxbot.NewLocationButton("📍 Поделиться GPS", false),
			).
			Row(
				maxbot.NewIntentCallbackButton("⬅️ Назад в главное меню", "menu_main", maxbot.IntentDefault),
			)

		_, err := c.ReplyKeyboard(text, kb)
		if err != nil {
			logger.Error("ошибка отправки кнопочного меню", slog.Any("error", err))
		}
		return err
	})

	// Callback reactions for color buttons
	b.OnCallbackPayload("act_pos", func(c *bot.Context) error {
		return c.AnswerNotification("Вы нажали зеленую кнопку (Positive)! ✅")
	})
	b.OnCallbackPayload("act_neg", func(c *bot.Context) error {
		return c.AnswerNotification("Вы нажали красную кнопку (Negative)! ❌")
	})
	b.OnCallbackPayload("act_prim", func(c *bot.Context) error {
		return c.AnswerNotification("Вы нажали синюю кнопку (Primary)! ⭐")
	})

	// 3. Module 2: Edit and Delete with live countdown
	b.OnCallbackPayload("menu_edit_delete", func(c *bot.Context) error {
		_ = c.AnswerNotification("Запуск обратного отсчёта...")
		msg, err := c.Reply("⏳ **Тест редактирования и удаления:**\nСообщение самоуничтожится через **3** сек...")
		if err != nil {
			logger.Error("ошибка отправки сообщения для редактирования", slog.Any("error", err))
			return err
		}

		msgID := msg.ID()
		logger.Info("сообщение для редактирования отправлено", slog.String("msg_id", msgID))

		go func(id string) {
			time.Sleep(1 * time.Second)
			_, err1 := b.Client().EditMessage(context.Background(), id, &maxbot.EditMessageRequest{
				Text:   "⏳ **Тест редактирования и удаления:**\nСообщение самоуничтожится через **2** сек...",
				Format: "markdown",
			})
			if err1 != nil {
				logger.Error("ошибка редактирования (2 сек)", slog.Any("error", err1))
			}

			time.Sleep(1 * time.Second)
			_, err2 := b.Client().EditMessage(context.Background(), id, &maxbot.EditMessageRequest{
				Text:   "⏳ **Тест редактирования и удаления:**\nСообщение самоуничтожится через **1** сек... 💥",
				Format: "markdown",
			})
			if err2 != nil {
				logger.Error("ошибка редактирования (1 сек)", slog.Any("error", err2))
			}

			time.Sleep(1 * time.Second)
			errDel := b.Client().DeleteMessage(context.Background(), id)
			if errDel != nil {
				logger.Error("ошибка удаления сообщения", slog.Any("error", errDel))
			} else {
				logger.Info("сообщение успешно удалено", slog.String("msg_id", id))
			}

			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewIntentCallbackButton("⬅️ Назад в меню", "menu_main", maxbot.IntentPrimary))
			_, _ = c.ReplyKeyboard("✅ **Успешно!** Сообщение было отредактировано 3 раза и удалено без остатка.", kb)
		}(msgID)

		return nil
	})

	// 4. Module 3: Instant Toast + Message Update in single request
	var clickCounter int
	b.OnCallbackPayload("menu_toast", func(c *bot.Context) error {
		clickCounter = 0
		text := fmt.Sprintf("⚡ **Тест одновременного тоста и замены:**\n\nНажмите кнопку ниже. Бот покажет всплывающее уведомление (toast) и обновит текст сообщения за **один** сетевой запрос `POST /answers`.\n\nСчётчик нажатий: **%d**", clickCounter)
		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewIntentCallbackButton("🔥 Кликни меня!", "act_click_count", maxbot.IntentPositive)).
			Row(maxbot.NewIntentCallbackButton("⬅️ Назад в меню", "menu_main", maxbot.IntentDefault))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	b.OnCallbackPayload("act_click_count", func(c *bot.Context) error {
		clickCounter++
		newText := fmt.Sprintf("⚡ **Тест одновременного тоста и замены:**\n\nНажмите кнопку ниже. Бот покажет всплывающее уведомление (toast) и обновит текст сообщения за **один** сетевой запрос `POST /answers`.\n\nСчётчик нажатий: **%d** 🚀", clickCounter)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewIntentCallbackButton("🔥 Кликни меня ещё раз!", "act_click_count", maxbot.IntentPositive)).
			Row(maxbot.NewIntentCallbackButton("⬅️ Назад в меню", "menu_main", maxbot.IntentDefault))

		// Atomic notification + message body update
		return c.AnswerCallback(&maxbot.AnswerCallbackRequest{
			Notification: fmt.Sprintf("Учтено! Клик №%d", clickCounter),
			Message: &maxbot.MessageBody{
				Text:        newText,
				Format:      "markdown",
				Attachments: []maxbot.Attachment{kb.ToAttachment()},
			},
		})
	})

	// 5. Module 4: Typing Indicator
	b.OnCallbackPayload("menu_typing", func(c *bot.Context) error {
		_ = c.AnswerNotification("Включаем typing_on на 2.5 сек...")
		_ = c.Typing()

		time.Sleep(2500 * time.Millisecond)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewIntentCallbackButton("⬅️ Назад в меню", "menu_main", maxbot.IntentPrimary))
		_, err := c.ReplyKeyboard("⌨️ **Индикатор присутствия проверен!**\nВы видели статус «печатает...» в заголовке диалога в течение 2.5 секунд.", kb)
		return err
	})

	// 6. Module 5: Media Upload
	b.OnCallbackPayload("menu_media", func(c *bot.Context) error {
		_ = c.AnswerNotification("Генерируем и загружаем тестовое изображение...")

		// Generate a valid 240x100 PNG image
		img := image.NewRGBA(image.Rect(0, 0, 240, 100))
		for x := 0; x < 240; x++ {
			for y := 0; y < 100; y++ {
				img.Set(x, y, color.RGBA{R: 30, G: 120, B: 240, A: 255})
			}
		}
		var imgBuf bytes.Buffer
		_ = png.Encode(&imgBuf, img)

		// Two-step upload
		uploadResp, err := b.Client().UploadMedia(context.Background(), "image", "badge.png", &imgBuf)
		if err != nil {
			logger.Error("ошибка загрузки медиа", slog.Any("error", err))
			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewIntentCallbackButton("⬅️ Назад в меню", "menu_main", maxbot.IntentDefault))
			_, _ = c.ReplyKeyboard(fmt.Sprintf("⚠️ Ошибка загрузки через `/uploads`: `%v`.", err), kb)
			return nil
		}

		logger.Info("медиа успешно загружено", slog.String("token", uploadResp.Token))

		// Send message attaching the uploaded token
		msgPayload := &maxbot.SendMessageRequest{
			Text:   "📸 **Медиа успешно загружено через двухэтапный API!**\n\nПолучен многоразовый токен: `" + uploadResp.Token + "`",
			Format: "markdown",
			Attachments: []maxbot.Attachment{
				{
					Type: "image",
					Payload: map[string]string{
						"token": uploadResp.Token,
					},
				},
			},
		}

		rec, err := c.Recipient()
		if err != nil {
			return err
		}
		_, err = b.Client().SendMessage(context.Background(), rec, msgPayload)
		if err != nil {
			logger.Error("ошибка отправки фото с токеном", slog.Any("error", err))
		}
		return err
	})

	// 7. Module 6: Deep links and referral generator
	b.OnCallbackPayload("menu_deeplink", func(c *bot.Context) error {
		_ = c.AnswerNotification("Генерируем реферальную ссылку...")

		me, err := b.Client().GetMe(context.Background())
		if err != nil {
			return err
		}

		botUser := me.Username
		if botUser == "" {
			botUser = fmt.Sprintf("id%d", me.UserID)
		}

		refPayload := "ref_tour_user123"
		refLink := fmt.Sprintf("https://max.ru/%s?start=%s", botUser, refPayload)

		text := fmt.Sprintf("🔗 **Реферальные ссылки и Deep Linking в MAX:**\n\n"+
			"Ваша сгенерированная ссылка с параметром запуска:\n`%s`\n\n"+
			"Нажмите кнопку ниже, чтобы скопировать её в буфер обмена. Если перейти по ней (или передать другу), бот поймает событие `bot_started` и прочитает payload `%s`!",
			refLink, refPayload)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewClipboardButton("📋 Скопировать реферальную ссылку", refLink)).
			Row(maxbot.NewIntentCallbackButton("⬅️ Назад в меню", "menu_main", maxbot.IntentPrimary))

		_, err = c.ReplyKeyboard(text, kb)
		if err != nil {
			logger.Error("ошибка отправки реферального меню", slog.Any("error", err))
		}
		return err
	})

	// 8. Module 7: How to add to channel / group
	b.OnCallbackPayload("menu_channel", func(c *bot.Context) error {
		_ = c.AnswerNotification("Инструкция по каналам")

		me, _ := b.Client().GetMe(context.Background())
		botUsername := me.Username
		if botUsername == "" {
			botUsername = "ВАШ_БОТ"
		}

		text := fmt.Sprintf("📢 **Как добавить бота в канал или группу MAX:**\n\n"+
			"1️⃣ Откройте ваш канал или группу в приложении **MAX**.\n"+
			"2️⃣ Нажмите на название канала вверху экрана для входа в профиль.\n"+
			"3️⃣ Перейдите в раздел **«Администраторы»** ➔ **«Добавить администратора»**.\n"+
			"4️⃣ В строке поиска введите: `@%s`.\n"+
			"5️⃣ Назначьте боту права (публикация сообщений, управление).\n"+
			"6️⃣ Сохраните!\n\n"+
			"🔔 **Что произойдёт дальше:**\n"+
			"Как только бот будет добавлен, он автоматически поймает событие `bot_added`, напечатает ID канала в консоли и в файле `logs/bot.log`, а также сразу пришлёт вам сюда в ЛС сообщение с точным `chat_id` канала для последующих тестов!", botUsername)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewIntentCallbackButton("⬅️ Назад в меню", "menu_main", maxbot.IntentPrimary))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// 9. Event: Bot Added to a Channel or Group
	b.OnBotAdded(func(c *bot.Context) error {
		chatID := c.Update.ChatID
		logger.Info("🎉 БОТ ДОБАВЛЕН В НОВЫЙ КАНАЛ ИЛИ ГРУППУ!",
			slog.Int64("chat_id", chatID),
			slog.Any("update", c.Update),
		)

		chatInfo, err := b.Client().GetChat(context.Background(), chatID)
		chatTitle := "Неизвестно"
		chatType := "группа/канал"
		if err == nil && chatInfo != nil {
			chatTitle = chatInfo.Title
			chatType = chatInfo.Type
		}

		notifyText := fmt.Sprintf("🎉 **Бот успешно добавлен в новый %s!**\n\n"+
			"📌 **Название:** %s\n"+
			"🆔 **Chat ID:** `%d`\n\n"+
			"Теперь вы можете использовать этот `chat_id` для отправки постов и закрепления сообщений через `maxbot.ToChat(%d)`!",
			chatType, chatTitle, chatID, chatID)

		// Send notification to the user in PM if last private user is known
		if lastPrivateUserID != 0 {
			_, _ = b.Client().SendTextMessage(context.Background(), maxbot.ToUser(lastPrivateUserID), notifyText)
		}

		// Also post greeting into the group/channel itself
		_, _ = b.Client().SendTextMessage(context.Background(), maxbot.ToChat(chatID), "👋 Привет всем! Я бот-ассистент, успешно подключен к этому чату.")

		return nil
	})

	// 10. Handler for incoming Contact / Location sharing
	b.OnMessage(func(c *bot.Context) error {
		msg := c.Message()
		if msg == nil {
			return nil
		}

		for _, att := range msg.Body.Attachments {
			if att.Type == "contact" {
				_, err := c.Reply("📱 **Получен контакт пользователя!**\nТелефон и данные успешно переданы через нативный интерфейс MAX.")
				return err
			}
			if att.Type == "location" {
				_, err := c.Reply("📍 **Получены GPS-координаты!**\nГеолокация успешно получена через интерфейс MAX.")
				return err
			}
		}

		return nil
	})

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("Feature Tour бот готов к работе. Запустите диалог в MAX командой /start")
	if err := b.StartLongPolling(ctx); err != nil && err != context.Canceled {
		logger.Error("ошибка выполнения Long Polling", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("бот остановлен")
}

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
