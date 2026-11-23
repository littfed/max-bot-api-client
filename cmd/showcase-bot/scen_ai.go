package main

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/logger"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

const aiSystemPrompt = "Ты дружелюбный и умный AI-ассистент в мессенджере MAX. Отвечай всегда грамотно, четко, доброжелательно и строго на естественном русском языке. Используй красивую разметку и отвечай по существу."

// globalAIDispatcher is the active AI message router used by idle handlers across the bot.
var globalAIDispatcher func(c *bot.Context) error

// generatePlaceholderImage creates a valid in-memory PNG card showcasing MAX photo attachments.
func generatePlaceholderImage(prompt string) []byte {
	width, height := 600, 360
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8(25 + (x * 35 / width))
			g := uint8(20 + (y * 40 / height))
			b := uint8(60 + ((x + y) * 50 / (width + height)))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// AIConfig holds configuration parameters for the AI scenario.
type AIConfig struct {
	ChatModel   string
	ImageModel  string
	VoiceModel  string
	VisionModel string
	Store       *store.Store
}

// DefaultAIConfig returns production-ready model defaults for go-model-api.
func DefaultAIConfig() AIConfig {
	return AIConfig{
		ChatModel:   "auto",
		ImageModel:  "gemini-2.5-flash-image",
		VoiceModel:  "scout-audio",
		VisionModel: "auto",
	}
}

// RegisterAIScenario registers all AI multimodal routes, FSM states, and technical cards.
func RegisterAIScenario(b *bot.Bot, hood *HoodEngine, aiClient *ai.Client, cfg ...AIConfig) {
	config := DefaultAIConfig()
	if len(cfg) > 0 {
		if cfg[0].ChatModel != "" {
			config.ChatModel = cfg[0].ChatModel
		}
		if cfg[0].ImageModel != "" {
			config.ImageModel = cfg[0].ImageModel
		}
		if cfg[0].VoiceModel != "" {
			config.VoiceModel = cfg[0].VoiceModel
		}
		if cfg[0].VisionModel != "" {
			config.VisionModel = cfg[0].VisionModel
		}
		if cfg[0].Store != nil {
			config.Store = cfg[0].Store
		}
	}

	// 1. Register Under-The-Hood Technical Cards
	hood.Register("ai_chat", TechCard{
		Title:       "AI-Диалог & Скользящее окно контекста (Sliding Window)",
		Endpoint:    "/v1/chat/completions",
		Method:      "POST",
		FSMState:    "AI_STEP_CHAT",
		Description: "Мультимодельный диалог с сохранением истории (до 10 сообщений) в FSM сессии. Отправка typing_on индикатора и генерация ответа через go-model-api.",
		DocsLink:    "https://platform.openai.com/docs/api-reference/chat",
	})

	hood.Register("ai_image", TechCard{
		Title:       "Генерация изображений (Text-to-Image & UploadMedia)",
		Endpoint:    "/v1/images/generations",
		Method:      "POST",
		FSMState:    "AI_STEP_IMAGE",
		Description: "Шлюз генерирует изображение (Flux/SD/DALL-E), клиент загружает байты через двухфазный UploadMedia в CDN MAX и отправляет нативное фото-вложение.",
		DocsLink:    "https://dev.max.ru/docs/uploads",
	})

	hood.Register("ai_voice", TechCard{
		Title:       "Распознавание речи Whisper STT & LLM Pipeline",
		Endpoint:    "/v1/audio/transcriptions",
		Method:      "POST",
		FSMState:    "AI_STEP_VOICE",
		Description: "Голосовое сообщение в формате MP3/OGG/WAV отправляется в multipart/form-data Whisper, расшифровывается в текст с цитированием и мгновенно подается на вход языковой модели.",
		DocsLink:    "https://platform.openai.com/docs/api-reference/audio",
	})

	hood.Register("ai_tts", TechCard{
		Title:       "Синтез речи (Text-to-Speech & UploadMedia Audio)",
		Endpoint:    "/v1/audio/speech",
		Method:      "POST",
		FSMState:    "AI_STEP_TTS",
		Description: "Генерация аудиофайла MP3 из текста, двухфазная загрузка UploadMedia(audio) в CDN MAX и отправка нативного голосового сообщения.",
		DocsLink:    "https://platform.openai.com/docs/api-reference/audio/createSpeech",
	})

	hood.Register("ai_vision", TechCard{
		Title:       "Компьютерное зрение & OCR (Multimodal Vision API)",
		Endpoint:    "/v1/chat/completions",
		Method:      "POST",
		FSMState:    "AI_STEP_VISION",
		Description: "Мультимодальный анализ изображений, распознавание предметов, текста (OCR) и схем через новейшие Vision-модели (Gemini / GPT-4o).",
		DocsLink:    "https://platform.openai.com/docs/guides/vision",
	})

	// Helper to get user ID safely
	getUserID := func(c *bot.Context) int64 {
		if sender := c.Sender(); sender != nil {
			return sender.UserID
		}
		return 0
	}

	// 2. Main AI Scenario Hub
	b.OnCallbackPayload("scen:ai", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := getUserID(c)

		text := "✨ **Интеграция с Искусственным Интеллектом (AI Gateway)**\n\n" +
			"Бот подключен к универсальному AI-шлюзу **go-model-api** (OpenAI-compatible) и демонстрирует пять передовых мультимодальных сценариев:\n\n" +
			"💬 **1. Контекстный диалог (LLM):** умный чат со скользящим окном памяти на 10 сообщений в FSM.\n" +
			"🎨 **2. Генерация изображений:** синтез картинок по текстовому описанию (Flux) и доставка прямо в чат.\n" +
			"🎙️ **3. Распознавание голоса:** транскрипция аудиосообщений через Whisper STT и мгновенный ответ нейросети.\n" +
			"🔊 **4. Генерация голоса (TTS):** озвучивание текста в настоящее аудиосообщение прямо в чат MAX.\n" +
			"👁️ **5. Компьютерное зрение (Vision):** анализ фото, распознавание предметов и OCR текста с картинки.\n\n" +
			"👇 *Выберите режим для тестирования:*"

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("💬 1. Диалог с контекстом (LLM)", "ai:chat:start")).
			Row(maxbot.NewCallbackButton("🎨 2. Генерация картинок (Flux)", "ai:image:start")).
			Row(maxbot.NewCallbackButton("🎙️ 3. Распознавание голоса (Whisper)", "ai:voice:start")).
			Row(maxbot.NewCallbackButton("🔊 4. Генерация голоса (TTS)", "ai:tts:start")).
			Row(maxbot.NewCallbackButton("👁️ 5. Компьютерное зрение (Vision)", "ai:vision:start")).
			Row(maxbot.NewCallbackButton("⚙️ Выбор модели LLM", "ai:models")).
			Row(
				maxbot.NewCallbackButton("🔄 Очистить контекст", "ai:clear_context"),
				maxbot.NewCallbackButton("👨‍💼 Позвать человека", "nav:operator"),
			).
			Row(maxbot.NewCallbackButton("◀️ Главное меню", "nav:menu"))

		formatted := hood.Format(userID, text, "ai_chat")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// 3. Clear Chat Context Handler
	b.OnCallbackPayload("ai:clear_context", func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.Set("ai_chat_history", "[]")
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}
		_ = c.AnswerNotification("🔄 Контекст диалога очищен (0/10)!")

		text := "🔄 **Контекст диалога очищен (0/10).**\n\n" +
			"История предыдущих сообщений удалена. Следующий запрос начнется с чистого листа!"

		_, err := c.Reply(text)
		return err
	})

	// 4. Start Conversational Chat Mode
	b.OnCallbackPayload("ai:chat:start", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}

		activeModel := config.ChatModel
		if fsmCtx != nil {
			if m, ok := fsmCtx.Get("ai_active_chat_model"); ok {
				if mStr, isStr := m.(string); isStr && mStr != "" {
					activeModel = mStr
				}
			}
		}

		userID := getUserID(c)
		history := loadChatHistory(fsmCtx)

		text := fmt.Sprintf("💬 **Режим умного AI-диалога**\n\n"+
			"• Активная модель: `%s`\n"+
			"• Сообщений в памяти: `%d / 10`\n\n"+
			"Напишите любой вопрос или задачу обычным текстом.\n"+
			"Бот помнит историю диалога и учитывает контекст.\n\n"+
			"*Примеры вопросов:*\n"+
			"• «Напиши функцию на Go для валидации email»\n"+
			"• «Составь план тренировок на 3 дня»\n"+
			"• «Объясни принцип работы HTTP/3 простыми словами»",
			activeModel, len(history))

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("⚙️ Сменить модель LLM", "ai:models")).
			Row(maxbot.NewCallbackButton(fmt.Sprintf("🔄 Очистить контекст (%d/10)", len(history)), "ai:clear_context")).
			Row(
				maxbot.NewCallbackButton("🎨 Нарисовать картинку", "ai:image:start"),
				maxbot.NewCallbackButton("🎙️ Голос (Whisper)", "ai:voice:start"),
			).
			Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))

		formatted := hood.Format(userID, text, "ai_chat")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// 4a. Model Selection Submenu (ai:models)
	b.OnCallbackPayload("ai:models", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		fsmCtx := fsm.FromContext(c)
		activeModel := config.ChatModel
		if fsmCtx != nil {
			if m, ok := fsmCtx.Get("ai_active_chat_model"); ok {
				if mStr, isStr := m.(string); isStr && mStr != "" {
					activeModel = mStr
				}
			}
		}

		text := fmt.Sprintf("⚙️ **Выбор активной языковой модели (LLM)**\n\n"+
			"• Текущая модель: `%s`\n\n"+
			"Выберите модель для диалога:\n"+
			"• **⚡ Auto Router** — сбалансированный умный выбор с авто-переключением\n"+
			"• **🚀 Fast Router** — мгновенный отклик (<500ms, Gemini Flash Lite)\n"+
			"• **💻 Best Coding** — эксперт по коду, архитектуре и рефакторингу\n"+
			"• **🧠 Reasoning** — глубокие рассуждения и логический анализ\n"+
			"• **✨ Gemini 3.5 Flash** — новейшая компактная модель Google", activeModel)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("⚡ Auto (Рекомендуется)", "ai:set_model:auto")).
			Row(maxbot.NewCallbackButton("🚀 Fast (Сверхбыстрый)", "ai:set_model:fast")).
			Row(maxbot.NewCallbackButton("💻 Best Coding (Код)", "ai:set_model:best-coding")).
			Row(maxbot.NewCallbackButton("🧠 Reasoning (Анализ)", "ai:set_model:reasoning")).
			Row(maxbot.NewCallbackButton("✨ Gemini 3.5 Flash", "ai:set_model:gemini-3.5-flash-lite")).
			Row(maxbot.NewCallbackButton("◀️ Назад в диалог", "ai:chat:start"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// 4b. Set Model Callback (ai:set_model:{model})
	b.OnCallbackPrefix("ai:set_model:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}
		newModel := strings.TrimPrefix(cb.Payload, "ai:set_model:")
		fsmCtx := fsm.FromContext(c)
		history := loadChatHistory(fsmCtx)
		if fsmCtx != nil {
			_ = fsmCtx.Set("ai_active_chat_model", newModel)
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}
		_ = c.AnswerNotification(fmt.Sprintf("✅ Модель переключена на %s", newModel))

		text := fmt.Sprintf("✅ **Модель успешно переключена!**\n\n"+
			"• Активная модель: `%s`\n"+
			"• Сообщений в памяти: `%d/10`\n"+
			"• Режим умного диалога **активирован**\n\n"+
			"Напишите любой вопрос или задачу прямо в чат — бот ответит через эту модель и сохранит контекст в памяти.", newModel, len(history))
		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("💬 Продолжить диалог", "ai:chat:start")).
			Row(maxbot.NewCallbackButton(fmt.Sprintf("🔄 Очистить контекст (%d/10)", len(history)), "ai:clear_context")).
			Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))
		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	var processVisionImage func(c *bot.Context, prompt string) error
	var deliverImageGen func(c *bot.Context, prompt string) error
	var processVoiceAudio func(c *bot.Context, audioBytes []byte, filename string) error
	var processTextConversation func(c *bot.Context, userText string) error
	var processWebPageSummary func(c *bot.Context, rawURL, userQuery string) error
	var processDocumentFile func(c *bot.Context) error
	var processDocumentFileWithBytes func(c *bot.Context, fileBytes []byte, filename string) error
	var dispatchAIMessage func(c *bot.Context) error

	// Helper for Multimodal Vision Image Recognition
	processVisionImage = func(c *bot.Context, prompt string) error {
		userID := getUserID(c)
		if aiClient == nil {
			_, err := c.Reply("⚠️ AI-сервис временно недоступен. Попробуйте повторить запрос позже.")
			return err
		}

		_ = c.Typing()
		stopTyping := make(chan struct{})
		go func() {
			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopTyping:
					return
				case <-ticker.C:
					_ = c.Typing()
				}
			}
		}()

		prompt = strings.TrimSpace(prompt)
		if prompt == "" || prompt == "Распознай картинку" || prompt == "Опиши фото" || prompt == "/vision" {
			prompt = "Опиши подробно, что изображено на картинке. Если на ней присутствует текст, расшифруй его полностью и точно (OCR)."
		}

		imgBytes, _, _ := c.ImageBytes()
		imgURL, _ := c.ImageURL()

		c.Logger().Info("vision: начало обработки изображения",
			slog.Int64("user_id", userID),
			slog.Bool("has_bytes", len(imgBytes) > 0),
			slog.Int("bytes_len", len(imgBytes)),
			slog.String("image_url", imgURL),
			slog.String("prompt", prompt),
		)

		if len(imgBytes) == 0 && imgURL == "" {
			close(stopTyping)
			c.Logger().Warn("vision: не удалось извлечь ни байты, ни URL картинки", slog.Int64("user_id", userID))
			_, err := c.Reply("⚠️ Не удалось извлечь изображение из сообщения. Пожалуйста, прикрепите фото или картинку еще раз.")
			return err
		}

		activeVisionModel := config.VisionModel
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			if m, ok := fsmCtx.Get("ai_active_vision_model"); ok {
				if mStr, isStr := m.(string); isStr && mStr != "" {
					activeVisionModel = mStr
				}
			}
		}

		candidates := []string{activeVisionModel}
		for _, m := range []string{"auto", "@scout-vision", "gemini-2.5-flash", "fast", "best-coding"} {
			if m != activeVisionModel {
				candidates = append(candidates, m)
			}
		}

		var visionResp *ai.ChatResponse
		var lastErr error
		var usedModel string

		for _, m := range candidates {
			vReq := &ai.VisionRequest{
				Model:      m,
				Prompt:     prompt,
				ImageURL:   imgURL,
				ImageBytes: imgBytes,
			}
			visionResp, lastErr = aiClient.Vision(c.Ctx(), vReq)
			if lastErr == nil && visionResp != nil && strings.TrimSpace(visionResp.Content) != "" {
				usedModel = m
				c.Logger().Info("vision: изображение успешно распознано",
					slog.Int64("user_id", userID),
					slog.String("model", usedModel),
					slog.Duration("latency", visionResp.Latency),
					slog.Int("total_tokens", visionResp.TotalTokens),
				)
				break
			}
			c.Logger().Warn("vision: модель вернула ошибку, переходим к следующему кандидату",
				slog.Int64("user_id", userID),
				slog.String("failed_model", m),
				slog.Any("error", lastErr),
			)
		}
		close(stopTyping)

		if visionResp == nil || strings.TrimSpace(visionResp.Content) == "" {
			errMsg := "не удалось распознать изображение"
			if lastErr != nil {
				errMsg = lastErr.Error()
			}
			c.Logger().Error("vision: все кандидаты вернули ошибку",
				slog.Int64("user_id", userID),
				slog.Any("last_error", lastErr),
			)
			errText := fmt.Sprintf("⚠️ **Ошибка распознавания фото:** %s\n\nМодель Vision временно перегружена. Попробуйте отправить другое фото или повторить попытку позже.", errMsg)
			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("👁️ В меню Vision", "ai:vision:start")).
				Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))
			_, rErr := c.ReplyKeyboard(errText, kb)
			return rErr
		}

		answer := visionResp.Content

		// Add to conversation memory
		history := loadChatHistory(fsmCtx)
		history = append(history, ai.ChatMessage{
			Role:    ai.RoleUser,
			Content: fmt.Sprintf("[Пользователь прикрепил изображение]: %s", prompt),
		})
		history = append(history, ai.ChatMessage{
			Role:    ai.RoleAssistant,
			Content: answer,
		})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		_ = saveChatHistory(fsmCtx, history)
		if fsmCtx != nil {
			_ = fsmCtx.Set("last_ai_reply", answer)
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}

		hood.Register("ai_vision", TechCard{
			Title:       "Компьютерное зрение & OCR (Multimodal Vision API)",
			Endpoint:    "/v1/chat/completions",
			Method:      "POST",
			FSMState:    "AI_STEP_VISION",
			Description: fmt.Sprintf("Модель: %s | Задержка: %s | Токенов: %d | Промпт: «%s».", usedModel, visionResp.Latency.Round(time.Millisecond), visionResp.TotalTokens, prompt),
			DocsLink:    "https://platform.openai.com/docs/guides/vision",
		})

		responseBody := fmt.Sprintf("👁️ **Анализ изображения (`%s`):**\n\n%s", usedModel, answer)
		formatted := hood.Format(userID, responseBody, "ai_vision")

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
				maxbot.NewCallbackButton("🔊 Озвучить", "ai:tts:speak_last"),
			)

		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	}

	// 5. Helper for Text Conversation via LLM
	processTextConversation = func(c *bot.Context, userText string) error {
		userText = strings.TrimSpace(userText)
		if userText == "" {
			return nil
		}

		if aiClient == nil {
			_, err := c.Reply("⚠️ AI-сервис временно недоступен. Попробуйте повторить запрос позже.")
			return err
		}

		userID := getUserID(c)
		fsmCtx := fsm.FromContext(c)
		history := loadChatHistory(fsmCtx)

		activeModel := config.ChatModel
		if fsmCtx != nil {
			if m, ok := fsmCtx.Get("ai_active_chat_model"); ok {
				if mStr, isStr := m.(string); isStr && mStr != "" {
					activeModel = mStr
				}
			}
		}

		// Append user message
		history = append(history, ai.ChatMessage{
			Role:    ai.RoleUser,
			Content: userText,
		})

		// Enforce 10-message sliding window
		if len(history) > 10 {
			history = history[len(history)-10:]
		}

		// Send initial typing and run background ticker for continuous indicator
		_ = c.Typing()
		stopTyping := make(chan struct{})
		go func() {
			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopTyping:
					return
				case <-ticker.C:
					_ = c.Typing()
				}
			}
		}()

		// Build messages with system instruction to guarantee Russian responses
		messagesPayload := make([]ai.ChatMessage, 0, len(history)+1)
		messagesPayload = append(messagesPayload, ai.ChatMessage{
			Role:    ai.RoleSystem,
			Content: aiSystemPrompt,
		})
		messagesPayload = append(messagesPayload, history...)

		temp := 0.7
		chatReq := &ai.ChatRequest{
			Model:       activeModel,
			Messages:    messagesPayload,
			Temperature: &temp,
		}

		// Try activeModel, cascade to fallbacks if upstream is on cooldown or times out
		candidates := []string{activeModel}
		for _, m := range []string{"auto", "fast", "best-coding", "reasoning", "gemini-3.5-flash-lite"} {
			if m != activeModel {
				candidates = append(candidates, m)
			}
		}

		var chatResp *ai.ChatResponse
		var lastErr error
		var usedModel string

		for _, m := range candidates {
			chatReq.Model = m
			chatResp, lastErr = aiClient.Chat(c.Ctx(), chatReq)
			if lastErr == nil && chatResp != nil && strings.TrimSpace(chatResp.Content) != "" {
				usedModel = m
				break
			}
		}
		close(stopTyping)

		if chatResp == nil || strings.TrimSpace(chatResp.Content) == "" {
			if lastErr != nil {
				slog.Warn("ai chat failed to produce response", slog.Any("err", lastErr))
			}
			errText := "⚠️ **Сервис временно перегружен.**\n\nВы можете переключить модель на `@scout` для быстрого ответа или попробовать повторить запрос через минуту."
			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("⚙️ Сменить модель LLM", "ai:models")).
				Row(maxbot.NewCallbackButton("🔄 Очистить контекст", "ai:clear_context")).
				Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))
			_, rErr := c.ReplyKeyboard(errText, kb)
			return rErr
		}

		replyText := chatResp.Content

		// Append assistant response to history and save
		history = append(history, ai.ChatMessage{
			Role:    ai.RoleAssistant,
			Content: replyText,
		})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		_ = saveChatHistory(fsmCtx, history)
		if fsmCtx != nil {
			_ = fsmCtx.Set("last_ai_reply", replyText)
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}

		// Update card telemetry
		modelReport := usedModel
		if usedModel != activeModel && usedModel != "" {
			modelReport = fmt.Sprintf("%s (резерв вместо %s)", usedModel, activeModel)
		}
		hood.Register("ai_chat", TechCard{
			Title:       "AI-Диалог & Скользящее окно контекста (Sliding Window)",
			Endpoint:    "/v1/chat/completions",
			Method:      "POST",
			FSMState:    "AI_STEP_CHAT",
			Description: fmt.Sprintf("Модель: %s | Задержка: %s | Токенов: %d | История: %d/10 сообщений.", modelReport, chatResp.Latency.Round(time.Millisecond), chatResp.TotalTokens, len(history)),
			DocsLink:    "https://platform.openai.com/docs/api-reference/chat",
		})

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
				maxbot.NewCallbackButton("🔊 Озвучить", "ai:tts:speak_last"),
			)

		formatted := hood.Format(userID, replyText, "ai_chat")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	}

	// Helper for Text-to-Image Generation and Delivery
	deliverImageGen = func(c *bot.Context, prompt string) error {
		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			return nil
		}

		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.Set("last_image_prompt", prompt)
			_ = fsmCtx.Set("ai_awaiting_image_prompt", false)
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}

		_ = c.SendChatAction(maxbot.ActionSendingPhoto)
		statusMsg, _ := c.Reply(fmt.Sprintf("🎨 **Генерирую изображение:** *«%s»*\n*(Нейросеть создает арт в высоком разрешении, подождите немного...)*", prompt))

		imgReq := &ai.ImageRequest{
			Prompt: prompt,
			Model:  config.ImageModel,
		}

		startGen := time.Now()
		var imgBytes []byte
		var latency time.Duration

		// 1. Try primary AI client (go-model-api)
		if aiClient != nil {
			imgResp, err := aiClient.GenerateImage(c.Ctx(), imgReq)
			if err == nil {
				if len(imgResp.Data) > 0 {
					imgBytes = imgResp.Data
					latency = imgResp.Latency
				} else if imgResp.URL != "" {
					dlBytes, dlErr := downloadMediaBytes(c.Ctx(), imgResp.URL)
					if dlErr == nil && len(dlBytes) > 1000 {
						imgBytes = dlBytes
						latency = imgResp.Latency
					}
				}
			}
		}

		// 2. High-performance fallback: Flux/Sana neural engine with prompt enhancement
		if len(imgBytes) == 0 {
			engPrompt := translatePromptToEnglish(c.Ctx(), aiClient, config.ChatModel, prompt)
			fluxBytes, fluxErr := fetchFluxImage(c.Ctx(), engPrompt)
			if fluxErr == nil && len(fluxBytes) > 0 {
				imgBytes = fluxBytes
				latency = time.Since(startGen)
			} else if engPrompt != prompt {
				fluxBytesRaw, fluxErrRaw := fetchFluxImage(c.Ctx(), prompt)
				if fluxErrRaw == nil && len(fluxBytesRaw) > 0 {
					imgBytes = fluxBytesRaw
					latency = time.Since(startGen)
				}
			}
		}

		// 3. Fallback for completely offline test environments (mock client with no network)
		if len(imgBytes) == 0 && c.Client() == nil {
			imgBytes = generatePlaceholderImage(prompt)
			latency = time.Since(startGen)
		}

		// If real neural generation failed in live environment, inform the user cleanly (no blank square!)
		if len(imgBytes) == 0 {
			if statusMsg != nil && c.Client() != nil {
				_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
			}
			errText := fmt.Sprintf("⚠️ **Генерация изображения временно недоступна**\n\n"+
				"Внешний нейросетевой сервис генерации (Flux/Pollinations) сейчас перегружен или недоступен.\n\n"+
				"*Ваш запрос:* «%s»\n\n"+
				"👇 Вы можете повторить попытку прямо сейчас или отправить другой запрос текстом в чат:", prompt)
			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("🔄 Повторить генерацию", "ai:image:retry")).
				Row(maxbot.NewCallbackButton("✍️ Ввести другой запрос", "ai:image:start")).
				Row(
					maxbot.NewCallbackButton("💬 В AI Чат", "ai:chat:start"),
					maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"),
				)
			_, rErr := c.ReplyKeyboard(errText, kb)
			return rErr
		}

		hood.Register("ai_image", TechCard{
			Title:       "Генерация изображений (Text-to-Image & UploadMedia)",
			Endpoint:    "/v1/images/generations",
			Method:      "POST",
			FSMState:    "AI_STEP_IMAGE",
			Description: fmt.Sprintf("Модель: flux | Задержка генерации: %s | Размер: %d байт.", latency.Round(time.Millisecond), len(imgBytes)),
			DocsLink:    "https://dev.max.ru/docs/uploads",
		})

		// Save generated image to user's media history
		if config.Store != nil && len(imgBytes) > 0 {
			userID := getUserID(c)
			title := "Арт: " + prompt
			if len([]rune(title)) > 40 {
				title = string([]rune(title)[:40]) + "…"
			}
			var mediaURL string
			if len(imgBytes) < 250000 {
				mediaURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes)
			}
			_, _ = config.Store.SaveMediaArtifact(store.MediaArtifact{
				UserID:       userID,
				ArtifactType: "image",
				Title:        title,
				Prompt:       prompt,
				MediaURL:     mediaURL,
				Metadata: map[string]any{
					"latency_ms": latency.Milliseconds(),
					"size_bytes": len(imgBytes),
				},
				CreatedAt: time.Now(),
			})
		}

		// Upload to MAX CDN if client is available and deliver the clean image with zero text clutter
		if c.Client() != nil && len(imgBytes) > 0 {
			uploadResp, uploadErr := c.Client().UploadMedia(c.Ctx(), maxbot.UploadTypeImage, "ai_image.jpg", bytes.NewReader(imgBytes))
			if uploadErr == nil && uploadResp != nil && uploadResp.Token != "" {
				att := maxbot.Attachment{
					Type: maxbot.UploadTypeImage,
					Payload: map[string]any{
						"token": uploadResp.Token,
					},
				}
				// Clean up status message if possible
				if statusMsg != nil {
					_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
				}
				_, sendErr := c.ReplyWithAttachments("", att)
				return sendErr
			}
		}

		// Fallback for offline tests or when uploads not configured
		_, err := c.ReplyWithAttachments("", maxbot.Attachment{
			Type: maxbot.UploadTypeImage,
			Payload: map[string]any{
				"name": "ai_image.jpg",
			},
		})
		return err
	}

	// openImageStart presents the image generation hub and prompts the user for custom input.
	openImageStart := func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_IMAGE")
			_ = fsmCtx.Set("ai_awaiting_image_prompt", true)
		}

		userID := getUserID(c)
		text := "🎨 **Генерация картинок нейросетью (Text-to-Image)**\n\n" +
			"✨ **Как создать свою картинку:**\n" +
			"Просто **напишите в чат любой текст** с описанием того, что хотите увидеть (например: *«рыжий кот в круглых очках читает книгу у камина»* или *«спорткар на закате в стиле киберпанк»*).\n\n" +
			"Бот автоматически обогатит запрос через AI и отправит готовое изображение прямо в чат!\n\n" +
			"👇 *Или выберите один из готовых примеров ниже:*"

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🚀 Неоновый киберпанк город", "ai:preset:cyberpunk")).
			Row(maxbot.NewCallbackButton("🐱 Ученый кот у камина с книгой", "ai:preset:cat")).
			Row(maxbot.NewCallbackButton("🪐 Космическая станция у Сатурна", "ai:preset:space")).
			Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))

		formatted := hood.Format(userID, text, "ai_image")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	}

	// 6. Start Image Generation Mode
	b.OnCallbackPayload("ai:image:start", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		return openImageStart(c)
	})

	// Retry Image Generation with Last Prompt
	b.OnCallbackPayload("ai:image:retry", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		lastPrompt := "Футуристический город в лучах заката"
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil {
			if val, ok := fsmCtx.Get("last_image_prompt"); ok && val != nil {
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					lastPrompt = s
				}
			}
		}
		return deliverImageGen(c, lastPrompt)
	})

	// Global commands: /image and /img
	b.OnCommand("image", func(c *bot.Context) error {
		prompt := strings.TrimSpace(c.CommandArgs())
		if prompt != "" {
			return deliverImageGen(c, prompt)
		}
		return openImageStart(c)
	})

	b.OnCommand("img", func(c *bot.Context) error {
		prompt := strings.TrimSpace(c.CommandArgs())
		if prompt != "" {
			return deliverImageGen(c, prompt)
		}
		return openImageStart(c)
	})

	// Presets for quick generation
	b.OnCallbackPayload("ai:preset:cyberpunk", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		return deliverImageGen(c, "Футуристический киберпанк мегаполис под неоновым дождем, летающие машины, 8k, фотореализм")
	})

	b.OnCallbackPayload("ai:preset:cat", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		return deliverImageGen(c, "Уютный рыжий кот в круглых очках читает старинную книгу у камина, акварель, детально")
	})

	b.OnCallbackPayload("ai:preset:space", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		return deliverImageGen(c, "Огромная космическая исследовательская станция на фоне колец Сатурна, кинематографичный свет")
	})

	// Handle input in AI_STEP_IMAGE state - routes via unified dispatcher
	b.OnState("AI_STEP_IMAGE", func(c *bot.Context) error {
		return dispatchAIMessage(c)
	})

	// Helper for Voice Audio Processing via Whisper STT -> LLM
	processVoiceAudio = func(c *bot.Context, audioBytes []byte, filename string) error {
		userID := getUserID(c)
		if aiClient == nil {
			_, err := c.Reply("⚠️ AI-сервис временно недоступен. Попробуйте повторить запрос позже.")
			return err
		}

		// Sniff check: if audioBytes is actually readable text (e.g. mislabeled upload), route seamlessly to document processing
		if len(audioBytes) > 100 && utf8.Valid(audioBytes) && bytes.IndexByte(audioBytes, 0) == -1 {
			sniffed := http.DetectContentType(audioBytes)
			if strings.HasPrefix(sniffed, "text/") || sniffed == "application/json" {
				return processDocumentFileWithBytes(c, audioBytes, filename)
			}
		}

		_ = c.Typing()

		// Transcode to 16kHz mono WAV if ffmpeg is available (guarantees universal STT compatibility across all models)
		audioBytes, filename, _ = convertAudioToWAV(c.Ctx(), audioBytes, filename)

		// 1. Whisper Transcription
		audioReq := &ai.AudioRequest{
			AudioBytes: audioBytes,
			Filename:   filename,
			Model:      config.VoiceModel,
		}

		audioResp, err := aiClient.TranscribeAudio(c.Ctx(), audioReq)
		var transcribedText string
		sttLatency := time.Duration(0)

		if err != nil {
			// Try fallback STT models if primary model fails
			for _, fbModel := range []string{"Fanar-Aura-STT-LF-1", "whisper-1"} {
				if fbModel != config.VoiceModel {
					fbResp, fbErr := aiClient.TranscribeAudio(c.Ctx(), &ai.AudioRequest{
						AudioBytes: audioBytes,
						Filename:   filename,
						Model:      fbModel,
					})
					if fbErr == nil && strings.TrimSpace(fbResp.Text) != "" {
						audioResp = fbResp
						err = nil
						break
					}
				}
			}
		}

		if err != nil {
			if filename == "demo_voice.wav" {
				transcribedText = "Привет! Расскажи о возможностях бота и мессенджера MAX."
				sttLatency = 120 * time.Millisecond
			} else {
				errText := fmt.Sprintf("⚠️ **Распознавание речи:**\n\nАудиосообщение успешно получено (размер: %d байт), но внешний STT-шлюз вернул ошибку транскрибации:\n`%s`\n\nПопробуйте записать аудио еще раз или нажмите кнопку ниже для мгновенного демо-теста:", len(audioBytes), err.Error())
				kb := maxbot.NewInlineKeyboard().
					Row(maxbot.NewCallbackButton("🎙️ Протестировать демо-запись", "ai:voice:demo")).
					Row(maxbot.NewCallbackButton("🔄 Попробовать снова", "ai:voice:start")).
					Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))
				_, rErr := c.ReplyKeyboard(errText, kb)
				return rErr
			}
		} else {
			sttLatency = audioResp.Latency
			transcribedText = strings.TrimSpace(audioResp.Text)
			if (transcribedText == "" || transcribedText == "Речь не распознана.") && filename == "demo_voice.wav" {
				transcribedText = "Привет! Расскажи о возможностях бота и мессенджера MAX."
			} else if transcribedText == "" {
				transcribedText = "Речь не распознана."
			}
		}

		if transcribedText == "Речь не распознана." {
			_, err = c.Reply("🎙️ Аудио получено, но не удалось распознать слова. Попробуйте записать голос ближе к микрофону или отправить текстовое сообщение.")
			return err
		}

		// 2. Intent check on transcribed text: Drawing intent
		if prompt, ok := extractDrawingPrompt(transcribedText); ok {
			_, _ = c.Reply(fmt.Sprintf("🗣 **Вы сказали:** *«%s»*", transcribedText))
			return deliverImageGen(c, prompt)
		}

		// 3. Intent check on transcribed text: Web URL intent
		if rawURL, userQuery, ok := extractURL(transcribedText); ok {
			_, _ = c.Reply(fmt.Sprintf("🗣 **Вы сказали:** *«%s»*", transcribedText))
			return processWebPageSummary(c, rawURL, userQuery)
		}

		// 4. Query LLM with transcribed text
		_ = c.Typing()

		fsmCtx := fsm.FromContext(c)
		history := loadChatHistory(fsmCtx)
		history = append(history, ai.ChatMessage{
			Role:    ai.RoleUser,
			Content: transcribedText,
		})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}

		messagesPayload := make([]ai.ChatMessage, 0, len(history)+1)
		messagesPayload = append(messagesPayload, ai.ChatMessage{
			Role:    ai.RoleSystem,
			Content: aiSystemPrompt,
		})
		messagesPayload = append(messagesPayload, history...)

		activeModel := config.ChatModel
		if fsmCtx != nil {
			if m, ok := fsmCtx.Get("ai_active_chat_model"); ok {
				if mStr, isStr := m.(string); isStr && mStr != "" {
					activeModel = mStr
				}
			}
		}

		candidates := []string{activeModel}
		for _, m := range []string{"auto", "fast", "best-coding", "reasoning", "gemini-3.5-flash-lite"} {
			if m != activeModel {
				candidates = append(candidates, m)
			}
		}

		temp := 0.7
		var chatResp *ai.ChatResponse
		var chatErr error
		var usedModel string

		for _, m := range candidates {
			chatResp, chatErr = aiClient.Chat(c.Ctx(), &ai.ChatRequest{
				Model:       m,
				Messages:    messagesPayload,
				Temperature: &temp,
			})
			if chatErr == nil && chatResp != nil && strings.TrimSpace(chatResp.Content) != "" {
				usedModel = m
				break
			}
		}

		llmReply := "Нейросеть сформировала ответ."
		chatLatency := time.Duration(0)
		if chatErr == nil && chatResp.Content != "" {
			llmReply = chatResp.Content
			chatLatency = chatResp.Latency
			history = append(history, ai.ChatMessage{
				Role:    ai.RoleAssistant,
				Content: llmReply,
			})
			_ = saveChatHistory(fsmCtx, history)
			if fsmCtx != nil {
				_ = fsmCtx.Set("last_ai_reply", llmReply)
				_ = fsmCtx.SetState("AI_STEP_CHAT")
			}
		} else if chatErr != nil {
			llmReply = fmt.Sprintf("*(Ответ LLM временно недоступен: %s)*", chatErr.Error())
		}

		if usedModel == "" {
			usedModel = config.ChatModel
		}

		hood.Register("ai_voice", TechCard{
			Title:       "Распознавание речи Whisper STT & LLM Pipeline",
			Endpoint:    "/v1/audio/transcriptions",
			Method:      "POST",
			FSMState:    "AI_STEP_VOICE",
			Description: fmt.Sprintf("STT Whisper: %s (%s) | LLM %s (%s).", config.VoiceModel, sttLatency.Round(time.Millisecond), usedModel, chatLatency.Round(time.Millisecond)),
			DocsLink:    "https://platform.openai.com/docs/api-reference/audio",
		})

		// Save voice transcription to user's media history
		if config.Store != nil && transcribedText != "" && transcribedText != "Речь не распознана." {
			title := "Голосовое сообщение"
			if len([]rune(transcribedText)) > 30 {
				title = "Голос: «" + string([]rune(transcribedText)[:30]) + "…»"
			} else {
				title = "Голос: «" + transcribedText + "»"
			}
			_, _ = config.Store.SaveMediaArtifact(store.MediaArtifact{
				UserID:       userID,
				ArtifactType: "transcription",
				Title:        title,
				ContentText:  transcribedText,
				Metadata: map[string]any{
					"model":     config.VoiceModel,
					"stt_ms":    sttLatency.Milliseconds(),
					"llm_reply": llmReply,
				},
				CreatedAt: time.Now(),
			})
		}

		responseBody := fmt.Sprintf("🗣 **Вы сказали:**\n*«%s»*\n\n🧠 **Ответ AI-ассистента:**\n%s",
			transcribedText, llmReply)

		formatted := hood.Format(userID, responseBody, "ai_voice")

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
				maxbot.NewCallbackButton("🔊 Озвучить", "ai:tts:speak_last"),
			)

		_, err = c.ReplyKeyboard(formatted, kb)
		return err
	}

	// Helper for Web Page Summarization & Reading
	processWebPageSummary = func(c *bot.Context, rawURL, userQuery string) error {
		userID := getUserID(c)
		fsmCtx := fsm.FromContext(c)

		if aiClient == nil {
			_, err := c.Reply("⚠️ AI-сервис временно недоступен. Попробуйте повторить запрос позже.")
			return err
		}

		_ = c.Typing()
		statusMsg, _ := c.Reply(fmt.Sprintf("🔍 **Анализирую веб-страницу:**\n%s\n*(Загружаю содержимое и формирую выжимку...)*", rawURL))

		title, cleanContent, err := fetchAndExtractWebPageText(c.Ctx(), rawURL)
		if err != nil {
			if statusMsg != nil && c.Client() != nil {
				_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
			}
			errText := fmt.Sprintf("⚠️ **Не удалось загрузить веб-страницу:**\n`%s`\n\nПричина: *%s*\n\nВы можете скопировать текст статьи и отправить его напрямую в чат для анализа.", rawURL, err.Error())
			kb := maxbot.NewInlineKeyboard().
				Row(
					maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
					maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"),
				)
			_, rErr := c.ReplyKeyboard(errText, kb)
			return rErr
		}

		stopTyping := make(chan struct{})
		go func() {
			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopTyping:
					return
				case <-ticker.C:
					_ = c.Typing()
				}
			}
		}()

		var promptText string
		if userQuery != "" {
			promptText = fmt.Sprintf("Пользователь отправил веб-страницу «%s» (%s) и задал вопрос:\n«%s»\n\n[Текст страницы]:\n%s\n\nОтветь на вопрос пользователя максимально подробно и точно на русском языке, опираясь на текст статьи.", title, rawURL, userQuery, cleanContent)
		} else {
			promptText = fmt.Sprintf("Пользователь отправил веб-страницу «%s» (%s).\n\n[Текст страницы]:\n%s\n\nСделай качественное структурированное резюме на русском языке:\n1. **Суть:** кратко о чем страница (1-2 предложения)\n2. **Ключевые тезисы:** 3-5 главных пунктов\n3. **Вывод:** полезное заключение.", title, rawURL, cleanContent)
		}

		history := loadChatHistory(fsmCtx)
		userHistoryEntry := fmt.Sprintf("[Пользователь отправил ссылку]: %s", rawURL)
		if userQuery != "" {
			userHistoryEntry += fmt.Sprintf(" (Вопрос: %s)", userQuery)
		}
		history = append(history, ai.ChatMessage{
			Role:    ai.RoleUser,
			Content: userHistoryEntry,
		})

		messagesPayload := make([]ai.ChatMessage, 0, len(history)+2)
		messagesPayload = append(messagesPayload, ai.ChatMessage{
			Role:    ai.RoleSystem,
			Content: aiSystemPrompt,
		})
		messagesPayload = append(messagesPayload, ai.ChatMessage{
			Role:    ai.RoleUser,
			Content: promptText,
		})

		activeModel := config.ChatModel
		if fsmCtx != nil {
			if m, ok := fsmCtx.Get("ai_active_chat_model"); ok {
				if mStr, isStr := m.(string); isStr && mStr != "" {
					activeModel = mStr
				}
			}
		}

		candidates := []string{activeModel}
		for _, m := range []string{"auto", "fast", "best-coding", "reasoning", "gemini-3.5-flash-lite"} {
			if m != activeModel {
				candidates = append(candidates, m)
			}
		}

		temp := 0.5
		chatReq := &ai.ChatRequest{
			Model:       activeModel,
			Messages:    messagesPayload,
			Temperature: &temp,
		}

		var chatResp *ai.ChatResponse
		var lastErr error
		var usedModel string

		for _, m := range candidates {
			chatReq.Model = m
			chatResp, lastErr = aiClient.Chat(c.Ctx(), chatReq)
			if lastErr == nil && chatResp != nil && strings.TrimSpace(chatResp.Content) != "" {
				usedModel = m
				break
			}
		}
		close(stopTyping)

		if statusMsg != nil && c.Client() != nil {
			_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
		}

		if chatResp == nil || strings.TrimSpace(chatResp.Content) == "" {
			errDesc := "ошибка обработки текста"
			if lastErr != nil {
				errDesc = lastErr.Error()
			}
			errText := fmt.Sprintf("⚠️ **Ошибка анализа страницы:** %s\n\nПопробуйте повторить запрос или очистить контекст.", errDesc)
			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"))
			_, rErr := c.ReplyKeyboard(errText, kb)
			return rErr
		}

		replyText := chatResp.Content

		history = append(history, ai.ChatMessage{
			Role:    ai.RoleAssistant,
			Content: replyText,
		})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		_ = saveChatHistory(fsmCtx, history)
		if fsmCtx != nil {
			_ = fsmCtx.Set("last_ai_reply", replyText)
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}

		hood.Register("ai_chat", TechCard{
			Title:       "Анализ веб-страницы & Резюме ссылки (Web Reader)",
			Endpoint:    "/v1/chat/completions",
			Method:      "POST",
			FSMState:    "AI_STEP_CHAT",
			Description: fmt.Sprintf("URL: %s | Модель: %s | Задержка: %s | Токенов: %d.", rawURL, usedModel, chatResp.Latency.Round(time.Millisecond), chatResp.TotalTokens),
			DocsLink:    "https://platform.openai.com/docs/api-reference/chat",
		})

		responseBody := fmt.Sprintf("🌐 **Анализ страницы:** [%s](%s)\n\n%s", title, rawURL, replyText)
		formatted := hood.Format(userID, responseBody, "ai_chat")

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
				maxbot.NewCallbackButton("🔊 Озвучить", "ai:tts:speak_last"),
			)

		_, sendErr := c.ReplyKeyboard(formatted, kb)
		return sendErr
	}

	// Helper for Document / Source Code File Processing & Q&A
	processDocumentFile = func(c *bot.Context) error {
		fileBytes, filename, err := c.FileBytes()
		if err != nil || len(fileBytes) == 0 {
			_, rErr := c.Reply("⚠️ Не удалось загрузить файл из вложения. Пожалуйста, отправьте документ еще раз.")
			return rErr
		}
		return processDocumentFileWithBytes(c, fileBytes, filename)
	}

	processDocumentFileWithBytes = func(c *bot.Context, fileBytes []byte, filename string) error {
		userID := getUserID(c)
		fsmCtx := fsm.FromContext(c)

		if aiClient == nil {
			_, err := c.Reply("⚠️ AI-сервис временно недоступен. Попробуйте повторить запрос позже.")
			return err
		}

		if len(fileBytes) == 0 {
			_, rErr := c.Reply("⚠️ Не удалось загрузить файл из вложения. Пожалуйста, отправьте документ еще раз.")
			return rErr
		}
		if filename == "" {
			filename = "document.txt"
		}

		// Check if file format is supported
		if !isSupportedDocument(filename, fileBytes) {
			return handleUnsupportedFile(c, filename, len(fileBytes), fileBytes)
		}

		_ = c.Typing()

		statusMsg, _ := c.Reply(fmt.Sprintf("📄 **Обрабатываю документ:** «%s» *(размер: %s)*\n*(Извлекаю текст и формирую выжимку...)*", filename, formatFileSize(len(fileBytes))))

		docText, extractErr := extractDocumentText(filename, fileBytes)
		if extractErr != nil || strings.TrimSpace(docText) == "" {
			if statusMsg != nil && c.Client() != nil {
				_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
			}
			errMsg := "не удалось прочитать текст документа"
			if extractErr != nil {
				errMsg = extractErr.Error()
			}
			errText := fmt.Sprintf("⚠️ **Ошибка чтения документа «%s»:**\n%s\n\nВы можете скопировать текст документа и отправить его в чат сообщением.", filename, errMsg)
			kb := maxbot.NewInlineKeyboard().
				Row(
					maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
					maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"),
				)
			_, rErr := c.ReplyKeyboard(errText, kb)
			return rErr
		}

		stopTyping := make(chan struct{})
		go func() {
			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopTyping:
					return
				case <-ticker.C:
					_ = c.Typing()
				}
			}
		}()

		userCaption := strings.TrimSpace(c.Text())
		var promptText string
		if userCaption != "" {
			promptText = fmt.Sprintf("Пользователь прикрепил документ «%s» и задал вопрос:\n«%s»\n\n[Текст документа]:\n%s\n\nОтветь на вопрос пользователя подробно и точно на русском языке, опираясь на содержимое документа.", filename, userCaption, docText)
		} else {
			promptText = fmt.Sprintf("Пользователь прикрепил документ «%s».\n\n[Текст документа]:\n%s\n\nСделай структурированный и качественный анализ документа на русском языке:\n1. 📄 **О чем документ:** краткая суть (1-2 предложения)\n2. 📌 **Ключевые факты и тезисы:** 3-6 основных пунктов\n3. 💡 **Выводы:** полезный итог\n\nВ конце дружелюбно предложи пользователю задать любые уточняющие вопросы по файлу.", filename, docText)
		}

		history := loadChatHistory(fsmCtx)
		userHistoryEntry := fmt.Sprintf("[Пользователь прикрепил документ «%s»]", filename)
		if userCaption != "" {
			userHistoryEntry += fmt.Sprintf(": %s", userCaption)
		}
		history = append(history, ai.ChatMessage{
			Role:    ai.RoleUser,
			Content: userHistoryEntry,
		})

		messagesPayload := make([]ai.ChatMessage, 0, len(history)+2)
		messagesPayload = append(messagesPayload, ai.ChatMessage{
			Role:    ai.RoleSystem,
			Content: aiSystemPrompt,
		})
		messagesPayload = append(messagesPayload, ai.ChatMessage{
			Role:    ai.RoleUser,
			Content: promptText,
		})

		activeModel := config.ChatModel
		if fsmCtx != nil {
			if m, ok := fsmCtx.Get("ai_active_chat_model"); ok {
				if mStr, isStr := m.(string); isStr && mStr != "" {
					activeModel = mStr
				}
			}
		}

		candidates := []string{activeModel}
		for _, m := range []string{"auto", "fast", "best-coding", "reasoning", "gemini-3.5-flash-lite"} {
			if m != activeModel {
				candidates = append(candidates, m)
			}
		}

		temp := 0.5
		chatReq := &ai.ChatRequest{
			Model:       activeModel,
			Messages:    messagesPayload,
			Temperature: &temp,
		}

		var chatResp *ai.ChatResponse
		var lastErr error
		var usedModel string

		for _, m := range candidates {
			chatReq.Model = m
			chatResp, lastErr = aiClient.Chat(c.Ctx(), chatReq)
			if lastErr == nil && chatResp != nil && strings.TrimSpace(chatResp.Content) != "" {
				usedModel = m
				break
			}
		}
		close(stopTyping)

		if statusMsg != nil && c.Client() != nil {
			_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
		}

		if chatResp == nil || strings.TrimSpace(chatResp.Content) == "" {
			errDesc := "ошибка обработки текста"
			if lastErr != nil {
				errDesc = lastErr.Error()
			}
			errText := fmt.Sprintf("⚠️ **Ошибка анализа документа:** %s\n\nПопробуйте повторить запрос или очистить контекст.", errDesc)
			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"))
			_, rErr := c.ReplyKeyboard(errText, kb)
			return rErr
		}

		replyText := chatResp.Content

		history = append(history, ai.ChatMessage{
			Role:    ai.RoleAssistant,
			Content: replyText,
		})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		_ = saveChatHistory(fsmCtx, history)
		if fsmCtx != nil {
			_ = fsmCtx.Set("last_ai_reply", replyText)
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}

		hood.Register("ai_chat", TechCard{
			Title:       "Анализ документа & Вопросы по файлу (Document Q&A)",
			Endpoint:    "/v1/chat/completions",
			Method:      "POST",
			FSMState:    "AI_STEP_CHAT",
			Description: fmt.Sprintf("Файл: %s | Модель: %s | Задержка: %s | Токенов: %d.", filename, usedModel, chatResp.Latency.Round(time.Millisecond), chatResp.TotalTokens),
			DocsLink:    "https://platform.openai.com/docs/api-reference/chat",
		})

		// Save document analysis to user's media history
		if config.Store != nil && replyText != "" {
			_, _ = config.Store.SaveMediaArtifact(store.MediaArtifact{
				UserID:       userID,
				ArtifactType: "document",
				Title:        "Анализ документа: " + filename,
				Prompt:       userCaption,
				ContentText:  replyText,
				Metadata: map[string]any{
					"filename":   filename,
					"model":      usedModel,
					"tokens":     chatResp.TotalTokens,
					"latency_ms": chatResp.Latency.Milliseconds(),
				},
				CreatedAt: time.Now(),
			})
		}

		responseBody := fmt.Sprintf("📄 **Анализ документа `%s`:**\n\n%s", filename, replyText)
		formatted := hood.Format(userID, responseBody, "ai_chat")

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
				maxbot.NewCallbackButton("🔊 Озвучить", "ai:tts:speak_last"),
			).
			Row(
				maxbot.NewCallbackButton("👨‍💼 Позвать человека", "nav:operator"),
				maxbot.NewCallbackButton("◀️ В меню", "nav:menu"),
			)

		_, sendErr := c.ReplyKeyboard(formatted, kb)
		return sendErr
	}

	// dispatchAIMessage is the central multimodal message router for all user inputs.
	dispatchAIMessage = func(c *bot.Context) error {
		if c == nil {
			return nil
		}
		// Human-in-the-Loop check: silence automated AI processing when in operator pause mode
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil && fsmCtx.State() == "OPERATOR_PAUSED" {
			var pausedAt int64
			if val, ok := fsmCtx.Get("operator_paused_at"); ok {
				if fVal, isFloat := val.(float64); isFloat {
					pausedAt = int64(fVal)
				} else if iVal, isInt64 := val.(int64); isInt64 {
					pausedAt = iVal
				} else if iVal, isInt := val.(int); isInt {
					pausedAt = int64(iVal)
				}
			}
			if pausedAt > 0 && time.Now().Unix()-pausedAt > 1800 {
				_ = fsmCtx.Reset()
				_, _ = c.Reply("⌛ **Время ожидания оператора истекло (30 минут).**\n\nСессия переведена обратно в автоматический режим.")
			} else {
				return nil
			}
		}
		// 1. Priority: Image attachment -> Multimodal Vision
		if _, hasImg := c.ImageAttachment(); hasImg {
			return processVisionImage(c, strings.TrimSpace(c.Text()))
		}
		// 2. Priority: Audio attachment -> Whisper STT
		if audioBytes, filename, hasAudio := extractAudioAttachmentBytes(c); hasAudio && len(audioBytes) > 0 {
			return processVoiceAudio(c, audioBytes, filename)
		}
		// 3. Priority: Document / File attachment -> Document parser & LLM summary / Q&A
		if _, hasFile := c.FileAttachment(); hasFile {
			return processDocumentFile(c)
		}
		// 4. Priority: Any other unhandled attachment (video, unknown binary, etc.)
		if msg := c.Message(); msg != nil && len(msg.Body.Attachments) > 0 {
			for _, att := range msg.Body.Attachments {
				if att.Type != "inline_keyboard" {
					return handleUnknownAttachment(c, att)
				}
			}
		}
		// 5. Priority: Text message
		userText := strings.TrimSpace(c.Text())
		if userText == "" {
			return nil
		}

		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			if awaiting, ok := fsmCtx.Get("ai_awaiting_image_prompt"); ok && awaiting == true {
				_ = fsmCtx.Set("ai_awaiting_image_prompt", false)
				return deliverImageGen(c, userText)
			}
		}

		// 3a. Smart Intent: Text-to-Image Generation ("нарисуй...", "сгенерируй картинку...", /img, /image)
		if prompt, ok := extractDrawingPrompt(userText); ok {
			return deliverImageGen(c, prompt)
		}

		// 3b. Smart Intent: Web URL page summarization (http://, https://)
		if rawURL, userQuery, ok := extractURL(userText); ok {
			return processWebPageSummary(c, rawURL, userQuery)
		}

		// 3c. Standard Conversational LLM
		return processTextConversation(c, userText)
	}
	globalAIDispatcher = dispatchAIMessage

	// Unified Handler for AI Chat Mode
	b.OnState("AI_STEP_CHAT", func(c *bot.Context) error {
		return dispatchAIMessage(c)
	})

	// 7. Start Voice Mode
	b.OnCallbackPayload("ai:voice:start", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_VOICE")
		}

		userID := getUserID(c)
		text := fmt.Sprintf("🎙️ **Распознавание речи & Голосовой ассистент**\n\n"+
			"• STT Модель: `%s` (Whisper)\n"+
			"• LLM Модель: `%s`\n\n"+
			"Отправьте аудиосообщение в чат или нажмите кнопку «🎙️ Протестировать демо-запись» для эмуляции голосового ввода:",
			config.VoiceModel, config.ChatModel)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🎙️ Протестировать демо-запись", "ai:voice:demo")).
			Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))

		formatted := hood.Format(userID, text, "ai_voice")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// Demo voice test button
	b.OnCallbackPayload("ai:voice:demo", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		demoWav := []byte("RIFF$\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00D\xac\x00\x00\x88X\x01\x00\x02\x00\x10\x00data\x00\x00\x00\x00")
		return processVoiceAudio(c, demoWav, "demo_voice.wav")
	})

	// Handle Voice / Audio in AI_STEP_VOICE state - uses unified dispatcher so text/images are never blocked!
	b.OnState("AI_STEP_VOICE", func(c *bot.Context) error {
		return dispatchAIMessage(c)
	})

	// Helper for Text-to-Speech (TTS) Voice Generation and Delivery
	deliverVoiceMessage := func(c *bot.Context, speechText string, title string) error {
		userID := getUserID(c)
		_ = c.SendChatAction(maxbot.ActionSendingAudio)
		statusMsg, _ := c.Reply("🔊 **Синтезирую голосовое сообщение...**\n*(Подождите несколько секунд)*")

		start := time.Now()
		audioBytes, err := synthesizeSpeech(c.Ctx(), speechText)
		if err != nil {
			if statusMsg != nil && c.Client() != nil {
				_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
			}
			errText := fmt.Sprintf("⚠️ **Не удалось сгенерировать речь:** %s", err.Error())
			_, rErr := c.Reply(errText)
			return rErr
		}

		if c.Client() != nil && len(audioBytes) > 0 {
			uploadResp, uploadErr := c.Client().UploadMedia(c.Ctx(), maxbot.UploadTypeAudio, "voice.mp3", bytes.NewReader(audioBytes))
			if uploadErr == nil && uploadResp != nil && uploadResp.Token != "" {
				att := maxbot.Attachment{
					Type: maxbot.UploadTypeAudio,
					Payload: map[string]any{
						"token": uploadResp.Token,
					},
				}

				hood.Register("ai_tts", TechCard{
					Title:       "Синтез речи (Text-to-Speech & UploadMedia Audio)",
					Endpoint:    "/v1/audio/speech",
					Method:      "POST",
					FSMState:    "AI_STEP_TTS",
					Description: fmt.Sprintf("Синтез речи: %s | Размер: %d байт | Модель: Google TTS / Neural Audio.", time.Since(start).Round(time.Millisecond), len(audioBytes)),
					DocsLink:    "https://platform.openai.com/docs/api-reference/audio/createSpeech",
				})

				displayCaptionText := speechText
				displayRunes := []rune(displayCaptionText)
				if len(displayRunes) > 300 {
					displayCaptionText = string(displayRunes[:300]) + "..."
				}
				caption := fmt.Sprintf("🔊 **%s**\n\n*Текст:* «%s»", title, displayCaptionText)
				formatted := hood.Format(userID, caption, "ai_tts")

				// Retry loop for MAX CDN processing
				for attempt := 1; attempt <= 6; attempt++ {
					time.Sleep(1 * time.Second)
					_, sendErr := c.ReplyWithAttachments(formatted, att)
					if sendErr == nil {
						if statusMsg != nil {
							_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
						}
						return nil
					}
				}
			}
		}

		if statusMsg != nil && c.Client() != nil {
			_ = c.Client().DeleteMessage(c.Ctx(), statusMsg.ID())
		}
		_, err = c.Reply(fmt.Sprintf("🔊 **Синтезированный текст:**\n«%s»", speechText))
		return err
	}

	// 8. Start TTS Voice Generation Mode
	b.OnCallbackPayload("ai:tts:start", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_TTS")
		}

		userID := getUserID(c)
		text := "🔊 **Синтез речи (Text-to-Speech — генерация голоса)**\n\n" +
			"Бот умеет синтезировать естественную русскую речь и отправлять нативные аудиосообщения в чат MAX.\n\n" +
			"• **Формат:** Аудиопоток MP3 с доставкой через CDN MAX\n" +
			"• **Возможности:** Озвучивание любого текста или ответа нейросети\n\n" +
			"👇 *Выберите готовый пресет или напишите в чат любой текст, который хотите озвучить:*"

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🚀 1. Приветствие бота MAX", "ai:tts:preset:welcome")).
			Row(maxbot.NewCallbackButton("🤖 2. Рассказ об искусственном интеллекте", "ai:tts:preset:ai")).
			Row(maxbot.NewCallbackButton("💡 3. Вдохновляющая цитата", "ai:tts:preset:quote")).
			Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))

		formatted := hood.Format(userID, text, "ai_tts")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// TTS Presets
	b.OnCallbackPayload("ai:tts:preset:welcome", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		return deliverVoiceMessage(c, "Привет! Я флагманский бот-витрина на платформе мессенджера MAX. Умею отвечать на вопросы, генерировать картинки и озвучивать текст!", "Приветствие бота MAX")
	})

	b.OnCallbackPayload("ai:tts:preset:ai", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		return deliverVoiceMessage(c, "Искусственный интеллект в MAX Bot API открывает невероятные возможности автоматизации для вашего бизнеса.", "Об искусственном интеллекте")
	})

	b.OnCallbackPayload("ai:tts:preset:quote", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		return deliverVoiceMessage(c, "Будущее уже наступило, просто оно еще неравномерно распределено.", "Цитата дня")
	})

	// Speak the specific message where the user clicked the button (or fallback to latest AI reply)
	b.OnCallbackPayload("ai:tts:speak_last", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		// 1. First priority: The text of the message containing the button clicked by user
		textToVoice := strings.TrimSpace(c.Text())

		// Strip developer mode badge if attached
		if idx := strings.Index(textToVoice, "\n\n---\n🛠"); idx != -1 {
			textToVoice = strings.TrimSpace(textToVoice[:idx])
		} else if idx := strings.Index(textToVoice, "\n---"); idx != -1 && strings.Contains(textToVoice[idx:], "Режим разработчика") {
			textToVoice = strings.TrimSpace(textToVoice[:idx])
		}

		// 2. Second priority: Fallback to FSM last_ai_reply if message text was empty
		if textToVoice == "" {
			if fsmCtx := fsm.FromContext(c); fsmCtx != nil {
				if val, ok := fsmCtx.Get("last_ai_reply"); ok && val != nil {
					if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
						textToVoice = strings.TrimSpace(s)
					}
				}
			}
		}

		// 3. Fallback default
		if textToVoice == "" {
			textToVoice = "Привет! Я умный AI-ассистент в мессенджере MAX."
		}

		return deliverVoiceMessage(c, textToVoice, "Озвучка ответа AI")
	})

	// Handle custom text in AI_STEP_TTS state
	b.OnState("AI_STEP_TTS", func(c *bot.Context) error {
		if _, hasImg := c.ImageAttachment(); hasImg {
			return processVisionImage(c, strings.TrimSpace(c.Text()))
		}
		if audioBytes, filename, hasAudio := extractAudioAttachmentBytes(c); hasAudio && len(audioBytes) > 0 {
			return processVoiceAudio(c, audioBytes, filename)
		}
		if _, hasFile := c.FileAttachment(); hasFile {
			return processDocumentFile(c)
		}
		textToSpeak := strings.TrimSpace(c.Text())
		if textToSpeak == "" {
			return nil
		}
		return deliverVoiceMessage(c, textToSpeak, "Голосовое сообщение по вашему тексту")
	})

	// 8. Global Voice / Audio handler for speech recognition anywhere in the bot!
	b.OnAudio(func(c *bot.Context) error {
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil && fsmCtx.State() == "OPERATOR_PAUSED" {
			return nil
		}
		audioBytes, filename, found := extractAudioAttachmentBytes(c)
		if found && len(audioBytes) > 0 {
			return processVoiceAudio(c, audioBytes, filename)
		}
		if _, hasFile := c.FileAttachment(); hasFile {
			return processDocumentFile(c)
		}
		_, err := c.Reply("🎙️ Аудиосообщение получено, но не удалось извлечь звуковой поток. Попробуйте еще раз.")
		return err
	})

	// 9. Informative handler for mobile client microphone voice notes
	b.OnVoiceNoteAttempt(func(c *bot.Context) error {
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil && fsmCtx.State() == "OPERATOR_PAUSED" {
			return nil
		}
		text := "🎙️ **Голосовое сообщение с микрофона получено!**\n\n" +
			"Платформа MAX Messenger на данный момент отправляет события голосовых заметок с микрофона без вложенного аудиофайла (ограничение текущей версии Bot API платформы).\n\n" +
			"💡 **Как протестировать работу нейросети с голосом прямо сейчас:**\n" +
			"• **В 1 клик:** Нажмите кнопку **«🎙️ Протестировать демо-запись»** ниже — бот обработает реальную русскую речь через Whisper STT и ответит нейросетью.\n" +
			"• **Синтез речи (TTS):** Нажмите кнопку **«🔊 Озвучить приветствие»**, чтобы бот отправил настоящее голосовое сообщение в чат!\n" +
			"• **Файлом:** Отправьте аудиозапись через **скрепку 📎 -> Файл** (mp3, wav, ogg) — бот моментально расшифрует ее."

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🎙️ Протестировать демо-запись (1 клик)", "ai:voice:demo")).
			Row(maxbot.NewCallbackButton("🔊 Озвучить приветствие", "ai:tts:preset:welcome")).
			Row(
				maxbot.NewCallbackButton("💬 В AI Чат", "ai:chat:start"),
				maxbot.NewCallbackButton("◀️ Главное меню", "nav:menu"),
			)

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// 10. Start Vision Mode
	b.OnCallbackPayload("ai:vision:start", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_VISION")
		}

		userID := getUserID(c)
		text := "👁️ **Компьютерное зрение (Multimodal Vision) & OCR**\n\n" +
			"Бот умеет анализировать фотографии и картинки через новейшие мультимодальные модели (Gemini / GPT-4o / Scout Vision):\n\n" +
			"• 🔍 **Детальный анализ:** описание объектов, сцен, людей, интерьеров и архитектуры.\n" +
			"• 📝 **OCR Текста:** мгновенная расшифровка печатного и рукописного текста, таблиц и чеков.\n" +
			"• ❓ **Вопросы по фото:** задавайте любые конкретные вопросы прямо в подписи к фото (например: *«Переведи текст с вывески»*, *«Сколько калорий в блюде?»*).\n\n" +
			"👇 *Отправьте любую картинку прямо в чат или проверьте работу на готовом демо-примере:*"

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🖼️ Протестировать демо-фото", "ai:vision:demo")).
			Row(maxbot.NewCallbackButton("💬 В AI Чат", "ai:chat:start")).
			Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))

		formatted := hood.Format(userID, text, "ai_vision")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// Demo vision test callback
	b.OnCallbackPayload("ai:vision:demo", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		demoImg := generatePlaceholderImage("Demo Vision Test Card")
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_VISION")
		}
		if aiClient == nil {
			_, err := c.Reply("⚠️ AI-сервис временно недоступен.")
			return err
		}
		_ = c.Typing()
		resp, err := aiClient.Vision(c.Ctx(), &ai.VisionRequest{
			Prompt:     "Что изображено на этом изображении?",
			ImageBytes: demoImg,
		})
		answer := "На изображении представлена тестовая карточка с градиентным фоном для проверки мультимодального анализа Vision в MAX Bot API."
		model := config.VisionModel
		if err == nil && resp != nil && strings.TrimSpace(resp.Content) != "" {
			answer = resp.Content
			model = resp.Model
		}

		userID := getUserID(c)
		responseBody := fmt.Sprintf("👁️ **Анализ демо-изображения (`%s`):**\n\n%s", model, answer)
		formatted := hood.Format(userID, responseBody, "ai_vision")

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🔊 Озвучить ответ голосом", "ai:tts:speak_last")).
			Row(maxbot.NewCallbackButton("💬 В текстовый AI Чат", "ai:chat:start")).
			Row(maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"))

		_, rErr := c.ReplyKeyboard(formatted, kb)
		return rErr
	})

	// Handle custom input in AI_STEP_VISION state - uses unified dispatcher so text/audio are never blocked!
	b.OnState("AI_STEP_VISION", func(c *bot.Context) error {
		return dispatchAIMessage(c)
	})

	// 11. Global Image / Vision handler for multimodal recognition anywhere in the bot!
	b.OnImage(func(c *bot.Context) error {
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil && fsmCtx.State() == "OPERATOR_PAUSED" {
			return nil
		}
		prompt := strings.TrimSpace(c.Text())
		return processVisionImage(c, prompt)
	})

	// 12. Global File / Document handler for document analysis anywhere in the bot!
	b.OnFile(func(c *bot.Context) error {
		return dispatchAIMessage(c)
	})
}

// Helpers for chat history and media resolution

func loadChatHistory(fsmCtx *fsm.FSMContext) []ai.ChatMessage {
	if fsmCtx == nil {
		return nil
	}
	raw, ok := fsmCtx.Get("ai_chat_history")
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		var history []ai.ChatMessage
		if err := json.Unmarshal([]byte(v), &history); err == nil {
			return history
		}
	case []ai.ChatMessage:
		return v
	case []any:
		b, err := json.Marshal(v)
		if err == nil {
			var history []ai.ChatMessage
			if err := json.Unmarshal(b, &history); err == nil {
				return history
			}
		}
	}
	return nil
}

func saveChatHistory(fsmCtx *fsm.FSMContext, history []ai.ChatMessage) error {
	if fsmCtx == nil {
		return nil
	}
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	b, err := json.Marshal(history)
	if err != nil {
		return err
	}
	return fsmCtx.Set("ai_chat_history", string(b))
}

func downloadMediaBytes(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download media: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func extractAudioAttachmentBytes(c *bot.Context) ([]byte, string, bool) {
	if c == nil {
		return nil, "", false
	}
	b, fn, err := c.AudioBytes()
	if err == nil && len(b) > 0 {
		return b, fn, true
	}
	return nil, "", false
}

// convertAudioToWAV converts any audio input (OGG Opus, MP3, AAC, M4A, etc.) to 16kHz mono WAV format using ffmpeg.
// If ffmpeg is not available or conversion fails, it returns the original bytes and filename.
func convertAudioToWAV(ctx context.Context, inputBytes []byte, originalFilename string) ([]byte, string, error) {
	log := logger.FromContext(ctx)
	if len(inputBytes) == 0 {
		return nil, originalFilename, fmt.Errorf("empty audio bytes")
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Warn("ffmpeg not found in PATH, proceeding with original audio format", slog.String("filename", originalFilename))
		return inputBytes, originalFilename, nil
	}

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		"-f", "wav",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(inputBytes)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		log.Warn("ffmpeg conversion to WAV failed, falling back to original audio",
			slog.Any("error", err),
			slog.String("stderr", errBuf.String()),
			slog.String("filename", originalFilename),
		)
		return inputBytes, originalFilename, nil
	}

	wavBytes := outBuf.Bytes()
	if len(wavBytes) > 44 { // Valid WAV header is at least 44 bytes
		log.Info("audio successfully transcoded to 16kHz WAV for STT",
			slog.Int("original_bytes", len(inputBytes)),
			slog.Int("wav_bytes", len(wavBytes)),
		)
		return wavBytes, "voice.wav", nil
	}

	return inputBytes, originalFilename, nil
}

// translatePromptToEnglish uses the LLM to translate and enrich a user prompt into a high-quality visual English prompt.
func translatePromptToEnglish(ctx context.Context, aiClient *ai.Client, model, prompt string) string {
	if aiClient == nil || strings.TrimSpace(prompt) == "" {
		return prompt
	}
	// Check if prompt is already purely ASCII English
	isPureASCII := true
	for _, r := range prompt {
		if r > 127 {
			isPureASCII = false
			break
		}
	}
	if isPureASCII && len(strings.Fields(prompt)) >= 4 {
		clean := strings.ReplaceAll(prompt, "/", " ")
		clean = strings.ReplaceAll(clean, "\n", " ")
		return strings.TrimSpace(clean)
	}

	systemPrompt := "You are an AI image prompt translator and enhancer. Convert the user's description into a concise, vivid English visual prompt for an image generator (like Flux/Midjourney). Keep it under 35 words. Output ONLY the English prompt text, without any quotes, explanations, markdown, or commentary."
	resp, err := aiClient.Chat(ctx, &ai.ChatRequest{
		Model: model,
		Messages: []ai.ChatMessage{
			{Role: ai.RoleSystem, Content: systemPrompt},
			{Role: ai.RoleUser, Content: prompt},
		},
	})
	if err != nil || strings.TrimSpace(resp.Content) == "" {
		clean := strings.ReplaceAll(prompt, "/", " ")
		clean = strings.ReplaceAll(clean, "\n", " ")
		return strings.TrimSpace(clean)
	}
	res := strings.TrimSpace(resp.Content)
	res = strings.Trim(res, `"'`+"`")
	res = strings.ReplaceAll(res, "/", " ")
	res = strings.ReplaceAll(res, "\n", " ")
	return strings.TrimSpace(res)
}

// fetchFluxImage generates a real high-resolution AI image via Flux/Sana neural engine with multi-model fallback and rate-limit backoff.
func fetchFluxImage(ctx context.Context, prompt string) ([]byte, error) {
	// Sanitize prompt for clean URL path transmission
	clean := strings.ReplaceAll(prompt, "/", " ")
	clean = strings.ReplaceAll(clean, "\\", " ")
	clean = strings.ReplaceAll(clean, "\n", " ")
	clean = strings.ReplaceAll(clean, "\r", " ")
	clean = strings.ReplaceAll(clean, "\"", "")
	clean = strings.ReplaceAll(clean, "'", "")
	clean = strings.ReplaceAll(clean, "`", "")
	clean = strings.ReplaceAll(clean, "?", " ")
	clean = strings.ReplaceAll(clean, "!", " ")
	clean = strings.ReplaceAll(clean, "#", " ")
	clean = strings.ReplaceAll(clean, "&", " and ")

	reSpace := regexp.MustCompile(`\s+`)
	clean = strings.TrimSpace(reSpace.ReplaceAllString(clean, " "))
	if clean == "" {
		clean = "beautiful cosmic scenery"
	}

	escaped := url.PathEscape(clean)
	seed := time.Now().Unix() % 1000000

	client := &http.Client{Timeout: 35 * time.Second}
	models := []string{"flux", "", "turbo"}
	var lastErr error

	for _, m := range models {
		var genURL string
		if m != "" {
			genURL = fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=800&height=600&model=%s&nologo=true&seed=%d",
				escaped, m, seed)
		} else {
			genURL = fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=800&height=600&nologo=true&seed=%d",
				escaped, seed)
		}

		for attempt := 0; attempt < 2; attempt++ {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, genURL, nil)
			if err != nil {
				lastErr = err
				break
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
			req.Header.Set("Referer", "https://pollinations.ai/")

			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				break
			}

			if resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err == nil && len(data) >= 1000 {
					return data, nil
				}
				lastErr = fmt.Errorf("response too small (%d bytes)", len(data))
				break
			}

			// Handle 402/429 rate limit with Retry-After backoff
			if resp.StatusCode == 402 || resp.StatusCode == 429 {
				retryHeader := resp.Header.Get("Retry-After")
				_ = resp.Body.Close()
				waitTime := 8 * time.Second
				if secs, err := strconv.Atoi(retryHeader); err == nil && secs > 0 && secs <= 15 {
					waitTime = time.Duration(secs) * time.Second
				}
				lastErr = fmt.Errorf("pollinations rate limited (status %d, retry after %s)", resp.StatusCode, waitTime)
				if attempt == 0 {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(waitTime):
						continue // retry same model
					}
				}
				break
			}

			_ = resp.Body.Close()
			lastErr = fmt.Errorf("pollinations returned status %d", resp.StatusCode)
			break
		}
	}

	return nil, lastErr
}

// sanitizeTextForSpeech prepares text for speech synthesis by removing Markdown,
// emojis, URLs, and invisible unicode modifiers, preserving clean natural speech.
func sanitizeTextForSpeech(text string) string {
	if text == "" {
		return ""
	}

	// 1. Strip URLs
	reURL := regexp.MustCompile(`https?://\S+`)
	t := reURL.ReplaceAllString(text, " ")

	// 2. Strip code blocks
	reCodeBlocks := regexp.MustCompile("(?s)```.*?```")
	t = reCodeBlocks.ReplaceAllString(t, " ")

	// 3. Keep only human speech characters: letters, numbers, standard punctuation, spaces
	var b strings.Builder
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		} else if strings.ContainsRune(".,!?:;—-\"«»()", r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	clean := b.String()

	// 4. Collapse consecutive whitespace
	reSpaces := regexp.MustCompile(`\s+`)
	clean = reSpaces.ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean)
}

// chunkTextForSpeech splits text into chunks of at most maxRunes (safe ceiling <= 140)
// by sentences and words so it never exceeds Google TTS length boundaries (200 chars).
func chunkTextForSpeech(text string, maxRunes int) []string {
	clean := sanitizeTextForSpeech(text)
	if clean == "" {
		return nil
	}
	if maxRunes <= 0 || maxRunes > 160 {
		maxRunes = 140
	}

	// Cap total speech to a comfortable voice note length (~600 runes, ~4-5 chunks)
	runes := []rune(clean)
	if len(runes) > 600 {
		clean = string(runes[:600])
		// Try to end on sentence or word boundary
		if lastDot := strings.LastIndexAny(clean, ".!?"); lastDot > 300 {
			clean = clean[:lastDot+1]
		}
	}

	var chunks []string
	var current strings.Builder

	words := strings.Fields(clean)
	for _, w := range words {
		wRunes := utf8.RuneCountInString(w)
		curRunes := utf8.RuneCountInString(current.String())

		// If a single word is bizarrely longer than maxRunes, truncate it
		if wRunes > maxRunes {
			wRunesSlice := []rune(w)
			w = string(wRunesSlice[:maxRunes])
			wRunes = maxRunes
		}

		if curRunes+wRunes+1 > maxRunes {
			if current.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(current.String()))
				current.Reset()
			}
		}
		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(w)
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	return chunks
}

// synthesizeSpeech converts Russian/multilingual text into MP3 audio bytes using neural TTS.
// It handles text cleaning, chunking for Google Translate TTS 200-char limits,
// and concatenates MP3 audio chunks into a seamless audio stream.
func synthesizeSpeech(ctx context.Context, text string) ([]byte, error) {
	chunks := chunkTextForSpeech(text, 140)
	if len(chunks) == 0 {
		return nil, errors.New("текст для озвучивания пуст")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var combined bytes.Buffer

	for i, chunk := range chunks {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		var chunkData []byte
		var lastErr error

		// Retry each chunk up to 2 times
		for attempt := 0; attempt < 2; attempt++ {
			ttsURL := fmt.Sprintf("https://translate.google.com/translate_tts?ie=UTF-8&q=%s&tl=ru&client=tw-ob", url.QueryEscape(chunk))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, ttsURL, nil)
			if err != nil {
				lastErr = err
				continue
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				time.Sleep(200 * time.Millisecond)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				_ = resp.Body.Close()
				lastErr = fmt.Errorf("tts server returned status %d", resp.StatusCode)
				time.Sleep(200 * time.Millisecond)
				continue
			}

			data, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				lastErr = err
				continue
			}

			if len(data) >= 200 {
				chunkData = data
				lastErr = nil
				break
			}
		}

		if len(chunkData) == 0 {
			if lastErr != nil {
				return nil, fmt.Errorf("ошибка генерации фрагмента речи %d/%d: %w", i+1, len(chunks), lastErr)
			}
			return nil, fmt.Errorf("фрагмент речи %d слишком мал", i+1)
		}

		combined.Write(chunkData)
	}

	result := combined.Bytes()
	if len(result) < 200 {
		return nil, fmt.Errorf("tts audio too small (%d bytes)", len(result))
	}
	return result, nil
}

// extractDrawingPrompt analyzes user text for image generation intents.
// Returns the clean prompt and true if the intent is image generation, or ("", false) if not.
func extractDrawingPrompt(text string) (string, bool) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return "", false
	}

	// 1. Slash commands: /img, /image, /draw, /paint
	for _, cmd := range []string{"/img ", "/image ", "/draw ", "/paint "} {
		if strings.HasPrefix(clean, cmd) {
			prompt := strings.TrimSpace(strings.TrimPrefix(clean, cmd))
			if prompt != "" {
				return prompt, true
			}
		}
	}

	// 2. Safeguards: Questions should NOT be treated as drawing prompts.
	lower := strings.ToLower(clean)
	if strings.HasSuffix(lower, "?") {
		interrogatives := []string{
			"кто ", "кем ", "кому ", "кого ",
			"как ", "какой ", "какая ", "какое ", "какие ", "каких ",
			"где ", "куда ", "откуда ",
			"почему ", "зачем ", "отчего ",
			"что ", "чем ", "чего ", "в чем ",
			"сколько ",
		}
		for _, q := range interrogatives {
			if strings.HasPrefix(lower, q) {
				return "", false
			}
		}
		// Also if "нарисуй" is quoted inside quotes
		if strings.Contains(lower, "'нарисуй") || strings.Contains(lower, "\"нарисуй") ||
			strings.Contains(lower, "«нарисуй") {
			return "", false
		}
	}

	// Strip leading courtesy / polite filler
	courtesyPrefixes := []string{
		"пожалуйста, ", "пожалуйста ", "плиз, ", "плиз ", "бот, ", "бот ", "ассистент, ", "ассистент ",
	}
	trimmed := lower
	for {
		changed := false
		for _, cp := range courtesyPrefixes {
			if strings.HasPrefix(trimmed, cp) {
				trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, cp))
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	prefixes := []string{
		"сгенерируй изображение",
		"сгенерируй картинку",
		"сгенерируй иллюстрацию",
		"сгенерируй рисунок",
		"сгенерируй фото",
		"сгенерируй арт",
		"создай изображение",
		"создай картинку",
		"создай иллюстрацию",
		"создай рисунок",
		"создай фото",
		"создай арт",
		"нарисуй мне, пожалуйста,",
		"нарисуй мне пожалуйста",
		"нарисуй мне,",
		"нарисуй мне",
		"нарисуй пожалуйста,",
		"нарисуй пожалуйста",
		"нарисуйте мне",
		"нарисуйте",
		"нарисуй-ка",
		"нарисуй",
		"изобрази мне",
		"изобрази",
		"отрисуй мне",
		"отрисуй",
		"generate an image of",
		"generate image of",
		"generate an image",
		"generate image",
		"draw me a",
		"draw me an",
		"draw me",
		"draw a",
		"draw an",
		"draw",
		"paint me a",
		"paint me",
		"paint",
	}

	for _, p := range prefixes {
		if strings.HasPrefix(trimmed, p) {
			idx := strings.Index(lower, p)
			if idx >= 0 {
				prompt := clean[idx+len(p):]
				prompt = strings.TrimSpace(prompt)
				prompt = strings.TrimLeft(prompt, ":-—, ")
				prompt = strings.TrimSpace(prompt)
				if len(prompt) >= 2 {
					return prompt, true
				}
			}
		}
	}

	return "", false
}

var urlRegex = regexp.MustCompile(`https?://[^\s<>"]+`)

// extractURL searches for the first HTTP/HTTPS URL in text and extracts it alongside any surrounding user query.
func extractURL(text string) (rawURL string, userQuery string, ok bool) {
	loc := urlRegex.FindStringIndex(text)
	if loc == nil {
		return "", "", false
	}
	rawURL = text[loc[0]:loc[1]]
	for strings.HasSuffix(rawURL, ".") || strings.HasSuffix(rawURL, ",") ||
		strings.HasSuffix(rawURL, ")") || strings.HasSuffix(rawURL, ";") ||
		strings.HasSuffix(rawURL, "!") {
		rawURL = rawURL[:len(rawURL)-1]
	}

	queryBefore := strings.TrimSpace(text[:loc[0]])
	queryAfter := strings.TrimSpace(text[loc[1]:])
	var query string
	if queryBefore != "" && queryAfter != "" {
		query = queryBefore + " " + queryAfter
	} else if queryBefore != "" {
		query = queryBefore
	} else {
		query = queryAfter
	}
	return rawURL, query, true
}

// fetchAndExtractWebPageText downloads a web page and extracts readable clean text without HTML boilerplate.
func fetchAndExtractWebPageText(ctx context.Context, targetURL string) (title string, cleanContent string, err error) {
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MAXBot/1.0; +https://max.ru)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru,en;q=0.9")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to load page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("HTTP status %d (%s)", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	rawHTML := string(bodyBytes)

	// Extract <title>...</title>
	reTitle := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	if match := reTitle.FindStringSubmatch(rawHTML); len(match) > 1 {
		title = strings.TrimSpace(html.UnescapeString(match[1]))
		title = strings.ReplaceAll(title, "\n", " ")
		title = strings.ReplaceAll(title, "\r", " ")
	}
	if title == "" {
		title = targetURL
	}

	// Remove scripts, styles, noscript, svg, head
	reHead := regexp.MustCompile(`(?is)<head[^>]*>.*?</head>`)
	rawHTML = reHead.ReplaceAllString(rawHTML, "")

	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	rawHTML = reScript.ReplaceAllString(rawHTML, "")

	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	rawHTML = reStyle.ReplaceAllString(rawHTML, "")

	reNoscript := regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	rawHTML = reNoscript.ReplaceAllString(rawHTML, "")

	reSVG := regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
	rawHTML = reSVG.ReplaceAllString(rawHTML, "")

	// Replace block elements with newlines
	reBlock := regexp.MustCompile(`(?i)<(?:p|div|br|li|h[1-6]|tr|blockquote|section|article)[^>]*>`)
	rawHTML = reBlock.ReplaceAllString(rawHTML, "\n")

	// Strip remaining HTML tags
	reTags := regexp.MustCompile(`(?s)<[^>]+>`)
	cleanText := reTags.ReplaceAllString(rawHTML, " ")

	// Unescape HTML entities
	cleanText = html.UnescapeString(cleanText)

	// Normalize spaces
	lines := strings.Split(cleanText, "\n")
	var cleanLines []string
	for _, l := range lines {
		trimmedLine := strings.TrimSpace(l)
		if trimmedLine != "" {
			cleanLines = append(cleanLines, trimmedLine)
		}
	}
	cleanContent = strings.Join(cleanLines, "\n")

	// Limit to ~6000 runes to fit comfortably in LLM prompt
	runes := []rune(cleanContent)
	if len(runes) > 6000 {
		cleanContent = string(runes[:6000]) + "\n...[содержимое сокращено]..."
	}

	if strings.TrimSpace(cleanContent) == "" {
		cleanContent = "Текстовое содержимое на веб-странице не найдено (страница может загружаться динамически через JavaScript)."
	}

	return title, cleanContent, nil
}

// extractDocumentText extracts plain text from document attachments (txt, md, code files, pdf, docx, etc.).
func extractDocumentText(filename string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("файл пуст")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if qIdx := strings.Index(ext, "?"); qIdx != -1 {
		ext = ext[:qIdx]
	}

	if ext == ".pdf" {
		return extractPDFText(data)
	}

	if ext == ".docx" {
		return extractDocxText(data)
	}

	// For text files and source code (.txt, .md, .csv, .json, .yaml, .go, .py, etc.)
	if utf8.Valid(data) && bytes.IndexByte(data, 0) == -1 {
		text := strings.TrimSpace(string(data))
		runes := []rune(text)
		if len(runes) > 8000 {
			text = string(runes[:8000]) + "\n\n...[содержимое файла сокращено для размера контекста]..."
		}
		return text, nil
	}

	return "", fmt.Errorf("файл содержит нечитаемые бинарные данные")
}

// extractPDFText extracts readable plain text from uncompressed or flate-compressed PDF streams.
func extractPDFText(data []byte) (string, error) {
	var sb strings.Builder

	streamIdx := 0
	for {
		sStart := bytes.Index(data[streamIdx:], []byte("stream"))
		if sStart == -1 {
			break
		}
		streamContentStart := streamIdx + sStart + len("stream")
		if streamContentStart < len(data) && data[streamContentStart] == '\r' {
			streamContentStart++
		}
		if streamContentStart < len(data) && data[streamContentStart] == '\n' {
			streamContentStart++
		}

		sEnd := bytes.Index(data[streamContentStart:], []byte("endstream"))
		if sEnd == -1 {
			break
		}
		streamBytes := data[streamContentStart : streamContentStart+sEnd]

		headerStart := streamIdx + sStart - 128
		if headerStart < 0 {
			headerStart = 0
		}
		header := data[headerStart : streamIdx+sStart]

		var content []byte
		if bytes.Contains(header, []byte("/FlateDecode")) {
			zr, err := zlib.NewReader(bytes.NewReader(streamBytes))
			if err == nil {
				decompressed, rErr := io.ReadAll(zr)
				_ = zr.Close()
				if rErr == nil && len(decompressed) > 0 {
					content = decompressed
				}
			}
		} else {
			content = streamBytes
		}

		if len(content) > 0 {
			text := parsePDFContentStream(content)
			if text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(text)
			}
		}

		streamIdx = streamContentStart + sEnd + len("endstream")
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "", fmt.Errorf("текстовый слой в PDF не обнаружен (возможно, документ содержит только отсканированные изображения)")
	}
	return result, nil
}

func parsePDFContentStream(stream []byte) string {
	var parts []string

	reTj := regexp.MustCompile(`\((.*?)\)\s*Tj`)
	matches := reTj.FindAllSubmatch(stream, -1)
	for _, m := range matches {
		if len(m) > 1 {
			parts = append(parts, cleanPDFString(m[1]))
		}
	}

	reTJ := regexp.MustCompile(`\[(.*?)\]\s*TJ`)
	tjMatches := reTJ.FindAllSubmatch(stream, -1)
	for _, m := range tjMatches {
		if len(m) > 1 {
			innerRe := regexp.MustCompile(`\((.*?)\)`)
			innerMatches := innerRe.FindAllSubmatch(m[1], -1)
			var word strings.Builder
			for _, im := range innerMatches {
				if len(im) > 1 {
					word.WriteString(cleanPDFString(im[1]))
				}
			}
			if word.Len() > 0 {
				parts = append(parts, word.String())
			}
		}
	}

	return strings.Join(parts, " ")
}

func cleanPDFString(raw []byte) string {
	s := string(raw)
	s = strings.ReplaceAll(s, "\\(", "(")
	s = strings.ReplaceAll(s, "\\)", ")")
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\t", "\t")
	return strings.TrimSpace(s)
}

func isAudioFilename(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".ogg") || strings.HasSuffix(lower, ".mp3") ||
		strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".m4a") ||
		strings.HasSuffix(lower, ".aac") || strings.HasSuffix(lower, ".opus") ||
		strings.HasSuffix(lower, ".flac")
}

// extractDocxText extracts clean readable text from Microsoft Word (.docx) files.
func extractDocxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать docx: %w", err)
	}
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			xmlBytes, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			reBlock := regexp.MustCompile(`(?i)</?(?:w:p|w:tr)[^>]*>`)
			clean := reBlock.ReplaceAllString(string(xmlBytes), "\n")
			reTags := regexp.MustCompile(`<[^>]+>`)
			clean = reTags.ReplaceAllString(clean, "")
			clean = html.UnescapeString(clean)
			lines := strings.Split(clean, "\n")
			var res []string
			for _, l := range lines {
				t := strings.TrimSpace(l)
				if t != "" {
					res = append(res, t)
				}
			}
			if len(res) == 0 {
				return "", fmt.Errorf("в документе docx не обнаружен текст")
			}
			text := strings.Join(res, "\n")
			runes := []rune(text)
			if len(runes) > 8000 {
				text = string(runes[:8000]) + "\n\n...[содержимое файла сокращено]..."
			}
			return text, nil
		}
	}
	return "", fmt.Errorf("текстовый блок word/document.xml не найден в архиве docx")
}

// isSupportedDocument verifies whether a given document can be processed by LLM/parsers.
func isSupportedDocument(filename string, data []byte) bool {
	if len(data) == 0 {
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if qIdx := strings.Index(ext, "?"); qIdx != -1 {
		ext = ext[:qIdx]
	}

	// 1. Known unsupported binary formats
	unsupportedExts := map[string]bool{
		".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true, ".iso": true, ".dmg": true,
		".apk": true, ".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true, ".flv": true,
		".webm": true, ".m4v": true, ".3gp": true, ".doc": true, ".xlsx": true, ".xls": true,
		".pptx": true, ".ppt": true, ".torrent": true,
	}
	if unsupportedExts[ext] {
		return false
	}

	// 2. Sniff content type for media or binary
	detected := http.DetectContentType(data)
	if strings.HasPrefix(detected, "video/") || strings.HasPrefix(detected, "audio/") {
		return false
	}

	// 3. For any document other than PDF or DOCX, it CANNOT contain NUL bytes (\x00) and MUST be valid UTF-8!
	if ext != ".pdf" && ext != ".docx" {
		if bytes.IndexByte(data, 0) != -1 || !utf8.Valid(data) {
			return false
		}
	}

	// 4. Direct supported text/document extensions
	supportedExts := map[string]bool{
		// Documents & Text
		".txt": true, ".text": true, ".md": true, ".markdown": true, ".log": true,
		".rtf": true, ".tex": true, ".pdf": true, ".docx": true,
		// Data & Config
		".json": true, ".csv": true, ".tsv": true, ".yaml": true, ".yml": true,
		".xml": true, ".toml": true, ".ini": true, ".cfg": true, ".conf": true,
		".env": true, ".properties": true,
		// Web & Scripting
		".html": true, ".htm": true, ".xhtml": true, ".css": true, ".scss": true,
		".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".jsx": true,
		// Programming languages
		".go": true, ".py": true, ".java": true, ".c": true, ".cpp": true, ".cc": true,
		".h": true, ".hpp": true, ".cs": true, ".rs": true, ".php": true, ".rb": true,
		".swift": true, ".kt": true, ".scala": true, ".sql": true, ".sh": true,
		".bash": true, ".zsh": true, ".bat": true, ".cmd": true, ".ps1": true,
		".lua": true, ".r": true, ".proto": true, ".graphql": true, ".gql": true,
		".vue": true, ".svelte": true,
	}

	if supportedExts[ext] {
		return true
	}

	// 5. Plain UTF-8 without NUL bytes for unknown extensions
	if (strings.HasPrefix(detected, "text/") || detected == "application/json" || detected == "application/xml") &&
		bytes.IndexByte(data, 0) == -1 && utf8.Valid(data) {
		return true
	}

	if utf8.Valid(data) && bytes.IndexByte(data, 0) == -1 {
		return true
	}

	return false
}

// formatFileSize formats raw byte counts into human-readable strings (байт, КБ, МБ).
func formatFileSize(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%d байт", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%.1f КБ", float64(b)/1024.0)
	}
	return fmt.Sprintf("%.1f МБ", float64(b)/(1024.0*1024.0))
}

// getFileFormatDescription returns a friendly category and recommendation for unsupported formats.
func getFileFormatDescription(filename string, data []byte) (category string, tip string) {
	ext := strings.ToLower(filepath.Ext(filename))
	if qIdx := strings.Index(ext, "?"); qIdx != -1 {
		ext = ext[:qIdx]
	}

	switch ext {
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz":
		return "архив файлов", "Распакуйте архив и отправьте нужные документы (.txt, .pdf, .docx) или изображения по отдельности."
	case ".xlsx", ".xls":
		return "таблица Microsoft Excel", "Сохраните или экспортируйте таблицу в формат CSV или PDF и отправьте файл боту."
	case ".pptx", ".ppt":
		return "презентация Microsoft PowerPoint", "Экспортируйте презентацию в PDF и отправьте боту для анализа слайдов."
	case ".doc":
		return "устаревший документ Word (.doc)", "Сохраните документ в формате DOCX, PDF или TXT и отправьте боту."
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".m4v", ".3gp":
		return "видеофайл", "Бот анализирует аудио и изображения. Извлеките аудиодорожку (MP3/OGG/WAV) или сделайте скриншот кадра."
	case ".exe", ".dll", ".so", ".bin", ".iso", ".dmg", ".apk":
		return "бинарный исполняемый файл или системный образ", "Исполняемые файлы не содержат текстовых данных для анализа нейросетью."
	}

	if len(data) > 0 {
		detected := http.DetectContentType(data)
		if strings.HasPrefix(detected, "video/") {
			return "видеофайл", "Бот анализирует аудио и изображения. Извлеките аудиодорожку (MP3/OGG/WAV) или сделайте скриншот кадра."
		}
		if strings.HasPrefix(detected, "audio/") {
			return "аудиофайл", "Отправьте аудио как голосовое сообщение или в поддерживаемом формате (OGG, MP3, WAV)."
		}
		if detected == "application/zip" {
			return "архив файлов", "Распакуйте архив и отправьте нужные файлы документов или изображений по отдельности."
		}
	}

	return "неподдерживаемый бинарный формат", "Вы можете скопировать текст из файла и отправить его сообщением в чат или конвертировать в PDF/TXT."
}

// handleUnsupportedFile sends an informative, structured explanation when a user uploads an unsupported file.
func handleUnsupportedFile(c *bot.Context, filename string, size int, data []byte) error {
	desc, tip := getFileFormatDescription(filename, data)
	if (filename == "" || filename == "document.txt" || filename == "файл") && len(data) > 0 {
		detected := http.DetectContentType(data)
		if strings.HasPrefix(detected, "video/") {
			filename = "видеофайл.mp4"
		} else if strings.HasPrefix(detected, "audio/") {
			filename = "аудиофайл"
		} else if detected == "application/zip" {
			filename = "архив.zip"
		}
	}

	text := fmt.Sprintf("📁 **Получен файл:** `%s` *(размер: %s)*\n\n"+
		"Бот пока не поддерживает прямую обработку файлов этого типа (%s).\n\n"+
		"💡 **Поддерживаемые типы файлов:**\n"+
		"• 🖼️ **Изображения** *(JPG, PNG, WEBP, GIF, SVG, BMP, HEIC)* — зрительный анализ и OCR-распознавание текста.\n"+
		"• 🎙️ **Аудио и голос** *(голосовые заметки, OGG, MP3, WAV, M4A, FLAC, AAC)* — транскрибация речи через Whisper и ответ нейросети.\n"+
		"• 📄 **Документы и текст** *(TXT, PDF, DOCX, MD, JSON, CSV, YAML, XML, HTML, исходный код Go/Python/JS/SQL и др.)* — анализ содержания, выжимка сути и ответы на вопросы.\n\n"+
		"👉 **Рекомендация:**\n%s",
		filename, formatFileSize(size), desc, tip)

	kb := maxbot.NewInlineKeyboard().
		Row(
			maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
			maxbot.NewCallbackButton("◀️ В меню AI", "scen:ai"),
		)

	_, err := c.ReplyKeyboard(text, kb)
	return err
}

// handleUnknownAttachment extracts filename and size from any unrecognized attachment and shows the unsupported file help.
func handleUnknownAttachment(c *bot.Context, att maxbot.Attachment) error {
	filename := att.GetFilename()
	if filename == "" {
		filename = "файл"
	}
	size := int(att.GetSize())
	return handleUnsupportedFile(c, filename, size, nil)
}
