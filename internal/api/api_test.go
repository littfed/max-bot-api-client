package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/internal/admin"
	"github.com/littfed/max-bot-api-client/internal/api"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/chat"
	"github.com/littfed/max-bot-api-client/internal/modules/reception"
	"github.com/littfed/max-bot-api-client/internal/modules/reminders"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/internal/modules/support"
	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
	_ "modernc.org/sqlite"
)

const (
	testBotToken  = "test_bot_token_secret_123"
	testJWTSecret = "test_jwt_secret_456"
)

func testMAXInitData() string {
	return webapp.SignInitData(testBotToken, 9999, "TestUser", "testuser")
}

func testAdminInitData() string {
	return webapp.SignInitData(testBotToken, 1001, "AdminUser", "adminuser")
}

type mockPlatformAdapter struct {
	platformName string
}

func (m *mockPlatformAdapter) Platform() string {
	return m.platformName
}

func (m *mockPlatformAdapter) SendMessage(ctx context.Context, recipientID string, body *platform.OutMessage) (string, error) {
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

func setupTestAPIServer(t *testing.T) *httptest.Server {
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
		(2, 'shop', 'admin', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(999, 'shop', 'user', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("failed to seed test users: %v", err)
	}

	// Seed user identities
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_identities (user_id, platform, platform_user_id, username, first_name) VALUES
		(1, 'max', '1001', 'adminuser', 'AdminUser'),
		(999, 'max', '9999', 'testuser', 'TestUser');
	`)
	if err != nil {
		t.Fatalf("failed to seed test identities: %v", err)
	}

	lockManager := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(sqlStore, nil)
	fsm := core.NewCompositeFSM(sqlStore, nil)
	adapter := &mockPlatformAdapter{platformName: platform.PlatformMAX}
	adminSvc := admin.NewService(sqlStore, guard, adapter)

	shopMod := shop.New(sqlStore)
	receptionMod := reception.New(sqlStore, lockManager)
	supportMod := support.New(sqlStore)
	chatMod := chat.New(sqlStore, nil)
	remindersSvc := reminders.NewService(sqlStore)
	remindersMod := reminders.New(remindersSvc, sqlStore)

	orch := core.NewOrchestrator(sqlStore, fsm, lockManager, guard)
	orch.RegisterModule(shopMod)
	orch.RegisterModule(receptionMod)
	orch.RegisterModule(supportMod)
	orch.RegisterModule(chatMod)
	orch.RegisterModule(remindersMod)

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
		RemindersModule: remindersMod,
		BotToken:        testBotToken,
		JWTSecret:       testJWTSecret,
		PlatformAdapter: adapter,
		AllowedOrigins:  []string{"https://combot.littfed.ru"},
		AllowDevHeaders: true,
	})

	ts := httptest.NewServer(router)
	t.Cleanup(func() {
		ts.Close()
	})
	return ts
}

func TestAPI_BootstrapAndModeSwitch(t *testing.T) {
	ts := setupTestAPIServer(t)

	// GET /api/v1/bootstrap
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/bootstrap", nil)
	req.Header.Set("X-Init-Data", testMAXInitData())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", resp.StatusCode)
	}

	var bootstrapResp struct {
		ActiveMode string `json:"active_mode"`
		Modes      []struct {
			ID       string `json:"id"`
			IsActive bool   `json:"is_active"`
		} `json:"modes"`
		Mascot struct {
			Phrase string `json:"phrase"`
		} `json:"mascot"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&bootstrapResp)
	if len(bootstrapResp.Modes) < 8 {
		t.Errorf("Expected at least 8 modes in bootstrap, got %d", len(bootstrapResp.Modes))
	}
	if bootstrapResp.ActiveMode != "shop" {
		t.Errorf("Expected active_mode 'shop', got %s", bootstrapResp.ActiveMode)
	}
	if bootstrapResp.Mascot.Phrase == "" {
		t.Errorf("Expected mascot phrase, got empty string")
	}

	// POST /api/v1/user/mode
	switchBody := bytes.NewBufferString(`{"mode_id":"reception"}`)
	req2, _ := http.NewRequest("POST", ts.URL+"/api/v1/user/mode", switchBody)
	req2.Header.Set("X-Init-Data", testMAXInitData())
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", resp2.StatusCode)
	}

	var switchResp struct {
		Success    bool   `json:"success"`
		ActiveMode string `json:"active_mode"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&switchResp)
	if !switchResp.Success || switchResp.ActiveMode != "reception" {
		t.Errorf("Expected success mode switch to 'reception', got %+v", switchResp)
	}

	// Verify mode changed via second bootstrap call
	req3, _ := http.NewRequest("GET", ts.URL+"/api/v1/bootstrap", nil)
	req3.Header.Set("X-Init-Data", testMAXInitData())
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", resp3.StatusCode)
	}

	var boot2 struct {
		ActiveMode string `json:"active_mode"`
	}
	_ = json.NewDecoder(resp3.Body).Decode(&boot2)
	if boot2.ActiveMode != "reception" {
		t.Errorf("Expected updated active_mode 'reception', got %s", boot2.ActiveMode)
	}
}

func TestAPI_AuthMiddleware(t *testing.T) {
	ts := setupTestAPIServer(t)

	// 1. Missing auth
	req1, _ := http.NewRequest("GET", ts.URL+"/api/v1/bootstrap", nil)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for missing auth, got %d", resp1.StatusCode)
	}

	// 2. Invalid InitData signature
	req2, _ := http.NewRequest("GET", ts.URL+"/api/v1/bootstrap", nil)
	req2.Header.Set("X-Init-Data", "user=%7B%22id%22%3A9999%7D&auth_date=12345678&hash=invalidhash")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid initData signature, got %d", resp2.StatusCode)
	}

	// 3. Dev Header Auth (X-Dev-User-ID) when AllowDevHeaders is true
	req3, _ := http.NewRequest("GET", ts.URL+"/api/v1/bootstrap", nil)
	req3.Header.Set("X-Dev-User-ID", "999")
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for X-Dev-User-ID, got %d", resp3.StatusCode)
	}

	// 4. JWT Bearer Auth with platform propagation
	jwtToken, err := api.GenerateJWT(api.JWTClaims{
		UserID:    999,
		Role:      "user",
		Platform:  "telegram",
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, testJWTSecret)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	req4, _ := http.NewRequest("GET", ts.URL+"/api/v1/bootstrap", nil)
	req4.Header.Set("Authorization", "Bearer "+jwtToken)
	resp4, err := http.DefaultClient.Do(req4)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", resp4.StatusCode)
	}

	// 5. Default router without AllowDevHeaders rejects X-Dev-User-ID
	prodRouter := api.NewRouter(api.RouterConfig{
		BotToken:  testBotToken,
		JWTSecret: testJWTSecret,
		// AllowDevHeaders defaults to false
	})
	tsProd := httptest.NewServer(prodRouter)
	defer tsProd.Close()

	reqProd, _ := http.NewRequest("GET", tsProd.URL+"/api/v1/bootstrap", nil)
	reqProd.Header.Set("X-Dev-User-ID", "999")
	respProd, err := http.DefaultClient.Do(reqProd)
	if err != nil {
		t.Fatalf("prod request failed: %v", err)
	}
	defer respProd.Body.Close()
	if respProd.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized when AllowDevHeaders=false, got %d", respProd.StatusCode)
	}
}

func TestAPI_MascotTapAndLead(t *testing.T) {
	ts := setupTestAPIServer(t)

	// GET /api/v1/mascot/tap
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/mascot/tap", nil)
	req.Header.Set("X-Init-Data", testMAXInitData())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", resp.StatusCode)
	}

	var tapResp struct {
		Phrase   string `json:"phrase"`
		Priority int    `json:"priority"`
		Emotion  string `json:"emotion"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tapResp)
	if tapResp.Phrase == "" {
		t.Errorf("Expected non-empty phrase from mascot tap")
	}

	// POST /api/v1/lead
	leadBody := bytes.NewBufferString(`{
		"contact": "+79991234567",
		"comment": "Хочу бота для отеля",
		"source_mode": "reception",
		"tested_modes": ["shop", "reception"]
	}`)
	reqLead, _ := http.NewRequest("POST", ts.URL+"/api/v1/lead", leadBody)
	reqLead.Header.Set("X-Init-Data", testMAXInitData())
	reqLead.Header.Set("Content-Type", "application/json")
	respLead, err := http.DefaultClient.Do(reqLead)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respLead.Body.Close()
	if respLead.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respLead.StatusCode)
	}

	var leadResp struct {
		Success bool  `json:"success"`
		LeadID  int64 `json:"lead_id"`
	}
	_ = json.NewDecoder(respLead.Body).Decode(&leadResp)
	if !leadResp.Success || leadResp.LeadID == 0 {
		t.Errorf("Expected lead creation success, got %+v", leadResp)
	}
}

func TestAPI_ShopEndpoints(t *testing.T) {
	ts := setupTestAPIServer(t)

	// GET /api/v1/shop/catalog
	reqCat, _ := http.NewRequest("GET", ts.URL+"/api/v1/shop/catalog", nil)
	reqCat.Header.Set("X-Init-Data", testMAXInitData())
	respCat, err := http.DefaultClient.Do(reqCat)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respCat.Body.Close()
	if respCat.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respCat.StatusCode)
	}

	var catResp struct {
		Products []store.Product `json:"products"`
	}
	_ = json.NewDecoder(respCat.Body).Decode(&catResp)
	if len(catResp.Products) == 0 {
		t.Errorf("Expected catalog products, got 0")
	}

	// POST /api/v1/shop/orders
	orderBody := bytes.NewBufferString(`{"product_id": "prod_guide"}`)
	reqOrder, _ := http.NewRequest("POST", ts.URL+"/api/v1/shop/orders", orderBody)
	reqOrder.Header.Set("X-Init-Data", testMAXInitData())
	reqOrder.Header.Set("Content-Type", "application/json")
	respOrder, err := http.DefaultClient.Do(reqOrder)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respOrder.Body.Close()
	if respOrder.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respOrder.StatusCode)
	}

	var orderResp struct {
		Order shop.Order `json:"order"`
	}
	_ = json.NewDecoder(respOrder.Body).Decode(&orderResp)
	if orderResp.Order.ID == "" || orderResp.Order.Status != "pending" {
		t.Errorf("Expected pending order, got %+v", orderResp)
	}

	// POST /api/v1/shop/orders/{id}/simulate
	simBody := bytes.NewBufferString(`{"action": "paid"}`)
	reqSim, _ := http.NewRequest("POST", ts.URL+"/api/v1/shop/orders/"+orderResp.Order.ID+"/simulate", simBody)
	reqSim.Header.Set("X-Init-Data", testMAXInitData())
	reqSim.Header.Set("Content-Type", "application/json")
	respSim, err := http.DefaultClient.Do(reqSim)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respSim.Body.Close()
	if respSim.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respSim.StatusCode)
	}

	var simResp struct {
		Result shop.PaymentSimulationResult `json:"result"`
	}
	_ = json.NewDecoder(respSim.Body).Decode(&simResp)
	if simResp.Result.Status != "paid" {
		t.Errorf("Expected status 'paid', got %s", simResp.Result.Status)
	}
}

func TestAPI_ReceptionEndpoints(t *testing.T) {
	ts := setupTestAPIServer(t)

	// GET /api/v1/reception/slots
	reqSlots, _ := http.NewRequest("GET", ts.URL+"/api/v1/reception/slots?service_id=srv_consult", nil)
	reqSlots.Header.Set("X-Init-Data", testMAXInitData())
	respSlots, err := http.DefaultClient.Do(reqSlots)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respSlots.Body.Close()
	if respSlots.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respSlots.StatusCode)
	}

	// POST /api/v1/reception/book
	slotTime := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	bookBody := bytes.NewBufferString(fmt.Sprintf(`{"service_id": "srv_consult", "slot_time": "%s"}`, slotTime.Format(time.RFC3339)))
	reqBook, _ := http.NewRequest("POST", ts.URL+"/api/v1/reception/book", bookBody)
	reqBook.Header.Set("X-Init-Data", testMAXInitData())
	reqBook.Header.Set("Content-Type", "application/json")
	respBook, err := http.DefaultClient.Do(reqBook)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respBook.Body.Close()
	if respBook.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respBook.StatusCode)
	}

	// Duplicate booking of same slot -> must fail (409 Conflict)
	bookBody2 := bytes.NewBufferString(fmt.Sprintf(`{"service_id": "srv_consult", "slot_time": "%s"}`, slotTime.Format(time.RFC3339)))
	reqBook2, _ := http.NewRequest("POST", ts.URL+"/api/v1/reception/book", bookBody2)
	reqBook2.Header.Set("X-Init-Data", testMAXInitData())
	reqBook2.Header.Set("Content-Type", "application/json")
	respBook2, err := http.DefaultClient.Do(reqBook2)
	if err != nil {
		t.Fatalf("Second booking failed: %v", err)
	}
	defer respBook2.Body.Close()
	if respBook2.StatusCode != http.StatusConflict && respBook2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 409 or 400 for duplicate booking, got %d", respBook2.StatusCode)
	}

	// GET /api/v1/reception/events
	reqEvt, _ := http.NewRequest("GET", ts.URL+"/api/v1/reception/events", nil)
	reqEvt.Header.Set("X-Init-Data", testMAXInitData())
	respEvt, err := http.DefaultClient.Do(reqEvt)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respEvt.Body.Close()
	if respEvt.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respEvt.StatusCode)
	}

	// POST /api/v1/reception/events/{id}/register
	reqReg, _ := http.NewRequest("POST", ts.URL+"/api/v1/reception/events/evt_launch/register", nil)
	reqReg.Header.Set("X-Init-Data", testMAXInitData())
	respReg, err := http.DefaultClient.Do(reqReg)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respReg.Body.Close()
	if respReg.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respReg.StatusCode)
	}

	// POST /api/v1/reception/onboarding
	onboardBody := bytes.NewBufferString(`{"answers": {"name": "Ivan", "business": "Ecommerce"}}`)
	reqOnboard, _ := http.NewRequest("POST", ts.URL+"/api/v1/reception/onboarding", onboardBody)
	reqOnboard.Header.Set("X-Init-Data", testMAXInitData())
	reqOnboard.Header.Set("Content-Type", "application/json")
	respOnboard, err := http.DefaultClient.Do(reqOnboard)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respOnboard.Body.Close()
	if respOnboard.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respOnboard.StatusCode)
	}
}

func TestAPI_SupportEndpoints(t *testing.T) {
	ts := setupTestAPIServer(t)

	// POST /api/v1/support/ask
	askBody := bytes.NewBufferString(`{"question": "Как оформить подписку?", "mode_scope": "shop"}`)
	reqAsk, _ := http.NewRequest("POST", ts.URL+"/api/v1/support/ask", askBody)
	reqAsk.Header.Set("X-Init-Data", testMAXInitData())
	reqAsk.Header.Set("Content-Type", "application/json")
	respAsk, err := http.DefaultClient.Do(reqAsk)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respAsk.Body.Close()
	if respAsk.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respAsk.StatusCode)
	}

	var askResp struct {
		Answer string  `json:"answer"`
		Score  float64 `json:"score"`
	}
	_ = json.NewDecoder(respAsk.Body).Decode(&askResp)
	if askResp.Answer == "" {
		t.Errorf("Expected answer from support ask")
	}

	// POST /api/v1/support/tickets
	ticketBody := bytes.NewBufferString(`{"subject": "Проблема с оплатой", "message": "Не проходит транзакция", "mode_scope": "shop"}`)
	reqTicket, _ := http.NewRequest("POST", ts.URL+"/api/v1/support/tickets", ticketBody)
	reqTicket.Header.Set("X-Init-Data", testMAXInitData())
	reqTicket.Header.Set("Content-Type", "application/json")
	respTicket, err := http.DefaultClient.Do(reqTicket)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respTicket.Body.Close()
	if respTicket.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respTicket.StatusCode)
	}

	// GET /api/v1/support/tickets
	reqTickets, _ := http.NewRequest("GET", ts.URL+"/api/v1/support/tickets", nil)
	reqTickets.Header.Set("X-Init-Data", testMAXInitData())
	respTickets, err := http.DefaultClient.Do(reqTickets)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respTickets.Body.Close()
	if respTickets.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respTickets.StatusCode)
	}
}

func TestAPI_ChatEndpoints(t *testing.T) {
	ts := setupTestAPIServer(t)

	// GET /api/v1/chat/config
	reqCfg, _ := http.NewRequest("GET", ts.URL+"/api/v1/chat/config", nil)
	reqCfg.Header.Set("X-Init-Data", testMAXInitData())
	respCfg, err := http.DefaultClient.Do(reqCfg)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respCfg.Body.Close()
	if respCfg.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respCfg.StatusCode)
	}

	var chatCfg struct {
		Roles []chat.Role `json:"roles"`
		Tones []chat.Tone `json:"tones"`
	}
	_ = json.NewDecoder(respCfg.Body).Decode(&chatCfg)
	if len(chatCfg.Roles) == 0 || len(chatCfg.Tones) == 0 {
		t.Errorf("Expected roles and tones in chat config, got %+v", chatCfg)
	}

	// POST /api/v1/chat/message
	msgBody := bytes.NewBufferString(`{"role_id": 1, "tone_id": 1, "message": "Привет, расскажи о боте"}`)
	reqMsg, _ := http.NewRequest("POST", ts.URL+"/api/v1/chat/message", msgBody)
	reqMsg.Header.Set("X-Init-Data", testMAXInitData())
	reqMsg.Header.Set("Content-Type", "application/json")
	respMsg, err := http.DefaultClient.Do(reqMsg)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respMsg.Body.Close()
	if respMsg.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respMsg.StatusCode)
	}

	// POST /api/v1/chat/clear
	reqClear, _ := http.NewRequest("POST", ts.URL+"/api/v1/chat/clear", nil)
	reqClear.Header.Set("X-Init-Data", testMAXInitData())
	respClear, err := http.DefaultClient.Do(reqClear)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respClear.Body.Close()
	if respClear.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respClear.StatusCode)
	}
}

func TestAPI_AdminEndpoints_RBAC(t *testing.T) {
	ts := setupTestAPIServer(t)

	// 1. Regular user trying to access admin endpoint -> 403 Forbidden
	req1, _ := http.NewRequest("GET", ts.URL+"/api/v1/admin/modes", nil)
	req1.Header.Set("X-Init-Data", testMAXInitData())
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-admin user, got %d", resp1.StatusCode)
	}

	// 2. Admin user accessing admin endpoints -> 200 OK
	reqModes, _ := http.NewRequest("GET", ts.URL+"/api/v1/admin/modes", nil)
	reqModes.Header.Set("X-Init-Data", testAdminInitData())
	respModes, err := http.DefaultClient.Do(reqModes)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respModes.Body.Close()
	if respModes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respModes.StatusCode)
	}

	// POST /api/v1/admin/modes/{id}/toggle
	toggleBody := bytes.NewBufferString(`{"is_active": false, "maintenance_message": "Техработы в магазине"}`)
	reqToggle, _ := http.NewRequest("POST", ts.URL+"/api/v1/admin/modes/shop/toggle", toggleBody)
	reqToggle.Header.Set("X-Init-Data", testAdminInitData())
	reqToggle.Header.Set("Content-Type", "application/json")
	respToggle, err := http.DefaultClient.Do(reqToggle)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respToggle.Body.Close()
	if respToggle.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respToggle.StatusCode)
	}

	// GET /api/v1/admin/leads
	reqLeads, _ := http.NewRequest("GET", ts.URL+"/api/v1/admin/leads", nil)
	reqLeads.Header.Set("X-Init-Data", testAdminInitData())
	respLeads, err := http.DefaultClient.Do(reqLeads)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respLeads.Body.Close()
	if respLeads.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respLeads.StatusCode)
	}

	// GET /api/v1/admin/tickets
	reqTickets, _ := http.NewRequest("GET", ts.URL+"/api/v1/admin/tickets", nil)
	reqTickets.Header.Set("X-Init-Data", testAdminInitData())
	respTickets, err := http.DefaultClient.Do(reqTickets)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respTickets.Body.Close()
	if respTickets.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respTickets.StatusCode)
	}

	// GET /api/v1/admin/audit-logs
	reqAudit, _ := http.NewRequest("GET", ts.URL+"/api/v1/admin/audit-logs", nil)
	reqAudit.Header.Set("X-Init-Data", testAdminInitData())
	respAudit, err := http.DefaultClient.Do(reqAudit)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respAudit.Body.Close()
	if respAudit.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respAudit.StatusCode)
	}
}

func TestAPI_CORS(t *testing.T) {
	ts := setupTestAPIServer(t)

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/v1/bootstrap", nil)
	req.Header.Set("Origin", "https://combot.littfed.ru")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Init-Data, Content-Type")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 200 or 204 for OPTIONS, got %d", resp.StatusCode)
	}
	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin == "" {
		t.Errorf("Expected Access-Control-Allow-Origin header")
	}
}

func TestAPI_UserMode_MaintenanceGuard(t *testing.T) {
	ts := setupTestAPIServer(t)

	// Attempting to switch to "channels" (seeded as inactive in DB)
	switchBody := bytes.NewBufferString(`{"mode_id":"channels"}`)
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/user/mode", switchBody)
	req.Header.Set("X-Init-Data", testMAXInitData())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden when switching to inactive mode, got %d", resp.StatusCode)
	}

	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&errResp)
	if !strings.Contains(errResp.Message, "разработке") && !strings.Contains(errResp.Message, "обслуживании") {
		t.Errorf("Expected maintenance message in error response, got %s", errResp.Message)
	}
}

func TestAPI_Admin_LeadStatusAndTicketReply(t *testing.T) {
	ts := setupTestAPIServer(t)

	// 1. User creates a lead
	leadBody := bytes.NewBufferString(`{
		"contact": "+79990001122",
		"comment": "Тест лида для админки",
		"source_mode": "shop",
		"tested_modes": ["shop"]
	}`)
	reqLead, _ := http.NewRequest("POST", ts.URL+"/api/v1/lead", leadBody)
	reqLead.Header.Set("X-Init-Data", testMAXInitData())
	reqLead.Header.Set("Content-Type", "application/json")
	respLead, err := http.DefaultClient.Do(reqLead)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respLead.Body.Close()
	if respLead.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respLead.StatusCode)
	}

	var leadResp struct {
		Success bool  `json:"success"`
		LeadID  int64 `json:"lead_id"`
	}
	_ = json.NewDecoder(respLead.Body).Decode(&leadResp)
	if leadResp.LeadID == 0 {
		t.Fatalf("Expected non-zero lead_id")
	}

	// Admin updates lead status
	statusBody := bytes.NewBufferString(`{"status": "in_progress"}`)
	reqStatus, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/admin/leads/%d/status", ts.URL, leadResp.LeadID), statusBody)
	reqStatus.Header.Set("X-Init-Data", testAdminInitData())
	reqStatus.Header.Set("Content-Type", "application/json")
	respStatus, err := http.DefaultClient.Do(reqStatus)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respStatus.Body.Close()
	if respStatus.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respStatus.StatusCode)
	}

	// 2. User creates a support ticket
	ticketBody := bytes.NewBufferString(`{
		"subject": "Срочный вопрос по оплате",
		"message": "Деньги списались, а доступ не открылся",
		"mode_scope": "shop"
	}`)
	reqTicket, _ := http.NewRequest("POST", ts.URL+"/api/v1/support/tickets", ticketBody)
	reqTicket.Header.Set("X-Init-Data", testMAXInitData())
	reqTicket.Header.Set("Content-Type", "application/json")
	respTicket, err := http.DefaultClient.Do(reqTicket)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respTicket.Body.Close()
	if respTicket.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respTicket.StatusCode)
	}

	var tResp struct {
		Ticket struct {
			ID int64 `json:"id"`
		} `json:"ticket"`
	}
	_ = json.NewDecoder(respTicket.Body).Decode(&tResp)
	if tResp.Ticket.ID == 0 {
		t.Fatalf("Expected non-zero ticket ID")
	}

	// Admin lists tickets and finds it
	reqList, _ := http.NewRequest("GET", ts.URL+"/api/v1/admin/tickets?status=open", nil)
	reqList.Header.Set("X-Init-Data", testAdminInitData())
	respList, err := http.DefaultClient.Do(reqList)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respList.Body.Close()
	if respList.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respList.StatusCode)
	}

	// Admin replies to ticket
	replyBody := bytes.NewBufferString(`{"message": "Доступ активирован вручную. Приятного пользования!"}`)
	reqReply, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/admin/tickets/%d/reply", ts.URL, tResp.Ticket.ID), replyBody)
	reqReply.Header.Set("X-Init-Data", testAdminInitData())
	reqReply.Header.Set("Content-Type", "application/json")
	respReply, err := http.DefaultClient.Do(reqReply)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respReply.Body.Close()
	if respReply.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respReply.StatusCode)
	}

	// Verify ticket status became answered
	reqAnswered, _ := http.NewRequest("GET", ts.URL+"/api/v1/admin/tickets?status=answered", nil)
	reqAnswered.Header.Set("X-Init-Data", testAdminInitData())
	respAnswered, err := http.DefaultClient.Do(reqAnswered)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respAnswered.Body.Close()
	if respAnswered.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d, want http.StatusOK", respAnswered.StatusCode)
	}

	var answeredResp struct {
		Tickets []store.SupportTicket `json:"tickets"`
	}
	_ = json.NewDecoder(respAnswered.Body).Decode(&answeredResp)
	found := false
	for _, tk := range answeredResp.Tickets {
		if tk.ID == tResp.Ticket.ID && tk.Status == "answered" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected ticket %d to have status 'answered', got: %+v", tResp.Ticket.ID, answeredResp.Tickets)
	}
}
