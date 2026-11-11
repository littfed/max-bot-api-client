package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// RegisterImageHandlers registers /image and /draw commands for text-to-image generation.
func RegisterImageHandlers(b *bot.Bot, gmaClient *gma.Client, sm *gma.SessionManager, logger *slog.Logger) {
	handleImageGen := func(c *bot.Context) error {
		_, args := c.Command()
		prompt := strings.TrimSpace(args)
		if prompt == "" {
			text := "🎨 **Генерация изображений нейросетью**\n\n" +
				"Пожалуйста, укажите текстовое описание того, что нужно нарисовать.\n\n" +
				"**Примеры использования:**\n" +
				"• `/image футуристический автомобиль летит над неоновым мегаполисом`\n" +
				"• `/draw уютный домик в заснеженных горах на рассвете акварелью`\n" +
				"• `/image космическая станция на орбите Юпитера, фотореализм 8k`"
			_, err := c.Reply(text)
			return err
		}

		userID := getUserID(c)
		_, _, imageModel := sm.GetModels(userID)

		// Indicate sending photo action
		_ = c.SendChatAction(maxbot.ActionSendingPhoto)

		logger.Info("генерация изображения через GMA",
			slog.Int64("user_id", userID),
			slog.String("model", imageModel),
			slog.String("prompt", prompt),
		)

		_, _ = c.Reply(fmt.Sprintf("🎨 **Генерирую изображение по запросу:** *«%s»*\n*(Модель: `%s`, подождите несколько секунд...)*", prompt, imageModel))

		// 1. Call GMA Image Generation API
		req := &gma.ImageGenerationRequest{
			Prompt: prompt,
			Model:  imageModel,
		}

		resp, err := gmaClient.GenerateImage(c.Ctx(), req)
		if err != nil {
			logger.Error("ошибка вызова /v1/images/generations", slog.Any("error", err), slog.String("model", imageModel))
			_, replyErr := c.Reply(fmt.Sprintf("⚠️ **Не удалось сгенерировать изображение:** %s\n\nПопробуйте изменить промпт или выбрать другую модель в `/model`.", err.Error()))
			return replyErr
		}

		if len(resp.Data) == 0 || (resp.Data[0].URL == "" && resp.Data[0].B64JSON == "") {
			_, replyErr := c.Reply("⚠️ Модель завершила работу, но не вернула данные изображения. Попробуйте еще раз.")
			return replyErr
		}

		// 2. Fetch image bytes from URL or Base64
		imgBytes, err := resolveImageBytes(c.Ctx(), resp.Data[0])
		if err != nil {
			logger.Error("ошибка получения байтов картинки", slog.Any("error", err))
			// If we have URL, we can still fall back to passing the URL directly
			if resp.Data[0].URL != "" {
				return sendImageByURL(c, resp.Data[0].URL, prompt, imageModel)
			}
			_, replyErr := c.Reply(fmt.Sprintf("⚠️ Ошибка загрузки сгенерированного файла: %s", err.Error()))
			return replyErr
		}

		// 3. Upload media directly to MAX Bot API
		if c.Client() != nil {
			uploadResp, uploadErr := c.Client().UploadMedia(c.Ctx(), maxbot.UploadTypeImage, "generated.png", bytes.NewReader(imgBytes))
			if uploadErr == nil && uploadResp != nil && uploadResp.Token != "" {
				recipient, rErr := c.Recipient()
				if rErr == nil {
					caption := fmt.Sprintf("🎨 **%s**\n*Модель:* `%s`", prompt, imageModel)
					msgReq := &maxbot.SendMessageRequest{
						Text:   caption,
						Format: "markdown",
						Attachments: []maxbot.Attachment{
							{
								Type: maxbot.UploadTypeImage,
								Payload: map[string]any{
									"token": uploadResp.Token,
								},
							},
						},
					}
					_, sendErr := c.Client().SendMessage(c.Ctx(), recipient, msgReq)
					return sendErr
				}
			} else if uploadErr != nil {
				logger.Warn("не удалось загрузить медиа в MAX, пробуем прямую ссылку", slog.Any("error", uploadErr))
			}
		}

		// Fallback: send as direct URL attachment
		if resp.Data[0].URL != "" {
			return sendImageByURL(c, resp.Data[0].URL, prompt, imageModel)
		}

		_, replyErr := c.Reply("🎨 Изображение успешно сгенерировано, но не может быть доставлено в чат.")
		return replyErr
	}

	b.OnCommand("image", handleImageGen)
	b.OnCommand("draw", handleImageGen)
}

// resolveImageBytes gets raw binary image bytes from URL or Base64.
func resolveImageBytes(ctx context.Context, item gma.ImageData) ([]byte, error) {
	if item.B64JSON != "" {
		return base64.StdEncoding.DecodeString(item.B64JSON)
	}

	if item.URL == "" {
		return nil, fmt.Errorf("empty image url and base64 data")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image from %s: %w", item.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download returned HTTP status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// sendImageByURL sends an image using external URL attachment payload.
func sendImageByURL(c *bot.Context, imgURL, prompt, model string) error {
	recipient, err := c.Recipient()
	if err != nil {
		_, rErr := c.Reply(fmt.Sprintf("🎨 **%s**\n*Модель:* `%s`\nСсылка: %s", prompt, model, imgURL))
		return rErr
	}

	caption := fmt.Sprintf("🎨 **%s**\n*Модель:* `%s`", prompt, model)
	msgReq := &maxbot.SendMessageRequest{
		Text:   caption,
		Format: "markdown",
		Attachments: []maxbot.Attachment{
			{
				Type: maxbot.UploadTypeImage,
				Payload: map[string]any{
					"url": imgURL,
				},
			},
		},
	}

	if c.Client() != nil {
		_, err := c.Client().SendMessage(c.Ctx(), recipient, msgReq)
		return err
	}
	_, err = c.Reply(caption + "\n" + imgURL)
	return err
}
