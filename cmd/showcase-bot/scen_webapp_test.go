package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
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

	// 1. Test Static HTML Endpoints (/webapp and /maxapp)
	for _, path := range []string{"/webapp", "/maxapp", "/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "MAX Mini App Корзина") {
			t.Fatalf("expected Mini App title in HTML body for %s, got: %s", path, rec.Body.String())
		}
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

	// 4. Test WebBridge Action POST in demo/unauthenticated mode
	actionReq := httptest.NewRequest(http.MethodPost, "/api/webapp/action", bytes.NewReader([]byte(`{
		"action": "submit_cart",
		"init_data": "demo=true&user_id=888",
		"payload": {"items": {"coffee": 1}, "total_price": 4990}
	}`)))
	actionRec := httptest.NewRecorder()
	mux.ServeHTTP(actionRec, actionReq)

	if actionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for demo initData in showcase-bot, got %d", actionRec.Code)
	}

	// 5. Test WebBridge Action POST with signed initData
	signedInit := webapp.SignInitData("test_token_123", 888, "Федор", "fedor_v")
	bodyJSON, _ := json.Marshal(map[string]any{
		"action":    "submit_cart",
		"init_data": signedInit,
		"payload":   map[string]any{"items": map[string]int{"coffee": 1}, "total_price": 4990},
	})
	signedReq := httptest.NewRequest(http.MethodPost, "/api/webapp/action", bytes.NewReader(bodyJSON))
	signedRec := httptest.NewRecorder()
	mux.ServeHTTP(signedRec, signedReq)

	if signedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for signed initData, got %d", signedRec.Code)
	}
}
