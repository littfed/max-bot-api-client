//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/admin"
	"github.com/littfed/max-bot-api-client/internal/api"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/chat"
	"github.com/littfed/max-bot-api-client/internal/modules/reception"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/internal/modules/support"
	"github.com/littfed/max-bot-api-client/internal/platform/max"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/store"
	_ "modernc.org/sqlite"
)

func setupTestServer(t *testing.T) http.Handler {
	t.Helper()

	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = dbConn.Close()
	})

	sqlStore, err := store.NewSQLStore(dbConn, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	// Seed test user
	ctx := context.Background()
	_, err = dbConn.ExecContext(ctx, `
		INSERT INTO users (id, active_mode_id, role, timezone, created_at, updated_at) VALUES
		(1, 'shop', 'admin', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
		INSERT INTO user_identities (user_id, platform, platform_user_id, username, first_name) VALUES
		(1, 'max', '1001', 'adminuser', 'AdminUser');
	`)
	if err != nil {
		t.Fatalf("failed to seed test user: %v", err)
	}

	compositeFSM := core.NewCompositeFSM(sqlStore, nil)
	lockManager := core.NewLockManager(nil)
	maintenanceGuard := core.NewMaintenanceGuard(sqlStore, nil)
	orchestrator := core.NewOrchestrator(sqlStore, compositeFSM, lockManager, maintenanceGuard)

	maxAdapter := max.NewAdapter(nil)
	adminService := admin.NewService(sqlStore, maintenanceGuard, maxAdapter)

	shopMod := shop.New(sqlStore)
	receptionMod := reception.New(sqlStore, lockManager)
	supportMod := support.New(sqlStore)
	chatMod := chat.New(sqlStore, nil)

	orchestrator.RegisterModule(shopMod)
	orchestrator.RegisterModule(receptionMod)
	orchestrator.RegisterModule(supportMod)
	orchestrator.RegisterModule(chatMod)

	mux := http.NewServeMux()

	apiRouter := api.NewRouter(api.RouterConfig{
		Store:           sqlStore,
		Guard:           maintenanceGuard,
		FSM:             compositeFSM,
		Orchestrator:    orchestrator,
		AdminService:    adminService,
		ShopModule:      shopMod,
		ReceptionModule: receptionMod,
		SupportModule:   supportMod,
		ChatModule:      chatMod,
		BotToken:        "test_bot_token",
		JWTSecret:       "test_jwt_secret",
		PlatformAdapter: maxAdapter,
		AllowDevHeaders: true,
	})
	mux.Handle("/api/v1/", apiRouter)

	b := bot.NewWithClient(nil)
	mux.HandleFunc("/webhook", b.WebhookHandler("test_webhook_secret"))

	if err := RegisterWebSPA(mux); err != nil {
		t.Fatalf("failed to register Web SPA: %v", err)
	}

	return mux
}

func TestIntegration_WebSPAAndAPI(t *testing.T) {
	handler := setupTestServer(t)

	// 1. Root route "/" serves embedded React SPA HTML
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	handler.ServeHTTP(recRoot, reqRoot)

	if recRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", recRoot.Code)
	}
	if !strings.Contains(recRoot.Body.String(), "<div id=\"root\"") {
		t.Errorf("expected embedded root div in response, got: %s", recRoot.Body.String())
	}

	// 2. Client-side route "/reception" falls back to SPA index.html
	reqSPA := httptest.NewRequest(http.MethodGet, "/reception", nil)
	recSPA := httptest.NewRecorder()
	handler.ServeHTTP(recSPA, reqSPA)

	if recSPA.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /reception fallback, got %d", recSPA.Code)
	}
	if !strings.Contains(recSPA.Body.String(), "<div id=\"root\"") {
		t.Errorf("expected embedded root div in fallback, got: %s", recSPA.Body.String())
	}

	// 3. API endpoint GET /api/v1/bootstrap
	reqBootstrap := httptest.NewRequest(http.MethodGet, "/api/v1/bootstrap", nil)
	reqBootstrap.Header.Set("X-Dev-User-ID", "1")
	reqBootstrap.Header.Set("X-Platform", "max")
	recBootstrap := httptest.NewRecorder()
	handler.ServeHTTP(recBootstrap, reqBootstrap)

	if recBootstrap.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/v1/bootstrap, got %d (body: %s)", recBootstrap.Code, recBootstrap.Body.String())
	}
	var bootstrapData map[string]any
	if err := json.Unmarshal(recBootstrap.Body.Bytes(), &bootstrapData); err != nil {
		t.Fatalf("failed to decode bootstrap json: %v", err)
	}
	if bootstrapData["active_mode"] != "shop" {
		t.Errorf("expected default active_mode shop, got %v", bootstrapData["active_mode"])
	}

	// 4. API endpoint GET /api/v1/shop/catalog
	reqCatalog := httptest.NewRequest(http.MethodGet, "/api/v1/shop/catalog", nil)
	reqCatalog.Header.Set("X-Dev-User-ID", "1")
	recCatalog := httptest.NewRecorder()
	handler.ServeHTTP(recCatalog, reqCatalog)

	if recCatalog.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/v1/shop/catalog, got %d", recCatalog.Code)
	}

	// 5. API endpoint GET /api/v1/mascot/tap
	reqMascot := httptest.NewRequest(http.MethodGet, "/api/v1/mascot/tap", nil)
	reqMascot.Header.Set("X-Dev-User-ID", "1")
	recMascot := httptest.NewRecorder()
	handler.ServeHTTP(recMascot, reqMascot)

	if recMascot.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/v1/mascot/tap, got %d", recMascot.Code)
	}

	// 6. Webhook POST /webhook accepts incoming JSON update
	upd := maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			MessageID: "msg_webhook_1",
			Sender:    &maxbot.User{UserID: 100},
			Recipient: &maxbot.MessageRecipient{ChatID: 200},
			Body:      maxbot.MessageBody{Text: "/start"},
		},
	}
	updBytes, _ := json.Marshal(upd)
	reqWebhook := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(updBytes))
	reqWebhook.Header.Set("X-Max-Bot-Api-Secret", "test_webhook_secret")
	recWebhook := httptest.NewRecorder()
	handler.ServeHTTP(recWebhook, reqWebhook)

	if recWebhook.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /webhook, got %d", recWebhook.Code)
	}
}
