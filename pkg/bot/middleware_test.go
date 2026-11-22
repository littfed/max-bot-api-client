package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/pkg/logger"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestRecoveryMiddlewareWithTraceID(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.New(
		logger.WithLevel(slog.LevelDebug),
		logger.WithOutput(&buf),
	)

	ctx := logger.WithContext(context.Background(), testLogger)
	c := NewContext(ctx, nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{Text: "trigger-panic"},
		},
	})

	handler := Recovery(testLogger)(func(c *Context) error {
		panic("simulated test panic!")
	})

	err := handler(c)
	if err == nil {
		t.Fatal("expected error from panic, got nil")
	}
	if !strings.Contains(err.Error(), "simulated test panic!") {
		t.Errorf("unexpected error message: %v", err)
	}

	out := strings.TrimSpace(buf.String())
	var logData map[string]interface{}
	if err := json.Unmarshal([]byte(out), &logData); err != nil {
		t.Fatalf("failed to unmarshal JSON log: %v, raw: %s", err, out)
	}

	if logData["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", logData["level"])
	}
	if logData["trace_id"] != c.TraceID() {
		t.Errorf("expected trace_id %q in log, got %v", c.TraceID(), logData["trace_id"])
	}
	if !strings.Contains(out, "stack_trace") {
		t.Errorf("expected stack_trace field in log output: %s", out)
	}
}

func TestLoggerMiddleware(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.New(
		logger.WithLevel(slog.LevelDebug),
		logger.WithOutput(&buf),
	)

	ctx := logger.WithContext(context.Background(), testLogger)
	c := NewContext(ctx, nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{UserID: 12345},
			Body:   maxbot.MessageBody{Text: "hello"},
		},
	})

	// Test success flow
	mw := Logger(testLogger)
	err := mw(func(c *Context) error {
		return nil
	})(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines (started + handled), got %d: %s", len(lines), buf.String())
	}

	var finishLog map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &finishLog); err != nil {
		t.Fatalf("failed to unmarshal finish log JSON: %v", err)
	}
	if finishLog["level"] != "INFO" {
		t.Errorf("expected level INFO, got %v", finishLog["level"])
	}
	if finishLog["trace_id"] != c.TraceID() {
		t.Errorf("expected trace_id %q, got %v", c.TraceID(), finishLog["trace_id"])
	}
	if _, ok := finishLog["duration_ms"]; !ok {
		t.Errorf("expected duration_ms in finish log")
	}

	// Test error flow
	buf.Reset()
	err = mw(func(c *Context) error {
		return errors.New("handler failed")
	})(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var errLog map[string]interface{}
	if err := json.Unmarshal([]byte(errLines[len(errLines)-1]), &errLog); err != nil {
		t.Fatalf("failed to unmarshal error log JSON: %v", err)
	}
	if errLog["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", errLog["level"])
	}
}
