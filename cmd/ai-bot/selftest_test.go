//go:build integration

package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// TestLiveSelfTest performs a live smoke/self-test against GMA and MAX Bot API.
func TestLiveSelfTest(t *testing.T) {
	token := os.Getenv("MAX_BOT_TOKEN")
	if token == "" {
		token = loadTokenFromDotEnv()
	}

	gmaURL := os.Getenv("GMA_BASE_URL")
	if gmaURL == "" {
		gmaURL = gma.DefaultBaseURL
	}

	gmaKey := os.Getenv("GMA_API_KEY")
	if gmaKey == "" {
		gmaKey = "app-key-1"
	}

	chatModel := os.Getenv("GMA_CHAT_MODEL")
	if chatModel == "" {
		chatModel = "neosantara/archipelago-core-70b"
	}

	// 1. Health Probe against Go Model API
	t.Run("01_GMA_HealthProbe", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, gmaURL+"/health", nil)
		if err != nil {
			t.Fatalf("failed to create health check request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GMA health check failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected GMA HTTP 200, got %d", resp.StatusCode)
		}
	})

	// 2. MAX Bot API Identity Verification
	if token != "" {
		t.Run("02_MAX_Bot_Identity", func(t *testing.T) {
			client, err := maxbot.NewClient(token)
			if err != nil {
				t.Fatalf("failed to init client: %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			user, err := client.GetMe(ctx)
			if err != nil {
				t.Fatalf("failed to call GetMe on MAX API: %v", err)
			}
			if user.UserID == 0 {
				t.Errorf("expected non-zero bot user ID, got %d", user.UserID)
			}
			t.Logf("✓ MAX Bot Connected: @%s (Name: %s, ID: %d)", user.Username, user.FirstName, user.UserID)
		})
	}

	// 3. Live LLM Chat Completion via Go Model API
	t.Run("03_GMA_LiveChatCompletion", func(t *testing.T) {
		client := gma.NewClient(gmaURL, gmaKey, gma.WithTimeout(30*time.Second))
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		req := &gma.ChatCompletionRequest{
			Model: chatModel,
			Messages: []gma.ChatMessage{
				gma.NewTextMessage(gma.RoleSystem, "Ты дружелюбный ассистент."),
				gma.NewTextMessage(gma.RoleUser, "Ответь одним словом: 'ТЕСТ_ПРОЙДЕН'"),
			},
		}

		resp, err := client.ChatCompletion(ctx, req)
		if err != nil {
			t.Fatalf("live chat completion failed: %v", err)
		}

		answer := resp.Text()
		if len(answer) == 0 {
			t.Fatalf("received empty text from model %s", chatModel)
		}
		t.Logf("✓ Live model %s replied: %q (tokens total: %d)", chatModel, answer, resp.Usage.TotalTokens)
	})

	// 4. In-Memory Session & Ring Buffer
	t.Run("04_SessionManager_RingBuffer", func(t *testing.T) {
		sm := gma.NewSessionManager(3, 10*time.Minute)
		userID := int64(777001)

		sm.AddUserMessage(userID, "Вопрос 1")
		sm.AddAssistantMessage(userID, "Ответ 1")
		sm.AddUserMessage(userID, "Вопрос 2")
		sm.AddAssistantMessage(userID, "Ответ 2")

		history := sm.GetHistory(userID)
		if len(history) != 3 {
			t.Fatalf("expected ring buffer capped at 3, got %d", len(history))
		}
		if history[0].TextContent() != "Ответ 1" {
			t.Errorf("oldest message should be 'Ответ 1', got %q", history[0].TextContent())
		}
		t.Logf("✓ Ring buffer sliding window verified: %d messages retained", len(history))
	})

	// 5. Bot Router Command & Menu Simulation
	t.Run("05_BotRouter_CommandSimulation", func(t *testing.T) {
		gmaClient := gma.NewClient(gmaURL, gmaKey)
		sm := gma.NewSessionManager(10, 10*time.Minute)
		b := bot.NewWithClient(nil)

		RegisterMenuHandlers(b, sm)
		RegisterChatHandlers(b, gmaClient, sm, nil)

		// Test /start command
		startUpdate := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			User:       &maxbot.User{UserID: 12345},
			Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "/start"}},
		}
		ctx := bot.NewContext(context.Background(), nil, startUpdate)
		if err := b.Router().Handle(ctx); err != nil {
			t.Errorf("error handling /start: %v", err)
		}

		// Test /model command
		modelUpdate := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			User:       &maxbot.User{UserID: 12345},
			Message:    &maxbot.Message{Body: maxbot.MessageBody{Text: "/model"}},
		}
		ctx = bot.NewContext(context.Background(), nil, modelUpdate)
		if err := b.Router().Handle(ctx); err != nil {
			t.Errorf("error handling /model: %v", err)
		}

		// Test model selection callback
		cbUpdate := &maxbot.Update{
			Callback: &maxbot.Callback{
				Payload: "set:chat:codestral-2",
				User:    &maxbot.User{UserID: 12345},
			},
		}
		ctx = bot.NewContext(context.Background(), nil, cbUpdate)
		if err := b.Router().Handle(ctx); err != nil {
			t.Errorf("error handling callback: %v", err)
		}

		curChat, _, _ := sm.GetModels(12345)
		if curChat != "codestral-2" {
			t.Errorf("expected codestral-2, got %s", curChat)
		}
		t.Logf("✓ Router handling and model switching callback verified: %s", curChat)
	})
}
