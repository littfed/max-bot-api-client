package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestChatFlow(t *testing.T) {
	// Mock GMA Server
	gmaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req gma.ChatCompletionRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			resp := gma.ChatCompletionResponse{
				Choices: []gma.ChatChoice{
					{
						Message: gma.ChatMessage{
							Role:    gma.RoleAssistant,
							Content: "Ответ на ваш вопрос: " + req.Messages[len(req.Messages)-1].TextContent(),
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer gmaMock.Close()

	gmaClient := gma.NewClient(gmaMock.URL, "")
	sm := gma.NewSessionManager(10, 1*time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	b := bot.NewWithClient(nil)
	RegisterChatHandlers(b, gmaClient, sm, logger)

	// Simulate user message
	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		User: &maxbot.User{
			UserID: 555,
		},
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "Какая погода?",
			},
		},
	}

	ctx := bot.NewContext(context.Background(), nil, update)
	err := b.Router().Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error in chat handler: %v", err)
	}

	// Verify history stored in memory
	history := sm.GetHistory(555)
	if len(history) != 2 {
		t.Fatalf("expected 2 messages in session history (user + assistant), got %d", len(history))
	}
	if history[0].TextContent() != "Какая погода?" {
		t.Errorf("unexpected first message: %s", history[0].TextContent())
	}
}

func TestClearFlow(t *testing.T) {
	sm := gma.NewSessionManager(10, 1*time.Hour)
	sm.AddUserMessage(777, "предыдущий вопрос")
	sm.AddAssistantMessage(777, "предыдущий ответ")

	if len(sm.GetHistory(777)) != 2 {
		t.Fatalf("expected 2 messages before clear")
	}

	b := bot.NewWithClient(nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	RegisterChatHandlers(b, nil, sm, logger)

	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		User: &maxbot.User{
			UserID: 777,
		},
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "/clear",
			},
		},
	}

	ctx := bot.NewContext(context.Background(), nil, update)
	err := b.Router().Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error in clear handler: %v", err)
	}

	if len(sm.GetHistory(777)) != 0 {
		t.Errorf("expected empty history after /clear, got %d", len(sm.GetHistory(777)))
	}
}

func TestVisionFlow(t *testing.T) {
	visionCalled := false
	gmaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req gma.ChatCompletionRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Model == "@scout-vision" {
				visionCalled = true
			}
			resp := gma.ChatCompletionResponse{
				Choices: []gma.ChatChoice{
					{
						Message: gma.ChatMessage{
							Role:    gma.RoleAssistant,
							Content: "На картинке изображен кот.",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer gmaMock.Close()

	gmaClient := gma.NewClient(gmaMock.URL, "")
	sm := gma.NewSessionManager(10, 1*time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	b := bot.NewWithClient(nil)
	RegisterVisionHandlers(b, gmaClient, sm, logger)

	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		User: &maxbot.User{
			UserID: 888,
		},
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "Что это за зверь?",
				Attachments: []maxbot.Attachment{
					{
						Type: maxbot.UploadTypeImage,
						Payload: map[string]any{
							"url": "https://example.com/cat.png",
						},
					},
				},
			},
		},
	}

	ctx := bot.NewContext(context.Background(), nil, update)
	err := b.Router().Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error in vision handler: %v", err)
	}

	if !visionCalled {
		t.Errorf("expected vision model to be called")
	}
}

func TestImageGenerationFlow(t *testing.T) {
	genCalled := false
	gmaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/images/generations" {
			var req gma.ImageGenerationRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Prompt == "sunset over mountain" {
				genCalled = true
			}
			resp := gma.ImageGenerationResponse{
				Created: 1700000000,
				Data: []gma.ImageData{
					{
						URL: "https://example.com/mountain.png",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer gmaMock.Close()

	gmaClient := gma.NewClient(gmaMock.URL, "")
	sm := gma.NewSessionManager(10, 1*time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	b := bot.NewWithClient(nil)
	RegisterImageHandlers(b, gmaClient, sm, logger)

	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		User: &maxbot.User{
			UserID: 999,
		},
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "/image sunset over mountain",
			},
		},
	}

	ctx := bot.NewContext(context.Background(), nil, update)
	err := b.Router().Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error in image generation handler: %v", err)
	}

	if !genCalled {
		t.Errorf("expected image generation API to be called")
	}
}

func TestModelSelectionMenu(t *testing.T) {
	sm := gma.NewSessionManager(10, 1*time.Hour)
	b := bot.NewWithClient(nil)
	RegisterMenuHandlers(b, sm)

	update := &maxbot.Update{
		Callback: &maxbot.Callback{
			Payload: "set:chat:garda-core",
			User: &maxbot.User{
				UserID: 1234,
			},
		},
	}

	ctx := bot.NewContext(context.Background(), nil, update)
	err := b.Router().Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error in callback: %v", err)
	}

	chatModel, _, _ := sm.GetModels(1234)
	if chatModel != "garda-core" {
		t.Errorf("expected model garda-core, got %s", chatModel)
	}
}
