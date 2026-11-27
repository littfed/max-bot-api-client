package images

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockAIGateway struct {
	chatFn          func(ctx context.Context, model, systemPrompt, userPrompt string) (string, error)
	listModelsFn    func(ctx context.Context) ([]string, error)
	visionFn        func(ctx context.Context, prompt, imageDataURI string) (string, error)
	generateImageFn func(ctx context.Context, prompt, model, size string) ([]byte, string, error)
	editImageFn     func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error)

	chatCalls          int
	visionCalls        int
	generateImageCalls int
	editImageCalls     int
}

func (m *mockAIGateway) Chat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	m.chatCalls++
	if m.chatFn != nil {
		return m.chatFn(ctx, model, systemPrompt, userPrompt)
	}
	return userPrompt, nil
}

func (m *mockAIGateway) ListModels(ctx context.Context) ([]string, error) {
	if m.listModelsFn != nil {
		return m.listModelsFn(ctx)
	}
	return nil, nil
}

func (m *mockAIGateway) Vision(ctx context.Context, prompt, imageDataURI string) (string, error) {
	m.visionCalls++
	if m.visionFn != nil {
		return m.visionFn(ctx, prompt, imageDataURI)
	}
	return "A studio portrait with soft lighting", nil
}

func (m *mockAIGateway) GenerateImage(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
	m.generateImageCalls++
	if m.generateImageFn != nil {
		return m.generateImageFn(ctx, prompt, model, size)
	}
	return []byte("generated-bytes"), "", nil
}

func (m *mockAIGateway) EditImage(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
	m.editImageCalls++
	if m.editImageFn != nil {
		return m.editImageFn(ctx, imageBytes, prompt, model, size)
	}
	return []byte("edited-bytes"), "", nil
}

func TestPipeline_AspectRatioToSize(t *testing.T) {
	tests := []struct {
		ratio string
		want  string
	}{
		{ratio: "1:1", want: "1024x1024"},
		{ratio: "16:9", want: "1344x768"},
		{ratio: "9:16", want: "768x1344"},
		{ratio: "4:3", want: "1152x896"},
		{ratio: "", want: "1024x1024"},
		{ratio: "unknown", want: "1024x1024"},
	}

	for _, tc := range tests {
		if got := aspectRatioToSize(tc.ratio); got != tc.want {
			t.Errorf("aspectRatioToSize(%q) = %q, want %q", tc.ratio, got, tc.want)
		}
	}
}

func TestPipeline_Generate_ReturnsPipelineGenerations(t *testing.T) {
	var gotPrompt, gotModel, gotSize string
	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			gotPrompt = prompt
			gotModel = model
			gotSize = size
			return []byte("png-data"), "", nil
		},
	}

	p := NewPipeline(gw)
	imgBytes, pipelineUsed, err := p.Generate(context.Background(), "a neon cyber cat", "flux", "16:9")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if pipelineUsed != PipelineGenerations {
		t.Fatalf("pipelineUsed = %q, want %q", pipelineUsed, PipelineGenerations)
	}
	if string(imgBytes) != "png-data" {
		t.Fatalf("imgBytes = %q, want %q", string(imgBytes), "png-data")
	}
	if gotPrompt != "a neon cyber cat" {
		t.Errorf("prompt = %q, want %q", gotPrompt, "a neon cyber cat")
	}
	if gotModel != "flux" {
		t.Errorf("model = %q, want %q", gotModel, "flux")
	}
	if gotSize != "1344x768" {
		t.Errorf("size = %q, want %q", gotSize, "1344x768")
	}
	if gw.chatCalls != 0 {
		t.Errorf("expected 0 chat calls for English prompt, got %d", gw.chatCalls)
	}
}

func TestPipeline_Generate_CyrillicPromptTranslationAndFallback(t *testing.T) {
	t.Run("translates Cyrillic prompt via Chat", func(t *testing.T) {
		var gotPrompt string
		gw := &mockAIGateway{
			chatFn: func(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
				return "A futuristic city at sunset", nil
			},
			generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
				gotPrompt = prompt
				return []byte("img"), "", nil
			},
		}
		p := NewPipeline(gw)
		_, used, err := p.Generate(context.Background(), "Футуристичный город на закате", "flux", "1:1")
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if used != PipelineGenerations {
			t.Fatalf("used = %q, want %q", used, PipelineGenerations)
		}
		if gw.chatCalls != 1 {
			t.Errorf("expected 1 chat call, got %d", gw.chatCalls)
		}
		if gotPrompt != "A futuristic city at sunset" {
			t.Errorf("gotPrompt = %q, want translated prompt", gotPrompt)
		}
	})

	t.Run("falls back to raw prompt when Chat fails", func(t *testing.T) {
		var gotPrompt string
		gw := &mockAIGateway{
			chatFn: func(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
				return "", errors.New("chat unavailable")
			},
			generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
				gotPrompt = prompt
				return []byte("img"), "", nil
			},
		}
		p := NewPipeline(gw)
		raw := "Кот в космосе"
		_, used, err := p.Generate(context.Background(), raw, "flux", "1:1")
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if used != PipelineGenerations {
			t.Fatalf("used = %q, want %q", used, PipelineGenerations)
		}
		if gotPrompt != raw {
			t.Errorf("gotPrompt = %q, want raw prompt %q", gotPrompt, raw)
		}
	})
}

func TestPipeline_Generate_ResolvesURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("downloaded-image-bytes"))
	}))
	defer srv.Close()

	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			return nil, srv.URL + "/result.png", nil
		},
	}

	p := NewPipeline(gw)
	imgBytes, used, err := p.Generate(context.Background(), "mountain lake", "flux", "4:3")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if used != PipelineGenerations {
		t.Fatalf("used = %q, want %q", used, PipelineGenerations)
	}
	if string(imgBytes) != "downloaded-image-bytes" {
		t.Fatalf("imgBytes = %q, want %q", string(imgBytes), "downloaded-image-bytes")
	}
}

func TestPipeline_Edit_PrimarySucceeds(t *testing.T) {
	var gotBytes []byte
	var gotModel, gotSize string
	gw := &mockAIGateway{
		editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
			gotBytes = imageBytes
			gotModel = model
			gotSize = size
			return []byte("edited-result"), "", nil
		},
	}

	p := NewPipeline(gw)
	src := []byte("source-image")
	imgBytes, used, err := p.Edit(context.Background(), src, "remove background", "gpt-image-1", "9:16")
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if used != PipelineEdits {
		t.Fatalf("used = %q, want %q", used, PipelineEdits)
	}
	if string(imgBytes) != "edited-result" {
		t.Fatalf("imgBytes = %q, want %q", string(imgBytes), "edited-result")
	}
	if string(gotBytes) != "source-image" || gotModel != "gpt-image-1" || gotSize != "768x1344" {
		t.Errorf("unexpected EditImage args: bytes=%q model=%q size=%q", string(gotBytes), gotModel, gotSize)
	}
	if gw.visionCalls != 0 || gw.chatCalls != 0 || gw.generateImageCalls != 0 {
		t.Errorf("expected no fallback calls, got vision=%d chat=%d gen=%d", gw.visionCalls, gw.chatCalls, gw.generateImageCalls)
	}
}

func TestPipeline_Edit_VisionFallbackOnUnsupported(t *testing.T) {
	unsupportedErrors := []error{
		errors.New("ai: gateway error (status 404): not found"),
		errors.New("ai: gateway error (status 405): method not allowed"),
		errors.New("ai: gateway error (status 501): not implemented"),
		errors.New("endpoint is not supported for this provider"),
		errors.New("unsupported operation"),
		errors.New("unknown endpoint /images/edits"),
		errors.New("invalid model for edits"),
	}

	for _, editErr := range unsupportedErrors {
		t.Run(editErr.Error(), func(t *testing.T) {
			var gotDataURI string
			var gotGenPrompt string
			gw := &mockAIGateway{
				editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
					return nil, "", editErr
				},
				visionFn: func(ctx context.Context, prompt, imageDataURI string) (string, error) {
					gotDataURI = imageDataURI
					return "A red sports car parked on wet asphalt under neon lights", nil
				},
				chatFn: func(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
					return "A blue sports car parked on wet asphalt under neon lights", nil
				},
				generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
					gotGenPrompt = prompt
					return []byte("fallback-generated"), "", nil
				},
			}

			p := NewPipeline(gw)
			// Minimal JPEG magic bytes so MIME detection yields image/jpeg.
			src := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
			imgBytes, used, err := p.Edit(context.Background(), src, "make the car blue", "flux", "1:1")
			if err != nil {
				t.Fatalf("Edit() error = %v", err)
			}
			if used != PipelineVisionFallback {
				t.Fatalf("used = %q, want %q", used, PipelineVisionFallback)
			}
			if string(imgBytes) != "fallback-generated" {
				t.Fatalf("imgBytes = %q, want %q", string(imgBytes), "fallback-generated")
			}
			if !strings.HasPrefix(gotDataURI, "data:image/jpeg;base64,") {
				t.Errorf("expected JPEG data URI, got %q", gotDataURI)
			}
			if gw.visionCalls != 1 || gw.chatCalls != 1 || gw.generateImageCalls != 1 {
				t.Errorf("expected 1 vision, 1 chat, 1 generate call; got %d, %d, %d", gw.visionCalls, gw.chatCalls, gw.generateImageCalls)
			}
			if gotGenPrompt != "A blue sports car parked on wet asphalt under neon lights" {
				t.Errorf("gotGenPrompt = %q", gotGenPrompt)
			}
		})
	}
}

func TestPipeline_Edit_DoesNotFallbackOnTransientErrors(t *testing.T) {
	transientErrors := []error{
		errors.New("ai: gateway error (status 500): internal server error"),
		context.Canceled,
		context.DeadlineExceeded,
	}

	for _, editErr := range transientErrors {
		t.Run(editErr.Error(), func(t *testing.T) {
			gw := &mockAIGateway{
				editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
					return nil, "", editErr
				},
			}

			p := NewPipeline(gw)
			_, _, err := p.Edit(context.Background(), []byte("img"), "brighten", "flux", "1:1")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if gw.visionCalls != 0 || gw.chatCalls != 0 || gw.generateImageCalls != 0 {
				t.Errorf("expected no fallback calls on transient error, got vision=%d chat=%d gen=%d",
					gw.visionCalls, gw.chatCalls, gw.generateImageCalls)
			}
		})
	}
}

func TestPipeline_ListImageModels(t *testing.T) {
	t.Run("filters image models from gateway", func(t *testing.T) {
		gw := &mockAIGateway{
			listModelsFn: func(ctx context.Context) ([]string, error) {
				return []string{"gpt-4o", "flux-dev", "gpt-image-1", "whisper-1"}, nil
			},
		}
		p := NewPipeline(gw)
		models, err := p.ListImageModels(context.Background())
		if err != nil {
			t.Fatalf("ListImageModels() error = %v", err)
		}
		if len(models) != 2 {
			t.Fatalf("expected 2 image models, got %d (%+v)", len(models), models)
		}
		if models[0].ID != "flux-dev" || models[1].ID != "gpt-image-1" {
			t.Errorf("unexpected models: %+v", models)
		}
	})

	t.Run("falls back to defaults when gateway fails", func(t *testing.T) {
		gw := &mockAIGateway{
			listModelsFn: func(ctx context.Context) ([]string, error) {
				return nil, errors.New("gateway down")
			},
		}
		p := NewPipeline(gw)
		models, err := p.ListImageModels(context.Background())
		if err != nil {
			t.Fatalf("ListImageModels() error = %v", err)
		}
		if len(models) < 2 {
			t.Fatalf("expected fallback default models, got %+v", models)
		}
	})
}
