package main

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// RegisterVisionHandlers registers the vision recognition handler for image attachments.
func RegisterVisionHandlers(b *bot.Bot, gmaClient *gma.Client, sm *gma.SessionManager, logger *slog.Logger) {
	// Match incoming messages that contain an image attachment
	b.OnImage(func(c *bot.Context) error {
		imageURL, _ := c.ImageURL()
		imgBytes, _, _ := c.ImageBytes()

		if imageURL == "" && len(imgBytes) == 0 {
			return nil
		}

		prompt := strings.TrimSpace(c.Text())
		if prompt == "" || prompt == "Распознай картинку" || prompt == "Опиши фото" || prompt == "/vision" {
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
			slog.Int("image_bytes_len", len(imgBytes)),
			slog.String("prompt", prompt),
		)

		var multimodalMsg gma.ChatMessage
		if len(imgBytes) > 0 {
			dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(imgBytes))
			multimodalMsg = gma.NewMultimodalMessage(prompt, dataURI)
		} else {
			multimodalMsg = gma.NewMultimodalMessage(prompt, imageURL)
		}

		req := &gma.ChatCompletionRequest{
			Model: visionModel,
			Messages: []gma.ChatMessage{
				gma.NewTextMessage(gma.RoleSystem, "Ты эксперт по компьютерному зрению и анализу изображений. Внимательно изучай детали картинок и давай четкие структурированные ответы на русском языке."),
				multimodalMsg,
			},
		}

		resp, err := gmaClient.ChatCompletion(c.Ctx(), req)
		if err != nil {
			logger.Error("ошибка распознавания изображения в GMA", slog.Any("error", err), slog.String("model", visionModel))
			_, replyErr := c.Reply(fmt.Sprintf("⚠️ **Ошибка распознавания фото:** %s\n\nУбедитесь, что изображение доступно, или смените модель Vision в `/model`.", err.Error()))
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
	c := &bot.Context{
		Update: &maxbot.Update{
			Message: msg,
		},
	}
	u, _ := c.ImageURL()
	return u
}
