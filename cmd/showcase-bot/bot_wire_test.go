package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/store"
	_ "modernc.org/sqlite"
)

func TestConvertContextToDomainEvent(t *testing.T) {
	// Message update
	msgUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			MessageID: "msg_999",
			Sender: &maxbot.User{
				UserID:    12345,
				Username:  "johndoe",
				FirstName: "John",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 67890,
			},
			Body: maxbot.MessageBody{
				Text: "/mode shop",
			},
		},
	}
	msgCtx := bot.NewContext(context.Background(), nil, msgUpdate)
	evt := convertContextToDomainEvent(msgCtx)

	if evt == nil {
		t.Fatal("expected non-nil domain event for message")
	}
	if evt.Platform != "max" {
		t.Errorf("expected platform max, got %s", evt.Platform)
	}
	if evt.PlatformUserID != "12345" {
		t.Errorf("expected platform user id 12345, got %s", evt.PlatformUserID)
	}
	if evt.Username != "johndoe" || evt.FirstName != "John" {
		t.Errorf("expected johndoe/John, got %s/%s", evt.Username, evt.FirstName)
	}
	if evt.Message == nil || evt.Message.Text != "/mode shop" {
		t.Errorf("expected message text /mode shop, got %+v", evt.Message)
	}

	// Callback update
	cbUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Callback: &maxbot.Callback{
			CallbackID: "cb_111",
			User: &maxbot.User{
				UserID:    54321,
				Username:  "alice",
				FirstName: "Alice",
			},
			Message: &maxbot.Message{
				MessageID: "msg_222",
				Recipient: &maxbot.MessageRecipient{
					ChatID: 333,
				},
			},
			Payload: "mode:select:shop",
		},
	}
	cbCtx := bot.NewContext(context.Background(), nil, cbUpdate)
	cbEvt := convertContextToDomainEvent(cbCtx)

	if cbEvt == nil {
		t.Fatal("expected non-nil domain event for callback")
	}
	if cbEvt.Callback == nil || cbEvt.Callback.Data != "mode:select:shop" {
		t.Errorf("expected callback data mode:select:shop, got %+v", cbEvt.Callback)
	}

	// Image message update
	imgUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			MessageID: "msg_img",
			Sender: &maxbot.User{
				UserID:    12345,
				Username:  "johndoe",
				FirstName: "John",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 67890,
			},
			Body: maxbot.MessageBody{
				Text: "сделай фон белым",
				Attachments: []maxbot.Attachment{
					{
						Type:    "image",
						Payload: map[string]any{"url": "https://cdn.example.com/photo.jpg"},
					},
				},
			},
		},
	}
	imgCtx := bot.NewContext(context.Background(), nil, imgUpdate)
	imgEvt := convertContextToDomainEvent(imgCtx)
	if imgEvt == nil || imgEvt.Message == nil {
		t.Fatal("expected non-nil domain event for image message")
	}
	if imgEvt.Message.ImageURL != "https://cdn.example.com/photo.jpg" {
		t.Errorf("expected ImageURL https://cdn.example.com/photo.jpg, got %q", imgEvt.Message.ImageURL)
	}
}

func TestWireBotToOrchestrator(t *testing.T) {
	var mu sync.Mutex
	var sentMessages []maxbot.SendMessageRequest

	// Mock MAX Bot API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/messages") {
			var req maxbot.SendMessageRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			sentMessages = append(sentMessages, req)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(maxbot.Message{MessageID: "msg_sent_123"})
			return
		}
		if strings.Contains(r.URL.Path, "/answers") {
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	client, err := maxbot.NewClient("test_token", maxbot.WithBaseURL(mockServer.URL), maxbot.WithRussianCerts(false))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer dbConn.Close()

	sqlStore, err := store.NewSQLStore(dbConn, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	fsm := core.NewCompositeFSM(sqlStore, nil)
	lockManager := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(sqlStore, nil)
	orch := core.NewOrchestrator(sqlStore, fsm, lockManager, guard)

	shopMod := shop.New(sqlStore)
	orch.RegisterModule(shopMod)

	b := bot.NewWithClient(client)
	wireBotToOrchestrator(b, orch, slog.Default())

	// 1. Test wired command /hub -> orchestrator lists modes
	hubUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Timestamp:  time.Now().Unix(),
		Message: &maxbot.Message{
			MessageID: "msg_hub",
			Sender: &maxbot.User{
				UserID:    42,
				Username:  "tester",
				FirstName: "Test",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 100,
			},
			Body: maxbot.MessageBody{
				Text: "/hub",
			},
		},
	}

	if err := b.ProcessUpdate(context.Background(), hubUpdate); err != nil {
		t.Fatalf("ProcessUpdate /hub failed: %v", err)
	}

	mu.Lock()
	if len(sentMessages) == 0 {
		t.Fatal("expected sent message for /hub command, got none")
	}
	lastMsg := sentMessages[len(sentMessages)-1]
	mu.Unlock()

	if !strings.Contains(lastMsg.Text, "Bot Hub") {
		t.Errorf("expected Bot Hub in response text for /hub, got: %s", lastMsg.Text)
	}
	if len(lastMsg.Attachments) == 0 {
		t.Errorf("expected inline buttons attached for /hub mode list")
	}

	// 2. Test wired command /mode shop -> switches mode
	modeUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Timestamp:  time.Now().Unix(),
		Message: &maxbot.Message{
			MessageID: "msg_mode",
			Sender: &maxbot.User{
				UserID:    42,
				Username:  "tester",
				FirstName: "Test",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 100,
			},
			Body: maxbot.MessageBody{
				Text: "/mode shop",
			},
		},
	}

	if err := b.ProcessUpdate(context.Background(), modeUpdate); err != nil {
		t.Fatalf("ProcessUpdate /mode shop failed: %v", err)
	}

	mu.Lock()
	lastMsg = sentMessages[len(sentMessages)-1]
	mu.Unlock()

	if !strings.Contains(lastMsg.Text, "shop") {
		t.Errorf("expected 'shop' in response text for /mode shop, got: %s", lastMsg.Text)
	}

	// 3. Test plain text message routing into active mode (shop)
	plainTextUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Timestamp:  time.Now().Unix(),
		Message: &maxbot.Message{
			MessageID: "msg_text",
			Sender: &maxbot.User{
				UserID:    42,
				Username:  "tester",
				FirstName: "Test",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 100,
			},
			Body: maxbot.MessageBody{
				Text: "покажи каталог",
			},
		},
	}

	if err := b.ProcessUpdate(context.Background(), plainTextUpdate); err != nil {
		t.Fatalf("ProcessUpdate plain text failed: %v", err)
	}

	mu.Lock()
	lastMsg = sentMessages[len(sentMessages)-1]
	mu.Unlock()

	if !strings.Contains(lastMsg.Text, "Каталог") && !strings.Contains(lastMsg.Text, "товаров") {
		t.Errorf("expected shop catalog in response to plain text message, got: %s", lastMsg.Text)
	}
}

type stubImagesModule struct {
	callbackCalls int
	messageCalls  int
}

func (m *stubImagesModule) ID() string    { return "images" }
func (m *stubImagesModule) Title() string { return "Фотостудия" }
func (m *stubImagesModule) HandleMessage(_ context.Context, _ *store.User, _ *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	m.messageCalls++
	return &core.ModuleResult{
		Text:    "images handled: " + msg.ImageURL,
		Handled: true,
	}, nil
}
func (m *stubImagesModule) HandleCallback(_ context.Context, _ *store.User, _ *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	m.callbackCalls++
	return &core.ModuleResult{
		Text:    "img cb: " + cb.Data,
		Handled: true,
	}, nil
}

func TestWireBotImagesRouting(t *testing.T) {
	var mu sync.Mutex
	var sentMessages []maxbot.SendMessageRequest

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/messages") {
			var req maxbot.SendMessageRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			sentMessages = append(sentMessages, req)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(maxbot.Message{MessageID: "msg_sent_img"})
			return
		}
		if strings.Contains(r.URL.Path, "/answers") {
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	client, err := maxbot.NewClient("test_token", maxbot.WithBaseURL(mockServer.URL), maxbot.WithRussianCerts(false))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer dbConn.Close()

	sqlStore, err := store.NewSQLStore(dbConn, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	fsm := core.NewCompositeFSM(sqlStore, nil)
	lockManager := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(sqlStore, nil)
	orch := core.NewOrchestrator(sqlStore, fsm, lockManager, guard)

	imgMod := &stubImagesModule{}
	orch.RegisterModule(imgMod)

	b := bot.NewWithClient(client)
	wireBotToOrchestrator(b, orch, slog.Default())
	t.Cleanup(func() { setGlobalOrchestratorImageHandler(nil) })

	// 1. Image message when NOT in images mode -> handler returns handled=false (allows Vision fallback)
	imgUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Timestamp:  time.Now().Unix(),
		Message: &maxbot.Message{
			MessageID: "msg_img_1",
			Sender:    &maxbot.User{UserID: 77, Username: "u77", FirstName: "U"},
			Recipient: &maxbot.MessageRecipient{ChatID: 700},
			Body: maxbot.MessageBody{
				Attachments: []maxbot.Attachment{{Type: "image", Payload: map[string]any{"url": "https://cdn.example.com/a.jpg"}}},
			},
		},
	}
	h := getGlobalOrchestratorImageHandler()
	if h == nil {
		t.Fatal("expected globalOrchestratorImageHandler to be registered")
	}
	handled, err := h(bot.NewContext(context.Background(), client, imgUpdate))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false when active mode is not images")
	}

	// 2. Switch user to images mode
	if err := b.ProcessUpdate(context.Background(), &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Timestamp:  time.Now().Unix(),
		Message: &maxbot.Message{
			MessageID: "msg_mode_img",
			Sender:    &maxbot.User{UserID: 77, Username: "u77", FirstName: "U"},
			Recipient: &maxbot.MessageRecipient{ChatID: 700},
			Body:      maxbot.MessageBody{Text: "/mode images"},
		},
	}); err != nil {
		t.Fatalf("switch mode failed: %v", err)
	}

	// 3. img: callback must be routed to orchestrator
	if err := b.ProcessUpdate(context.Background(), &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Timestamp:  time.Now().Unix(),
		Callback: &maxbot.Callback{
			CallbackID: "cb_img_1",
			User:       &maxbot.User{UserID: 77, Username: "u77", FirstName: "U"},
			Message:    &maxbot.Message{MessageID: "msg_cb", Recipient: &maxbot.MessageRecipient{ChatID: 700}},
			Payload:    "img:home",
		},
	}); err != nil {
		t.Fatalf("img callback failed: %v", err)
	}
	if imgMod.callbackCalls != 1 {
		t.Fatalf("expected 1 callback call to images module, got %d", imgMod.callbackCalls)
	}

	// 4. Image message when in images mode -> handled=true
	handled, err = h(bot.NewContext(context.Background(), client, imgUpdate))
	if err != nil {
		t.Fatalf("unexpected error in images mode: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true when active mode is images")
	}
	if imgMod.messageCalls != 1 {
		t.Fatalf("expected 1 message call to images module, got %d", imgMod.messageCalls)
	}
}
