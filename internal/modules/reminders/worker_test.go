package reminders

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type mockPlatformAdapter struct {
	mu           sync.Mutex
	platform     string
	sentMessages []mockSent
}

type mockSent struct {
	RecipientID string
	Msg         *platform.OutMessage
}

func newMockPlatformAdapter(plat string) *mockPlatformAdapter {
	return &mockPlatformAdapter{
		platform:     plat,
		sentMessages: make([]mockSent, 0),
	}
}

func (m *mockPlatformAdapter) Platform() string {
	return m.platform
}

func (m *mockPlatformAdapter) SendMessage(ctx context.Context, recipientID string, body *platform.OutMessage) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, mockSent{
		RecipientID: recipientID,
		Msg:         body,
	})
	return "mid_test_123", nil
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

func TestWorker_ProcessPending(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "worker_test.db")

	s, err := store.NewFromEnv("file:"+dbPath, "")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	sqlStore := s.SQL()
	ctx := context.Background()

	// 1. Setup user and identity
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "user_worker_1", "worker_user", "WorkerTester", "+79997776655")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}

	service := NewService(sqlStore)
	mockAdapter := newMockPlatformAdapter("max")
	adapters := map[string]platform.PlatformAdapter{
		"max": mockAdapter,
	}

	worker := NewWorker(service, sqlStore, adapters, 100*time.Millisecond, 50)

	// 2. Create reminder with remind_at in the past
	rem := &store.Reminder{
		UserID:         user.ID,
		Title:          "Проверить логи продакшена",
		Description:    "Dokploy container status",
		RemindAt:       time.Now().UTC().Add(-1 * time.Minute),
		RecurrenceRule: "none",
		Status:         "pending",
		SourceType:     "user",
		ABVariant:      "A",
	}
	remID, err := sqlStore.CreateReminder(ctx, rem)
	if err != nil {
		t.Fatalf("CreateReminder failed: %v", err)
	}

	// 3. Run ProcessPendingOnce
	processedCount, err := worker.ProcessPendingOnce(ctx)
	if err != nil {
		t.Fatalf("ProcessPendingOnce failed: %v", err)
	}
	if processedCount != 1 {
		t.Fatalf("expected 1 processed reminder, got %d", processedCount)
	}

	// 4. Verify message sent via adapter
	mockAdapter.mu.Lock()
	if len(mockAdapter.sentMessages) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(mockAdapter.sentMessages))
	}
	sent := mockAdapter.sentMessages[0]
	mockAdapter.mu.Unlock()

	if sent.RecipientID != "user_worker_1" {
		t.Errorf("expected recipient 'user_worker_1', got %q", sent.RecipientID)
	}
	if !strings.Contains(sent.Msg.Text, "Проверить логи продакшена") {
		t.Errorf("expected message text to contain title, got: %s", sent.Msg.Text)
	}
	if len(sent.Msg.Buttons) != 2 {
		t.Fatalf("expected 2 button rows, got %d", len(sent.Msg.Buttons))
	}

	// 5. Verify reminder status changed to 'sent'
	updatedRem, err := sqlStore.GetReminderByID(ctx, remID)
	if err != nil {
		t.Fatalf("GetReminderByID failed: %v", err)
	}
	if updatedRem.Status != "sent" {
		t.Errorf("expected status 'sent', got %q", updatedRem.Status)
	}
	if updatedRem.LastSentAt == nil {
		t.Errorf("expected LastSentAt to be set")
	}

	// 6. Running ProcessPendingOnce again should process 0 (SKIP LOCKED / status != pending)
	secondRun, err := worker.ProcessPendingOnce(ctx)
	if err != nil {
		t.Fatalf("second ProcessPendingOnce failed: %v", err)
	}
	if secondRun != 0 {
		t.Errorf("expected 0 processed on second run, got %d", secondRun)
	}
}
