package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestGuardScenarioWorkflow(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())
	RegisterGuardScenario(botInstance, hood, nil)

	userID := int64(707)
	chatID := int64(808)

	makeCtx := func(payload string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: ""},
			},
			Callback: &maxbot.Callback{
				CallbackID: "cb_guard",
				Payload:    payload,
				User:       &maxbot.User{UserID: userID},
			},
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	// 1. Open Guard Menu
	if err := botInstance.Router().Handle(makeCtx("scen:guard")); err != nil {
		t.Fatalf("guard menu failed: %v", err)
	}

	// 2. Launch in-dialog simulation (sim_join)
	if err := botInstance.Router().Handle(makeCtx("guard:sim_join")); err != nil {
		t.Fatalf("sim_join failed: %v", err)
	}

	// 3. User clicks "Я человек" button
	verifyPayload := fmt.Sprintf("guard:v:1000:%d", userID)
	if err := botInstance.Router().Handle(makeCtx(verifyPayload)); err != nil {
		t.Fatalf("captcha verify callback failed: %v", err)
	}

	// 4. Test timeout kick simulation
	if err := botInstance.Router().Handle(makeCtx("guard:sim_timeout")); err != nil {
		t.Fatalf("sim_timeout failed: %v", err)
	}

	// 5. Test onBotAdded event
	ctxAdded := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeBotAdded,
		ChatID:     -100200300,
	})
	if err := botInstance.Router().Handle(ctxAdded); err != nil {
		t.Fatalf("bot added handler failed: %v", err)
	}
}
