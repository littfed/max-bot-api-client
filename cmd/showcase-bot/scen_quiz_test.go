package main

import (
	"context"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestQuizScenarioWorkflow(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())
	RegisterQuizScenario(botInstance, hood)

	userID := int64(333)
	chatID := int64(444)

	makeCtx := func(payload string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: ""},
			},
			Callback: &maxbot.Callback{
				CallbackID: "cb_quiz",
				Payload:    payload,
				User:       &maxbot.User{UserID: userID},
			},
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	// 1. Start quiz
	if err := botInstance.Router().Handle(makeCtx("scen:quiz")); err != nil {
		t.Fatalf("start quiz failed: %v", err)
	}

	// 2. Answer question 1 correctly (idx 1)
	if err := botInstance.Router().Handle(makeCtx("quiz:ans:0:1")); err != nil {
		t.Fatalf("answer q1 failed: %v", err)
	}

	// 3. Answer question 2 correctly (idx 1)
	if err := botInstance.Router().Handle(makeCtx("quiz:ans:1:1")); err != nil {
		t.Fatalf("answer q2 failed: %v", err)
	}

	// 4. Answer question 3 incorrectly (idx 0 instead of 2)
	if err := botInstance.Router().Handle(makeCtx("quiz:ans:2:0")); err != nil {
		t.Fatalf("answer q3 failed: %v", err)
	}

	// Quiz completed and session reset
	key := fsm.SessionKey{UserID: userID, ChatID: chatID}
	st, _ := storage.GetState(context.Background(), key)
	if st != fsm.StateEmpty {
		t.Fatalf("expected state empty after quiz completion, got %s", st)
	}
}
