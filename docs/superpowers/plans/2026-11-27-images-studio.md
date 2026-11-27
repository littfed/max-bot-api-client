# Images Studio («Фотостудия») Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the 9th Bot Hub mode `images` («Фотостудия») supporting Text-to-Image generation, Image-to-Image editing, iterative refinement with revision history, preset scenarios with Before/After examples, S3/MinIO object storage (with local memory fallback), and hybrid Chat + Mini App UI.

**Architecture:** Modular monolith pattern matching `internal/modules/posts` and `internal/modules/reminders`. `internal/modules/images` owns the PostgreSQL store, declarative preset catalog, hybrid AI pipeline (`EditImage` `/v1/images/edits` primary + `Vision` + `GenerateImage` auto-fallback), and `BotModule` chat handler (`img:` prefix). `internal/storage` provides `ObjectStorage` (`S3Storage` via AWS/MinIO-compatible HTTP or `minio-go`, plus `MemoryStorage` fallback). `internal/api/handlers_images.go` exposes REST endpoints, and `web/src/components/modes/ImagesView.tsx` renders the 3-tab Mini App UI.

**Tech Stack:** Go 1.23+, PostgreSQL (`pgx`/`database/sql`), `pkg/ai`, S3/MinIO (`github.com/minio/minio-go/v7`), React 18, TypeScript, Tailwind CSS.

**Spec:** [docs/superpowers/specs/2026-11-27-images-studio-design.md](../specs/2026-11-27-images-studio-design.md)

## Global Constraints

- Mode ID: `images`, Title: `Фотостудия`, callback prefix: `img:`.
- Upload validation: only `image/jpeg`, `image/png`, `image/webp` detected via `http.DetectContentType`, max size `10 * 1024 * 1024` bytes (10 MB).
- Quota Guard: `create` (with generation/edit) and `refine` consume 1 daily quota unit; `list`, `get`, `restore`, `delete` do not.
- Ownership: every `projects/{id}` endpoint verifies `project.UserID == authUser.ID`, returning `403`/`404` otherwise.
- AI timeouts: 90s for image generation/edit, 15s for S3 operations.
- On AI failure during `refine` or `edit`, `current_image_url` of the last successful revision must remain intact.

---

### Task 1: Add `EditImage` (`/v1/images/edits`) to `pkg/ai`

**Files:**
- Modify: `pkg/ai/types.go:126-141`
- Modify: `pkg/ai/client.go:270-385`
- Test: `pkg/ai/ai_test.go`

**Interfaces:**
- Produces: `type ImageEditRequest struct { ImageBytes []byte; Filename string; Prompt string; Model string; Size string; ResponseFormat string }` and `func (c *Client) EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error)`

- [ ] **Step 1: Write the failing test in `pkg/ai/ai_test.go`**

```go
func TestEditImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			t.Fatalf("expected /images/edits, got %s", r.URL.Path)
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
		f, _, err := r.FormFile("image")
		if err != nil {
			t.Fatalf("missing image form file: %v", err)
		}
		_ = f.Close()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aGVsbG8="}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	resp, err := client.EditImage(context.Background(), &ImageEditRequest{
		ImageBytes: []byte{0x89, 0x50, 0x4E, 0x47},
		Filename:   "source.png",
		Prompt:     "make background dark",
		Model:      "gpt-image-1",
	})
	if err != nil {
		t.Fatalf("EditImage error: %v", err)
	}
	if string(resp.Data) != "hello" {
		t.Fatalf("expected decoded bytes 'hello', got %q", string(resp.Data))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/ai -run TestEditImage -v`
Expected: FAIL (`undefined: ImageEditRequest` / `client.EditImage undefined`)

- [ ] **Step 3: Implement `ImageEditRequest` and `Client.EditImage`**

Add `ImageEditRequest` to `pkg/ai/types.go` right after `ImageResponse`, and implement `EditImage` in `pkg/ai/client.go` using `multipart.NewWriter` (writing `image` file field + `prompt`, `model`, `size`, `response_format` form fields) and parsing the OpenAI-compatible `{ "data": [{ "b64_json": ..., "url": ... }] }` response.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/ai -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/ai/types.go pkg/ai/client.go pkg/ai/ai_test.go
git commit -m "feat(ai): add EditImage multipart support for /images/edits"
```

---

### Task 2: `internal/storage` (`ObjectStorage`, `S3Storage`, `MemoryStorage`)

**Files:**
- Create: `internal/storage/s3.go`
- Create: `internal/storage/s3_test.go`

**Interfaces:**
- Produces:
  - `type ObjectStorage interface { PutObject(ctx context.Context, key, contentType string, data []byte) (string, error); GetObject(ctx context.Context, key string) ([]byte, string, error); DeleteObject(ctx context.Context, key string) error }`
  - `type S3Config struct { Endpoint, Bucket, AccessKey, SecretKey, PublicURL string; UseSSL bool }`
  - `func NewObjectStorageFromEnv(ctx context.Context) (ObjectStorage, error)`
  - `func NewMemoryStorage(publicBaseURL string) *MemoryStorage`

- [ ] **Step 1: Write the failing test in `internal/storage/s3_test.go`**

```go
package storage

import (
	"context"
	"testing"
)

func TestMemoryStorageCRUD(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStorage("https://cdn.example.com")

	url, err := st.PutObject(ctx, "images/1/2/v1.png", "image/png", []byte("png-bytes"))
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if url != "https://cdn.example.com/images/1/2/v1.png" {
		t.Fatalf("unexpected url: %s", url)
	}

	data, ct, err := st.GetObject(ctx, "images/1/2/v1.png")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if ct != "image/png" || string(data) != "png-bytes" {
		t.Fatalf("unexpected data/ct: %q %q", string(data), ct)
	}

	if err := st.DeleteObject(ctx, "images/1/2/v1.png"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, _, err := st.GetObject(ctx, "images/1/2/v1.png"); err == nil {
		t.Fatal("expected error after delete")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage -v`
Expected: FAIL (`package internal/storage` does not exist)

- [ ] **Step 3: Implement `internal/storage/s3.go`**

Add `github.com/minio/minio-go/v7` (`go get github.com/minio/minio-go/v7`), implement `ObjectStorage` interface, `MemoryStorage` (thread-safe `sync.RWMutex` map), `S3Storage` (wrapping `*minio.Client`, ensuring bucket exists on init), and `NewObjectStorageFromEnv` (returns `S3Storage` when `S3_ENDPOINT` is non-empty, otherwise `MemoryStorage` with `/api/v1/images/assets`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/storage/s3.go internal/storage/s3_test.go
git commit -m "feat(storage): add ObjectStorage interface with S3/MinIO and memory fallback"
```

---

### Task 3: PostgreSQL Migration `00006_images_studio.sql`

**Files:**
- Create: `pkg/db/migrations/postgres/00006_images_studio.sql`

- [ ] **Step 1: Create migration SQL**

Create `pkg/db/migrations/postgres/00006_images_studio.sql` with `image_projects` and `image_revisions` tables, foreign keys, `idx_image_projects_user_updated`, and `UNIQUE (project_id, version)` matching Section 3.1 of the spec.

- [ ] **Step 2: Verify migration syntax and Go embed**

Run: `go test ./pkg/db/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/db/migrations/postgres/00006_images_studio.sql
git commit -m "feat(db): add 00006_images_studio migration for image_projects and image_revisions"
```

---

### Task 4: Domain Types, Presets Catalog, and Store (`internal/modules/images`)

**Files:**
- Create: `internal/modules/images/types.go`
- Create: `internal/modules/images/presets.go`
- Create: `internal/modules/images/store.go`
- Test: `internal/modules/images/store_test.go`

**Interfaces:**
- Produces:
  - `Project`, `Revision`, `Preset` structs in `types.go`
  - `ListPresets() []Preset`, `GetPreset(id string) (Preset, bool)` in `presets.go`
  - `Store` (`NewStore(db *sql.DB)` or in-memory test store interface) with `CreateProject`, `GetProject`, `ListProjectsByUser`, `UpdateProject`, `DeleteProject`, `AddRevision`, `ListRevisions`

- [ ] **Step 1: Write failing tests in `internal/modules/images/store_test.go`**

Test `ListPresets()` returns all 8 presets (5 `edit`, 3 `generate`) with non-empty `ExamplePrompt` and `ExampleAfterURL`, and `GetPreset("bg_replace")` returns `ok == true`. Also test `Store` round-trip (`CreateProject`, `AddRevision`, `ListRevisions`, `ListProjectsByUser`) using a mock/sqlmock or test helper matching `internal/modules/posts/store_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/images -v`
Expected: FAIL

- [ ] **Step 3: Implement `types.go`, `presets.go`, and `store.go`**

Implement all 8 presets from Spec Section 4.1 (`bg_replace`, `product_card`, `retouch_light`, `object_remove`, `style_transfer`, `social_poster`, `brand_logo`, `sticker_pack`) and the PostgreSQL `Store` with `image_projects` and `image_revisions` queries.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/images -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/images/types.go internal/modules/images/presets.go internal/modules/images/store.go internal/modules/images/store_test.go
git commit -m "feat(images): add domain types, 8 preset scenarios with examples, and postgres store"
```

---

### Task 5: Hybrid AI Pipeline (`internal/modules/images/ai_pipeline.go`)

**Files:**
- Create: `internal/modules/images/ai_pipeline.go`
- Test: `internal/modules/images/ai_pipeline_test.go`

**Interfaces:**
- Consumes: `pkg/ai` (`GenerateImage`, `EditImage`, `Vision`, `Chat`, `ListModels`)
- Produces: `type AIGateway interface`, `type Pipeline struct`, `func (p *Pipeline) Generate(ctx, prompt, model, aspectRatio string) ([]byte, string, error)`, `func (p *Pipeline) Edit(ctx, srcBytes []byte, prompt, model, aspectRatio string) (imgBytes []byte, pipelineUsed string, err error)`

- [ ] **Step 1: Write failing tests in `internal/modules/images/ai_pipeline_test.go`**

Test three cases with a mock `AIGateway`:
1. `Generate` calls `GenerateImage` and returns `"images_generations"`.
2. `Edit` calls `EditImage` and on success returns `"images_edits"`.
3. `Edit` when `EditImage` returns an unsupported error (e.g. HTTP 404/501) automatically falls back to `Vision` + `Chat` prompt merge + `GenerateImage` and returns `"vision_fallback"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/images -run TestPipeline -v`
Expected: FAIL

- [ ] **Step 3: Implement `ai_pipeline.go`**

Implement `AIGateway`, `aiClientGateway` (wrapping `*ai.Client` with 90s image timeout and 5m model cache), and `Pipeline` with `aspectRatioToSize` mapping (`1:1` → `1024x1024`, `16:9` → `1344x768`, `9:16` → `768x1344`, `4:3` → `1152x896`), `Generate`, and `Edit` with `isEditUnsupportedError` auto-fallback.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/images -run TestPipeline -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/images/ai_pipeline.go internal/modules/images/ai_pipeline_test.go
git commit -m "feat(images): add hybrid AI pipeline with EditImage primary and Vision fallback"
```

---

### Task 6: Domain Service & Chat BotModule (`internal/modules/images/service.go`, `module.go`)

**Files:**
- Create: `internal/modules/images/service.go`
- Create: `internal/modules/images/module.go`
- Test: `internal/modules/images/service_test.go`

**Interfaces:**
- Consumes: `Store`, `Pipeline`, `storage.ObjectStorage`, `core.FSM`
- Produces:
  - `Service` methods: `ListPresets`, `ListModels`, `ListProjects`, `GetProject`, `CreateAndRun`, `Refine`, `Restore`, `DeleteProject`, `GetAsset`
  - `Module` implementing `modules.BotModule` (`ID() = "images"`, `Title() = "Фотостудия"`, `HandleMessage`, `HandleCallback` with `img:` prefix)

- [ ] **Step 1: Write failing tests in `internal/modules/images/service_test.go`**

Test:
1. `CreateAndRun` (`generate` mode) uploads result to `ObjectStorage`, saves `v1` revision, sets status `ready`.
2. `Refine` downloads `v1` from `ObjectStorage`, runs `Pipeline.Edit`, saves `v2` revision; when `Pipeline.Edit` fails, `current_image_url` remains `v1`.
3. `Restore` creates `v3` (`action_type: "restore"`) pointing to `v1`'s image.
4. Ownership check returns `ErrForbidden` when another `userID` accesses `GetProject` / `Refine` / `Restore` / `DeleteProject`.
5. `Module.HandleCallback` handles `img:presets`, `img:preset:bg_replace`, `img:try:social_poster`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/images -v`
Expected: FAIL

- [ ] **Step 3: Implement `service.go` and `module.go`**

Implement `Service` (MIME check via `http.DetectContentType` for JPEG/PNG/WEBP, 10MB max limit, quota guard hook, S3 key generation) and `Module` (chat flow for `idle`, `preset_browse`, `awaiting_input`, `ready`, photo attachment handling, and `img:` callbacks).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/images -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/images/service.go internal/modules/images/module.go internal/modules/images/service_test.go
git commit -m "feat(images): add Service and BotModule chat handlers"
```

---

### Task 7: REST API Handlers & `showcase-bot` Wiring

**Files:**
- Create: `internal/api/handlers_images.go`
- Create: `internal/api/handlers_images_test.go`
- Modify: `internal/api/router.go:43-62,181-245`
- Modify: `cmd/showcase-bot/main.go` (wire `storage.NewObjectStorageFromEnv`, `images.NewModule`, `ImagesModule` in `RouterConfig`, and `images` mode seeding if applicable)

**Interfaces:**
- Consumes: `images.Service`
- Produces:
  - `GET /api/v1/images/presets`
  - `GET /api/v1/images/models`
  - `GET /api/v1/images/projects`
  - `GET /api/v1/images/projects/{id}`
  - `POST /api/v1/images/projects`
  - `POST /api/v1/images/projects/{id}/refine`
  - `POST /api/v1/images/projects/{id}/restore`
  - `DELETE /api/v1/images/projects/{id}`
  - `GET /api/v1/images/assets/{key...}` (serves `MemoryStorage` assets in dev/local mode)

- [ ] **Step 1: Write failing tests in `internal/api/handlers_images_test.go`**

Test:
1. `GET /api/v1/images/presets` returns 8 presets.
2. `POST /api/v1/images/projects` rejects non-image MIME (`text/plain`) with `400`.
3. `GET /api/v1/images/projects/{id}` returns `403`/`404` for a project owned by another user.
4. Full flow: `POST /projects` → `POST /projects/{id}/refine` → `POST /projects/{id}/restore` → `DELETE /projects/{id}`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api -run TestImages -v`
Expected: FAIL

- [ ] **Step 3: Implement `handlers_images.go`, mount routes in `router.go`, and wire in `cmd/showcase-bot/main.go`**

- [ ] **Step 4: Run all Go tests**

Run: `go test ./internal/api ./internal/modules/images ./internal/storage ./pkg/ai ./cmd/showcase-bot -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers_images.go internal/api/handlers_images_test.go internal/api/router.go cmd/showcase-bot/main.go
git commit -m "feat(api,showcase-bot): mount /api/v1/images endpoints and wire images module"
```

---

### Task 8: Static Preset Assets & Mini App UI (`web/`)

**Files:**
- Create: `web/public/presets/*.png` (lightweight placeholder/sample before-after PNGs for the 8 presets so `/presets/*.png` resolves cleanly)
- Create: `web/src/types/images.ts`
- Create: `web/src/api/images.ts`
- Create: `web/src/components/modes/ImagesView.tsx`
- Modify: `web/src/App.tsx:25-56,265-272`
- Modify: `web/src/components/hub/HubGrid.tsx:34-43` (add 9th `ModeCard` for `images`)

- [ ] **Step 1: Add TypeScript types (`web/src/types/images.ts`) and API client (`web/src/api/images.ts`)**

Define `ImagePreset`, `ImageRevision`, `ImageProject`, `ImageModelOption`, and methods `getPresets`, `getModels`, `listProjects`, `getProject`, `createProject` (FormData / JSON), `refineProject`, `restoreProject`, `deleteProject`.

- [ ] **Step 2: Generate/add static preset preview images in `web/public/presets/`**

Ensure every `ExampleBeforeURL` and `ExampleAfterURL` referenced in `presets.go` exists under `web/public/presets/`.

- [ ] **Step 3: Build `ImagesView.tsx` with 3 tabs («Сценарии», «Студия», «Мои работы»)**

- «Сценарии»: category filter (`Все`, `Исправление`, `Генерация`), cards with Before/After toggle, example prompt, 1-tap «Попробовать пример», and «Со своим фото/запросом».
- «Студия»: mode switch (`generate` / `edit`), photo upload dropzone, prompt input, aspect ratio selector (`1:1`, `16:9`, `9:16`, `4:3`), model selector, result preview with Before/After comparison toggle & download button, iterative refine input, and revision strip (`v1`, `v2`…) with «Вернуть версию».
- «Мои работы»: grid of user's projects opening directly in «Студия».

- [ ] **Step 4: Wire `images` mode into `App.tsx` and `HubGrid.tsx`**

- [ ] **Step 5: Verify TypeScript & Vite build**

Run: `cd web; npx tsc --noEmit; npm run build`
Expected: Exit code 0, clean build.

- [ ] **Step 6: Commit**

```bash
git add web/
git commit -m "feat(webapp): add ImagesView studio UI, preset examples, and 9th HubGrid mode"
```

---

## Plan Self-Review

1. **Spec coverage:**
   - 9th Hub mode (`images` / «Фотостудия»): Task 7 & Task 8.
   - 8 preset scenarios with Before/After examples & 1-tap try: Task 4 & Task 8.
   - `EditImage` (`/v1/images/edits`) + `Vision + GenerateImage` fallback: Task 1 & Task 5.
   - S3/MinIO `ObjectStorage` + memory fallback: Task 2.
   - DB migration `00006_images_studio.sql`: Task 3.
   - `BotModule` chat flow + REST API + ownership/MIME/size/quota guards: Task 6 & Task 7.
2. **Placeholder scan:** No TBDs, TODOs, or vague steps.
3. **Type consistency:** `ObjectStorage`, `ImageEditRequest`, `Project`, `Revision`, `Preset` match across all tasks.
