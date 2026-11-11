package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestWebappScenarioWorkflow(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())

	mux := http.NewServeMux()
	RegisterWebappScenario(botInstance, hood, mux, "test_token_123", "8080")

	// 1. Test Static HTML Endpoint
	req := httptest.NewRequest(http.MethodGet, "/webapp", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /webapp, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "MAX Mini App Корзина") {
		t.Fatalf("expected Mini App title in HTML body, got: %s", rec.Body.String())
	}

	// 2. Test scen:webapp launch callback
	ctxLaunch := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Callback: &maxbot.Callback{
			CallbackID: "cb_app",
			Payload:    "scen:webapp",
			User:       &maxbot.User{UserID: 888},
		},
	})
	if err := botInstance.Router().Handle(ctxLaunch); err != nil {
		t.Fatalf("launch webapp failed: %v", err)
	}

	// 3. Test webapp:simulate_order callback
	ctxSim := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Callback: &maxbot.Callback{
			CallbackID: "cb_sim",
			Payload:    "webapp:simulate_order",
			User:       &maxbot.User{UserID: 888},
		},
	})
	if err := botInstance.Router().Handle(ctxSim); err != nil {
		t.Fatalf("simulate order failed: %v", err)
	}

	// 4. Test WebBridge Action POST
	actionReq := httptest.NewRequest(http.MethodPost, "/api/webapp/action", bytes.NewReader([]byte(`{
		"action": "submit_cart",
		"init_data": "invalid_signature",
		"payload": {"test": true}
	}`)))
	actionRec := httptest.NewRecorder()
	mux.ServeHTTP(actionRec, actionReq)

	// Since init_data is invalid signature, it should return 401 Unauthorized (safe and expected!)
	if actionRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for bad initData, got %d", actionRec.Code)
	}
}
