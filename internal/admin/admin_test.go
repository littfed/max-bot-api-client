package admin_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/admin"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
	_ "modernc.org/sqlite"
)

type mockPlatformAdapter struct {
	mu           sync.Mutex
	platformName string
	sentMessages []sentMsg
}

type sentMsg struct {
	RecipientID string
	Body        *platform.OutMessage
}

func newMockPlatformAdapter(platformName string) *mockPlatformAdapter {
	if platformName == "" {
		platformName = platform.PlatformMAX
	}
	return &mockPlatformAdapter{
		platformName: platformName,
		sentMessages: make([]sentMsg, 0),
	}
}

func (m *mockPlatformAdapter) Platform() string {
	return m.platformName
}

func (m *mockPlatformAdapter) SendMessage(ctx context.Context, recipientID string, body *platform.OutMessage) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, sentMsg{RecipientID: recipientID, Body: body})
	return "mock_mid_1", nil
}

func (m *mockPlatformAdapter) EditMessage(ctx context.Context, messageID string, body *platform.OutMessage) error {
	return nil
}

func (m *mockPlatformAdapter) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	return nil
}

func (m *mockPlatformAdapter) CheckMembership(ctx context.Context, chatID string, userID string) (bool, error) {
	return true, nil
}

func (m *mockPlatformAdapter) SendChatAction(ctx context.Context, recipientID string, action string) error {
	return nil
}

type testHarness struct {
	adminSvc *admin.Service
	store    *store.SQLStore
	guard    *core.MaintenanceGuard
	adapter  *mockPlatformAdapter
}

func setupTestHarness(t *testing.T) *testHarness {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	sqlStore, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	guard := core.NewMaintenanceGuard(sqlStore, nil)
	adapter := newMockPlatformAdapter(platform.PlatformMAX)
	svc := admin.NewService(sqlStore, guard, adapter)

	// Seed users with various roles:
	// User 1: superadmin
	// User 2: admin
	// User 999: normal user
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, active_mode_id, role, timezone, created_at, updated_at) VALUES
		(1, 'shop', 'superadmin', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(2, 'shop', 'admin', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(999, 'shop', 'user', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("failed to seed test users: %v", err)
	}

	return &testHarness{
		adminSvc: svc,
		store:    sqlStore,
		guard:    guard,
		adapter:  adapter,
	}
}

func setupTestAdminService(t *testing.T) *admin.Service {
	return setupTestHarness(t).adminSvc
}

func TestAdminService_ToggleModeWithAudit(t *testing.T) {
	h := setupTestHarness(t)
	adminSvc := h.adminSvc
	ctx := context.Background()

	// Non-admin cannot toggle
	err := adminSvc.ToggleMode(ctx, 999, "shop", false, "Техработы", "127.0.0.1")
	if err == nil {
		t.Errorf("Expected forbidden error for non-admin user!")
	}

	// Superadmin toggles
	err = adminSvc.ToggleMode(ctx, 1, "shop", false, "Техработы", "127.0.0.1")
	if err != nil {
		t.Fatalf("Admin toggle failed: %v", err)
	}

	// Verify mode was toggled in guard
	active, msg, err := h.guard.IsActive(ctx, "shop")
	if err != nil {
		t.Fatalf("guard.IsActive failed: %v", err)
	}
	if active {
		t.Errorf("expected mode 'shop' to be inactive")
	}
	if msg != "Техработы" {
		t.Errorf("expected maintenance message 'Техработы', got %q", msg)
	}

	// Verify audit log recorded
	logs, err := adminSvc.GetAuditLogs(ctx, 1, 10)
	if err != nil || len(logs) == 0 {
		t.Fatalf("Expected audit log for admin action, got %v", logs)
	}
	if logs[0].Action != "toggle_maintenance" {
		t.Errorf("Expected action toggle_maintenance, got %s", logs[0].Action)
	}
	if logs[0].AdminID != 1 {
		t.Errorf("Expected admin_id 1, got %d", logs[0].AdminID)
	}
	if logs[0].TargetMode != "shop" {
		t.Errorf("Expected target_mode shop, got %s", logs[0].TargetMode)
	}
	if logs[0].IP != "127.0.0.1" {
		t.Errorf("Expected IP 127.0.0.1, got %s", logs[0].IP)
	}
}

func TestAdminService_RBAC_ForbiddenForNonAdmin(t *testing.T) {
	h := setupTestHarness(t)
	svc := h.adminSvc
	ctx := context.Background()

	nonAdminID := int64(999)

	if err := svc.ToggleMode(ctx, nonAdminID, "shop", true, "", "127.0.0.1"); err != admin.ErrForbidden {
		t.Errorf("ToggleMode expected ErrForbidden, got %v", err)
	}

	if _, err := svc.ListLeads(ctx, nonAdminID, "new"); err != admin.ErrForbidden {
		t.Errorf("ListLeads expected ErrForbidden, got %v", err)
	}

	if err := svc.UpdateLeadStatus(ctx, nonAdminID, 1, "closed", "127.0.0.1"); err != admin.ErrForbidden {
		t.Errorf("UpdateLeadStatus expected ErrForbidden, got %v", err)
	}

	if _, err := svc.ListTickets(ctx, nonAdminID, "open"); err != admin.ErrForbidden {
		t.Errorf("ListTickets expected ErrForbidden, got %v", err)
	}

	if err := svc.ReplyTicket(ctx, nonAdminID, 1, "test message", "127.0.0.1"); err != admin.ErrForbidden {
		t.Errorf("ReplyTicket expected ErrForbidden, got %v", err)
	}

	if _, err := svc.GetAuditLogs(ctx, nonAdminID, 10); err != admin.ErrForbidden {
		t.Errorf("GetAuditLogs expected ErrForbidden, got %v", err)
	}

	// Unknown user should also be forbidden
	if err := svc.ToggleMode(ctx, 404040, "shop", true, "", "127.0.0.1"); err != admin.ErrForbidden {
		t.Errorf("ToggleMode with non-existent user expected ErrForbidden, got %v", err)
	}
}

func TestAdminService_LeadManagement(t *testing.T) {
	h := setupTestHarness(t)
	svc := h.adminSvc
	ctx := context.Background()

	// 1. Create test lead
	leadID, err := h.store.SaveLead(ctx, &store.Lead{
		UserID:      999,
		Platform:    "max",
		SourceMode:  "shop",
		TestedModes: json.RawMessage(`["shop"]`),
		Contact:     "+79991234567",
		Comment:     "Хочу бота для магазина",
		Status:      "new",
	})
	if err != nil {
		t.Fatalf("SaveLead failed: %v", err)
	}

	// 2. Admin lists leads
	adminID := int64(2) // role: admin
	leads, err := svc.ListLeads(ctx, adminID, "new")
	if err != nil {
		t.Fatalf("ListLeads failed: %v", err)
	}
	if len(leads) != 1 || leads[0].ID != leadID {
		t.Fatalf("expected 1 lead with id %d, got %+v", leadID, leads)
	}

	// 3. Admin updates lead status
	err = svc.UpdateLeadStatus(ctx, adminID, leadID, "in_progress", "192.168.1.1")
	if err != nil {
		t.Fatalf("UpdateLeadStatus failed: %v", err)
	}

	// Verify status updated in DB
	leadsNew, err := svc.ListLeads(ctx, adminID, "new")
	if err != nil {
		t.Fatalf("ListLeads('new') failed: %v", err)
	}
	if len(leadsNew) != 0 {
		t.Errorf("expected 0 new leads, got %d", len(leadsNew))
	}

	leadsInProgress, err := svc.ListLeads(ctx, adminID, "in_progress")
	if err != nil {
		t.Fatalf("ListLeads('in_progress') failed: %v", err)
	}
	if len(leadsInProgress) != 1 || leadsInProgress[0].Status != "in_progress" {
		t.Errorf("expected 1 lead in_progress, got %+v", leadsInProgress)
	}

	// 4. Verify audit log
	logs, err := svc.GetAuditLogs(ctx, adminID, 5)
	if err != nil || len(logs) == 0 {
		t.Fatalf("failed to retrieve audit logs: %v", err)
	}
	if logs[0].Action != "update_lead_status" {
		t.Errorf("expected audit action update_lead_status, got %s", logs[0].Action)
	}
	if logs[0].IP != "192.168.1.1" {
		t.Errorf("expected IP 192.168.1.1, got %s", logs[0].IP)
	}
}

func TestAdminService_TicketManagementAndReply(t *testing.T) {
	h := setupTestHarness(t)
	svc := h.adminSvc
	ctx := context.Background()

	// Register user identity for test user
	user, err := h.store.ResolveUserIdentity(ctx, "max", "user_max_999", "tester", "Test User", "+79991234567")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}

	// 1. Create a support ticket
	ticket := &store.SupportTicket{
		UserID:    user.ID,
		Platform:  "max",
		ModeScope: "support",
		Subject:   "Не работает корзина",
		Status:    "open",
	}
	ticketID, err := h.store.CreateSupportTicket(ctx, ticket)
	if err != nil {
		t.Fatalf("CreateSupportTicket failed: %v", err)
	}

	// 2. Admin lists open tickets
	adminID := int64(1) // superadmin
	tickets, err := svc.ListTickets(ctx, adminID, "open")
	if err != nil {
		t.Fatalf("ListTickets failed: %v", err)
	}
	if len(tickets) != 1 || tickets[0].ID != ticketID {
		t.Fatalf("expected 1 open ticket with id %d, got %+v", ticketID, tickets)
	}

	// 3. Admin replies to ticket
	replyText := "Добрый день! Мы исправили проблему с корзиной. Попробуйте снова."
	err = svc.ReplyTicket(ctx, adminID, ticketID, replyText, "10.0.0.1")
	if err != nil {
		t.Fatalf("ReplyTicket failed: %v", err)
	}

	// Verify reply sent to user via platform adapter
	if len(h.adapter.sentMessages) != 1 {
		t.Fatalf("expected 1 sent message via adapter, got %d", len(h.adapter.sentMessages))
	}
	sent := h.adapter.sentMessages[0]
	if sent.RecipientID != "user_max_999" {
		t.Errorf("expected recipient user_max_999, got %q", sent.RecipientID)
	}
	if sent.Body == nil || sent.Body.Text != replyText {
		t.Errorf("expected message text %q, got %+v", replyText, sent.Body)
	}

	// Verify ticket status changed to 'answered'
	updatedTicket, err := h.store.GetSupportTicket(ctx, ticketID)
	if err != nil {
		t.Fatalf("GetSupportTicket failed: %v", err)
	}
	if updatedTicket.Status != "answered" {
		t.Errorf("expected ticket status 'answered', got %q", updatedTicket.Status)
	}

	// Verify support message saved in DB
	msgs, err := h.store.GetSupportMessages(ctx, ticketID)
	if err != nil {
		t.Fatalf("GetSupportMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 support message, got %d", len(msgs))
	}
	if msgs[0].SenderType != "operator" || msgs[0].MessageText != replyText {
		t.Errorf("unexpected support message: %+v", msgs[0])
	}

	// Verify audit log
	logs, err := svc.GetAuditLogs(ctx, adminID, 5)
	if err != nil || len(logs) == 0 {
		t.Fatalf("GetAuditLogs failed: %v", err)
	}
	if logs[0].Action != "reply_ticket" {
		t.Errorf("expected audit action reply_ticket, got %s", logs[0].Action)
	}
	if logs[0].IP != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", logs[0].IP)
	}
}

func TestAdminService_GetAuditLogsLimit(t *testing.T) {
	h := setupTestHarness(t)
	svc := h.adminSvc
	ctx := context.Background()
	adminID := int64(1)

	// Perform 3 actions
	_ = svc.ToggleMode(ctx, adminID, "shop", false, "msg1", "127.0.0.1")
	_ = svc.ToggleMode(ctx, adminID, "reception", false, "msg2", "127.0.0.1")
	_ = svc.ToggleMode(ctx, adminID, "support", false, "msg3", "127.0.0.1")

	// Query with limit 2
	logs, err := svc.GetAuditLogs(ctx, adminID, 2)
	if err != nil {
		t.Fatalf("GetAuditLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("expected exactly 2 logs, got %d", len(logs))
	}
	if logs[0].TargetMode != "support" {
		t.Errorf("expected latest log to be support, got %s", logs[0].TargetMode)
	}
}

func TestAdminService_MultiPlatformAdapterRouting(t *testing.T) {
	h := setupTestHarness(t)
	svc := h.adminSvc
	ctx := context.Background()

	// Register Telegram adapter
	tgAdapter := newMockPlatformAdapter(platform.PlatformTelegram)
	svc.RegisterAdapter(tgAdapter)

	// Create user with Telegram identity
	tgUser, err := h.store.ResolveUserIdentity(ctx, "telegram", "tg_user_777", "tgtester", "Telegram User", "")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}

	// Create Telegram ticket
	tgTicketID, err := h.store.CreateSupportTicket(ctx, &store.SupportTicket{
		UserID:    tgUser.ID,
		Platform:  "telegram",
		ModeScope: "support",
		Subject:   "Вопрос по Telegram",
		Status:    "open",
	})
	if err != nil {
		t.Fatalf("CreateSupportTicket failed: %v", err)
	}

	// Admin replies to Telegram ticket
	err = svc.ReplyTicket(ctx, 1, tgTicketID, "Ответ для Telegram", "127.0.0.1")
	if err != nil {
		t.Fatalf("ReplyTicket failed: %v", err)
	}

	// Verify dispatched specifically to Telegram adapter, NOT MAX adapter
	if len(tgAdapter.sentMessages) != 1 {
		t.Fatalf("expected 1 message in telegram adapter, got %d", len(tgAdapter.sentMessages))
	}
	if tgAdapter.sentMessages[0].RecipientID != "tg_user_777" {
		t.Errorf("expected tg recipient tg_user_777, got %s", tgAdapter.sentMessages[0].RecipientID)
	}
	if len(h.adapter.sentMessages) != 0 {
		t.Errorf("expected 0 messages in MAX adapter, got %d", len(h.adapter.sentMessages))
	}

	// Create ticket with unmatched platform (e.g. "discord")
	discordTicketID, err := h.store.CreateSupportTicket(ctx, &store.SupportTicket{
		UserID:    tgUser.ID,
		Platform:  "discord",
		ModeScope: "support",
		Subject:   "Discord issue",
		Status:    "open",
	})
	if err != nil {
		t.Fatalf("CreateSupportTicket for discord failed: %v", err)
	}

	// Reply to ticket without matching adapter should gracefully succeed (warning logged, ticket answered)
	err = svc.ReplyTicket(ctx, 1, discordTicketID, "Ответ для Discord", "127.0.0.1")
	if err != nil {
		t.Fatalf("ReplyTicket for unrouted platform should not fail: %v", err)
	}

	discordTicket, err := h.store.GetSupportTicket(ctx, discordTicketID)
	if err != nil || discordTicket.Status != "answered" {
		t.Errorf("expected discord ticket to be marked answered, got %v", discordTicket)
	}
}

type errAuditStore struct {
	admin.Store
}

func (s *errAuditStore) SaveAdminAuditLog(ctx context.Context, log *store.AdminAuditLog) error {
	return sql.ErrConnDone
}

func TestAdminService_AuditLogErrorDoesNotBreakAdminAction(t *testing.T) {
	h := setupTestHarness(t)
	wrappedStore := &errAuditStore{Store: h.store}
	svc := admin.NewService(wrappedStore, h.guard, h.adapter)
	ctx := context.Background()

	// ToggleMode should still succeed even if audit logging encounters an error
	err := svc.ToggleMode(ctx, 1, "shop", false, "Техработы", "127.0.0.1")
	if err != nil {
		t.Fatalf("ToggleMode failed despite audit logging error: %v", err)
	}

	// Mode should be toggled
	active, _, err := h.guard.IsActive(ctx, "shop")
	if err != nil || active {
		t.Errorf("expected mode shop to be inactive, got active=%v, err=%v", active, err)
	}
}

