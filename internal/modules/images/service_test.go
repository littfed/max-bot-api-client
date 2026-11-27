package images

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

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
	if afterFail.Status != StatusFailed {
		t.Fatalf("expected StatusFailed on AI failure, got %q", afterFail.Status)
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

func TestService_CreateAndRun_ModeEdit(t *testing.T) {
	srcPNG := makeTestPNGBytes(t)
	editedPNG := append(makeTestPNGBytes(t), []byte("edited")...)
	gw := &mockAIGateway{
		editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
			return editedPNG, "", nil
		},
	}
	svc, _, memStorage := newTestService(t, gw)

	// Invalid image bytes rejected.
	if _, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID:      15,
		Mode:        ModeEdit,
		Prompt:      "Clean background",
		SourceImage: []byte("not-an-image"),
	}); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected ErrInvalidImage, got %v", err)
	}

	// Valid source upload succeeds.
	proj, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID:      15,
		Mode:        ModeEdit,
		PresetID:    "bg_replace",
		Prompt:      "Replace with studio white",
		SourceImage: srcPNG,
	})
	if err != nil {
		t.Fatalf("CreateAndRun edit failed: %v", err)
	}
	if proj.SourceImageKey == "" || proj.SourceImageURL == "" {
		t.Fatalf("expected source image key and URL, got key=%q url=%q", proj.SourceImageKey, proj.SourceImageURL)
	}
	if proj.CurrentVersion != 1 || proj.Status != StatusReady {
		t.Fatalf("expected ready v1 project, got status=%q ver=%d", proj.Status, proj.CurrentVersion)
	}
	if _, _, err := memStorage.GetObject(context.Background(), proj.SourceImageKey); err != nil {
		t.Fatalf("expected source image in storage: %v", err)
	}
}

func TestService_DeleteProject_AndOwnership(t *testing.T) {
	srcPNG := makeTestPNGBytes(t)
	v1PNG := append(makeTestPNGBytes(t), []byte("v1")...)
	v2PNG := append(makeTestPNGBytes(t), []byte("v2")...)
	gw := &mockAIGateway{
		editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
			if strings.Contains(prompt, "v2") {
				return v2PNG, "", nil
			}
			return v1PNG, "", nil
		},
	}
	svc, st, memStorage := newTestService(t, gw)

	proj, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID:      50,
		Mode:        ModeEdit,
		Prompt:      "Initial edit",
		SourceImage: srcPNG,
	})
	if err != nil {
		t.Fatalf("CreateAndRun failed: %v", err)
	}
	refined, err := svc.Refine(context.Background(), 50, proj.ID, "Refine v2", "")
	if err != nil {
		t.Fatalf("Refine failed: %v", err)
	}

	// Ownership checks on Refine, Restore, DeleteProject.
	if _, err := svc.Refine(context.Background(), 99, proj.ID, "Hijack", ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden on Refine, got %v", err)
	}
	if _, err := svc.Restore(context.Background(), 99, proj.ID, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden on Restore, got %v", err)
	}
	if err := svc.DeleteProject(context.Background(), 99, proj.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden on DeleteProject, got %v", err)
	}

	keys := []string{
		refined.SourceImageKey,
		refined.Revisions[0].ImageKey,
		refined.Revisions[1].ImageKey,
	}

	if err := svc.DeleteProject(context.Background(), 50, proj.ID); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
	if _, err := st.GetProject(context.Background(), proj.ID); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound after delete, got %v", err)
	}
	for _, k := range keys {
		if _, _, err := memStorage.GetObject(context.Background(), k); !errors.Is(err, storage.ErrObjectNotFound) {
			t.Fatalf("expected object %q to be deleted from storage, got %v", k, err)
		}
	}
}

func TestModule_HandleMessage_And_ExtendedCallbacks(t *testing.T) {
	pngOut := makeTestPNGBytes(t)
	failRefine := false
	gw := &mockAIGateway{
		generateImageFn: func(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
			return pngOut, "", nil
		},
		editImageFn: func(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
			if failRefine {
				return nil, "", errors.New("ai: refine error")
			}
			return pngOut, "", nil
		},
	}
	svc, st, _ := newTestService(t, gw)
	mod := New(svc, "https://app.example.com/images")
	user := &store.User{ID: 77}

	// Non-img callback -> Handled: false.
	res, err := mod.HandleCallback(context.Background(), user, nil, &core.IncomingCallback{Data: "pst:menu"})
	if err != nil || res.Handled {
		t.Fatalf("expected Handled=false for non-img callback, got handled=%v err=%v", res.Handled, err)
	}

	// /start command -> home menu.
	res, err = mod.HandleMessage(context.Background(), user, nil, &core.IncomingMessage{Command: "start"})
	if err != nil || !res.Handled || res.NewStep != StepIdle {
		t.Fatalf("expected home menu on /start, got step=%q err=%v", res.NewStep, err)
	}

	// img:mode:generate -> StepAwaitingInput.
	res, err = mod.HandleCallback(context.Background(), user, nil, &core.IncomingCallback{Data: "img:mode:generate"})
	if err != nil || res.NewStep != StepAwaitingInput {
		t.Fatalf("expected StepAwaitingInput for img:mode:generate, got %q err=%v", res.NewStep, err)
	}

	// Text in StepAwaitingInput -> CreateAndRun -> StepReady.
	awaitState := &core.State{UserID: 77, ModeID: "images", CurrentStep: StepAwaitingInput, Payload: res.NewPayload}
	res, err = mod.HandleMessage(context.Background(), user, awaitState, &core.IncomingMessage{Text: "Neon cyberpunk fox"})
	if err != nil || res.NewStep != StepReady {
		t.Fatalf("expected StepReady after prompt, got %q err=%v", res.NewStep, err)
	}

	// Text in StepReady -> Refine -> v2.
	readyState := &core.State{UserID: 77, ModeID: "images", CurrentStep: StepReady, Payload: res.NewPayload}
	res, err = mod.HandleMessage(context.Background(), user, readyState, &core.IncomingMessage{Text: "Add rain"})
	if err != nil || res.NewStep != StepReady || !strings.Contains(res.Text, "(v2)") {
		t.Fatalf("expected v2 after refine, got text=%q err=%v", res.Text, err)
	}

	// Refine failure at v2 must preserve undo button!
	failRefine = true
	res, err = mod.HandleMessage(context.Background(), user, readyState, &core.IncomingMessage{Text: "Broken edit"})
	if err != nil || res.NewStep != StepReady {
		t.Fatalf("expected StepReady on refine failure, got %q err=%v", res.NewStep, err)
	}
	hasUndo := false
	for _, row := range res.Buttons {
		for _, btn := range row {
			if strings.HasPrefix(btn.Data, "img:undo:") {
				hasUndo = true
			}
		}
	}
	if !hasUndo {
		t.Fatalf("expected undo button to be preserved on v2 refine failure, buttons=%+v", res.Buttons)
	}
	failRefine = false

	// img:undo:<id> -> v3 restored.
	var p fsmPayload
	_ = json.Unmarshal(readyState.Payload, &p)
	res, err = mod.HandleCallback(context.Background(), user, readyState, &core.IncomingCallback{Data: "img:undo:" + strconvFormat(p.ActiveProjectID)})
	if err != nil || !strings.Contains(res.Text, "(v3)") {
		t.Fatalf("expected v3 after undo, got text=%q err=%v", res.Text, err)
	}

	// img:use: and img:mode:edit
	res, err = mod.HandleCallback(context.Background(), user, nil, &core.IncomingCallback{Data: "img:use:bg_replace"})
	if err != nil || res.NewStep != StepAwaitingInput {
		t.Fatalf("expected StepAwaitingInput for img:use:bg_replace, got %q err=%v", res.NewStep, err)
	}
	res, err = mod.HandleCallback(context.Background(), user, nil, &core.IncomingCallback{Data: "img:mode:edit"})
	if err != nil || res.NewStep != StepAwaitingInput {
		t.Fatalf("expected StepAwaitingInput for img:mode:edit, got %q err=%v", res.NewStep, err)
	}

	// Create a failed/unready project (CurrentVersion == 0) and verify img:history filters it and img:project: rejects it.
	unready := &Project{
		UserID:      77,
		Mode:        ModeGenerate,
		Title:       "Failed draft",
		ImageModel:  "flux",
		AspectRatio: "1:1",
		Status:      StatusFailed,
	}
	if err := st.CreateProject(context.Background(), unready); err != nil {
		t.Fatalf("create unready project: %v", err)
	}

	res, err = mod.HandleCallback(context.Background(), user, nil, &core.IncomingCallback{Data: "img:history"})
	if err != nil || !res.Handled {
		t.Fatalf("img:history failed: %v", err)
	}
	for _, row := range res.Buttons {
		for _, btn := range row {
			if strings.Contains(btn.Data, strconvFormat(unready.ID)) {
				t.Fatalf("expected unready project %d to be filtered from history", unready.ID)
			}
		}
	}

	res, err = mod.HandleCallback(context.Background(), user, nil, &core.IncomingCallback{Data: "img:project:" + strconvFormat(unready.ID)})
	if err != nil || res.NewStep != StepIdle {
		t.Fatalf("expected StepIdle for unready project, got %q err=%v", res.NewStep, err)
	}

	res, err = mod.HandleCallback(context.Background(), user, nil, &core.IncomingCallback{Data: "img:project:" + strconvFormat(p.ActiveProjectID)})
	if err != nil || res.NewStep != StepReady {
		t.Fatalf("expected StepReady for ready project, got %q err=%v", res.NewStep, err)
	}
}

func strconvFormat(id int64) string {
	return strconv.FormatInt(id, 10)
}

func TestService_UUIDKeysAndExampleAssetsFS(t *testing.T) {
	st := NewMemoryStore()
	obj := storage.NewMemoryStorage("/api/v1/images/assets")
	customExample := makeTestPNGBytes(t)

	var seenSrc []byte
	gw := &mockAIGateway{
		editImageFn: func(_ context.Context, imgBytes []byte, prompt, model, size string) ([]byte, string, error) {
			seenSrc = append([]byte(nil), imgBytes...)
			return makeTestPNGBytes(t), "edit", nil
		},
	}
	svc := NewService(st, NewPipeline(gw), obj)
	svc.SetExampleAssetsFS(fstest.MapFS{
		"presets/bg_replace_before.png": &fstest.MapFile{Data: customExample},
	})

	proj, err := svc.CreateAndRun(context.Background(), CreateProjectInput{
		UserID:           10,
		Mode:             ModeEdit,
		PresetID:         "bg_replace",
		Prompt:           "White studio background",
		UseExampleSource: true,
	})
	if err != nil {
		t.Fatalf("CreateAndRun failed: %v", err)
	}

	if !bytes.Equal(seenSrc, customExample) {
		t.Fatalf("expected CreateAndRun to load example image from exampleAssetsFS (got %d bytes, want %d)", len(seenSrc), len(customExample))
	}

	srcKeyRe := regexp.MustCompile(`^images/10/\d+/source_[0-9a-f]{32}\.png$`)
	v1KeyRe := regexp.MustCompile(`^images/10/\d+/v1_[0-9a-f]{32}\.png$`)
	if !srcKeyRe.MatchString(proj.SourceImageKey) {
		t.Fatalf("expected UUID source key, got %q", proj.SourceImageKey)
	}
	if !v1KeyRe.MatchString(proj.CurrentImageKey) {
		t.Fatalf("expected UUID v1 key, got %q", proj.CurrentImageKey)
	}
}

func TestValidateCreateProjectInput_ExtendedChecks(t *testing.T) {
	longModel := strings.Repeat("m", 129)
	cases := []struct {
		name    string
		input   CreateProjectInput
		wantErr bool
	}{
		{
			name: "valid aspect ratio and model",
			input: CreateProjectInput{
				Mode:        ModeGenerate,
				Prompt:      "A cat",
				AspectRatio: "16:9",
				ImageModel:  "flux",
			},
			wantErr: false,
		},
		{
			name: "invalid aspect ratio",
			input: CreateProjectInput{
				Mode:        ModeGenerate,
				Prompt:      "A cat",
				AspectRatio: "21:9",
			},
			wantErr: true,
		},
		{
			name: "image model too long",
			input: CreateProjectInput{
				Mode:       ModeGenerate,
				Prompt:     "A cat",
				ImageModel: longModel,
			},
			wantErr: true,
		},
		{
			name: "mismatched mode and preset_id",
			input: CreateProjectInput{
				Mode:     ModeGenerate,
				PresetID: "bg_replace", // ModeEdit preset
				Prompt:   "A cat",
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCreateProjectInput(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestModule_ChatPhotoUploadAndPresetUX(t *testing.T) {
	st := NewMemoryStore()
	obj := storage.NewMemoryStorage("/api/v1/images/assets")
	samplePNG := makeTestPNGBytes(t)

	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(samplePNG)
	}))
	defer imgSrv.Close()

	var editCalled int
	gw := &mockAIGateway{
		editImageFn: func(_ context.Context, imgBytes []byte, prompt, model, size string) ([]byte, string, error) {
			editCalled++
			if !bytes.Equal(imgBytes, samplePNG) {
				t.Fatalf("expected uploaded photo bytes in edit pipeline")
			}
			return makeTestPNGBytes(t), "edit", nil
		},
	}
	svc := NewService(st, NewPipeline(gw), obj)
	mod := New(svc, "https://app.example.com")
	user := &store.User{ID: 88}

	// 1. renderPresetDetail must use StepPresetBrowse
	detailRes, err := mod.HandleCallback(context.Background(), user, nil, &core.IncomingCallback{Data: "img:preset:bg_replace"})
	if err != nil || detailRes.NewStep != StepPresetBrowse {
		t.Fatalf("expected StepPresetBrowse for renderPresetDetail, got %q err=%v", detailRes.NewStep, err)
	}

	// 2. Photo with caption -> immediate CreateAndRun in edit mode
	res, err := mod.HandleMessage(context.Background(), user, nil, &core.IncomingMessage{
		Text:     "Сделай фон белым",
		ImageURL: imgSrv.URL + "/photo.png",
	})
	if err != nil || !res.Handled || res.NewStep != StepReady {
		t.Fatalf("expected StepReady after photo+caption, got step=%q err=%v", res.NewStep, err)
	}
	if editCalled != 1 {
		t.Fatalf("expected edit pipeline to be called once, got %d", editCalled)
	}

	// Verify projectActionButtons includes "Другой сценарий" (img:presets)
	hasPresetsBtn := false
	for _, row := range res.Buttons {
		for _, btn := range row {
			if btn.Data == "img:presets" {
				hasPresetsBtn = true
			}
		}
	}
	if !hasPresetsBtn {
		t.Fatalf("expected img:presets button in projectActionButtons, got %+v", res.Buttons)
	}

	// 3. Photo without caption -> StepAwaitingInput, then text -> CreateAndRun
	res2, err := mod.HandleMessage(context.Background(), user, nil, &core.IncomingMessage{
		ImageBytes: samplePNG,
	})
	if err != nil || !res2.Handled || res2.NewStep != StepAwaitingInput {
		t.Fatalf("expected StepAwaitingInput after photo without caption, got step=%q err=%v", res2.NewStep, err)
	}
	awaitState := &core.State{UserID: 88, ModeID: "images", CurrentStep: StepAwaitingInput, Payload: res2.NewPayload}
	res3, err := mod.HandleMessage(context.Background(), user, awaitState, &core.IncomingMessage{
		Text: "Убери тень",
	})
	if err != nil || !res3.Handled || res3.NewStep != StepReady {
		t.Fatalf("expected StepReady after follow-up text, got step=%q err=%v", res3.NewStep, err)
	}
	if editCalled != 2 {
		t.Fatalf("expected edit pipeline to be called twice, got %d", editCalled)
	}
}
