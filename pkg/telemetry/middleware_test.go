package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestTelemetryMiddlewareCommands(t *testing.T) {
	collector := NewCollector(WithBufferSize(50))
	mw := Middleware(collector)

	// Command update
	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{UserID: 111, Name: "User1"},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 222,
			},
			Body: maxbot.MessageBody{
				Text: "/ping arg",
			},
		},
	}

	c := bot.NewContext(context.Background(), nil, update)

	handlerCalled := false
	handler := mw(func(ctx *bot.Context) error {
		handlerCalled = true
		time.Sleep(5 * time.Millisecond)
		return nil
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !handlerCalled {
		t.Fatalf("handler was not called")
	}

	events := collector.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 command event, got %d", len(events))
	}

	cmdEv := events[0]
	if cmdEv.Type != EventCommand || cmdEv.Action != "/ping" || cmdEv.UserID != 111 || cmdEv.ChatID != 222 {
		t.Errorf("unexpected command event fields: %+v", cmdEv)
	}
	if cmdEv.Duration < 5*time.Millisecond {
		t.Errorf("expected duration >= 5ms, got %v", cmdEv.Duration)
	}
}

func TestTelemetryMiddlewareButtonClick(t *testing.T) {
	collector := NewCollector(WithBufferSize(50))
	mw := Middleware(collector)

	msgTime := time.Now().Add(-2 * time.Second).UnixMilli()
	cbTime := time.Now().UnixMilli()

	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Timestamp:  cbTime,
		Message: &maxbot.Message{
			Timestamp: msgTime,
			Recipient: &maxbot.MessageRecipient{ChatID: 555},
		},
		Callback: &maxbot.Callback{
			CallbackID: "cb_1",
			Payload:    FormatTrackedPayload("select_plan", "pricing", "tr_1"),
			Timestamp:  cbTime,
			User:       &maxbot.User{UserID: 999, Name: "Buyer"},
		},
	}

	c := bot.NewContext(context.Background(), nil, update)

	handler := mw(func(ctx *bot.Context) error {
		// Test custom business event tracking inside handler
		Track(ctx, "view_checkout", map[string]any{"plan": "pro"})
		return nil
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	events := collector.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events (click + custom), got %d", len(events))
	}

	clickEv := events[0]
	if clickEv.Type != EventButtonClick || clickEv.Action != "select_plan" || clickEv.Screen != "pricing" {
		t.Errorf("unexpected click event: %+v", clickEv)
	}
	if clickEv.TimeDelta < 1*time.Second {
		t.Errorf("expected TimeDelta >= 1s, got %v", clickEv.TimeDelta)
	}

	customEv := events[1]
	if customEv.Type != EventCustom || customEv.Action != "view_checkout" || customEv.Properties["plan"] != "pro" {
		t.Errorf("unexpected custom event: %+v", customEv)
	}
}

func TestTelemetryMiddlewareErrorHandler(t *testing.T) {
	collector := NewCollector(WithBufferSize(50))
	mw := Middleware(collector)

	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{UserID: 333},
			Body:   maxbot.MessageBody{Text: "/fail"},
		},
	}

	c := bot.NewContext(context.Background(), nil, update)

	handler := mw(func(ctx *bot.Context) error {
		return errors.New("db query failed")
	})

	err := handler(c)
	if err == nil {
		t.Fatalf("expected error from handler")
	}

	events := collector.Events()
	// Should record error event + command event with error
	if len(events) != 2 {
		t.Fatalf("expected 2 events (error + command), got %d", len(events))
	}

	errEv := events[0]
	if errEv.Type != EventError || errEv.Error != "db query failed" {
		t.Errorf("unexpected error event: %+v", errEv)
	}
}
