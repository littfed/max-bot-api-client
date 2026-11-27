package images

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/storage"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

func makeTestPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func makeTestJPEGBytes() []byte {
	// Minimal JPEG SOI + APP0 header recognized by http.DetectContentType.
	return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}
}

func makeTestWEBPBytes() []byte {
	// Minimal RIFF....WEBP header recognized by http.DetectContentType.
	return []byte{'R', 'I', 'F', 'F', 0x24, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}
}

func newTestService(t *testing.T, gw AIGateway) (*Service, *MemoryStore, *storage.MemoryStorage) {
	t.Helper()
	st := NewMemoryStore()
	memStorage := storage.NewMemoryStorage("https://cdn.example.com")
	pipeline := NewPipeline(gw)
	svc := NewService(st, pipeline, memStorage)
	return svc, st, memStorage
}

func TestValidateImageBytes(t *testing.T) {
	pngData := makeTestPNGBytes(t)
	jpegData := makeTestJPEGBytes()
	webpData := makeTestWEBPBytes()

	tests := []struct {
		name     string
		data     []byte
		wantMIME string
		wantExt  string
		wantErr  error
	}{
		{
			name:     "valid png",
			data:     pngData,
			wantMIME: "image/png",
			wantExt:  ".png",
		},
		{
			name:     "valid jpeg",
			data:     jpegData,
			wantMIME: "image/jpeg",
			wantExt:  ".jpg",
		},
		{
			name:     "valid webp",
			data:     webpData,
			wantMIME: "image/webp",
			wantExt:  ".webp",
		},
		{
			name:    "plain text rejected",
			data:    []byte("hello world plain text"),
			wantErr: ErrInvalidImage,
		},
		{
			name:    "empty rejected",
			data:    nil,
			wantErr: ErrInvalidImage,
		},
		{
			name:    "too large rejected",
			data:    make([]byte, MaxUploadBytes+1),
			wantErr: ErrImageTooLarge,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mimeType, ext, err := ValidateImageBytes(tc.data)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mimeType != tc.wantMIME {
				t.Fatalf("expected mime %q, got %q", tc.wantMIME, mimeType)
			}
			if ext != tc.wantExt {
				t.Fatalf("expected ext %q, got %q", tc.wantExt, ext)
			}
		})
	}
}

func TestService_CreateAndRun_Generate(t *testing.T) {
	pngOut := makeTestPNGBytes(t)
	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			return pngOut, "", nil
		},
	}
	svc, _, memStorage := newTestService(t, gw)

	proj, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID:      101,
		Mode:        ModeGenerate,
		Prompt:      "Minimalist coffee cup",
		AspectRatio: "16:9",
		ImageModel:  "flux",
	})
	if err != nil {
		t.Fatalf("CreateAndRun failed: %v", err)
	}
	if proj.Status != StatusReady {
		t.Fatalf("expected status %q, got %q", StatusReady, proj.Status)
	}
	if proj.CurrentVersion != 1 {
		t.Fatalf("expected version 1, got %d", proj.CurrentVersion)
	}
	if proj.CurrentImageKey == "" || proj.CurrentImageURL == "" {
		t.Fatalf("expected non-empty image key and URL, got key=%q url=%q", proj.CurrentImageKey, proj.CurrentImageURL)
	}
	if len(proj.Revisions) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(proj.Revisions))
	}
	if proj.Revisions[0].ActionType != ActionGenerate || proj.Revisions[0].PipelineUsed != PipelineGenerations {
		t.Fatalf("unexpected revision metadata: %+v", proj.Revisions[0])
	}

	gotBytes, _, err := memStorage.GetObject(context.Background(), proj.CurrentImageKey)
	if err != nil {
		t.Fatalf("expected object in storage: %v", err)
	}
	if !bytes.Equal(gotBytes, pngOut) {
		t.Fatalf("stored bytes mismatch")
	}
}

func TestService_CreateAndRun_AIFailureSetsFailedStatus(t *testing.T) {
	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			return nil, "", errors.New("ai: gateway timeout")
		},
	}
	svc, st, _ := newTestService(t, gw)

	_, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID: 101,
		Mode:   ModeGenerate,
		Prompt: "Failing prompt",
	})
	if err == nil {
		t.Fatal("expected error on AI failure")
	}

	projects, listErr := st.ListProjectsByUser(context.Background(), 101)
	if listErr != nil {
		t.Fatalf("list projects failed: %v", listErr)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 persisted project, got %d", len(projects))
	}
	if projects[0].Status != StatusFailed {
		t.Fatalf("expected StatusFailed, got %q", projects[0].Status)
	}
	if !strings.Contains(projects[0].ErrorMessage, "gateway timeout") {
		t.Fatalf("expected error message to be saved, got %q", projects[0].ErrorMessage)
	}
}

func TestService_Refine_SuccessAndFailurePreservesV1(t *testing.T) {
	v1Bytes := makeTestPNGBytes(t)
	v2Bytes := append(makeTestPNGBytes(t), []byte("v2")...)

	shouldFail := false
	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			return v1Bytes, "", nil
		},
		editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
			if shouldFail {
				return nil, "", errors.New("ai: upstream 500")
			}
			return v2Bytes, "", nil
		},
	}
	svc, _, _ := newTestService(t, gw)

	proj, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID: 7,
		Mode:   ModeGenerate,
		Prompt: "Initial cat",
	})
	if err != nil {
		t.Fatalf("CreateAndRun failed: %v", err)
	}
	v1URL := proj.CurrentImageURL
	v1Key := proj.CurrentImageKey

	// Failure during Refine must preserve v1 intact and set ErrorMessage.
	shouldFail = true
	if _, err := svc.Refine(context.Background(), 7, proj.ID, "Make it blue", ""); err == nil {
		t.Fatal("expected Refine to fail when AI fails")
	}
	afterFail, err := svc.GetProject(context.Background(), 7, proj.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if afterFail.CurrentVersion != 1 || afterFail.CurrentImageURL != v1URL || afterFail.CurrentImageKey != v1Key {
		t.Fatalf("v1 must be preserved on AI failure, got version=%d url=%q", afterFail.CurrentVersion, afterFail.CurrentImageURL)
	}
	if afterFail.ErrorMessage == "" {
		t.Fatal("expected ErrorMessage to be recorded on failure")
	}

	// Success during Refine creates v2.
	shouldFail = false
	refined, err := svc.Refine(context.Background(), 7, proj.ID, "Add sunglasses", "gpt-image-1")
	if err != nil {
		t.Fatalf("Refine failed: %v", err)
	}
	if refined.CurrentVersion != 2 {
		t.Fatalf("expected version 2, got %d", refined.CurrentVersion)
	}
	if refined.CurrentImageURL == v1URL {
		t.Fatal("expected new URL for v2")
	}
	if refined.ErrorMessage != "" {
		t.Fatalf("expected ErrorMessage cleared, got %q", refined.ErrorMessage)
	}
	if len(refined.Revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(refined.Revisions))
	}
}

func TestService_Restore_CreatesV3PointingToV1(t *testing.T) {
	v1Bytes := makeTestPNGBytes(t)
	v2Bytes := append(makeTestPNGBytes(t), []byte("v2")...)

	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			return v1Bytes, "", nil
		},
		editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
			return v2Bytes, "", nil
		},
	}
	svc, _, _ := newTestService(t, gw)

	proj, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID: 11,
		Mode:   ModeGenerate,
		Prompt: "Mountain lake",
	})
	if err != nil {
		t.Fatalf("CreateAndRun failed: %v", err)
	}
	v1Key := proj.CurrentImageKey
	v1URL := proj.CurrentImageURL

	if _, err := svc.Refine(context.Background(), 11, proj.ID, "Add sunset", ""); err != nil {
		t.Fatalf("Refine failed: %v", err)
	}

	restored, err := svc.Restore(context.Background(), 11, proj.ID, 1)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	if restored.CurrentVersion != 3 {
		t.Fatalf("expected version 3 after restore, got %d", restored.CurrentVersion)
	}
	if restored.CurrentImageKey != v1Key || restored.CurrentImageURL != v1URL {
		t.Fatalf("expected v3 to point to v1 key/url, got key=%q url=%q", restored.CurrentImageKey, restored.CurrentImageURL)
	}
	if len(restored.Revisions) != 3 {
		t.Fatalf("expected 3 revisions, got %d", len(restored.Revisions))
	}
	v3 := restored.Revisions[2]
	if v3.Version != 3 || v3.ActionType != ActionRestore || v3.PipelineUsed != PipelineRestore {
		t.Fatalf("unexpected v3 revision: %+v", v3)
	}
	if v3.ImageKey != v1Key || v3.ImageURL != v1URL {
		t.Fatalf("expected v3 revision to point to v1, got %+v", v3)
	}
}

func TestService_GetProject_Ownership(t *testing.T) {
	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			return makeTestPNGBytes(t), "", nil
		},
	}
	svc, _, _ := newTestService(t, gw)

	proj, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID: 100,
		Mode:   ModeGenerate,
		Prompt: "Private project",
	})
	if err != nil {
		t.Fatalf("CreateAndRun failed: %v", err)
	}

	if _, err := svc.GetProject(context.Background(), 200, proj.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for different user, got %v", err)
	}
}

func TestModule_Callbacks(t *testing.T) {
	pngOut := makeTestPNGBytes(t)
	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			return pngOut, "", nil
		},
		editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
			return pngOut, "", nil
		},
	}
	svc, _, _ := newTestService(t, gw)
	mod := New(svc, "https://app.example.com/images")

	if mod.ID() != "images" {
		t.Fatalf("expected ID 'images', got %q", mod.ID())
	}
	if mod.Title() != "Фотостудия" {
		t.Fatalf("expected Title 'Фотостудия', got %q", mod.Title())
	}

	user := &store.User{ID: 42}
	state := &core.State{UserID: 42, ModeID: "images", CurrentStep: StepIdle, Payload: json.RawMessage("{}")}

	// 1. img:presets
	res, err := mod.HandleCallback(context.Background(), user, state, &core.IncomingCallback{Data: "img:presets"})
	if err != nil {
		t.Fatalf("img:presets failed: %v", err)
	}
	if !res.Handled || len(res.Buttons) == 0 {
		t.Fatalf("expected handled result with buttons for img:presets")
	}

	// 2. img:preset:bg_replace
	res, err = mod.HandleCallback(context.Background(), user, state, &core.IncomingCallback{Data: "img:preset:bg_replace"})
	if err != nil {
		t.Fatalf("img:preset:bg_replace failed: %v", err)
	}
	if !res.Handled || !strings.Contains(res.Text, "Удалить / заменить фон") {
		t.Fatalf("expected preset title in response, got %q", res.Text)
	}

	// 3. img:try:social_poster
	res, err = mod.HandleCallback(context.Background(), user, state, &core.IncomingCallback{Data: "img:try:social_poster"})
	if err != nil {
		t.Fatalf("img:try:social_poster failed: %v", err)
	}
	if !res.Handled || res.NewStep != StepReady {
		t.Fatalf("expected StepReady after img:try:social_poster, got %q", res.NewStep)
	}
	if !strings.Contains(res.Text, "https://cdn.example.com/") {
		t.Fatalf("expected generated image URL in text, got %q", res.Text)
	}
}
