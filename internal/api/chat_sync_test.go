package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/admin"
	"github.com/littfed/max-bot-api-client/internal/api"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/chat"
	"github.com/littfed/max-bot-api-client/internal/modules/reception"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/internal/modules/support"
	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
)

type recordingPlatformAdapter struct {
	mu       sync.Mutex
	messages []string
}

func (r *recordingPlatformAdapter) Platform() string { return "max" }
func (r *recordingPlatformAdapter) SendMessage(ctx context.Context, recipientID string, body *platform.OutMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if body != nil {
		r.messages = append(r.messages, body.Text)
	}
	return nil
}
func (r *recordingPlatformAdapter) EditMessage(ctx context.Context, messageID string, body *platform.OutMessage) error {
	return nil
}
func (r *recordingPlatformAdapter) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	return nil
}
func (r *recordingPlatformAdapter) CheckMembership(ctx context.Context, chatID string, userID string) (bool, error) {
	return true, nil
}
func (r *recordingPlatformAdapter) SendChatAction(ctx context.Context, recipientID string, action string) error {
	return nil
}

func setupTestAPIServerWithAdapter(t *testing.T, adapter platform.PlatformAdapter) (*httptest.Server, *chat.Module, *core.CompositeFSM) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	sqlStore, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	ctx := context.Background()

	// Seed users
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, active_mode_id, role, timezone, created_at, updated_at) VALUES
		(1, 'shop', 'superadmin', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(999, 'shop', 'user', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("failed to seed test users: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO user_identities (user_id, platform, platform_user_id, username, first_name) VALUES
		(999, 'max', '9999', 'testuser', 'TestUser');
	`)
	if err != nil {
		t.Fatalf("failed to seed test identities: %v", err)
	}

	lockManager := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(sqlStore, nil)
	fsm := core.NewCompositeFSM(sqlStore, nil)
	adminSvc := admin.NewService(sqlStore, guard, adapter)

	shopMod := shop.New(sqlStore)
	receptionMod := reception.New(sqlStore, lockManager)
	supportMod := support.New(sqlStore)
	chatMod := chat.New(sqlStore, nil)

	orch := core.NewOrchestrator(sqlStore, fsm, lockManager, guard)
	orch.RegisterModule(shopMod)
	orch.RegisterModule(receptionMod)
	orch.RegisterModule(supportMod)
	orch.RegisterModule(chatMod)

	router := api.NewRouter(api.RouterConfig{
		Store:           sqlStore,
		Guard:           guard,
		FSM:             fsm,
		Orchestrator:    orch,
		AdminService:    adminSvc,
		ShopModule:      shopMod,
		ReceptionModule: receptionMod,
		SupportModule:   supportMod,
		ChatModule:      chatMod,
		BotToken:        testBotToken,
		JWTSecret:       testJWTSecret,
		PlatformAdapter: adapter,
		AllowDevHeaders: true,
	})

	validMAXInitData = webapp.SignInitData(testBotToken, 9999, "TestUser", "testuser")

	ts := httptest.NewServer(router)
	t.Cleanup(func() {
		ts.Close()
	})
	return ts, chatMod, fsm
}

// 1. Symptom: Duplicate "Режим успешно переключен" notification when mode is already active
func TestBug_DuplicateModeSwitchNotification(t *testing.T) {
	recorder := &recordingPlatformAdapter{}
	ts, _, _ := setupTestAPIServerWithAdapter(t, recorder)

	// Set initial user mode to "chat"
	switchBody := bytes.NewBufferString(`{"mode_id":"chat"}`)
	req1, _ := http.NewRequest("POST", ts.URL+"/api/v1/user/mode", switchBody)
	req1.Header.Set("X-Init-Data", validMAXInitData)
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("First switch to chat failed: %v", err)
	}
	resp1.Body.Close()

	initialCount := len(recorder.messages)
	if initialCount == 0 {
		t.Fatalf("Expected at least 1 message after first switch, got 0")
	}

	// User clicks Chat in MiniApp again (mode is ALREADY "chat")
	switchBody2 := bytes.NewBufferString(`{"mode_id":"chat"}`)
	req2, _ := http.NewRequest("POST", ts.URL+"/api/v1/user/mode", switchBody2)
	req2.Header.Set("X-Init-Data", validMAXInitData)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("Second switch to chat failed: %v", err)
	}
	resp2.Body.Close()

	// Assert: When mode is already chat, it MUST NOT send another switch notification!
	if len(recorder.messages) > initialCount {
		t.Errorf("BUG REPRODUCED: Redundant notification sent when mode was already active! Messages count grew from %d to %d: %v",
			initialCount, len(recorder.messages), recorder.messages)
	}
}

// 2. Symptom: Role/tone and chat history out of sync between MiniApp and Bot Chat
func TestBug_ChatSyncBetweenMiniAppAndBot(t *testing.T) {
	recorder := &recordingPlatformAdapter{}
	ts, chatMod, fsm := setupTestAPIServerWithAdapter(t, recorder)

	// In Mini-App: User sets settings or sends message with Role 3 (Маркетолог) and Tone 4 (Юмористический)
	settingsBody := bytes.NewBufferString(`{"role_id": 3, "tone_id": 4}`)
	reqSettings, _ := http.NewRequest("POST", ts.URL+"/api/v1/chat/settings", settingsBody)
	reqSettings.Header.Set("X-Init-Data", validMAXInitData)
	reqSettings.Header.Set("Content-Type", "application/json")
	respSettings, err := http.DefaultClient.Do(reqSettings)
	if err != nil {
		t.Fatalf("POST /chat/settings failed: %v", err)
	}
	respSettings.Body.Close()
	if respSettings.StatusCode != http.StatusOK {
		t.Errorf("BUG REPRODUCED: POST /api/v1/chat/settings does not exist or returned %d", respSettings.StatusCode)
	}

	// User sends a message in Mini-App
	msgBody := bytes.NewBufferString(`{"role_id": 3, "tone_id": 4, "message": "Придумай слоган"}`)
	reqMsg, _ := http.NewRequest("POST", ts.URL+"/api/v1/chat/message", msgBody)
	reqMsg.Header.Set("X-Init-Data", validMAXInitData)
	reqMsg.Header.Set("Content-Type", "application/json")
	respMsg, err := http.DefaultClient.Do(reqMsg)
	if err != nil || respMsg.StatusCode != http.StatusOK {
		t.Fatalf("Chat message failed: code=%d, err=%v", respMsg.StatusCode, err)
	}
	respMsg.Body.Close()

	// 2a. Check if MiniApp can fetch its session state (role, tone, history)
	reqSession, _ := http.NewRequest("GET", ts.URL+"/api/v1/chat/session", nil)
	reqSession.Header.Set("X-Init-Data", validMAXInitData)
	respSession, err := http.DefaultClient.Do(reqSession)
	if err != nil {
		t.Fatalf("GET /chat/session request failed: %v", err)
	}
	defer respSession.Body.Close()

	if respSession.StatusCode != http.StatusOK {
		t.Errorf("BUG REPRODUCED: GET /api/v1/chat/session does not exist or returned %d", respSession.StatusCode)
	} else {
		var sessionResp struct {
			RoleID   int                `json:"role_id"`
			ToneID   int                `json:"tone_id"`
			Messages []chat.ChatMessage `json:"messages"`
		}
		_ = json.NewDecoder(respSession.Body).Decode(&sessionResp)
		if sessionResp.RoleID != 3 || sessionResp.ToneID != 4 {
			t.Errorf("BUG REPRODUCED: expected session role=3, tone=4, got role=%d, tone=%d", sessionResp.RoleID, sessionResp.ToneID)
		}
		if len(sessionResp.Messages) < 2 {
			t.Errorf("BUG REPRODUCED: expected at least 2 messages in session history, got %d", len(sessionResp.Messages))
		}
	}

	// 2b. Now user sends message in Bot Chat (in MAX Messenger):
	// The Bot Chat handler (HandleMessage) must respect Role 3 and Tone 4 set in Mini-App!
	u := &store.User{ID: 999}
	st, err := fsm.Get(context.Background(), 999, "chat")
	if err != nil {
		t.Fatalf("fsm.Get failed: %v", err)
	}
	res, err := chatMod.HandleMessage(context.Background(), u, st, &core.IncomingMessage{Text: "Привет"})
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	// Check if header in bot chat says Маркетолог and Юмористический (or role 3, tone 4)
	if !strings.Contains(res.Text, "Маркетолог") {
		t.Errorf("BUG REPRODUCED: Bot chat did not receive role from Mini-App! Got text: %s", res.Text)
	}
}
