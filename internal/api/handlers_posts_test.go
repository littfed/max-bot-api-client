package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/api"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/posts"
	"github.com/littfed/max-bot-api-client/pkg/store"
	_ "modernc.org/sqlite"
)

type mockPostsAIGateway struct {
	chatResponses []string
	chatIdx       int
	imageURL      string
}

func (m *mockPostsAIGateway) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.chatIdx < len(m.chatResponses) {
		resp := m.chatResponses[m.chatIdx]
		m.chatIdx++
		return resp, nil
	}
	return `{"variants":[{"platform":"max","text":"Refined text","hashtags":["#test"],"cta":"Act now"}]}`, nil
}

func (m *mockPostsAIGateway) Vision(ctx context.Context, prompt, imageURL string) (string, error) {
	return "Image summary", nil
}

func (m *mockPostsAIGateway) GenerateImage(ctx context.Context, prompt, aspectRatio string) (string, error) {
	if m.imageURL != "" {
		return m.imageURL, nil
	}
	return "https://example.com/generated.png", nil
}

func setupTestPostsAPIServer(t *testing.T) (*httptest.Server, *core.CompositeFSM) {
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
		(1, 'posts', 'superadmin', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(999, 'posts', 'user', 'UTC+3', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("failed to seed test users: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO user_identities (user_id, platform, platform_user_id, username, first_name) VALUES
		(1, 'max', '1001', 'adminuser', 'AdminUser'),
		(999, 'max', '9999', 'testuser', 'TestUser');
	`)
	if err != nil {
		t.Fatalf("failed to seed test identities: %v", err)
	}

	fsm := core.NewCompositeFSM(sqlStore, nil)
	gw := &mockPostsAIGateway{
		chatResponses: []string{
			// 1. GenerateHooks
			`{"hooks":[{"id":"h1","title":"Боль","text":"Почему падают охваты?","angle":"problem"}]}`,
			// 2. GeneratePackage
			`{"variants":[{"platform":"max","text":"Первый пост для MAX","hashtags":["#контент"],"cta":"Подпишись"}],"image_prompt":"Minimalist studio desk"}`,
			// 3. Refine
			`{"variants":[{"platform":"max","text":"Обновленный пост для MAX","hashtags":["#контент"],"cta":"Напиши нам"}]}`,
		},
		imageURL: "https://example.com/studio.png",
	}
	postsSvc := posts.NewService(posts.NewMemoryStore(), posts.NewPipeline(gw))
	postsMod := posts.New(postsSvc, "https://example.com/webapp")

	router := api.NewRouter(api.RouterConfig{
		Store:           sqlStore,
		FSM:             fsm,
		PostsModule:     postsMod,
		BotToken:        testBotToken,
		JWTSecret:       testJWTSecret,
		AllowDevHeaders: true,
	})

	ts := httptest.NewServer(router)
	t.Cleanup(func() {
		ts.Close()
	})
	return ts, fsm
}

func TestPostsAPI_Unauthorized(t *testing.T) {
	ts, _ := setupTestPostsAPIServer(t)
	client := ts.Client()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/posts/state"},
		{http.MethodPost, "/api/v1/posts/profile"},
		{http.MethodPost, "/api/v1/posts/drafts"},
		{http.MethodPatch, "/api/v1/posts/drafts/1/brief"},
		{http.MethodPost, "/api/v1/posts/drafts/1/hooks"},
		{http.MethodPost, "/api/v1/posts/drafts/1/generate"},
		{http.MethodPost, "/api/v1/posts/drafts/1/refine"},
		{http.MethodPatch, "/api/v1/posts/drafts/1/variants/max"},
		{http.MethodPost, "/api/v1/posts/drafts/1/image"},
	}

	for _, ep := range endpoints {
		req, err := http.NewRequest(ep.method, ts.URL+ep.path, nil)
		if err != nil {
			t.Fatalf("NewRequest(%s %s) failed: %v", ep.method, ep.path, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Do(%s %s) failed: %v", ep.method, ep.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s without auth: expected 401, got %d", ep.method, ep.path, resp.StatusCode)
		}
	}
}

func TestPostsAPI_ValidationAndOwnership(t *testing.T) {
	ts, _ := setupTestPostsAPIServer(t)
	client := ts.Client()

	// 1. Validation error (400) when raw_idea and source_image_url are both empty
	badReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/posts/drafts", bytes.NewReader([]byte(`{"raw_idea":"","source_image_url":""}`)))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set("X-Init-Data", testMAXInitData())
	badResp, err := client.Do(badReq)
	if err != nil {
		t.Fatalf("POST invalid draft failed: %v", err)
	}
	badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty raw_idea and source_image_url, got %d", badResp.StatusCode)
	}

	// 2. User A (testMAXInitData -> user 999) creates a draft
	createBody, _ := json.Marshal(map[string]any{"raw_idea": "Идея пользователя A"})
	createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/posts/drafts", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Init-Data", testMAXInitData())
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("POST create draft failed: %v", err)
	}
	if createResp.StatusCode != http.StatusCreated {
		createResp.Body.Close()
		t.Fatalf("expected 201 Created, got %d", createResp.StatusCode)
	}
	var created struct {
		Draft posts.PostDraft `json:"draft"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		createResp.Body.Close()
		t.Fatalf("decode created draft: %v", err)
	}
	createResp.Body.Close()
	draftID := created.Draft.ID

	// 3. User B (testAdminInitData -> user 1) tries to access/modify User A's draft -> 404
	crossChecks := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, fmt.Sprintf("/api/v1/posts/state?draft_id=%d", draftID), nil},
		{http.MethodPatch, fmt.Sprintf("/api/v1/posts/drafts/%d/brief", draftID), map[string]any{"goal": "leads"}},
		{http.MethodPost, fmt.Sprintf("/api/v1/posts/drafts/%d/hooks", draftID), nil},
		{http.MethodPost, fmt.Sprintf("/api/v1/posts/drafts/%d/generate", draftID), map[string]any{"hook_id": "h1"}},
		{http.MethodPost, fmt.Sprintf("/api/v1/posts/drafts/%d/refine", draftID), map[string]any{"platform": "max", "instruction": "shorter"}},
		{http.MethodPatch, fmt.Sprintf("/api/v1/posts/drafts/%d/variants/max", draftID), map[string]any{"text": "hacked"}},
		{http.MethodPost, fmt.Sprintf("/api/v1/posts/drafts/%d/image", draftID), map[string]any{"prompt": "hacked"}},
	}

	for _, tc := range crossChecks {
		var bodyReader *bytes.Reader
		if tc.body != nil {
			b, _ := json.Marshal(tc.body)
			bodyReader = bytes.NewReader(b)
		} else {
			bodyReader = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(tc.method, ts.URL+tc.path, bodyReader)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Init-Data", testAdminInitData())
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s failed: %v", tc.method, tc.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("ownership check %s %s: expected 404, got %d", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestPostsAPI_HappyPath(t *testing.T) {
	ts, fsm := setupTestPostsAPIServer(t)
	client := ts.Client()

	doJSON := func(method, path string, body any, wantStatus int) *http.Response {
		t.Helper()
		var buf bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&buf).Encode(body); err != nil {
				t.Fatalf("encode body: %v", err)
			}
		}
		req, err := http.NewRequest(method, ts.URL+path, &buf)
		if err != nil {
			t.Fatalf("NewRequest(%s %s): %v", method, path, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Init-Data", testMAXInitData())
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Do(%s %s): %v", method, path, err)
		}
		if resp.StatusCode != wantStatus {
			resp.Body.Close()
			t.Fatalf("%s %s: expected status %d, got %d", method, path, wantStatus, resp.StatusCode)
		}
		return resp
	}

	// 1. POST /api/v1/posts/profile
	profResp := doJSON(http.MethodPost, "/api/v1/posts/profile", map[string]any{
		"name":              "Studio",
		"niche":             "SaaS",
		"target_audience":   "Founders",
		"tone_of_voice":     "confident",
		"default_platforms": []string{"max"},
		"is_default":        true,
	}, http.StatusOK)
	var profOut struct {
		Profile posts.BrandProfile `json:"profile"`
	}
	if err := json.NewDecoder(profResp.Body).Decode(&profOut); err != nil {
		profResp.Body.Close()
		t.Fatalf("decode profile: %v", err)
	}
	profResp.Body.Close()
	if profOut.Profile.ID == 0 || profOut.Profile.Name != "Studio" {
		t.Fatalf("unexpected profile: %+v", profOut.Profile)
	}

	// 2. POST /api/v1/posts/drafts
	draftResp := doJSON(http.MethodPost, "/api/v1/posts/drafts", map[string]any{
		"raw_idea": "Запуск новой фичи",
	}, http.StatusCreated)
	var draftOut struct {
		Draft posts.PostDraft `json:"draft"`
	}
	if err := json.NewDecoder(draftResp.Body).Decode(&draftOut); err != nil {
		draftResp.Body.Close()
		t.Fatalf("decode draft: %v", err)
	}
	draftResp.Body.Close()
	draftID := draftOut.Draft.ID
	if draftID == 0 {
		t.Fatal("expected non-zero draft ID")
	}

	// Verify active_draft_id synced in FSM
	fsmState, err := fsm.Get(context.Background(), 999, "posts")
	if err != nil {
		t.Fatalf("fsm.Get failed: %v", err)
	}
	var fsmPayload posts.FSMStatePayload
	if err := json.Unmarshal(fsmState.Payload, &fsmPayload); err != nil {
		t.Fatalf("unmarshal fsm payload: %v", err)
	}
	if fsmPayload.ActiveDraftID != draftID {
		t.Fatalf("expected FSM active_draft_id=%d, got %d", draftID, fsmPayload.ActiveDraftID)
	}

	// 3. GET /api/v1/posts/state
	stateResp := doJSON(http.MethodGet, "/api/v1/posts/state", nil, http.StatusOK)
	var stateOut posts.StudioState
	if err := json.NewDecoder(stateResp.Body).Decode(&stateOut); err != nil {
		stateResp.Body.Close()
		t.Fatalf("decode state: %v", err)
	}
	stateResp.Body.Close()
	if stateOut.ActiveDraft == nil || stateOut.ActiveDraft.ID != draftID {
		t.Fatalf("expected active_draft ID %d in state, got %+v", draftID, stateOut.ActiveDraft)
	}
	if len(stateOut.Drafts) != 1 || len(stateOut.Profiles) != 1 {
		t.Fatalf("expected 1 draft and 1 profile, got drafts=%d profiles=%d", len(stateOut.Drafts), len(stateOut.Profiles))
	}

	// 4. PATCH /api/v1/posts/drafts/{id}/brief
	briefResp := doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/posts/drafts/%d/brief", draftID), map[string]any{
		"goal":             "engagement",
		"tone":             "friendly",
		"target_platforms": []string{"max"},
	}, http.StatusOK)
	briefResp.Body.Close()

	// 5. POST /api/v1/posts/drafts/{id}/hooks
	hooksResp := doJSON(http.MethodPost, fmt.Sprintf("/api/v1/posts/drafts/%d/hooks", draftID), nil, http.StatusOK)
	var hooksOut struct {
		Draft posts.PostDraft `json:"draft"`
	}
	if err := json.NewDecoder(hooksResp.Body).Decode(&hooksOut); err != nil {
		hooksResp.Body.Close()
		t.Fatalf("decode hooks: %v", err)
	}
	hooksResp.Body.Close()
	if len(hooksOut.Draft.Hooks) == 0 {
		t.Fatal("expected generated hooks")
	}

	// 6. POST /api/v1/posts/drafts/{id}/generate
	genResp := doJSON(http.MethodPost, fmt.Sprintf("/api/v1/posts/drafts/%d/generate", draftID), map[string]any{
		"hook_id": hooksOut.Draft.Hooks[0].ID,
	}, http.StatusOK)
	var genOut struct {
		Draft posts.PostDraft `json:"draft"`
	}
	if err := json.NewDecoder(genResp.Body).Decode(&genOut); err != nil {
		genResp.Body.Close()
		t.Fatalf("decode generate: %v", err)
	}
	genResp.Body.Close()
	if len(genOut.Draft.Variants) == 0 || genOut.Draft.GeneratedImageURL != "https://example.com/studio.png" {
		t.Fatalf("unexpected generated draft: %+v", genOut.Draft)
	}

	// 7. POST /api/v1/posts/drafts/{id}/refine
	refResp := doJSON(http.MethodPost, fmt.Sprintf("/api/v1/posts/drafts/%d/refine", draftID), map[string]any{
		"platform":    "max",
		"instruction": "Сделай короче",
	}, http.StatusOK)
	var refOut struct {
		Draft posts.PostDraft `json:"draft"`
	}
	if err := json.NewDecoder(refResp.Body).Decode(&refOut); err != nil {
		refResp.Body.Close()
		t.Fatalf("decode refine: %v", err)
	}
	refResp.Body.Close()
	if len(refOut.Draft.Variants) == 0 || refOut.Draft.Variants[0].Text != "Обновленный пост для MAX" {
		t.Fatalf("unexpected refined variants: %+v", refOut.Draft.Variants)
	}

	// 8. PATCH /api/v1/posts/drafts/{id}/variants/{platform}
	varResp := doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/posts/drafts/%d/variants/max", draftID), map[string]any{
		"text":     "Ручная правка текста",
		"hashtags": []string{"#manual"},
		"cta":      "Жми",
	}, http.StatusOK)
	var varOut struct {
		Draft posts.PostDraft `json:"draft"`
	}
	if err := json.NewDecoder(varResp.Body).Decode(&varOut); err != nil {
		varResp.Body.Close()
		t.Fatalf("decode variant update: %v", err)
	}
	varResp.Body.Close()
	if varOut.Draft.Variants[0].Text != "Ручная правка текста" {
		t.Fatalf("unexpected manual variant: %+v", varOut.Draft.Variants)
	}

	// 9. POST /api/v1/posts/drafts/{id}/image
	imgResp := doJSON(http.MethodPost, fmt.Sprintf("/api/v1/posts/drafts/%d/image", draftID), map[string]any{
		"prompt": "Bright isometric studio",
	}, http.StatusOK)
	var imgOut struct {
		Draft posts.PostDraft `json:"draft"`
	}
	if err := json.NewDecoder(imgResp.Body).Decode(&imgOut); err != nil {
		imgResp.Body.Close()
		t.Fatalf("decode image regen: %v", err)
	}
	imgResp.Body.Close()
	if imgOut.Draft.ImagePrompt != "Bright isometric studio" || imgOut.Draft.GeneratedImageURL == "" {
		t.Fatalf("unexpected image regen result: %+v", imgOut.Draft)
	}
}
