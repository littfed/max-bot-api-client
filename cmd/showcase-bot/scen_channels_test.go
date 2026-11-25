package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

func TestChannelsScenarioWorkflow(t *testing.T) {
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
		AIClient:   nil,
		ChatModel:  "gemini-2.5-flash",
		MiniAppURL: "https://combot.littfed.ru/maxapp",
	}
	RegisterChannelsScenario(botInstance, hood, cfg)

	// 1. Test scen:channels menu callback
	ctxMenu := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Callback: &maxbot.Callback{
			CallbackID: "cb_chan_menu",
			Payload:    "scen:channels",
			User:       &maxbot.User{UserID: 396393759},
		},
	})
	if err := botInstance.Router().Handle(ctxMenu); err != nil {
		t.Fatalf("channels menu failed: %v", err)
	}

	// 2. Test Live Reactions Callback (rx:{mid}:{emoji})
	postMID := "mid.post.test.123"
	ctxRx := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Callback: &maxbot.Callback{
			CallbackID: "cb_rx_1",
			Payload:    "rx:" + postMID + ":👍",
			User:       &maxbot.User{UserID: 396393759},
		},
	})
	if err := botInstance.Router().Handle(ctxRx); err != nil {
		t.Fatalf("reaction handler failed: %v", err)
	}

	counts, ok := appStore.GetReactions(postMID)
	if !ok || counts["👍"] != 1 {
		t.Errorf("expected 👍 count 1 in store, got %+v", counts)
	}

	// 3. Test Poll Voting Callback (poll:{pollID}:{optIdx})
	pollID := "poll_test_456"
	_ = appStore.CreatePoll(store.Poll{
		ID:       pollID,
		ChatID:   -78368186601503,
		Question: "Тестовый вопрос?",
		Options:  []string{"Вариант 1", "Вариант 2"},
		Votes:    make(map[int64]int),
	})

	ctxPoll := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Callback: &maxbot.Callback{
			CallbackID: "cb_poll_1",
			Payload:    "poll:" + pollID + ":0",
			User:       &maxbot.User{UserID: 396393759},
		},
	})
	if err := botInstance.Router().Handle(ctxPoll); err != nil {
		t.Fatalf("poll vote failed: %v", err)
	}

	poll, ok := appStore.GetPoll(pollID)
	if !ok || len(poll.Votes) != 1 {
		t.Errorf("expected 1 vote recorded in poll, got %+v", poll)
	}

	// 4. Test Giveaway Join Callback (gw:{gwID}:join)
	gwID := "gw_test_789"
	_ = appStore.CreateGiveaway(store.Giveaway{
		ID:              gwID,
		ChatID:          -78368186601503,
		Title:           "Мерч MAX",
		PromoCode:       "PROMO-MAX-25",
		DiscountPercent: 25,
		Participants:    make(map[int64]string),
	})

	ctxGW := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Callback: &maxbot.Callback{
			CallbackID: "cb_gw_1",
			Payload:    "gw:" + gwID + ":join",
			User:       &maxbot.User{UserID: 396393759, FirstName: "Федор"},
		},
	})
	if err := botInstance.Router().Handle(ctxGW); err != nil {
		t.Fatalf("giveaway join failed: %v", err)
	}

	gw, ok := appStore.GetGiveaway(gwID)
	if !ok || len(gw.Participants) != 1 {
		t.Errorf("expected 1 participant in giveaway, got %+v", gw)
	}

	// 5. Test Progress Bar & Poll Text Renderers
	bar0 := renderBar(0)
	bar50 := renderBar(50)
	bar100 := renderBar(100)
	if !strings.Contains(bar0, "░░░░░░░░░░") || !strings.Contains(bar50, "█████░░░░░") || !strings.Contains(bar100, "██████████") {
		t.Errorf("unexpected progress bar rendering: 0=%s 50=%s 100=%s", bar0, bar50, bar100)
	}
}

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
		t.Fatalf("poll 'live_poll_1' not found in store, voting failed silently")
	}
	if len(poll.Votes) != 1 {
		t.Fatalf("vote was not recorded, got %d votes", len(poll.Votes))
	}
}
