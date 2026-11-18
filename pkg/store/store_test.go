package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestStorePersistenceAndOperations(t *testing.T) {
	tempDir := t.TempDir()
	storeFile := filepath.Join(tempDir, "test_store.json")

	// 1. Initialize store
	s, err := New(storeFile)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Verify seeded channels
	chans := s.ListChannels()
	if len(chans) < 2 {
		t.Errorf("expected at least 2 seeded channels, got %d", len(chans))
	}
	testCh, ok := s.GetChannel(-78368186601503)
	if !ok || testCh.Title != "Тест" {
		t.Errorf("expected channel 'Тест' seeded, got %+v", testCh)
	}

	// 2. Order Operations
	ord := Order{
		ID:           "WEB-10001",
		UserID:       396393759,
		UserName:     "Федор",
		Items:        []OrderItem{{ID: "merch_1", Title: "Футболка", Price: 1990, Quantity: 1}},
		Total:        1990,
		DeliveryType: "pickup",
		Status:       "accepted",
	}
	if err := s.CreateOrder(ord); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	retrieved, ok := s.GetOrder("WEB-10001")
	if !ok || retrieved.Total != 1990 {
		t.Errorf("expected order retrieved with total 1990, got %+v", retrieved)
	}
	userOrders := s.ListOrdersByUser(396393759)
	if len(userOrders) != 1 {
		t.Errorf("expected 1 order for user, got %d", len(userOrders))
	}

	// 3. Reactions Operations
	postMID := "mid.channel.post.123"
	counts, err := s.ToggleReaction(postMID, -78368186601503, 396393759, "👍")
	if err != nil {
		t.Fatalf("failed toggle reaction: %v", err)
	}
	if counts["👍"] != 1 {
		t.Errorf("expected 👍 count 1, got %d", counts["👍"])
	}

	// User switches to 🔥
	counts, err = s.ToggleReaction(postMID, -78368186601503, 396393759, "🔥")
	if err != nil {
		t.Fatalf("failed toggle reaction switch: %v", err)
	}
	if counts["👍"] != 0 || counts["🔥"] != 1 {
		t.Errorf("expected 👍 0 and 🔥 1, got %+v", counts)
	}

	// User clicks 🔥 again to unvote
	counts, err = s.ToggleReaction(postMID, -78368186601503, 396393759, "🔥")
	if err != nil {
		t.Fatalf("failed unvote reaction: %v", err)
	}
	if counts["🔥"] != 0 {
		t.Errorf("expected 🔥 0 after unvote, got %+v", counts)
	}

	// 4. Poll Operations
	poll := Poll{
		ID:       "poll_test_1",
		ChatID:   -78368186601503,
		Question: "Какой фреймворк лучше?",
		Options:  []string{"Go net/http", "Fiber", "Gin"},
	}
	if err := s.CreatePoll(poll); err != nil {
		t.Fatalf("failed to create poll: %v", err)
	}
	countsP, percents, err := s.VotePoll("poll_test_1", 101, 0)
	if err != nil {
		t.Fatalf("failed vote: %v", err)
	}
	if countsP[0] != 1 || percents[0] != 100 {
		t.Errorf("expected 100%% for opt 0, got counts=%v percents=%v", countsP, percents)
	}
	countsP, percents, err = s.VotePoll("poll_test_1", 102, 1)
	if err != nil {
		t.Fatalf("failed vote 2: %v", err)
	}
	if percents[0] != 50 || percents[1] != 50 {
		t.Errorf("expected 50%%/50%%, got %v", percents)
	}

	// 5. Giveaway Operations
	gw := Giveaway{
		ID:              "gw_1",
		ChatID:          -78368186601503,
		Title:           "Розыгрыш худи",
		PromoCode:       "PROMO-MAX-2026",
		DiscountPercent: 20,
	}
	if err := s.CreateGiveaway(gw); err != nil {
		t.Fatalf("failed create giveaway: %v", err)
	}
	if err := s.AddGiveawayParticipant("gw_1", 396393759, "Федор"); err != nil {
		t.Fatalf("failed add participant: %v", err)
	}
	winners, err := s.DrawGiveawayWinners("gw_1", 1)
	if err != nil || len(winners) != 1 || winners[0] != 396393759 {
		t.Errorf("expected winner 396393759, got %v err %v", winners, err)
	}

	// 6. Test File Reload & Persistence
	s2, err := New(storeFile)
	if err != nil {
		t.Fatalf("failed to reload store from file: %v", err)
	}
	reloadedOrd, ok := s2.GetOrder("WEB-10001")
	if !ok || reloadedOrd.Total != 1990 {
		t.Errorf("expected persisted order in reloaded store, got %+v", reloadedOrd)
	}
	reloadedPoll, ok := s2.GetPoll("poll_test_1")
	if !ok || len(reloadedPoll.Votes) != 2 {
		t.Errorf("expected persisted poll votes, got %+v", reloadedPoll)
	}
}

func TestStoreConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	storeFile := filepath.Join(tempDir, "concurrent_store.json")

	s, err := New(storeFile)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	const goroutines = 20
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				uid := int64(gid*1000 + i)
				_, _ = s.ToggleReaction("mid.concurrent", -78368186601503, uid, "👍")
				_ = s.CreateOrder(Order{
					ID:     os.Getenv("NONEXISTENT") + string(rune(uid)),
					UserID: uid,
					Total:  100,
				})
			}
		}(g)
	}

	wg.Wait()

	rxs, ok := s.GetReactions("mid.concurrent")
	if !ok || rxs["👍"] != goroutines*iterations {
		t.Errorf("expected %d reactions, got %d", goroutines*iterations, rxs["👍"])
	}
}
