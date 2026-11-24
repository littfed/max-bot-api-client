package main

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/store"
	_ "modernc.org/sqlite"
)

func TestConvertContextToDomainEvent(t *testing.T) {
	// Message update
	msgUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			MessageID: "msg_999",
			Sender: &maxbot.User{
				UserID:    12345,
				Username:  "johndoe",
				FirstName: "John",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 67890,
			},
			Body: maxbot.MessageBody{
				Text: "/mode shop",
			},
		},
	}
	msgCtx := bot.NewContext(context.Background(), nil, msgUpdate)
	evt := convertContextToDomainEvent(msgCtx)

	if evt == nil {
		t.Fatal("expected non-nil domain event for message")
	}
	if evt.Platform != "max" {
		t.Errorf("expected platform max, got %s", evt.Platform)
	}
	if evt.PlatformUserID != "12345" {
		t.Errorf("expected platform user id 12345, got %s", evt.PlatformUserID)
	}
	if evt.Username != "johndoe" || evt.FirstName != "John" {
		t.Errorf("expected johndoe/John, got %s/%s", evt.Username, evt.FirstName)
	}
	if evt.Message == nil || evt.Message.Text != "/mode shop" {
		t.Errorf("expected message text /mode shop, got %+v", evt.Message)
	}

	// Callback update
	cbUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Callback: &maxbot.Callback{
			CallbackID: "cb_111",
			User: &maxbot.User{
				UserID:    54321,
				Username:  "alice",
				FirstName: "Alice",
			},
			Message: &maxbot.Message{
				MessageID: "msg_222",
				Recipient: &maxbot.MessageRecipient{
					ChatID: 333,
				},
			},
			Payload: "mode:select:shop",
		},
	}
	cbCtx := bot.NewContext(context.Background(), nil, cbUpdate)
	cbEvt := convertContextToDomainEvent(cbCtx)

	if cbEvt == nil {
		t.Fatal("expected non-nil domain event for callback")
	}
	if cbEvt.Callback == nil || cbEvt.Callback.Data != "mode:select:shop" {
		t.Errorf("expected callback data mode:select:shop, got %+v", cbEvt.Callback)
	}
}

func TestWireBotToOrchestrator(t *testing.T) {
	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer dbConn.Close()

	sqlStore, err := store.NewSQLStore(dbConn, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	fsm := core.NewCompositeFSM(sqlStore, nil)
	lockManager := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(sqlStore, nil)
	orch := core.NewOrchestrator(sqlStore, fsm, lockManager, guard)

	shopMod := shop.New(sqlStore)
	orch.RegisterModule(shopMod)

	b := bot.NewWithClient(nil)
	wireBotToOrchestrator(b, orch, slog.Default())

	// Test processing an update via bot router
	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Timestamp:  time.Now().Unix(),
		Message: &maxbot.Message{
			MessageID: "msg_test",
			Sender: &maxbot.User{
				UserID:    42,
				Username:  "tester",
				FirstName: "Test",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 100,
			},
			Body: maxbot.MessageBody{
				Text: "/help",
			},
		},
	}

	err = b.ProcessUpdate(context.Background(), update)
	if err != nil {
		t.Fatalf("ProcessUpdate failed: %v", err)
	}
}
