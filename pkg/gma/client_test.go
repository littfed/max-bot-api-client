package gma

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatCompletion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Model != "@scout" {
			t.Errorf("expected model @scout, got %s", req.Model)
		}
		if len(req.Messages) == 0 {
			t.Errorf("expected messages, got none")
		}

		resp := ChatCompletionResponse{
			ID:    "chatcmpl-123",
			Model: "@scout",
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    RoleAssistant,
						Content: "Привет! Чем я могу помочь?",
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-key")
	res, err := client.ChatCompletion(context.Background(), &ChatCompletionRequest{
		Model: "@scout",
		Messages: []ChatMessage{
			NewTextMessage(RoleUser, "Привет"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Text() != "Привет! Чем я могу помочь?" {
		t.Errorf("unexpected answer: %q", res.Text())
	}
}

func TestGenerateImage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			http.NotFound(w, r)
			return
		}

		var req ImageGenerationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Prompt != "cyberpunk city" {
			t.Errorf("unexpected prompt: %s", req.Prompt)
		}

		resp := ImageGenerationResponse{
			Created: 1700000000,
			Data: []ImageData{
				{
					URL: "https://gma.littfed.ru/files/city.png",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "")
	res, err := client.GenerateImage(context.Background(), &ImageGenerationRequest{
		Prompt: "cyberpunk city",
		Model:  "flux",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Data) != 1 || res.Data[0].URL != "https://gma.littfed.ru/files/city.png" {
		t.Errorf("unexpected response data: %+v", res.Data)
	}
}

func TestMultimodalMessage(t *testing.T) {
	msg := NewMultimodalMessage("Что на картинке?", "https://example.com/cat.jpg")
	if msg.Role != RoleUser {
		t.Errorf("expected role user, got %s", msg.Role)
	}
	parts, ok := msg.Content.([]ChatContentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected 2 content parts, got %+v", msg.Content)
	}
	if parts[0].Text != "Что на картинке?" {
		t.Errorf("unexpected text: %s", parts[0].Text)
	}
	if parts[1].ImageURL == nil || parts[1].ImageURL.URL != "https://example.com/cat.jpg" {
		t.Errorf("unexpected image url: %+v", parts[1].ImageURL)
	}
}
