package livetest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/telemetry"
)

// TestShowcaseBotAIE2EJourney verifies the complete AI multimodal journey
// (chat with sliding window, text-to-image synthesis, whisper voice STT) end-to-end.
func TestShowcaseBotAIE2EJourney(t *testing.T) {
	// 1. Mock OpenAI-compatible AI Model Gateway
	mockGateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			reply := fmt.Sprintf("AI response to %d message(s)", len(messages))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "chat-e2e-123",
				"choices": []map[string]any{
					{
						"message": map[string]string{
							"role":    "assistant",
							"content": reply,
						},
					},
				},
				"usage": map[string]int{
					"total_tokens": 88,
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
				"text": "Привет, расскажи о возможностях бота",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockGateway.Close()

	// 2. Setup Showcase Bot Environment
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	collector := telemetry.NewCollector(telemetry.WithBufferSize(100))
	aiClient := ai.NewClient(mockGateway.URL, "test-token")

	b := bot.NewWithClient(nil)
	b.Use(fsm.Middleware(storage))
	b.Use(telemetry.Middleware(collector))

	// Simulate Scenario Routes
	userID := int64(1001)
	chatID := int64(2002)
	sessionKey := fsm.SessionKey{UserID: userID, ChatID: chatID}

	// Dynamic AI menu entry
	b.OnCallbackPayload("scen:ai", func(c *bot.Context) error {
		return c.AnswerNotification("Меню AI открыто")
	})

	// Conversational Chat Handlers
	b.OnCallbackPayload("ai:chat:start", func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}
		return c.AnswerNotification("Режим диалога активен")
	})

	b.OnState("AI_STEP_CHAT", func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		userText := c.Text()

		var history []ai.ChatMessage
		if raw, ok := fsmCtx.Get("ai_chat_history"); ok {
			_ = json.Unmarshal([]byte(raw.(string)), &history)
		}
		history = append(history, ai.ChatMessage{Role: ai.RoleUser, Content: userText})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}

		resp, err := aiClient.Chat(c.Ctx(), &ai.ChatRequest{
			Model:    "gpt-4o-mini",
			Messages: history,
		})
		if err != nil {
			return err
		}

		history = append(history, ai.ChatMessage{Role: ai.RoleAssistant, Content: resp.Content})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		histJSON, _ := json.Marshal(history)
		_ = fsmCtx.Set("ai_chat_history", string(histJSON))

		_, err = c.Reply(resp.Content)
		return err
	})

	b.OnCallbackPayload("ai:clear_context", func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.Set("ai_chat_history", "[]")
		}
		return c.AnswerNotification("Контекст очищен")
	})

	// Text-to-Image Handlers
	b.OnCallbackPayload("ai:image:start", func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_IMAGE")
		}
		return c.AnswerNotification("Режим генерации картинок")
	})

	b.OnState("AI_STEP_IMAGE", func(c *bot.Context) error {
		prompt := c.Text()
		imgResp, err := aiClient.GenerateImage(c.Ctx(), &ai.ImageRequest{
			Prompt: prompt,
			Model:  "flux",
		})
		if err != nil {
			return err
		}
		if len(imgResp.Data) == 0 {
			return fmt.Errorf("empty image data")
		}
		_, err = c.Reply(fmt.Sprintf("Изображение сгенерировано: %d байт", len(imgResp.Data)))
		return err
	})

	// Voice Whisper STT Handlers
	b.OnCallbackPayload("ai:voice:start", func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.SetState("AI_STEP_VOICE")
		}
		return c.AnswerNotification("Режим распознавания речи")
	})

	b.OnCallbackPayload("ai:voice:demo", func(c *bot.Context) error {
		demoWav := []byte("RIFF$\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00D\xac\x00\x00\x88X\x01\x00\x02\x00\x10\x00data\x00\x00\x00\x00")
		audioResp, err := aiClient.TranscribeAudio(c.Ctx(), &ai.AudioRequest{
			AudioBytes: demoWav,
			Filename:   "voice.wav",
			Model:      "whisper-1",
		})
		if err != nil {
			return err
		}
		if audioResp.Text == "" {
			return fmt.Errorf("empty audio transcription")
		}
		_, err = c.Reply(fmt.Sprintf("Вы сказали: %s", audioResp.Text))
		return err
	})

	// Context Generator Helper
	makeCtx := func(text string, payload ...string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: text},
			},
		}
		if len(payload) > 0 {
			u.Callback = &maxbot.Callback{
				CallbackID: "cb_ai_e2e",
				Payload:    payload[0],
				User:       &maxbot.User{UserID: userID},
			}
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	// 3. Execute Step-by-Step E2E Journey
	// Step 1: Open AI hub
	if err := b.Router().Handle(makeCtx("", "scen:ai")); err != nil {
		t.Fatalf("Step 1 scen:ai failed: %v", err)
	}

	// Step 2: Open Chat mode
	if err := b.Router().Handle(makeCtx("", "ai:chat:start")); err != nil {
		t.Fatalf("Step 2 ai:chat:start failed: %v", err)
	}
	state, _ := storage.GetState(context.Background(), sessionKey)
	if state != "AI_STEP_CHAT" {
		t.Fatalf("expected AI_STEP_CHAT, got %s", state)
	}

	// Step 3: Multi-turn conversation (6 turns -> 12 messages -> 10 sliding window)
	for turn := 1; turn <= 6; turn++ {
		msg := fmt.Sprintf("Turn %d message", turn)
		if err := b.Router().Handle(makeCtx(msg)); err != nil {
			t.Fatalf("Turn %d message handling failed: %v", turn, err)
		}
	}

	// Verify Sliding Window strictly capped at 10
	fsmCtx := fsm.NewFSMContext(context.Background(), storage, sessionKey)
	raw, ok := fsmCtx.Get("ai_chat_history")
	if !ok {
		t.Fatal("expected ai_chat_history to exist in FSM")
	}
	var hist []ai.ChatMessage
	_ = json.Unmarshal([]byte(raw.(string)), &hist)
	if len(hist) != 10 {
		t.Fatalf("expected sliding window length 10, got %d", len(hist))
	}

	// Step 4: Clear context
	if err := b.Router().Handle(makeCtx("", "ai:clear_context")); err != nil {
		t.Fatalf("Step 4 ai:clear_context failed: %v", err)
	}
	rawAfterClear, _ := fsmCtx.Get("ai_chat_history")
	_ = json.Unmarshal([]byte(rawAfterClear.(string)), &hist)
	if len(hist) != 0 {
		t.Fatalf("expected empty history after clear, got %d", len(hist))
	}

	// Step 5: Text-to-Image Generation
	if err := b.Router().Handle(makeCtx("", "ai:image:start")); err != nil {
		t.Fatalf("Step 5 ai:image:start failed: %v", err)
	}
	if err := b.Router().Handle(makeCtx("Cyberpunk city in neon rain")); err != nil {
		t.Fatalf("Image prompt handling failed: %v", err)
	}

	// Step 6: Voice Whisper STT
	if err := b.Router().Handle(makeCtx("", "ai:voice:start")); err != nil {
		t.Fatalf("Step 6 ai:voice:start failed: %v", err)
	}
	if err := b.Router().Handle(makeCtx("", "ai:voice:demo")); err != nil {
		t.Fatalf("Voice demo handling failed: %v", err)
	}

	t.Log("Showcase bot AI E2E journey successfully passed all checks.")
}

// TestLiveAIGatewayConnectivity tests live gateway ping if AI_GATEWAY_URL is provided in environment.
func TestLiveAIGatewayConnectivity(t *testing.T) {
	gatewayURL := strings.TrimSpace(os.Getenv("AI_GATEWAY_URL"))
	if gatewayURL == "" {
		t.Skip("Skipping live AI Gateway connectivity test: AI_GATEWAY_URL is not set")
	}

	apiKey := strings.TrimSpace(os.Getenv("AI_GATEWAY_KEY"))
	client := ai.NewClient(gatewayURL, apiKey, ai.WithTimeout(10*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("AI Gateway at %s is not reachable: %v", gatewayURL, err)
	}
	t.Logf("AI Gateway at %s successfully reachable.", gatewayURL)
}
