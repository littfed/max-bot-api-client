package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/logger"
)

func TestAIClientWorkflow(t *testing.T) {
	// 1. Setup mock AI gateway
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Auth check
		if r.Header.Get("Authorization") != "Bearer test-key-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []string{"gpt-4o-mini", "flux"}})

		case "/v1/chat/completions":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    "chatcmpl-123",
				"model": req["model"],
				"choices": []map[string]any{
					{
						"message": map[string]string{
							"role":    "assistant",
							"content": "Привет! Я AI-ассистент в MAX.",
						},
					},
				},
				"usage": map[string]int{
					"prompt_tokens":     15,
					"completion_tokens": 8,
					"total_tokens":      23,
				},
			})

		case "/v1/images/generations":
			fakeImg := []byte("fake_png_image_bytes")
			b64 := base64.StdEncoding.EncodeToString(fakeImg)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": time.Now().Unix(),
				"data": []map[string]string{
					{"b64_json": b64},
				},
			})

		case "/v1/audio/transcriptions":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"text": "Сколько стоит доставка в Москву?",
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL+"/v1", "test-key-secret", WithTimeout(5*time.Second))

	ctx := context.Background()

	// 1. Ping
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// 2. Chat
	chatResp, err := client.Chat(ctx, &ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []ChatMessage{
			{Role: RoleUser, Content: "Привет"},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if chatResp.Content != "Привет! Я AI-ассистент в MAX." {
		t.Fatalf("unexpected content: %s", chatResp.Content)
	}
	if chatResp.TotalTokens != 23 {
		t.Fatalf("expected total tokens 23, got %d", chatResp.TotalTokens)
	}

	// 3. Image Generation
	imgResp, err := client.GenerateImage(ctx, &ImageRequest{
		Prompt: "Футуристичный робот-бариста в неоновом кафе",
		Model:  "flux",
		Size:   "512x512",
	})
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}
	if string(imgResp.Data) != "fake_png_image_bytes" {
		t.Fatalf("unexpected image bytes: %s", string(imgResp.Data))
	}

	// 4. Audio Transcription
	audioResp, err := client.TranscribeAudio(ctx, &AudioRequest{
		AudioBytes: []byte("fake_audio_wav_data"),
		Filename:   "voice.wav",
		Model:      "whisper-1",
	})
	if err != nil {
		t.Fatalf("TranscribeAudio failed: %v", err)
	}
	if audioResp.Text != "Сколько стоит доставка в Москву?" {
		t.Fatalf("unexpected transcription text: %s", audioResp.Text)
	}

	// 5. Vision Analysis
	visionResp, err := client.Vision(ctx, &VisionRequest{
		Prompt:     "Распознай картинку",
		ImageURL:   "https://example.com/test.jpg",
		ImageBytes: []byte("fake_image_bytes"),
	})
	if err != nil {
		t.Fatalf("Vision failed: %v", err)
	}
	if visionResp.Content != "Привет! Я AI-ассистент в MAX." {
		t.Fatalf("unexpected vision content: %s", visionResp.Content)
	}
}

func TestAIClientErrors(t *testing.T) {
	client := NewClient("http://127.0.0.1:54321/v1", "invalid-key")
	ctx := context.Background()

	// 1. Ping on offline endpoint
	err := client.Ping(ctx)
	if err == nil {
		t.Fatal("expected ping error on offline server")
	}

	// 2. Empty messages
	_, err = client.Chat(ctx, &ChatRequest{})
	if err == nil {
		t.Fatal("expected error on empty messages")
	}

	// 3. Empty image prompt
	_, err = client.GenerateImage(ctx, &ImageRequest{})
	if err == nil {
		t.Fatal("expected error on empty prompt")
	}

	// 4. Empty audio
	_, err = client.TranscribeAudio(ctx, &AudioRequest{})
	if err == nil {
		t.Fatal("expected error on empty audio")
	}
}

func TestAIClientStructuredLogging(t *testing.T) {
	var buf bytes.Buffer
	testLog := logger.New(
		logger.WithLevel(slog.LevelDebug),
		logger.WithOutput(&buf),
	)

	ctx := logger.WithContext(context.Background(), testLog.With(
		slog.String("trace_id", "tr_test_ai_123"),
	))

	// Mock server returning an error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"model_overloaded"}}`))
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, "sk-test-secret")
	_, err := client.Chat(ctx, &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{NewTextMessage(RoleUser, "test")},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	out := buf.String()
	if !strings.Contains(out, "tr_test_ai_123") {
		t.Errorf("expected trace_id in log output: %s", out)
	}
	if !strings.Contains(out, "model_overloaded") {
		t.Errorf("expected response_body with model_overloaded in log output: %s", out)
	}
	if !strings.Contains(out, "ai chat gateway error response") {
		t.Errorf("expected error log message: %s", out)
	}
}
