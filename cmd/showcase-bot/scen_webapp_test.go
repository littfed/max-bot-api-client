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
	"github.com/littfed/max-bot-api-client/pkg/store"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
)

func TestWebappScenarioWorkflow(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	appStore, _ := store.New("")

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())

	mux := http.NewServeMux()
	RegisterWebappScenario(botInstance, hood, mux, "test_token_123", "8080", appStore, nil, nil, "gemini-2.5-flash")

	// 1. Test Static HTML Endpoints (/webapp and /maxapp)
	for _, path := range []string{"/webapp", "/maxapp", "/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "MAX Mini App Suite") {
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

	// 4. Test WebBridge Action POST rejects demo/unauthenticated initData
	actionReq := httptest.NewRequest(http.MethodPost, "/api/webapp/action", bytes.NewReader([]byte(`{
		"action": "submit_cart",
		"init_data": "demo=true&user_id=888",
		"payload": {"items": {"coffee": 1}, "total_price": 4990}
	}`)))
	actionRec := httptest.NewRecorder()
	mux.ServeHTTP(actionRec, actionReq)

	if actionRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated demo initData, got %d", actionRec.Code)
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

	// 6. Test GET /api/webapp/orders
	ordersReq := httptest.NewRequest(http.MethodGet, "/api/webapp/orders", nil)
	ordersReq.Header.Set("X-Init-Data", signedInit)
	ordersRec := httptest.NewRecorder()
	mux.ServeHTTP(ordersRec, ordersReq)
	if ordersRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/webapp/orders, got %d", ordersRec.Code)
	}

	// 7. Test POST /api/webapp/spin
	spinReq := httptest.NewRequest(http.MethodPost, "/api/webapp/spin", nil)
	spinReq.Header.Set("X-Init-Data", signedInit)
	spinRec := httptest.NewRecorder()
	mux.ServeHTTP(spinRec, spinReq)
	if spinRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/webapp/spin, got %d", spinRec.Code)
	}

	// 8. Test POST /api/webapp/publish_post with unauthorized user (Access denied check)
	pubJSON, _ := json.Marshal(map[string]any{
		"channel_id": -78368186601503,
		"text":       "Test unauthorized post",
	})
	pubReq := httptest.NewRequest(http.MethodPost, "/api/webapp/publish_post", bytes.NewReader(pubJSON))
	pubReq.Header.Set("X-Init-Data", signedInit)
	pubRec := httptest.NewRecorder()
	mux.ServeHTTP(pubRec, pubReq)
	var pubResp map[string]any
	_ = json.Unmarshal(pubRec.Body.Bytes(), &pubResp)
	if pubResp["success"] == true {
		t.Fatalf("expected publish_post to reject non-admin, got success=true")
	}

	// 9. Test WebBridge Action book_service
	bookJSON, _ := json.Marshal(map[string]any{
		"action":    "book_service",
		"init_data": signedInit,
		"payload": map[string]any{
			"service_id":   "srv_audit",
			"service_name": "Архитектурный аудит Go / MAX",
			"price":        5000,
			"specialist":   "Алексей (Lead Go)",
			"slot":         "Завтра, 14:00",
			"user_name":    "Федор В",
			"phone":        "+7 999 123-45-67",
			"comment":      "Хочу аудит WebApp",
		},
	})
	bookReq := httptest.NewRequest(http.MethodPost, "/api/webapp/action", bytes.NewReader(bookJSON))
	bookRec := httptest.NewRecorder()
	mux.ServeHTTP(bookRec, bookReq)
	if bookRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for book_service action, got %d", bookRec.Code)
	}
	var bookResp map[string]any
	_ = json.Unmarshal(bookRec.Body.Bytes(), &bookResp)
	if bookResp["success"] != true {
		t.Fatalf("expected success=true for book_service, got %v", bookResp)
	}

	// 10. Test POST /api/webapp/generate_image
	imgGenJSON, _ := json.Marshal(map[string]any{
		"prompt": "Go Gopher Mascot",
		"style":  "cyberpunk",
	})
	imgGenReq := httptest.NewRequest(http.MethodPost, "/api/webapp/generate_image", bytes.NewReader(imgGenJSON))
	imgGenReq.Header.Set("X-Init-Data", signedInit)
	imgGenRec := httptest.NewRecorder()
	mux.ServeHTTP(imgGenRec, imgGenReq)
	if imgGenRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/webapp/generate_image, got %d", imgGenRec.Code)
	}
	var imgGenResp map[string]any
	_ = json.Unmarshal(imgGenRec.Body.Bytes(), &imgGenResp)
	if imgGenResp["success"] != true || imgGenResp["image_url"] == "" {
		t.Fatalf("expected image_url in response, got %v", imgGenResp)
	}

	// 11. Test POST /api/webapp/ai_chat
	chatJSON, _ := json.Marshal(map[string]any{
		"prompt": "Расскажи про MAX Bot API",
		"model":  "auto",
	})
	chatReq := httptest.NewRequest(http.MethodPost, "/api/webapp/ai_chat", bytes.NewReader(chatJSON))
	chatReq.Header.Set("X-Init-Data", signedInit)
	chatRec := httptest.NewRecorder()
	mux.ServeHTTP(chatRec, chatReq)
	if chatRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/webapp/ai_chat, got %d", chatRec.Code)
	}
	var chatResp map[string]any
	_ = json.Unmarshal(chatRec.Body.Bytes(), &chatResp)
	if chatResp["success"] != true || chatResp["text"] == "" {
		t.Fatalf("expected text in chat response, got %v", chatResp)
	}
}

func TestMiniAppAccessibilityAndUXGuidelines(t *testing.T) {
	// Ticket #47: Accessibility & Web Interface Guidelines
	if strings.Contains(miniAppHTML, "user-scalable=no") || strings.Contains(miniAppHTML, "maximum-scale=1.0") {
		t.Errorf("miniAppHTML contains zoom-blocking viewport properties")
	}
	if !strings.Contains(miniAppHTML, "viewport-fit=cover") {
		t.Errorf("miniAppHTML must include viewport-fit=cover")
	}
	if !strings.Contains(miniAppHTML, ":focus-visible") {
		t.Errorf("miniAppHTML must define :focus-visible styling")
	}
	if !strings.Contains(miniAppHTML, "role=\"tab\"") {
		t.Errorf("miniAppHTML bottom nav must use semantic role=tab")
	}
	if !strings.Contains(miniAppHTML, "prefers-reduced-motion") {
		t.Errorf("miniAppHTML must support prefers-reduced-motion")
	}
	if !strings.Contains(miniAppHTML, "aria-live=\"polite\"") {
		t.Errorf("miniAppHTML toast container must include aria-live=polite")
	}
	if !strings.Contains(miniAppHTML, "aria-label=\"Закрыть\"") {
		t.Errorf("miniAppHTML modal close button must have aria-label")
	}

	// Ticket #48: Sticky Bottom MainButton in Thumb Zone
	if !strings.Contains(miniAppHTML, "sticky-main-btn") {
		t.Errorf("miniAppHTML must define sticky-main-btn")
	}
	if !strings.Contains(miniAppHTML, "main-btn-qty") || !strings.Contains(miniAppHTML, "main-btn-total") {
		t.Errorf("miniAppHTML sticky main button must have live qty and total elements")
	}

	// Ticket #49: Haptic feedback, Skeletons, and Empty States
	if !strings.Contains(miniAppHTML, "triggerHaptic") {
		t.Errorf("miniAppHTML must implement triggerHaptic engine")
	}
	if !strings.Contains(miniAppHTML, "skeleton-card") || !strings.Contains(miniAppHTML, "shimmer") {
		t.Errorf("miniAppHTML must support shimmer skeletons")
	}
	if !strings.Contains(miniAppHTML, "empty-state") {
		t.Errorf("miniAppHTML must support empty-state component")
	}

	// Clean Mini App UI & Respectful User Identity
	if strings.Contains(miniAppHTML, "MAX SuperApp Suite") {
		t.Errorf("miniAppHTML must not contain redundant legacy header 'MAX SuperApp Suite'")
	}
	if strings.Contains(miniAppHTML, "Гость (Демо)") || strings.Contains(miniAppHTML, "(Гость)") {
		t.Errorf("miniAppHTML must not display discriminatory 'Гость' badges")
	}

	// Ticket #56: Media & History Gallery
	if !strings.Contains(miniAppHTML, "btn-sub-gallery") {
		t.Errorf("miniAppHTML must include btn-sub-gallery subnav button")
	}
	if !strings.Contains(miniAppHTML, "ai-sub-gallery") {
		t.Errorf("miniAppHTML must include ai-sub-gallery container")
	}
	if !strings.Contains(miniAppHTML, "loadMediaHistory") {
		t.Errorf("miniAppHTML must implement loadMediaHistory function")
	}
	if !strings.Contains(miniAppHTML, "copyGalleryText") {
		t.Errorf("miniAppHTML must implement copyGalleryText function")
	}
}

func TestWebappMediaHistoryAPI(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	appStore, _ := store.New("")
	botInstance := bot.NewWithClient(nil)
	mux := http.NewServeMux()
	RegisterWebappScenario(botInstance, hood, mux, "test_token_123", "8080", appStore, nil, nil, "gemini-2.5-flash")

	const testUID int64 = 777123

	// 1. Pre-seed media artifacts
	_, err := appStore.SaveMediaArtifact(store.MediaArtifact{
		UserID:       testUID,
		ArtifactType: "image",
		Title:        "Арт: Neon Gopher",
		Prompt:       "Cyberpunk Neon Gopher",
		MediaURL:     "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
		CreatedAt:    time.Now().Add(-10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("failed to save image artifact: %v", err)
	}

	_, err = appStore.SaveMediaArtifact(store.MediaArtifact{
		UserID:       testUID,
		ArtifactType: "transcription",
		Title:        "Голосовое сообщение",
		ContentText:  "Привет, это тестовая голосовая заметка",
		CreatedAt:    time.Now().Add(-5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("failed to save transcription artifact: %v", err)
	}

	_, err = appStore.SaveMediaArtifact(store.MediaArtifact{
		UserID:       testUID,
		ArtifactType: "document",
		Title:        "Анализ документа: spec.pdf",
		ContentText:  "Документ содержит описание архитектуры",
		CreatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save doc artifact: %v", err)
	}

	signedHistInit := webapp.SignInitData("test_token_123", testUID, "HistUser", "hist_user")

	// 2. Query all history for testUID
	reqAll := httptest.NewRequest(http.MethodGet, "/api/webapp/history", nil)
	reqAll.Header.Set("X-Init-Data", signedHistInit)
	recAll := httptest.NewRecorder()
	mux.ServeHTTP(recAll, reqAll)

	if recAll.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recAll.Code)
	}

	var respAll struct {
		Success bool                  `json:"success"`
		Items   []store.MediaArtifact `json:"items"`
	}
	if err := json.Unmarshal(recAll.Body.Bytes(), &respAll); err != nil {
		t.Fatalf("failed to unmarshal history json: %v", err)
	}
	if !respAll.Success || len(respAll.Items) != 3 {
		t.Fatalf("expected 3 history items, got %d (success=%v)", len(respAll.Items), respAll.Success)
	}

	// 3. Query filtered by type=image
	reqImg := httptest.NewRequest(http.MethodGet, "/api/webapp/history?type=image", nil)
	reqImg.Header.Set("X-Init-Data", signedHistInit)
	recImg := httptest.NewRecorder()
	mux.ServeHTTP(recImg, reqImg)

	var respImg struct {
		Success bool                  `json:"success"`
		Items   []store.MediaArtifact `json:"items"`
	}
	_ = json.Unmarshal(recImg.Body.Bytes(), &respImg)
	if len(respImg.Items) != 1 || respImg.Items[0].ArtifactType != "image" {
		t.Fatalf("expected 1 image artifact, got %d", len(respImg.Items))
	}

	// 4. Query filtered by type=transcription
	reqSTT := httptest.NewRequest(http.MethodGet, "/api/webapp/history?type=transcription", nil)
	reqSTT.Header.Set("X-Init-Data", signedHistInit)
	recSTT := httptest.NewRecorder()
	mux.ServeHTTP(recSTT, reqSTT)

	var respSTT struct {
		Success bool                  `json:"success"`
		Items   []store.MediaArtifact `json:"items"`
	}
	_ = json.Unmarshal(recSTT.Body.Bytes(), &respSTT)
	if len(respSTT.Items) != 1 || respSTT.Items[0].ArtifactType != "transcription" {
		t.Fatalf("expected 1 transcription artifact, got %d", len(respSTT.Items))
	}

	// 5. Test POST /api/webapp/generate_image automatically saves artifact
	genReq := httptest.NewRequest(http.MethodPost, "/api/webapp/generate_image", bytes.NewReader([]byte(`{
		"prompt": "Gopher in Space",
		"style": "Cyberpunk"
	}`)))
	genReq.Header.Set("X-Init-Data", signedHistInit)
	genRec := httptest.NewRecorder()
	mux.ServeHTTP(genRec, genReq)

	if genRec.Code != http.StatusOK {
		t.Fatalf("generate_image failed: %d", genRec.Code)
	}

	// Re-check image count (should now be 2)
	imgList, _ := appStore.ListMediaArtifacts(testUID, "image", 10)
	if len(imgList) != 2 {
		t.Fatalf("expected 2 image artifacts after generate_image, got %d", len(imgList))
	}
}

func TestOrdersHistoryInBot(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	appStore, _ := store.New("")
	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())

	mux := http.NewServeMux()
	RegisterWebappScenario(botInstance, hood, mux, "test_token_123", "8080", appStore, nil, nil, "gemini-2.5-flash")

	testUID := int64(999123)

	// 1. Test /orders command when user has 0 orders
	ctxEmpty := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "/orders",
			},
			Sender: &maxbot.User{UserID: testUID, FirstName: "Алексей"},
		},
	})
	if err := botInstance.Router().Handle(ctxEmpty); err != nil {
		t.Fatalf("handle /orders empty failed: %v", err)
	}

	// 2. Add an order and a booking
	_ = appStore.CreateOrder(store.Order{
		ID:           "WEB-11111",
		UserID:       testUID,
		UserName:     "Алексей",
		Items:        []store.OrderItem{{Title: "Худи Go Gopher", Quantity: 1, Price: 4990}},
		Total:        4990,
		DeliveryType: "delivery",
		Status:       "paid",
		CreatedAt:    time.Now(),
	})
	_ = appStore.CreateOrder(store.Order{
		ID:           "BOK-22222",
		UserID:       testUID,
		UserName:     "Алексей",
		Items:        []store.OrderItem{{Title: "Аудит архитектуры", Quantity: 1, Price: 0}},
		Total:        0,
		DeliveryType: "booking",
		Address:      "Специалист: Lead Go | Слот: Завтра, 11:00",
		Status:       "accepted",
		CreatedAt:    time.Now().Add(1 * time.Minute),
	})

	// 3. Test nav:orders callback
	ctxCallback := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Callback: &maxbot.Callback{
			CallbackID: "cb_orders",
			Payload:    "nav:orders",
			User:       &maxbot.User{UserID: testUID, FirstName: "Алексей"},
		},
	})
	if err := botInstance.Router().Handle(ctxCallback); err != nil {
		t.Fatalf("handle nav:orders callback failed: %v", err)
	}

	// 4. Test /orders command with existing items
	ctxWithOrders := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "/orders",
			},
			Sender: &maxbot.User{UserID: testUID, FirstName: "Алексей"},
		},
	})
	if err := botInstance.Router().Handle(ctxWithOrders); err != nil {
		t.Fatalf("handle /orders with orders failed: %v", err)
	}
}
