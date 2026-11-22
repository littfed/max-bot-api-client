package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/pkg/logger"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestContextTraceIDAndLogger(t *testing.T) {
	var buf bytes.Buffer
	baseLogger := logger.New(
		logger.WithLevel(slog.LevelDebug),
		logger.WithOutput(&buf),
	)

	ctx := logger.WithContext(context.Background(), baseLogger)

	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{
				UserID:   777888,
				Username: "tester",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 999111,
			},
			Body: maxbot.MessageBody{
				Text: "hello world",
			},
		},
	}

	c := NewContext(ctx, nil, update)

	traceID := c.TraceID()
	if traceID == "" || !strings.HasPrefix(traceID, "tr_") {
		t.Fatalf("expected trace_id with 'tr_' prefix, got %q", traceID)
	}

	c.Logger().Info("processing message", slog.String("action", "test"))

	out := strings.TrimSpace(buf.String())
	var logData map[string]interface{}
	if err := json.Unmarshal([]byte(out), &logData); err != nil {
		t.Fatalf("failed to unmarshal log JSON: %v, raw: %s", err, out)
	}

	if logData["trace_id"] != traceID {
		t.Errorf("expected trace_id %q, got %v", traceID, logData["trace_id"])
	}
	if logData["user_id"] != float64(777888) {
		t.Errorf("expected user_id 777888, got %v", logData["user_id"])
	}
	if logData["chat_id"] != float64(999111) {
		t.Errorf("expected chat_id 999111, got %v", logData["chat_id"])
	}
	if logData["update_type"] != maxbot.UpdateTypeMessageCreated {
		t.Errorf("expected update_type %q, got %v", maxbot.UpdateTypeMessageCreated, logData["update_type"])
	}
}

func TestAttachmentClassification(t *testing.T) {
	// 1. Text file (3418 bytes) must be classified as FileAttachment, NOT AudioAttachment
	txtData := strings.Repeat("A", 3418)
	txtUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Attachments: []maxbot.Attachment{
					{
						Type: maxbot.UploadTypeFile,
						Payload: map[string]any{
							"title":        "notes.txt",
							"data":         txtData,
							"content_type": "text/plain; charset=utf-8",
						},
					},
				},
			},
		},
	}
	cTxt := NewContext(context.Background(), nil, txtUpdate)
	if _, isAudio := cTxt.AudioAttachment(); isAudio {
		t.Errorf("expected text file NOT to be classified as AudioAttachment")
	}
	if _, isFile := cTxt.FileAttachment(); !isFile {
		t.Errorf("expected text file to be classified as FileAttachment")
	}

	// 2. Audio file (.ogg) with generic "file" type must be classified as AudioAttachment
	audioUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Attachments: []maxbot.Attachment{
					{
						Type: "file",
						Payload: map[string]any{
							"name":         "voice.ogg",
							"data":         "T2dnUwACAAAAAAAAA...",
							"content_type": "audio/ogg",
						},
					},
				},
			},
		},
	}
	cAudio := NewContext(context.Background(), nil, audioUpdate)
	if _, isAudio := cAudio.AudioAttachment(); !isAudio {
		t.Errorf("expected .ogg file to be classified as AudioAttachment")
	}
	if _, isFile := cAudio.FileAttachment(); isFile {
		t.Errorf("expected .ogg file NOT to be classified as FileAttachment")
	}

	// 3. Image file (.webp) with generic "file" type must be classified as ImageAttachment
	webpUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Attachments: []maxbot.Attachment{
					{
						Type: "file",
						Payload: map[string]any{
							"filename":     "photo.webp",
							"data":         "UklGR...",
							"content_type": "image/webp",
						},
					},
				},
			},
		},
	}
	cWebp := NewContext(context.Background(), nil, webpUpdate)
	if _, isImg := cWebp.ImageAttachment(); !isImg {
		t.Errorf("expected .webp file to be classified as ImageAttachment")
	}
	if _, isFile := cWebp.FileAttachment(); isFile {
		t.Errorf("expected .webp file NOT to be classified as FileAttachment")
	}
}
