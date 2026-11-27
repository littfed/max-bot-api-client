package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/api"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/images"
	"github.com/littfed/max-bot-api-client/internal/storage"
	"github.com/littfed/max-bot-api-client/pkg/store"
	_ "modernc.org/sqlite"
)

type mockImagesAIGateway struct {
	generatedPNG []byte
	editedPNG    []byte
	models       []string
}

func (m *mockImagesAIGateway) Chat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	return "enhanced prompt: " + userPrompt, nil
}

func (m *mockImagesAIGateway) ListModels(ctx context.Context) ([]string, error) {
	if len(m.models) > 0 {
		return m.models, nil
	}
	return []string{"flux", "gpt-image-1"}, nil
}

func (m *mockImagesAIGateway) Vision(ctx context.Context, prompt, imageDataURI string) (string, error) {
	return "A studio product photo", nil
}

func (m *mockImagesAIGateway) GenerateImage(ctx context.Context, prompt, model, size string) ([]byte, string, error) {
	return m.generatedPNG, "image/png", nil
}

func (m *mockImagesAIGateway) EditImage(ctx context.Context, imageBytes []byte, prompt, model, size string) ([]byte, string, error) {
	if len(m.editedPNG) > 0 {
		return m.editedPNG, "image/png", nil
	}
	return m.generatedPNG, "image/png", nil
}

func makeTestPNG(r, g, b uint8) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	c := color.RGBA{R: r, G: g, B: b, A: 255}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func setupTestImagesAPIServer(t *testing.T) *httptest.Server {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	sqlStore, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, active_mode_id, role, timezone, created_at, updated_at) VALUES
		(1, 'images', 'user', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(2, 'images', 'user', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("failed to seed test users: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO user_identities (user_id, platform, platform_user_id, username, first_name) VALUES
		(1, 'max', '1001', 'user1', 'UserOne'),
		(2, 'max', '1002', 'user2', 'UserTwo');
	`)
	if err != nil {
		t.Fatalf("failed to seed test identities: %v", err)
	}

	guard := core.NewMaintenanceGuard(sqlStore, nil)
	fsm := core.NewCompositeFSM(sqlStore, nil)
	lockMgr := core.NewLockManager(nil)
	orch := core.NewOrchestrator(sqlStore, fsm, lockMgr, guard)
	gw := &mockImagesAIGateway{
		generatedPNG: makeTestPNG(10, 120, 220),
		editedPNG:    makeTestPNG(220, 80, 40),
	}
	pipeline := images.NewPipeline(gw)
	objStorage := storage.NewMemoryStorage("/api/v1/images/assets")
	svc := images.NewService(images.NewMemoryStore(), pipeline, objStorage)
	mod := images.New(svc, "https://example.com/maxapp")

	router := api.NewRouter(api.RouterConfig{
		Store:           sqlStore,
		Guard:           guard,
		FSM:             fsm,
		Orchestrator:    orch,
		ImagesModule:    mod,
		BotToken:        "test-token",
		JWTSecret:       "test-jwt-secret",
		AllowDevHeaders: true,
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

func doImagesReq(t *testing.T, method, url, userID string, body io.Reader, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("NewRequest(%s %s): %v", method, url, err)
	}
	if userID != "" {
		req.Header.Set("X-Dev-User-ID", userID)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do(%s %s): %v", method, url, err)
	}
	return resp
}

func TestImagesAPI_ListPresets(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	resp := doImagesReq(t, http.MethodGet, srv.URL+"/api/v1/images/presets", "1", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload struct {
		Presets []images.Preset `json:"presets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Presets) != 8 {
		t.Fatalf("expected 8 presets, got %d", len(payload.Presets))
	}
}

func TestImagesAPI_CreateProject_InvalidMultipartFile(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("mode", "edit")
	_ = mw.WriteField("prompt", "Make it studio")
	fw, err := mw.CreateFormFile("source_image", "notes.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write([]byte("plain text is not an image"))
	_ = mw.Close()

	resp := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", &body, mw.FormDataContentType())
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400 for text/plain file, got %d: %s", resp.StatusCode, string(raw))
	}
}

func TestImagesAPI_GetProject_ForbiddenForOtherUser(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	createBody := strings.NewReader(`{"mode":"generate","prompt":"Minimalist coffee cup","aspect_ratio":"1:1"}`)
	createResp := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", createBody, "application/json")
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated && createResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(createResp.Body)
		t.Fatalf("expected 200/201 creating project, got %d: %s", createResp.StatusCode, string(raw))
	}

	var proj images.Project
	if err := json.NewDecoder(createResp.Body).Decode(&proj); err != nil {
		t.Fatalf("decode created project: %v", err)
	}

	getResp := doImagesReq(t, http.MethodGet, fmt.Sprintf("%s/api/v1/images/projects/%d", srv.URL, proj.ID), "2", nil, "")
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for other user, got %d", getResp.StatusCode)
	}
}

func TestImagesAPI_FullFlow(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	// 1. POST /api/v1/images/projects (multipart with valid PNG)
	var mpBody bytes.Buffer
	mw := multipart.NewWriter(&mpBody)
	_ = mw.WriteField("mode", "edit")
	_ = mw.WriteField("preset_id", "product_card")
	_ = mw.WriteField("prompt", "Белый фон и мягкая тень")
	_ = mw.WriteField("aspect_ratio", "1:1")
	_ = mw.WriteField("image_model", "flux")
	fw, err := mw.CreateFormFile("source_image", "product.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write(makeTestPNG(120, 120, 120))
	_ = mw.Close()

	createResp := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", &mpBody, mw.FormDataContentType())
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated && createResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(createResp.Body)
		t.Fatalf("create project failed: %d: %s", createResp.StatusCode, string(raw))
	}

	var proj images.Project
	if err := json.NewDecoder(createResp.Body).Decode(&proj); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	if proj.ID == 0 || proj.CurrentVersion != 1 || proj.CurrentImageURL == "" {
		t.Fatalf("unexpected created project: %+v", proj)
	}

	// 2. POST /api/v1/images/projects/{id}/refine
	refineBody := strings.NewReader(`{"prompt":"Сделай свет теплее","image_model":"gpt-image-1"}`)
	refineResp := doImagesReq(t, http.MethodPost, fmt.Sprintf("%s/api/v1/images/projects/%d/refine", srv.URL, proj.ID), "1", refineBody, "application/json")
	defer refineResp.Body.Close()
	if refineResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(refineResp.Body)
		t.Fatalf("refine failed: %d: %s", refineResp.StatusCode, string(raw))
	}

	var refined images.Project
	if err := json.NewDecoder(refineResp.Body).Decode(&refined); err != nil {
		t.Fatalf("decode refined project: %v", err)
	}
	if refined.CurrentVersion != 2 {
		t.Fatalf("expected CurrentVersion=2 after refine, got %d", refined.CurrentVersion)
	}

	// 3. POST /api/v1/images/projects/{id}/restore
	restoreBody := strings.NewReader(`{"version":1}`)
	restoreResp := doImagesReq(t, http.MethodPost, fmt.Sprintf("%s/api/v1/images/projects/%d/restore", srv.URL, proj.ID), "1", restoreBody, "application/json")
	defer restoreResp.Body.Close()
	if restoreResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(restoreResp.Body)
		t.Fatalf("restore failed: %d: %s", restoreResp.StatusCode, string(raw))
	}

	var restored images.Project
	if err := json.NewDecoder(restoreResp.Body).Decode(&restored); err != nil {
		t.Fatalf("decode restored project: %v", err)
	}
	if restored.CurrentVersion != 3 {
		t.Fatalf("expected CurrentVersion=3 after restore, got %d", restored.CurrentVersion)
	}
	if restored.CurrentImageURL != proj.CurrentImageURL {
		t.Fatalf("expected restored CurrentImageURL %q to equal v1 URL %q", restored.CurrentImageURL, proj.CurrentImageURL)
	}

	// 4. GET /api/v1/images/assets/{key...} (without auth header, simulating browser <img>)
	assetResp := doImagesReq(t, http.MethodGet, srv.URL+restored.CurrentImageURL, "", nil, "")
	defer assetResp.Body.Close()
	if assetResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(assetResp.Body)
		t.Fatalf("get asset failed: %d: %s", assetResp.StatusCode, string(raw))
	}
	if ct := assetResp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Fatalf("expected image/* Content-Type, got %q", ct)
	}
	assetBytes, _ := io.ReadAll(assetResp.Body)
	if len(assetBytes) == 0 {
		t.Fatalf("expected non-empty asset body")
	}

	// 5. DELETE /api/v1/images/projects/{id}
	delResp := doImagesReq(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/images/projects/%d", srv.URL, proj.ID), "1", nil, "")
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(delResp.Body)
		t.Fatalf("delete project failed: %d: %s", delResp.StatusCode, string(raw))
	}

	// Verify project is gone (404)
	afterDelResp := doImagesReq(t, http.MethodGet, fmt.Sprintf("%s/api/v1/images/projects/%d", srv.URL, proj.ID), "1", nil, "")
	defer afterDelResp.Body.Close()
	if afterDelResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", afterDelResp.StatusCode)
	}
}

func TestImagesAPI_Unauthorized(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/images/presets"},
		{http.MethodGet, "/api/v1/images/projects"},
	} {
		resp := doImagesReq(t, tc.method, srv.URL+tc.path, "", nil, "")
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s without auth: expected 401, got %d", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestImagesAPI_UploadTooLarge(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("mode", "edit")
	_ = mw.WriteField("prompt", "Studio shot")
	fw, err := mw.CreateFormFile("source_image", "huge.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	oversize := make([]byte, images.MaxUploadBytes+1024)
	copy(oversize, makeTestPNG(10, 20, 30))
	_, _ = fw.Write(oversize)
	_ = mw.Close()

	resp := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", &body, mw.FormDataContentType())
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 413 for >10 MiB upload, got %d: %s", resp.StatusCode, string(raw))
	}
}

func TestImagesAPI_ListModelsAndProjects(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	// GET /api/v1/images/models
	modelsResp := doImagesReq(t, http.MethodGet, srv.URL+"/api/v1/images/models", "1", nil, "")
	defer modelsResp.Body.Close()
	if modelsResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /models expected 200, got %d", modelsResp.StatusCode)
	}
	var modelsPayload struct {
		Models []images.ModelOption `json:"models"`
	}
	if err := json.NewDecoder(modelsResp.Body).Decode(&modelsPayload); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(modelsPayload.Models) == 0 {
		t.Fatalf("expected non-empty models list")
	}

	// Create a project for user 1
	createResp := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", strings.NewReader(`{"mode":"generate","prompt":"Neon cat"}`), "application/json")
	createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project expected 201, got %d", createResp.StatusCode)
	}

	// GET /api/v1/images/projects for user 1 -> 1 project
	list1Resp := doImagesReq(t, http.MethodGet, srv.URL+"/api/v1/images/projects", "1", nil, "")
	defer list1Resp.Body.Close()
	if list1Resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /projects expected 200, got %d", list1Resp.StatusCode)
	}
	var list1 struct {
		Projects []images.Project `json:"projects"`
	}
	if err := json.NewDecoder(list1Resp.Body).Decode(&list1); err != nil {
		t.Fatalf("decode user 1 projects: %v", err)
	}
	if len(list1.Projects) != 1 {
		t.Fatalf("expected 1 project for user 1, got %d", len(list1.Projects))
	}

	// GET /api/v1/images/projects for user 2 -> 0 projects
	list2Resp := doImagesReq(t, http.MethodGet, srv.URL+"/api/v1/images/projects", "2", nil, "")
	defer list2Resp.Body.Close()
	var list2 struct {
		Projects []images.Project `json:"projects"`
	}
	if err := json.NewDecoder(list2Resp.Body).Decode(&list2); err != nil {
		t.Fatalf("decode user 2 projects: %v", err)
	}
	if len(list2.Projects) != 0 {
		t.Fatalf("expected 0 projects for user 2, got %d", len(list2.Projects))
	}
}

func TestImagesAPI_NotFoundAndForbiddenActions(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	// 404 on nonexistent project
	nfResp := doImagesReq(t, http.MethodGet, srv.URL+"/api/v1/images/projects/999999", "1", nil, "")
	nfResp.Body.Close()
	if nfResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent project, got %d", nfResp.StatusCode)
	}

	// Create project as user 1
	createResp := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", strings.NewReader(`{"mode":"generate","prompt":"Glass vase"}`), "application/json")
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating project, got %d", createResp.StatusCode)
	}
	var proj images.Project
	if err := json.NewDecoder(createResp.Body).Decode(&proj); err != nil {
		t.Fatalf("decode project: %v", err)
	}

	// User 2 cannot refine, restore, or delete user 1's project -> 403
	refResp := doImagesReq(t, http.MethodPost, fmt.Sprintf("%s/api/v1/images/projects/%d/refine", srv.URL, proj.ID), "2", strings.NewReader(`{"prompt":"blue"}`), "application/json")
	refResp.Body.Close()
	if refResp.StatusCode != http.StatusForbidden {
		t.Fatalf("refine by other user: expected 403, got %d", refResp.StatusCode)
	}

	restResp := doImagesReq(t, http.MethodPost, fmt.Sprintf("%s/api/v1/images/projects/%d/restore", srv.URL, proj.ID), "2", strings.NewReader(`{"version":1}`), "application/json")
	restResp.Body.Close()
	if restResp.StatusCode != http.StatusForbidden {
		t.Fatalf("restore by other user: expected 403, got %d", restResp.StatusCode)
	}

	delResp := doImagesReq(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/images/projects/%d", srv.URL, proj.ID), "2", nil, "")
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusForbidden {
		t.Fatalf("delete by other user: expected 403, got %d", delResp.StatusCode)
	}
}

func TestImagesAPI_QuotaExceeded(t *testing.T) {
	srv := setupTestImagesAPIServer(t)

	// Invalid requests (400 / 404) must NOT consume quota.
	badCreate := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", strings.NewReader(`{"mode":"generate","prompt":""}`), "application/json")
	badCreate.Body.Close()
	if badCreate.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty prompt, got %d", badCreate.StatusCode)
	}

	badRefine := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects/999999/refine", "1", strings.NewReader(`{"prompt":"test"}`), "application/json")
	badRefine.Body.Close()
	if badRefine.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent project refine, got %d", badRefine.StatusCode)
	}

	// Default daily_quota from 00006_images_studio.sql is 50; all 50 valid requests must still succeed.
	for i := 0; i < 50; i++ {
		resp := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", strings.NewReader(`{"mode":"generate","prompt":"quota test"}`), "application/json")
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("request %d: expected 201, got %d", i+1, resp.StatusCode)
		}
	}

	// 51st request must return 429 Too Many Requests.
	overResp := doImagesReq(t, http.MethodPost, srv.URL+"/api/v1/images/projects", "1", strings.NewReader(`{"mode":"generate","prompt":"over quota"}`), "application/json")
	overResp.Body.Close()
	if overResp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when daily quota exceeded, got %d", overResp.StatusCode)
	}
}
