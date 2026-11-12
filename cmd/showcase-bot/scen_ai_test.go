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

	// Step D: Clear context
	if err := botInstance.Router().Handle(makeCallbackCtx("ai:clear_context")); err != nil {
		t.Fatalf("clear context failed: %v", err)
	}
	historyAfterClear := loadChatHistory(fsmCtx)
	if len(historyAfterClear) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(historyAfterClear))
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

	// Step G: Verify Under-The-Hood Technical Cards
	cards := []string{"ai_chat", "ai_image", "ai_voice"}
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
