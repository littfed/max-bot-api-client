package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestAIMenuVisibility(t *testing.T) {
	hood := NewHoodEngine()
	userID := int64(12345)

	// 1. By default, AI is not available
	hood.SetAIAvailable(false)
	kbOff := BuildMainMenuKeyboard(hood, userID)
	for _, row := range kbOff.Rows() {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Искусственный Интеллект") || btn.Payload == "scen:ai" {
				t.Fatalf("expected AI button to be hidden when AI is not available")
			}
		}
	}

	// 2. When AI is available, AI button should appear
	hood.SetAIAvailable(true)
	kbOn := BuildMainMenuKeyboard(hood, userID)
	found := false
	for _, row := range kbOn.Rows() {
		for _, btn := range row {
			if btn.Payload == "scen:ai" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected AI button to be present when AI is available")
	}
}

func TestAIScenarioFullWorkflow(t *testing.T) {
	// 1. Mock OpenAI-compatible AI Gateway
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/models":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "gpt-4o-mini"}},
			})
		case "/chat/completions":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			messages, _ := req["messages"].([]any)
			replyContent := fmt.Sprintf("Ответ нейросети на сообщение #%d", len(messages))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "chatcmpl-test",
				"choices": []map[string]any{
					{
						"message": map[string]string{
							"role":    "assistant",
							"content": replyContent,
						},
					},
				},
				"usage": map[string]int{
					"total_tokens": 42,
				},
			})
		case "/images/generations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": time.Now().Unix(),
				"data": []map[string]string{
					{
						"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
					},
				},
			})
		case "/audio/transcriptions":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"text": "Тестовое голосовое сообщение пользователя",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// 2. Setup Showcase Bot with AI Client
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	aiClient := ai.NewClient(mockServer.URL, "test-api-key")

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())
	RegisterAIScenario(botInstance, hood, aiClient)

	userID := int64(777)
	chatID := int64(888)

	makeCallbackCtx := func(payload string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: ""},
			},
			Callback: &maxbot.Callback{
				CallbackID: "cb_ai_test",
				Payload:    payload,
				User:       &maxbot.User{UserID: userID},
			},
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	makeTextCtx := func(text string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: text},
			},
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	makeVoiceCtx := func() *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type: maxbot.UploadTypeAudio,
							Payload: map[string]any{
								"url": mockServer.URL + "/dummy.ogg",
							},
						},
					},
				},
			},
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	makeImageCtx := func(text, imgURL string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body: maxbot.MessageBody{
					Text: text,
					Attachments: []maxbot.Attachment{
						{
							Type: maxbot.UploadTypeImage,
							Payload: map[string]any{
								"url": imgURL,
							},
						},
					},
				},
			},
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	key := fsm.SessionKey{UserID: userID, ChatID: chatID}

	// Step A: Navigate to AI Hub
	if err := botInstance.Router().Handle(makeCallbackCtx("scen:ai")); err != nil {
		t.Fatalf("navigate to scen:ai failed: %v", err)
	}

	// Step B: Enter AI Chat Mode
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:chat:start")); err != nil {
		t.Fatalf("start ai chat failed: %v", err)
	}
	st, _ := storage.GetState(context.Background(), key)
	if st != "AI_STEP_CHAT" {
		t.Fatalf("expected state AI_STEP_CHAT, got %s", st)
	}

	// Step C: Send multiple messages to test multi-turn chat and 10-message sliding window
	for i := 1; i <= 8; i++ {
		text := fmt.Sprintf("Вопрос пользователя номер %d", i)
		if err := botInstance.Router().Handle(makeTextCtx(text)); err != nil {
			t.Fatalf("send message %d failed: %v", i, err)
		}
	}

	// After 8 user messages and 8 assistant replies, total history generated is 16.
	// But sliding window must cap it to exactly 10!
	fsmCtx := fsm.NewFSMContext(context.Background(), storage, key)
	history := loadChatHistory(fsmCtx)
	if len(history) != 10 {
		t.Fatalf("expected sliding window to preserve exactly 10 messages, got %d", len(history))
	}
	// Verify last message is assistant reply
	if history[len(history)-1].Role != ai.RoleAssistant {
		t.Fatalf("expected last message to be assistant role, got %s", history[len(history)-1].Role)
	}

	// Step D: Model Selection Submenu and Model Switching
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:models")); err != nil {
		t.Fatalf("navigate to ai:models failed: %v", err)
	}
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:set_model:fast")); err != nil {
		t.Fatalf("set model to fast failed: %v", err)
	}
	if curModel, _ := fsmCtx.Get("ai_active_chat_model"); curModel != "fast" {
		t.Fatalf("expected active model to be fast, got %v", curModel)
	}
	st, _ = storage.GetState(context.Background(), key)
	if st != "AI_STEP_CHAT" {
		t.Fatalf("expected state AI_STEP_CHAT after set_model, got %s", st)
	}

	// Step E: Clear context
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:clear_context")); err != nil {
		t.Fatalf("clear context failed: %v", err)
	}
	historyAfterClear := loadChatHistory(fsmCtx)
	if len(historyAfterClear) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(historyAfterClear))
	}
	st, _ = storage.GetState(context.Background(), key)
	if st != "AI_STEP_CHAT" {
		t.Fatalf("expected state AI_STEP_CHAT after clear_context, got %s", st)
	}

	// Step E: Enter Text-to-Image Mode
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:image:start")); err != nil {
		t.Fatalf("start ai image failed: %v", err)
	}
	st, _ = storage.GetState(context.Background(), key)
	if st != "AI_STEP_IMAGE" {
		t.Fatalf("expected state AI_STEP_IMAGE, got %s", st)
	}

	// Test Preset Image Generation
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:preset:cyberpunk")); err != nil {
		t.Fatalf("preset cyberpunk generation failed: %v", err)
	}

	// Test Custom Text Prompt Image Generation
	if err := botInstance.Router().Handle(makeTextCtx("Футуристический робот в саду")); err != nil {
		t.Fatalf("custom image prompt failed: %v", err)
	}

	// Test Retry Image Generation with Last Prompt
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:image:retry")); err != nil {
		t.Fatalf("retry image generation failed: %v", err)
	}

	// Test Slash Command /image with prompt
	if err := botInstance.Router().Handle(makeTextCtx("/image Кот на скейтборде")); err != nil {
		t.Fatalf("/image command with prompt failed: %v", err)
	}

	// Test Slash Command /img with prompt
	if err := botInstance.Router().Handle(makeTextCtx("/img Неоновый закат")); err != nil {
		t.Fatalf("/img command with prompt failed: %v", err)
	}

	// Test Slash Command /image without arguments (opens image start screen)
	if err := botInstance.Router().Handle(makeTextCtx("/image")); err != nil {
		t.Fatalf("/image command without arguments failed: %v", err)
	}

	// Step F: Enter Voice Mode
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:voice:start")); err != nil {
		t.Fatalf("start ai voice failed: %v", err)
	}
	st, _ = storage.GetState(context.Background(), key)
	if st != "AI_STEP_VOICE" {
		t.Fatalf("expected state AI_STEP_VOICE, got %s", st)
	}

	// Test Voice Demo Simulation Button
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:voice:demo")); err != nil {
		t.Fatalf("voice demo button failed: %v", err)
	}

	// Test Voice Message Attachment
	if err := botInstance.Router().Handle(makeVoiceCtx()); err != nil {
		t.Fatalf("voice attachment message failed: %v", err)
	}

	// Step G: Enter Vision Mode
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:vision:start")); err != nil {
		t.Fatalf("start ai vision failed: %v", err)
	}
	st, _ = storage.GetState(context.Background(), key)
	if st != "AI_STEP_VISION" {
		t.Fatalf("expected state AI_STEP_VISION, got %s", st)
	}

	// Test Vision Demo Button
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:vision:demo")); err != nil {
		t.Fatalf("vision demo button failed: %v", err)
	}

	// Test Vision Image Attachment with prompt "Распознай картинку"
	if err := botInstance.Router().Handle(makeImageCtx("Распознай картинку", mockServer.URL+"/photo.jpg")); err != nil {
		t.Fatalf("image vision attachment failed: %v", err)
	}

	// Step H: Verify Under-The-Hood Technical Cards
	cards := []string{"ai_chat", "ai_image", "ai_voice", "ai_vision"}
	for _, cardKey := range cards {
		hood.mu.RLock()
		card, ok := hood.cards[cardKey]
		hood.mu.RUnlock()
		if !ok {
			t.Fatalf("expected technical card %s to be registered", cardKey)
		}
		if card.Endpoint == "" || card.Method == "" || card.FSMState == "" {
			t.Fatalf("card %s missing required metadata: %+v", cardKey, card)
		}
	}
}

func TestOnIdleMessageContextPreservation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			var req ai.ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			userMsgCount := 0
			for _, m := range req.Messages {
				if m.Role == ai.RoleUser {
					userMsgCount++
				}
			}
			resp := map[string]any{
				"id":      "chatcmpl-test",
				"model":   req.Model,
				"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": fmt.Sprintf("Ответ на вопрос %d", userMsgCount)}}},
				"usage":   map[string]any{"total_tokens": 10},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	aiClient := ai.NewClient(srv.URL, "test-key")
	hood := NewHoodEngine()
	hood.SetAIAvailable(true)

	botInstance := bot.NewWithClient(nil)
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()
	botInstance.Use(fsm.Middleware(storage))

	RegisterMenuRoutes(botInstance, hood, aiClient)
	RegisterAIScenario(botInstance, hood, aiClient)

	key := fsm.SessionKey{UserID: 101, ChatID: 101}
	makeTextCtx := func(text string) *bot.Context {
		c := bot.NewContext(context.Background(), nil, &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: 101, FirstName: "Tester"},
				Recipient: &maxbot.MessageRecipient{ChatID: 101, UserID: 101, ChatType: "dialog"},
				Body:      maxbot.MessageBody{Text: text},
			},
		})
		c.Set("fsm_context", fsm.NewFSMContext(context.Background(), storage, key))
		return c
	}

	// Message 1 from Idle: "Запомни кодовое слово"
	if err := botInstance.Router().Handle(makeTextCtx("Запомни кодовое слово")); err != nil {
		t.Fatalf("first idle message failed: %v", err)
	}

	fsmCtx := fsm.NewFSMContext(context.Background(), storage, key)
	hist1 := loadChatHistory(fsmCtx)
	if len(hist1) != 2 {
		t.Fatalf("expected 2 messages in history after first message, got %d", len(hist1))
	}
	if hist1[0].Content != "Запомни кодовое слово" || hist1[0].Role != ai.RoleUser {
		t.Fatalf("unexpected user message: %+v", hist1[0])
	}

	st, _ := storage.GetState(context.Background(), key)
	if st != "AI_STEP_CHAT" {
		t.Fatalf("expected state transitioned to AI_STEP_CHAT, got %s", st)
	}

	// Message 2: "Какое было слово?"
	if err := botInstance.Router().Handle(makeTextCtx("Какое было слово?")); err != nil {
		t.Fatalf("second message failed: %v", err)
	}

	hist2 := loadChatHistory(fsmCtx)
	if len(hist2) != 4 {
		t.Fatalf("expected 4 messages in history after second message, got %d", len(hist2))
	}
	if hist2[2].Content != "Какое было слово?" || hist2[2].Role != ai.RoleUser {
		t.Fatalf("unexpected second user message: %+v", hist2[2])
	}
}

func TestVisionImageRecognitionPrompt(t *testing.T) {
	var receivedMessages []any

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			var raw map[string]any
			_ = json.NewDecoder(r.Body).Decode(&raw)
			receivedMessages, _ = raw["messages"].([]any)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "chatcmpl-vision",
				"choices": []map[string]any{
					{
						"message": map[string]string{
							"role":    "assistant",
							"content": "На картинке изображен рыжий кот в круглых очках.",
						},
					},
				},
				"usage": map[string]int{"total_tokens": 50},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	aiClient := ai.NewClient(mockServer.URL, "test-api-key")
	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))

	RegisterMenuRoutes(botInstance, hood, aiClient)
	RegisterAIScenario(botInstance, hood, aiClient)

	// User sends photo with text "Распознай картинку" from idle mode
	c := bot.NewContext(context.Background(), nil, &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender:    &maxbot.User{UserID: 202, FirstName: "Alex"},
			Recipient: &maxbot.MessageRecipient{ChatID: 202, UserID: 202, ChatType: "dialog"},
			Body: maxbot.MessageBody{
				Text: "Распознай картинку",
				Attachments: []maxbot.Attachment{
					{
						Type: maxbot.UploadTypeImage,
						Payload: map[string]any{
							"url": mockServer.URL + "/cat.jpg",
						},
					},
				},
			},
		},
	})
	key := fsm.SessionKey{UserID: 202, ChatID: 202}
	c.Set("fsm_context", fsm.NewFSMContext(context.Background(), storage, key))

	if err := botInstance.Router().Handle(c); err != nil {
		t.Fatalf("handle vision image failed: %v", err)
	}

	if len(receivedMessages) < 2 {
		t.Fatalf("expected at least 2 messages in vision request, got %d", len(receivedMessages))
	}

	// Verify history saved
	fsmCtx := fsm.NewFSMContext(context.Background(), storage, key)
	history := loadChatHistory(fsmCtx)
	if len(history) != 2 {
		t.Fatalf("expected 2 messages in history (user vision prompt + assistant reply), got %d", len(history))
	}
	if history[1].Content != "На картинке изображен рыжий кот в круглых очках." {
		t.Fatalf("unexpected reply in history: %s", history[1].Content)
	}
}
