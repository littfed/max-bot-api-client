package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLStoreOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_store.db")

	s, err := NewFromEnv("file:"+dbPath, "")
	if err != nil {
		t.Fatalf("failed to create SQL store: %v", err)
	}
	defer s.Close()

	if s.SQL() == nil {
		t.Fatalf("expected SQL backend to be initialized")
	}

	// 1. Channel operations & default seeds
	chans := s.ListChannels()
	if len(chans) < 2 {
		t.Fatalf("expected at least 2 seeded channels, got %d", len(chans))
	}
	ch, ok := s.GetChannel(-78368186601503)
	if !ok || ch.Title != "Тест" {
		t.Fatalf("expected seeded channel 'Тест', got %+v", ch)
	}

	// 2. Order operations
	ord := Order{
		ID:           "ORD-SQL-001",
		UserID:       12345,
		UserName:     "Тестер",
		Items:        []OrderItem{{ID: "srv_turnkey", Title: "MAX-бот", Price: 50000, Quantity: 1}},
		Total:        50000,
		DeliveryType: "consultation",
		Status:       "accepted",
		CreatedAt:    time.Now(),
	}
	if err := s.CreateOrder(ord); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	retrievedOrd, ok := s.GetOrder("ORD-SQL-001")
	if !ok || retrievedOrd.Total != 50000 || len(retrievedOrd.Items) != 1 {
		t.Fatalf("expected retrieved order with total 50000, got %+v", retrievedOrd)
	}

	userOrders := s.ListOrdersByUser(12345)
	if len(userOrders) != 1 {
		t.Fatalf("expected 1 order for user, got %d", len(userOrders))
	}

	if err := s.UpdateOrderStatus("ORD-SQL-001", "completed"); err != nil {
		t.Fatalf("failed to update order status: %v", err)
	}
	retrievedOrd, _ = s.GetOrder("ORD-SQL-001")
	if retrievedOrd.Status != "completed" {
		t.Fatalf("expected status completed, got %s", retrievedOrd.Status)
	}

	// 3. Reactions operations
	postMID := "mid.post.sql.1"
	counts, err := s.ToggleReaction(postMID, -78368186601503, 12345, "🔥")
	if err != nil {
		t.Fatalf("failed toggle reaction: %v", err)
	}
	if counts["🔥"] != 1 {
		t.Fatalf("expected 🔥 count 1, got %+v", counts)
	}

	// Toggle same reaction removes vote
	counts, err = s.ToggleReaction(postMID, -78368186601503, 12345, "🔥")
	if err != nil {
		t.Fatalf("failed unvote reaction: %v", err)
	}
	if counts["🔥"] != 0 {
		t.Fatalf("expected 🔥 count 0 after unvote, got %+v", counts)
	}

	// 4. Poll operations
	poll := Poll{
		ID:       "poll_sql_1",
		ChatID:   -78368186601503,
		Question: "Какой диалект SQL используем?",
		Options:  []string{"PostgreSQL", "SQLite", "Оба"},
	}
	if err := s.CreatePoll(poll); err != nil {
		t.Fatalf("failed create poll: %v", err)
	}

	countsP, percents, err := s.VotePoll("poll_sql_1", 12345, 2)
	if err != nil {
		t.Fatalf("failed vote poll: %v", err)
	}
	if countsP[2] != 1 || percents[2] != 100 {
		t.Fatalf("expected 1 vote (100%%) for option 2, got counts=%v, percents=%v", countsP, percents)
	}

	retrievedPoll, ok := s.GetPoll("poll_sql_1")
	if !ok || len(retrievedPoll.Votes) != 1 {
		t.Fatalf("expected retrieved poll with 1 vote, got %+v", retrievedPoll)
	}

	// 5. Giveaway operations
	gw := Giveaway{
		ID:              "gw_sql_1",
		ChatID:          -78368186601503,
		Title:           "SQL Merch Giveaway",
		PromoCode:       "SQL-2026",
		DiscountPercent: 25,
	}
	if err := s.CreateGiveaway(gw); err != nil {
		t.Fatalf("failed create giveaway: %v", err)
	}

	if err := s.AddGiveawayParticipant("gw_sql_1", 12345, "Тестер 1"); err != nil {
		t.Fatalf("failed add participant: %v", err)
	}
	if err := s.AddGiveawayParticipant("gw_sql_1", 67890, "Тестер 2"); err != nil {
		t.Fatalf("failed add participant 2: %v", err)
	}

	winners, err := s.DrawGiveawayWinners("gw_sql_1", 1)
	if err != nil || len(winners) != 1 {
		t.Fatalf("expected 1 winner drawn, got winners=%v, err=%v", winners, err)
	}

	// 6. Media Artifacts operations
	artifact := MediaArtifact{
		UserID:       12345,
		ArtifactType: "image",
		Title:        "Futuristic Go Gopher",
		Prompt:       "Cyberpunk gopher wearing neon sunglasses",
		MediaURL:     "https://example.com/gopher.png",
		Metadata: map[string]any{
			"model": "imagen-3",
		},
	}
	saved, err := s.SaveMediaArtifact(artifact)
	if err != nil {
		t.Fatalf("failed to save media artifact: %v", err)
	}
	if saved.ID == 0 {
		t.Fatalf("expected non-zero ID for saved artifact, got %d", saved.ID)
	}

	// Add voice transcription artifact
	_, err = s.SaveMediaArtifact(MediaArtifact{
		UserID:       12345,
		ArtifactType: "transcription",
		Title:        "Голосовая заметка",
		ContentText:  "Привет, вот отчет по развертыванию MAX бота",
	})
	if err != nil {
		t.Fatalf("failed to save transcription artifact: %v", err)
	}

	// List all artifacts for user
	allArtifacts, err := s.ListMediaArtifacts(12345, "", 10)
	if err != nil {
		t.Fatalf("failed to list artifacts: %v", err)
	}
	if len(allArtifacts) != 2 {
		t.Fatalf("expected 2 artifacts for user, got %d", len(allArtifacts))
	}

	// List only images
	imageArtifacts, err := s.ListMediaArtifacts(12345, "image", 10)
	if err != nil {
		t.Fatalf("failed to list image artifacts: %v", err)
	}
	if len(imageArtifacts) != 1 || imageArtifacts[0].ArtifactType != "image" {
		t.Fatalf("expected 1 image artifact, got %+v", imageArtifacts)
	}
}

func TestStoreFallbackToJSON(t *testing.T) {
	tempDir := t.TempDir()
	jsonPath := filepath.Join(tempDir, "fallback.json")

	// Empty database URL should fall back to JSON store
	s, err := NewFromEnv("", jsonPath)
	if err != nil {
		t.Fatalf("failed fallback init: %v", err)
	}
	if s.SQL() != nil {
		t.Fatalf("expected SQL backend to be nil on fallback")
	}

	// Verify MediaArtifacts work on JSON/memory store as well
	saved, err := s.SaveMediaArtifact(MediaArtifact{
		UserID:       999,
		ArtifactType: "document",
		Title:        "Анализ спецификации",
		ContentText:  "Документ успешно обработан",
	})
	if err != nil {
		t.Fatalf("failed save media artifact on json store: %v", err)
	}
	if saved.ID != 1 {
		t.Fatalf("expected ID 1, got %d", saved.ID)
	}

	list, err := s.ListMediaArtifacts(999, "document", 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 document listed from json store, got %d, err %v", len(list), err)
	}
}
