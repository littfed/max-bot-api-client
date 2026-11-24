package store

import (
	"context"
	"database/sql"
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

func setupTestStore(t *testing.T) *SQLStore {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_bothub.db")
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	s, err := NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return s
}

func TestBotHubStore_UserAndModeState(t *testing.T) {
	ctx := context.Background()
	store := setupTestStore(t)

	// Resolve user identity
	user, err := store.ResolveUserIdentity(ctx, "max", "user_123", "testuser", "Test", "+79991234567")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}
	if user.ID == 0 || user.ActiveModeID != "chat" {
		t.Errorf("Unexpected user: %+v", user)
	}

	// Verify GetUser
	gotUser, err := store.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if gotUser.ID != user.ID || gotUser.ActiveModeID != "chat" {
		t.Errorf("Unexpected GetUser result: %+v", gotUser)
	}

	// Resolve again should return same user
	user2, err := store.ResolveUserIdentity(ctx, "max", "user_123", "newuser", "Test2", "+79991234567")
	if err != nil {
		t.Fatalf("ResolveUserIdentity second call failed: %v", err)
	}
	if user2.ID != user.ID {
		t.Errorf("Expected same user ID %d, got %d", user.ID, user2.ID)
	}

	// Resolve again with empty phone should preserve existing phone
	user3, err := store.ResolveUserIdentity(ctx, "max", "user_123", "newuser", "Test2", "")
	if err != nil {
		t.Fatalf("ResolveUserIdentity with empty phone failed: %v", err)
	}
	if user3.ID != user.ID {
		t.Errorf("Expected same user ID %d, got %d", user.ID, user3.ID)
	}
	var savedPhone string
	err = store.db.QueryRowContext(ctx, "SELECT phone FROM user_identities WHERE platform = 'max' AND platform_user_id = 'user_123'").Scan(&savedPhone)
	if err != nil {
		t.Fatalf("failed to query identity phone: %v", err)
	}
	if savedPhone != "+79991234567" {
		t.Errorf("expected preserved phone '+79991234567', got %q", savedPhone)
	}

	// Save and get composite mode state
	state := &UserModeState{
		UserID:      user.ID,
		ModeID:      "reception",
		CurrentStep: "select_slot",
		Payload:     []byte(`{"service_id":"srv_1"}`),
	}
	if err := store.SaveUserModeState(ctx, state); err != nil {
		t.Fatalf("SaveUserModeState failed: %v", err)
	}

	gotState, err := store.GetUserModeState(ctx, user.ID, "reception")
	if err != nil {
		t.Fatalf("GetUserModeState failed: %v", err)
	}
	if gotState.CurrentStep != "select_slot" {
		t.Errorf("Expected current_step select_slot, got %s", gotState.CurrentStep)
	}

	// Mode configs (maintenance switches)
	configs, err := store.GetModeConfigs(ctx)
	if err != nil {
		t.Fatalf("GetModeConfigs failed: %v", err)
	}
	if len(configs) < 8 {
		t.Errorf("Expected at least 8 default mode configs, got %d", len(configs))
	}

	// Toggle mode active
	if err := store.SetModeActive(ctx, "reminders", true, ""); err != nil {
		t.Fatalf("SetModeActive failed: %v", err)
	}
	configsAfter, err := store.GetModeConfigs(ctx)
	if err != nil {
		t.Fatalf("GetModeConfigs after update failed: %v", err)
	}
	var remindersActive bool
	for _, cfg := range configsAfter {
		if cfg.ModeID == "reminders" {
			remindersActive = cfg.IsActive
			break
		}
	}
	if !remindersActive {
		t.Errorf("Expected reminders mode to be active after SetModeActive")
	}

	// Leads operations
	leadID, err := store.SaveLead(ctx, &Lead{
		UserID:      user.ID,
		Platform:    "max",
		SourceMode:  "shop",
		TestedModes: []byte(`["shop", "reception"]`),
		Contact:     "+79991234567",
		Comment:     "Хочу бота для магазина",
		Status:      "new",
	})
	if err != nil {
		t.Fatalf("SaveLead failed: %v", err)
	}
	if leadID == 0 {
		t.Errorf("Expected non-zero lead ID, got %d", leadID)
	}

	leads, err := store.GetLeads(ctx, "new")
	if err != nil {
		t.Fatalf("GetLeads failed: %v", err)
	}
	if len(leads) == 0 {
		t.Errorf("Expected at least 1 lead, got 0")
	}

	// Mascot phrases
	phrases, err := store.GetMascotPhrases(ctx)
	if err != nil {
		t.Fatalf("GetMascotPhrases failed: %v", err)
	}
	if len(phrases) == 0 {
		t.Errorf("Expected seeded mascot phrases, got 0")
	}
}

func TestSQLStore_AdminAndSupportOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_admin_support.db")

	s, err := NewFromEnv("file:"+dbPath, "")
	if err != nil {
		t.Fatalf("failed to create SQL store: %v", err)
	}
	defer s.Close()

	sqlStore := s.SQL()
	ctx := context.Background()

	// 1. Resolve user identity and verify GetUserIdentity
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "user_admin_test_1", "tester1", "Test", "+79990001122")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}

	ident, err := sqlStore.GetUserIdentity(ctx, user.ID, "max")
	if err != nil {
		t.Fatalf("GetUserIdentity failed: %v", err)
	}
	if ident.PlatformUserID != "user_admin_test_1" {
		t.Errorf("expected platform user id 'user_admin_test_1', got %q", ident.PlatformUserID)
	}

	// 2. Lead operations: save and update status
	leadID, err := sqlStore.SaveLead(ctx, &Lead{
		UserID:     user.ID,
		Platform:   "max",
		SourceMode: "shop",
		Contact:    "+79990001122",
		Status:     "new",
	})
	if err != nil {
		t.Fatalf("SaveLead failed: %v", err)
	}
	if err := sqlStore.UpdateLeadStatus(ctx, leadID, "closed"); err != nil {
		t.Fatalf("UpdateLeadStatus failed: %v", err)
	}
	closedLeads, err := sqlStore.GetLeads(ctx, "closed")
	if err != nil || len(closedLeads) != 1 || closedLeads[0].ID != leadID {
		t.Fatalf("expected 1 closed lead with id %d, got %+v", leadID, closedLeads)
	}

	// 3. Support ticket operations
	ticketID, err := sqlStore.CreateSupportTicket(ctx, &SupportTicket{
		UserID:    user.ID,
		Platform:  "max",
		ModeScope: "support",
		Subject:   "Тестовый тикет",
		Status:    "open",
	})
	if err != nil {
		t.Fatalf("CreateSupportTicket failed: %v", err)
	}
	ticket, err := sqlStore.GetSupportTicket(ctx, ticketID)
	if err != nil || ticket.Subject != "Тестовый тикет" {
		t.Fatalf("GetSupportTicket failed: %v", err)
	}

	openTickets, err := sqlStore.GetSupportTickets(ctx, "open")
	if err != nil || len(openTickets) == 0 {
		t.Fatalf("GetSupportTickets('open') failed: %v", err)
	}

	if err := sqlStore.UpdateSupportTicketStatus(ctx, ticketID, "answered"); err != nil {
		t.Fatalf("UpdateSupportTicketStatus failed: %v", err)
	}
	updatedTicket, err := sqlStore.GetSupportTicket(ctx, ticketID)
	if err != nil || updatedTicket.Status != "answered" {
		t.Fatalf("expected status 'answered', got %v", updatedTicket)
	}

	// 4. Support message operations
	msg := &SupportMessage{
		TicketID:    ticketID,
		SenderType:  "operator",
		MessageText: "Здравствуйте!",
	}
	if err := sqlStore.SaveSupportMessage(ctx, msg); err != nil {
		t.Fatalf("SaveSupportMessage failed: %v", err)
	}
	msgs, err := sqlStore.GetSupportMessages(ctx, ticketID)
	if err != nil || len(msgs) != 1 || msgs[0].MessageText != "Здравствуйте!" {
		t.Fatalf("GetSupportMessages failed: %v", err)
	}

	// 5. Admin audit log operations
	auditLog := &AdminAuditLog{
		AdminID:     user.ID,
		Action:      "test_action",
		TargetMode:  "shop",
		PayloadDiff: []byte(`{"key":"value"}`),
		IP:          "127.0.0.1",
	}
	if err := sqlStore.SaveAdminAuditLog(ctx, auditLog); err != nil {
		t.Fatalf("SaveAdminAuditLog failed: %v", err)
	}
	logs, err := sqlStore.GetAdminAuditLogs(ctx, 10)
	if err != nil || len(logs) == 0 || logs[0].Action != "test_action" {
		t.Fatalf("GetAdminAuditLogs failed: %v, got %+v", err, logs)
	}
}
