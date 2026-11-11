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

func TestExtractImageURL(t *testing.T) {
	tests := []struct {
		name    string
		msg     *maxbot.Message
		wantURL string
	}{
		{
			name:    "nil message returns empty",
			msg:     nil,
			wantURL: "",
		},
		{
			name: "payload map with direct url",
			msg: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type: maxbot.UploadTypeImage,
							Payload: map[string]any{
								"url": "https://example.com/photo.jpg",
							},
						},
					},
				},
			},
			wantURL: "https://example.com/photo.jpg",
		},
		{
			name: "payload map with file_url",
			msg: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type: "photo",
							Payload: map[string]any{
								"file_url": "https://example.com/receipt.png",
							},
						},
					},
				},
			},
			wantURL: "https://example.com/receipt.png",
		},
		{
			name: "payload string with https url",
			msg: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type:    maxbot.UploadTypeImage,
							Payload: "https://example.com/direct.jpg",
						},
					},
				},
			},
			wantURL: "https://example.com/direct.jpg",
		},
		{
			name: "nested photos map",
			msg: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type: "photo",
							Payload: map[string]any{
								"photos": map[string]any{
									"0": map[string]any{
										"url": "https://example.com/nested.png",
									},
								},
							},
						},
					},
				},
			},
			wantURL: "https://example.com/nested.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractImageURL(tt.msg)
			if got != tt.wantURL {
				t.Errorf("extractImageURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestRegisterVisionHandlers(t *testing.T) {
	var capturedModel string
	var capturedPrompt string

	gmaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req gma.ChatCompletionRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			capturedModel = req.Model

			// Find multimodal message
			for _, m := range req.Messages {
				if parts, ok := m.Content.([]any); ok {
					for _, p := range parts {
						if pMap, isMap := p.(map[string]any); isMap {
							if pMap["type"] == "text" {
								capturedPrompt, _ = pMap["text"].(string)
							}
						}
					}
				}
			}

			resp := gma.ChatCompletionResponse{
				Choices: []gma.ChatChoice{
					{
						Message: gma.ChatMessage{
							Role:    gma.RoleAssistant,
							Content: "Распознан котенок на зеленой траве.",
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
		User:       &maxbot.User{UserID: 888},
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "Что это за животное?",
				Attachments: []maxbot.Attachment{
					{
						Type: maxbot.UploadTypeImage,
						Payload: map[string]any{
							"url": "https://example.com/animal.jpg",
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

	if capturedModel != "@scout-vision" {
		t.Errorf("expected model @scout-vision, got %s", capturedModel)
	}
	if capturedPrompt != "Что это за животное?" {
		t.Errorf("expected prompt 'Что это за животное?', got %s", capturedPrompt)
	}
}
