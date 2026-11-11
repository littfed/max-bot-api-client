package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// RegisterVisionHandlers registers the vision recognition handler for image attachments.
func RegisterVisionHandlers(b *bot.Bot, gmaClient *gma.Client, sm *gma.SessionManager, logger *slog.Logger) {
	// Match any incoming message that has an image attachment
	b.Router().OnMessage(func(c *bot.Context) error {
		msg := c.Message()
		if msg == nil {
			return nil
		}

		imageURL := extractImageURL(msg)
		if imageURL == "" {
			return nil // No image attached, let other handlers process
		}

		prompt := strings.TrimSpace(msg.Body.Text)
		if prompt == "" {
			prompt = "Опиши подробно, что изображено на этом изображении. Если на картинке присутствует текст, приведи его полную и точную расшифровку (OCR)."
		}

		userID := getUserID(c)
		_, visionModel, _ := sm.GetModels(userID)

		// Indicate typing in chat
		_ = c.Typing()

		logger.Info("обработка изображения через Vision",
			slog.Int64("user_id", userID),
			slog.String("model", visionModel),
			slog.String("image_url", imageURL),
			slog.String("prompt", prompt),
		)

		req := &gma.ChatCompletionRequest{
			Model: visionModel,
			Messages: []gma.ChatMessage{
				gma.NewTextMessage(gma.RoleSystem, "Ты эксперт по компьютерному зрению и анализу изображений. Внимательно изучай детали картинок и давай четкие структурированные ответы на русском языке."),
				gma.NewMultimodalMessage(prompt, imageURL),
			},
		}

		resp, err := gmaClient.ChatCompletion(c.Ctx(), req)
		if err != nil {
			logger.Error("ошибка распознавания изображения в GMA", slog.Any("error", err), slog.String("model", visionModel))
			_, replyErr := c.Reply(fmt.Sprintf("⚠️ **Ошибка распознавания фото:** %s\n\nУбедитесь, что изображение доступно по прямой ссылке, или смените модель Vision в `/model`.", err.Error()))
			return replyErr
		}

		answer := resp.Text()
		if strings.TrimSpace(answer) == "" {
			answer = "*(Нейросеть не смогла распознать детали на этом изображении)*"
		}

		// Also record into user memory so they can ask follow-up questions
		sm.AddUserMessage(userID, fmt.Sprintf("[Пользователь прикрепил изображение]: %s", prompt))
		sm.AddAssistantMessage(userID, answer)

		_, err = c.Reply(fmt.Sprintf("👁️ **Анализ изображения (`%s`):**\n\n%s", visionModel, answer))
		return err
	})
}

// extractImageURL extracts an image URL from the message attachments if present.
func extractImageURL(msg *maxbot.Message) string {
	if msg == nil {
		return ""
	}

	for _, att := range msg.Body.Attachments {
		if att.Type != maxbot.UploadTypeImage && att.Type != "photo" {
			continue
		}

		switch payload := att.Payload.(type) {
		case map[string]any:
			if u, ok := payload["url"].(string); ok && u != "" {
				return u
			}
			if u, ok := payload["file_url"].(string); ok && u != "" {
				return u
			}
			if u, ok := payload["src"].(string); ok && u != "" {
				return u
			}
			if photos, ok := payload["photos"].(map[string]any); ok {
				for _, v := range photos {
					if pMap, ok := v.(map[string]any); ok {
						if u, ok := pMap["url"].(string); ok && u != "" {
							return u
						}
					}
				}
			}
		case string:
			if strings.HasPrefix(payload, "http://") || strings.HasPrefix(payload, "https://") {
				return payload
			}
		}
	}

	return ""
}
