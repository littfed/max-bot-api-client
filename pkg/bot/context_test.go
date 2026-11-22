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
