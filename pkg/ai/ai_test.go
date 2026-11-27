package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func TestEditImage(t *testing.T) {
	t.Run("success multipart upload and b64_json response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/images/edits" {
				t.Fatalf("expected /images/edits, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
			}
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			if got := r.FormValue("prompt"); got != "make background dark" {
				t.Fatalf("unexpected prompt: %s", got)
			}
			if got := r.FormValue("model"); got != "gpt-image-1" {
				t.Fatalf("unexpected model: %s", got)
			}
			if got := r.FormValue("size"); got != "1024x1024" {
				t.Fatalf("unexpected size: %s", got)
			}
			if got := r.FormValue("response_format"); got != "b64_json" {
				t.Fatalf("unexpected response_format: %s", got)
			}
			f, fh, err := r.FormFile("image")
			if err != nil {
				t.Fatalf("missing image form file: %v", err)
			}
			_ = f.Close()
			if fh.Filename != "source.png" {
				t.Fatalf("unexpected filename: %s", fh.Filename)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"b64_json":"aGVsbG8="}]}`))
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "test-key")
		resp, err := client.EditImage(context.Background(), &ImageEditRequest{
			ImageBytes:     []byte{0x89, 0x50, 0x4E, 0x47},
			Filename:       "source.png",
			Prompt:         "make background dark",
			Model:          "gpt-image-1",
			Size:           "1024x1024",
			ResponseFormat: "b64_json",
		})
		if err != nil {
			t.Fatalf("EditImage error: %v", err)
		}
		if string(resp.Data) != "hello" {
			t.Fatalf("expected decoded bytes 'hello', got %q", string(resp.Data))
		}
		if resp.Model != "gpt-image-1" {
			t.Fatalf("expected model gpt-image-1, got %q", resp.Model)
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		client := NewClient("http://localhost:8080/v1", "test-key")
		ctx := context.Background()

		if _, err := client.EditImage(ctx, nil); err == nil {
			t.Fatal("expected error for nil request")
		}
		if _, err := client.EditImage(ctx, &ImageEditRequest{Prompt: "edit"}); err == nil {
			t.Fatal("expected error for empty ImageBytes")
		}
		if _, err := client.EditImage(ctx, &ImageEditRequest{ImageBytes: []byte{1, 2, 3}, Prompt: "   "}); err == nil {
			t.Fatal("expected error for empty Prompt")
		}
	})

	t.Run("non-2xx status error includes status code", func(t *testing.T) {
		for _, code := range []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented} {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"error":"not supported"}`))
			}))
			client := NewClient(srv.URL, "test-key")
			_, err := client.EditImage(context.Background(), &ImageEditRequest{
				ImageBytes: []byte{0x89, 0x50, 0x4E, 0x47},
				Prompt:     "edit",
			})
			srv.Close()
			if err == nil {
				t.Fatalf("expected error for status %d", code)
			}
			want := fmt.Sprintf("status %d", code)
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("expected %q in error, got: %v", want, err)
			}
		}
	})
}
