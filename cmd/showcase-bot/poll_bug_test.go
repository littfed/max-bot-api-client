package main

import (
	"context"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// TestPollCallbackAutoRegisterAndVoting verifies that incoming callbacks from channels
// with unknown poll IDs are gracefully auto-registered and the vote is properly recorded
// without silent failures.
func TestPollCallbackAutoRegisterAndVoting(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	appStore, err := store.New("")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())

	cfg := ChannelScenarioConfig{
		Client:     nil,
		Store:      appStore,
		MiniAppURL: "https://combot.littfed.ru/maxapp",
	}
	RegisterChannelsScenario(botInstance, hood, cfg)

	// User clicks on button with payload "poll:live_poll_1:2"
	ctx := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Callback: &maxbot.Callback{
			CallbackID: "cb_poll_repro",
			Payload:    "poll:live_poll_1:2",
			User:       &maxbot.User{UserID: 396393759},
		},
		Message: &maxbot.Message{
			Recipient: &maxbot.MessageRecipient{ChatID: -78368186601503},
			Body: maxbot.MessageBody{
				MID:  "mid.ffffb8b97c136fe101a1ee6e75d228b4",
				Text: "📊 Опрос сообщества...",
			},
		},
	})

	err = botInstance.Router().Handle(ctx)
	if err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}

	// ASSERTION: The vote MUST be recorded in the store and the poll MUST exist!
	poll, ok := appStore.GetPoll("live_poll_1")
	if !ok {
		t.Fatalf("BUG REPRODUCED: poll 'live_poll_1' not found in store, voting failed silently!")
	}
	if len(poll.Votes) != 1 {
		t.Fatalf("BUG REPRODUCED: vote was not recorded, got %d votes", len(poll.Votes))
	}
}
