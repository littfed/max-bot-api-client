package main

import (
	"context"
	"encoding/base64"
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

func TestResolveImageBytes(t *testing.T) {
	// Sample small PNG payload
	sampleBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	b64Data := base64.StdEncoding.EncodeToString(sampleBytes)

	// Mock file server for URL resolution
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(sampleBytes)
	}))
	defer ts.Close()

	tests := []struct {
		name      string
		item      gma.ImageData
		wantLen   int
		wantError bool
	}{
		{
			name: "base64 payload decoded directly",
			item: gma.ImageData{
				B64JSON: b64Data,
			},
			wantLen:   len(sampleBytes),
			wantError: false,
		},
		{
			name: "url payload downloaded via http",
			item: gma.ImageData{
				URL: ts.URL + "/image.png",
			},
			wantLen:   len(sampleBytes),
			wantError: false,
		},
		{
			name:      "empty data produces error",
			item:      gma.ImageData{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveImageBytes(context.Background(), tt.item)
			if (err != nil) != tt.wantError {
				t.Fatalf("resolveImageBytes() error = %v, wantError = %v", err, tt.wantError)
			}
			if !tt.wantError && len(got) != tt.wantLen {
				t.Errorf("len(got) = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestRegisterImageHandlers(t *testing.T) {
	var requestedPrompt string
	gmaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/images/generations" {
			var req gma.ImageGenerationRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			requestedPrompt = req.Prompt

			resp := gma.ImageGenerationResponse{
				Created: 1700000000,
				Data: []gma.ImageData{
					{
						URL: "https://example.com/art.png",
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
		User:       &maxbot.User{UserID: 999},
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Text: "/image futuristic sky train",
			},
		},
	}

	ctx := bot.NewContext(context.Background(), nil, update)
	err := b.Router().Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error in image handler: %v", err)
	}

	if requestedPrompt != "futuristic sky train" {
		t.Errorf("expected prompt 'futuristic sky train', got %q", requestedPrompt)
	}
}
