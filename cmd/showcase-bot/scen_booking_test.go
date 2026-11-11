package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestBookingScenarioWorkflow(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())
	RegisterBookingScenario(botInstance, hood)

	userID := int64(111)
	chatID := int64(222)

	makeCtx := func(text string, payload ...string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: text},
			},
		}
		if len(payload) > 0 {
			u.Callback = &maxbot.Callback{
				CallbackID: "cb_booking_test",
				Payload:    payload[0],
				User:       &maxbot.User{UserID: userID},
			}
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	// 1. Open booking menu
	cOpen := makeCtx("", "scen:booking")
	if err := botInstance.Router().Handle(cOpen); err != nil {
		t.Fatalf("booking menu failed: %v", err)
	}

	// 2. Select service (Spa massage)
	cService := makeCtx("", "booking:srv:srv_spa")
	if err := botInstance.Router().Handle(cService); err != nil {
		t.Fatalf("select service failed: %v", err)
	}

	// 3. Select time slot
	cSlot := makeCtx("", "booking:slot:srv_spa:Завтра, 14:30")
	if err := botInstance.Router().Handle(cSlot); err != nil {
		t.Fatalf("select slot failed: %v", err)
	}

	// Verify state is BOOKING_STEP_PHONE
	key := fsm.SessionKey{UserID: userID, ChatID: chatID}
	st, err := storage.GetState(context.Background(), key)
	if err != nil || st != "BOOKING_STEP_PHONE" {
		t.Fatalf("expected state BOOKING_STEP_PHONE, got %s (err: %v)", st, err)
	}

	// 4. Submit phone number
	cPhone := makeCtx("+7 (912) 345-67-89")
	if err := botInstance.Router().Handle(cPhone); err != nil {
		t.Fatalf("phone submission failed: %v", err)
	}

	// Verify session reset after confirmation
	st, _ = storage.GetState(context.Background(), key)
	if st != fsm.StateEmpty {
		t.Fatalf("expected empty state after booking confirmation, got %s", st)
	}
}

func TestFindService(t *testing.T) {
	s := findService("srv_cut")
	if s == nil || !strings.Contains(s.Name, "стрижка") {
		t.Fatalf("expected haircut service, got %+v", s)
	}

	sNil := findService("unknown")
	if sNil != nil {
		t.Fatalf("expected nil for unknown service")
	}
}
