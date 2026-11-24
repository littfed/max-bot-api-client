package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/admin"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/chat"
	"github.com/littfed/max-bot-api-client/internal/modules/reception"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/internal/modules/support"
	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
	_ "modernc.org/sqlite"
)

type diagPlatformAdapter struct {
	mu           sync.Mutex
	actions      []string
	edits        []string
	typingCalled bool
	typingTarget string
}

func (d *diagPlatformAdapter) Platform() string { return "max" }
func (d *diagPlatformAdapter) SendMessage(ctx context.Context, recipientID string, body *platform.OutMessage) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	mid := fmt.Sprintf("mid_%d", len(d.actions)+1)
	d.actions = append(d.actions, fmt.Sprintf("send:%s:%s", recipientID, body.Text))
	return mid, nil
}
func (d *diagPlatformAdapter) EditMessage(ctx context.Context, messageID string, body *platform.OutMessage) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.edits = append(d.edits, fmt.Sprintf("edit:%s:%s", messageID, body.Text))
	return nil
}
func (d *diagPlatformAdapter) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	return nil
}
func (d *diagPlatformAdapter) CheckMembership(ctx context.Context, chatID string, userID string) (bool, error) {
	return true, nil
}
func (d *diagPlatformAdapter) SendChatAction(ctx context.Context, recipientID string, action string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.actions = append(d.actions, fmt.Sprintf("action:%s:%s", recipientID, action))
	if action == "typing_on" || action == "typing" {
		d.typingCalled = true
		d.typingTarget = recipientID
	}
	return nil
}

// TestDiagnosingBugs reproduces the 4 user-reported bugs:
// 1. Mini-app chat messages lost after rebuild/restart
// 2. Admin mode toggle fails with error / 403 for default user or owner
// 3. Admin tickets list fails or doesn't show user messages
// 4. Bot in MAX doesn't show typing action when user sends message in mini-app
func TestDiagnosingBugs_ChatPersistence(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	sqlStore, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	ctx := context.Background()
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "396393759", "littfed", "Fedor", "")
	if err != nil {
		t.Fatalf("failed to resolve user: %v", err)
	}

	// 1. First run: user sends message in chat module
	chatMod1 := chat.New(sqlStore, nil)
	_, err = chatMod1.SendMessage(ctx, user.ID, 1, 1, "Привет! Помоги рассчитать юнит-экономику")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	s1, err := chatMod1.GetSession(ctx, user.ID)
	if err != nil || len(s1.Messages) == 0 {
		t.Fatalf("expected messages in session 1, got %v (len %d)", err, len(s1.Messages))
	}

	// 2. Simulate server restart: new chat module with fresh in-memory state
	chatMod2 := chat.New(sqlStore, nil)
	s2, err := chatMod2.GetSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetSession failed on new module: %v", err)
	}

	if len(s2.Messages) == 0 {
		t.Errorf("BUG 1 REPRODUCED: chat messages were lost after module/server restart! len=%d", len(s2.Messages))
	}
}

func TestDiagnosingBugs_AdminModeToggleAndTickets(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	sqlStore, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	ctx := context.Background()
	// Owner / Admin user
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "396393759", "littfed", "Fedor", "")
	if err != nil {
		t.Fatalf("failed to resolve user: %v", err)
	}

	adapter := &diagPlatformAdapter{}
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

	token, _ := GenerateJWT(JWTClaims{UserID: user.ID, Platform: "max"}, "secret")
	apiRouter := NewRouter(RouterConfig{
		Store:           sqlStore,
		Guard:           guard,
		FSM:             fsm,
		Orchestrator:    orch,
		AdminService:    adminSvc,
		ShopModule:      shopMod,
		ReceptionModule: receptionMod,
		SupportModule:   supportMod,
		ChatModule:      chatMod,
		JWTSecret:       "secret",
		PlatformAdapter: adapter,
		AllowDevHeaders: true,
	})

	ts := httptest.NewServer(apiRouter)
	defer ts.Close()

	// 1. Try toggling mode
	togglePayload := `{"is_active":false,"maintenance_message":"Техобслуживание"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/modes/shop/toggle", bytes.NewBufferString(togglePayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("toggle request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("BUG 2 REPRODUCED: admin toggle mode returned status %d (expected 200 OK)", resp.StatusCode)
	}

	// 2. Create support ticket as user
	ticketPayload := `{"subject":"Вопрос по заказу","message":"Не пришел чек на почту"}`
	reqTicket, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/support/tickets", bytes.NewBufferString(ticketPayload))
	reqTicket.Header.Set("Authorization", "Bearer "+token)
	reqTicket.Header.Set("Content-Type", "application/json")
	respTicket, err := http.DefaultClient.Do(reqTicket)
	if err != nil {
		t.Fatalf("create ticket failed: %v", err)
	}
	defer respTicket.Body.Close()

	// 3. Read admin tickets
	reqAdminTickets, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/tickets", nil)
	reqAdminTickets.Header.Set("Authorization", "Bearer "+token)
	respAdminTickets, err := http.DefaultClient.Do(reqAdminTickets)
	if err != nil {
		t.Fatalf("get admin tickets failed: %v", err)
	}
	defer respAdminTickets.Body.Close()

	if respAdminTickets.StatusCode != http.StatusOK {
		t.Errorf("BUG 3A REPRODUCED: admin get tickets returned status %d (expected 200 OK)", respAdminTickets.StatusCode)
	} else {
		var ticketsResp struct {
			Tickets []struct {
				ID      int64  `json:"id"`
				Subject string `json:"subject"`
				Message string `json:"message"`
			} `json:"tickets"`
		}
		_ = json.NewDecoder(respAdminTickets.Body).Decode(&ticketsResp)
		if len(ticketsResp.Tickets) == 0 {
			t.Errorf("BUG 3B REPRODUCED: tickets list is empty")
		} else if ticketsResp.Tickets[0].Message == "" {
			t.Errorf("BUG 3C REPRODUCED: ticket message is empty, user's question not shown to admin")
		}
	}
}

func TestDiagnosingBugs_BotTypingIndicatorFromMiniApp(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	sqlStore, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	ctx := context.Background()
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "396393759", "littfed", "Fedor", "")
	if err != nil {
		t.Fatalf("failed to resolve user: %v", err)
	}

	adapter := &diagPlatformAdapter{}
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

	token, _ := GenerateJWT(JWTClaims{UserID: user.ID, Platform: "max"}, "secret")
	apiRouter := NewRouter(RouterConfig{
		Store:           sqlStore,
		Guard:           guard,
		FSM:             fsm,
		Orchestrator:    orch,
		AdminService:    adminSvc,
		ShopModule:      shopMod,
		ReceptionModule: receptionMod,
		SupportModule:   supportMod,
		ChatModule:      chatMod,
		JWTSecret:       "secret",
		PlatformAdapter: adapter,
		AllowDevHeaders: true,
	})

	ts := httptest.NewServer(apiRouter)
	defer ts.Close()

	// Send message in chat from mini app
	msgPayload := `{"role_id":1,"tone_id":1,"message":"Привет, как дела?"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/chat/message", bytes.NewBufferString(msgPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("chat message failed: %v", err)
	}
	defer resp.Body.Close()

	adapter.mu.Lock()
	typingCalled := adapter.typingCalled
	adapter.mu.Unlock()

	if !typingCalled {
		t.Errorf("BUG 4 REPRODUCED: bot did not trigger typing action in messenger when mini-app sent chat message")
	}
}

func TestDiagnosingBugs_AdminModeToggleAndID(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	sqlStore, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	ctx := context.Background()
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "396393759", "littfed", "Fedor", "")
	if err != nil {
		t.Fatalf("failed to resolve user: %v", err)
	}

	adapter := &diagPlatformAdapter{}
	guard := core.NewMaintenanceGuard(sqlStore, nil)
	fsm := core.NewCompositeFSM(sqlStore, nil)
	adminSvc := admin.NewService(sqlStore, guard, adapter)

	token, _ := GenerateJWT(JWTClaims{UserID: user.ID, Role: "admin", Platform: "max"}, "secret")
	apiRouter := NewRouter(RouterConfig{
		Store:           sqlStore,
		Guard:           guard,
		FSM:             fsm,
		AdminService:    adminSvc,
		JWTSecret:       "secret",
		PlatformAdapter: adapter,
		AllowDevHeaders: true,
	})

	ts := httptest.NewServer(apiRouter)
	defer ts.Close()

	// 1. Check GET /api/v1/admin/modes returns id field
	reqModes, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/modes", nil)
	reqModes.Header.Set("Authorization", "Bearer "+token)
	respModes, err := http.DefaultClient.Do(reqModes)
	if err != nil {
		t.Fatalf("GET admin/modes failed: %v", err)
	}
	defer respModes.Body.Close()

	if respModes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", respModes.StatusCode)
	}

	var modesResp struct {
		Modes []struct {
			ID                 string `json:"id"`
			ModeID             string `json:"mode_id"`
			Title              string `json:"title"`
			IsActive           bool   `json:"is_active"`
			MaintenanceMessage string `json:"maintenance_message"`
		} `json:"modes"`
	}
	if err := json.NewDecoder(respModes.Body).Decode(&modesResp); err != nil {
		t.Fatalf("decode admin modes failed: %v", err)
	}

	if len(modesResp.Modes) == 0 {
		t.Fatalf("expected mode configs, got 0")
	}

	foundSupport := false
	for _, m := range modesResp.Modes {
		if m.ID == "" {
			t.Errorf("BUG DETECTED: mode item has empty 'id' field, title=%s", m.Title)
		}
		if m.ID == "support" {
			foundSupport = true
			if !m.IsActive {
				t.Errorf("expected support mode to initially be active")
			}
		}
	}
	if !foundSupport {
		t.Fatalf("support mode not found in modes list")
	}

	// 2. Toggle support mode to inactive
	togglePayload := `{"is_active":false,"maintenance_message":"Техобслуживание поддержки"}`
	reqToggle, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/modes/support/toggle", bytes.NewBufferString(togglePayload))
	reqToggle.Header.Set("Authorization", "Bearer "+token)
	reqToggle.Header.Set("Content-Type", "application/json")
	respToggle, err := http.DefaultClient.Do(reqToggle)
	if err != nil {
		t.Fatalf("toggle request failed: %v", err)
	}
	defer respToggle.Body.Close()

	if respToggle.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on toggle, got %d", respToggle.StatusCode)
	}

	// 3. Verify in DB and via API that support is now inactive and shop is still active
	reqModes2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/modes", nil)
	reqModes2.Header.Set("Authorization", "Bearer "+token)
	respModes2, err := http.DefaultClient.Do(reqModes2)
	if err != nil {
		t.Fatalf("GET admin/modes #2 failed: %v", err)
	}
	defer respModes2.Body.Close()

	var modesResp2 struct {
		Modes []struct {
			ID       string `json:"id"`
			IsActive bool   `json:"is_active"`
		} `json:"modes"`
	}
	_ = json.NewDecoder(respModes2.Body).Decode(&modesResp2)

	for _, m := range modesResp2.Modes {
		if m.ID == "support" && m.IsActive {
			t.Errorf("BUG DETECTED: support mode was not disabled in DB/API")
		}
		if m.ID == "shop" && !m.IsActive {
			t.Errorf("BUG DETECTED: shop mode was accidentally disabled when toggling support!")
		}
	}
}

func TestDiagnosingBugs_ModeSwitchInPlaceEdit(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	sqlStore, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	ctx := context.Background()
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "396393759", "littfed", "Fedor", "")
	if err != nil {
		t.Fatalf("failed to resolve user: %v", err)
	}

	adapter := &diagPlatformAdapter{}
	guard := core.NewMaintenanceGuard(sqlStore, nil)
	fsm := core.NewCompositeFSM(sqlStore, nil)

	token, _ := GenerateJWT(JWTClaims{UserID: user.ID, Platform: "max"}, "secret")
	apiRouter := NewRouter(RouterConfig{
		Store:           sqlStore,
		Guard:           guard,
		FSM:             fsm,
		JWTSecret:       "secret",
		PlatformAdapter: adapter,
		AllowDevHeaders: true,
	})

	ts := httptest.NewServer(apiRouter)
	defer ts.Close()

	// 1. Switch to 'support' mode for the first time
	req1, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/user/mode", bytes.NewBufferString(`{"mode_id":"support"}`))
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("mode switch 1 failed: %v", err)
	}
	defer resp1.Body.Close()

	adapter.mu.Lock()
	actionCount1 := len(adapter.actions)
	editCount1 := len(adapter.edits)
	adapter.mu.Unlock()

	if actionCount1 != 1 {
		t.Errorf("expected 1 SendMessage call on first mode switch, got %d", actionCount1)
	}
	if editCount1 != 0 {
		t.Errorf("expected 0 EditMessage calls on first mode switch, got %d", editCount1)
	}

	// 2. Switch to 'chat' mode without intervening messages
	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/user/mode", bytes.NewBufferString(`{"mode_id":"chat"}`))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("mode switch 2 failed: %v", err)
	}
	defer resp2.Body.Close()

	adapter.mu.Lock()
	actionCount2 := len(adapter.actions)
	editCount2 := len(adapter.edits)
	lastEdit := ""
	if editCount2 > 0 {
		lastEdit = adapter.edits[editCount2-1]
	}
	adapter.mu.Unlock()

	// Verify that SendMessage was NOT called again, but EditMessage WAS called!
	if actionCount2 != 1 {
		t.Errorf("BUG DETECTED: SendMessage was called again (%d times), duplicate message sent instead of editing previous!", actionCount2)
	}
	if editCount2 != 1 {
		t.Errorf("expected 1 EditMessage call, got %d", editCount2)
	}
	if !strings.Contains(lastEdit, "chat") {
		t.Errorf("expected edited message text to mention 'chat', got %s", lastEdit)
	}
}
