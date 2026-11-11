package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// RegisterChatHandlers registers commands (/start, /help, /clear) and LLM text handlers.
func RegisterChatHandlers(b *bot.Bot, gmaClient *gma.Client, sm *gma.SessionManager, logger *slog.Logger) {
	// Command: /start
	b.OnStart(func(c *bot.Context) error {
		userID := getUserID(c)
		chatModel, visionModel, imageModel := sm.GetModels(userID)

		text := fmt.Sprintf("🤖 **Добро пожаловать в AI Ассистент!**\n\n"+
			"Я универсальный интеллектуальный бот, подключенный к шлюзу **Go Model API** (`gma.littfed.ru`).\n\n"+
			"✨ **Что я умею:**\n"+
			"• 💬 **Диалог:** Просто напишите мне любой вопрос или задачу текстом.\n"+
			"• 👁️ **Анализ фото:** Отправьте картинку (можно с вопросом) для распознавания.\n"+
			"• 🎨 **Генерация картинок:** Напишите `/image <описание>` или `/draw <описание>`.\n"+
			"• ⚙️ **Выбор моделей:** Нажмите `/model` для переключения активных нейросетей.\n"+
			"• 🧹 **Очистка памяти:** Напишите `/clear` для начала нового контекста.\n\n"+
			"⚡ **Активные модели:**\n"+
			"• Чат: `%s`\n"+
			"• Vision: `%s`\n"+
			"• Генерация: `%s`", chatModel, visionModel, imageModel)

		kb := BuildQuickActionsKeyboard()
		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Command: /help
	b.OnCommand("help", func(c *bot.Context) error {
		text := "📖 **Справка по командам AI Ассистента**\n\n" +
			"• `/start` — Перезапустить бота и открыть главное меню\n" +
			"• `/help` — Показать эту справку\n" +
			"• `/clear` — Очистить историю диалога (начать с чистого листа)\n" +
			"• `/model` — Просмотр и переключение AI-моделей (текст, vision, фото)\n" +
			"• `/image <промпт>` — Сгенерировать изображение по текстовому описанию\n" +
			"• `/draw <промпт>` — Альтернативная команда для рисования\n\n" +
			"💡 *Подсказка:* При отправке изображения бот автоматически распознает его и ответит на ваш вопрос."

		kb := BuildQuickActionsKeyboard()
		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Command: /clear
	b.OnCommand("clear", func(c *bot.Context) error {
		userID := getUserID(c)
		sm.Clear(userID)
		_, err := c.Reply("🧹 **Память диалога очищена!** Следующее сообщение начнет контекст с чистого листа.")
		return err
	})

	// Callback: action:clear
	b.OnCallbackPayload("action:clear", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := getUserID(c)
		sm.Clear(userID)
		_, err := c.Reply("🧹 **Память диалога очищена!**")
		return err
	})

	// Callback: nav:help
	b.OnCallbackPayload("nav:help", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		text := "📖 **Возможности AI Ассистента:**\n\n" +
			"1. Задавайте вопросы на любую тему — бот помнит до 12 последних реплик.\n" +
			"2. Отправляйте фото или документы с картинками для OCR и анализа.\n" +
			"3. Используйте `/image <текст>` для создания авторских артов."
		_, err := c.Reply(text)
		return err
	})

	// Core Text Message Handler (Fallback for plain user messages)
	b.OnMessage(func(c *bot.Context) error {
		msg := c.Message()
		if msg == nil {
			return nil
		}

		// If this message has an image attachment, let the vision handler process it
		if hasImageAttachment(msg) {
			return nil // Handled by vision matcher
		}

		text := strings.TrimSpace(msg.Body.Text)
		if text == "" {
			return nil
		}

		// Ignore commands starting with slash (they are handled by specific OnCommand routes)
		if strings.HasPrefix(text, "/") {
			return nil
		}

		userID := getUserID(c)
		chatModel, _, _ := sm.GetModels(userID)

		// Indicate typing in chat
		_ = c.Typing()

		// Build conversation messages with history
		history := sm.GetHistory(userID)
		messages := make([]gma.ChatMessage, 0, len(history)+2)
		messages = append(messages, gma.NewTextMessage(gma.RoleSystem, "Ты дружелюбный, умный и полезный AI-ассистент в мессенджере MAX. Отвечай емко, по делу и форматируй ответ в markdown."))
		messages = append(messages, history...)
		messages = append(messages, gma.NewTextMessage(gma.RoleUser, text))

		// Call GMA Chat API
		req := &gma.ChatCompletionRequest{
			Model:    chatModel,
			Messages: messages,
		}

		resp, err := gmaClient.ChatCompletion(c.Ctx(), req)
		if err != nil {
			logger.Error("ошибка обращения к Go Model API", slog.Any("error", err), slog.String("model", chatModel))
			_, replyErr := c.Reply(fmt.Sprintf("⚠️ **Ошибка AI-шлюза:** %s\n\nПопробуйте повторить запрос позже или переключите модель через `/model`.", err.Error()))
			return replyErr
		}

		answer := resp.Text()
		if strings.TrimSpace(answer) == "" {
			answer = "*(Модель вернула пустой ответ)*"
		}

		// Save conversation turns to ring-buffer
		sm.AddUserMessage(userID, text)
		sm.AddAssistantMessage(userID, answer)

		_, err = c.Reply(answer)
		return err
	})
}

// getUserID extracts a stable user identifier from Context.
func getUserID(c *bot.Context) int64 {
	if sender := c.Sender(); sender != nil && sender.UserID != 0 {
		return sender.UserID
	}
	if c.Update != nil && c.Update.ChatID != 0 {
		return c.Update.ChatID
	}
	return 1
}

// hasImageAttachment checks if the message contains an image or photo attachment.
func hasImageAttachment(msg *maxbot.Message) bool {
	if msg == nil {
		return false
	}
	for _, att := range msg.Body.Attachments {
		if att.Type == maxbot.UploadTypeImage || att.Type == "photo" {
			return true
		}
	}
	return false
}
