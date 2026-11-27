# Posts Assistant («Контент-студия») Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать в Bot Hub новый режим-помощник «Контент-студия» (`posts`) с полным управляемым пайплайном создания постов от идеи (текст или фото) до готового контент-пакета под MAX, Telegram, VK и Instagram, синхронизированного между чатом и Mini App.

**Architecture:** Модуль `internal/modules/posts` инкапсулирует бизнес-логику в едином сервисе `posts.Service`, который вызывается как чат-модулем `BotModule` (префикс `pst:`), так и REST API (`/api/modules/posts/*`). Состояние визарда хранится в композитном FSM (`user_mode_states`), а содержательные данные (`brand_profiles`, `post_drafts`) — в PostgreSQL; специфика площадок описывается декларативно через `PlatformSpec`.

**Tech Stack:** Go 1.23+, PostgreSQL + Goose, `pkg/ai` (GMA gateway: Chat / Vision / GenerateImage), `internal/core` (Orchestrator, CompositeFSM, Guards), React 18 + TypeScript + Vite + Tailwind (`web/`).

**Spec:** [2026-11-27-posts-assistant-design.md](file:///C:/mimocode/max-bot-api-client/docs/superpowers/specs/2026-11-27-posts-assistant-design.md)

## Global Constraints

- Целевые площадки MVP: `max`, `telegram`, `vk`, `instagram`; выход — готовый контент-пакет (без автопостинга в соцсети).
- Вход: текстовая идея и/или изображение (без парсинга ссылок и аудио в MVP).
- Идентификатор модуля: `ID() = "posts"`, `Title() = "Контент-студия"`, префикс callback-данных: `pst:`.
- Все AI-вызовы проходят через интерфейс поверх `pkg/ai.Client` с таймаутами (30s текст/Vision, 60s картинка), 1 автоповтором при невалидном JSON и graceful degradation при сбое генерации изображения.
- Каждый HTTP-эндпоинт `/api/modules/posts/*` обязан проверять `UserFromContext(ctx)` и ownership по `user_id` на каждый `draft_id` и `profile_id`.
- Никаких дублирующих `if/else` веток под каждую соцсеть: правила площадок берутся из реестра `PlatformSpecs`.

---

### Task 1: Миграция БД, доменные типы и SQL-репозиторий (`brand_profiles`, `post_drafts`)

**Files:**
- Create: `pkg/db/migrations/postgres/00004_posts_studio.sql`
- Create: `internal/modules/posts/types.go`
- Create: `internal/modules/posts/store.go`
- Test: `internal/modules/posts/store_test.go`

**Interfaces:**
- Consumes: `*store.SQLStore` (доступ к `*sql.DB` через `sqlStore.DB()`)
- Produces:
  - Типы `BrandProfile`, `PostDraft`, `PostHook`, `PostVariant`, константы статусов `DraftStatus*` и шагов `Step*`
  - `Repository` интерфейс и `PGStore` (`NewPGStore(db *sql.DB) *PGStore`) с методами:
    - `GetDefaultProfile(ctx context.Context, userID int64) (*BrandProfile, error)`
    - `ListProfiles(ctx context.Context, userID int64) ([]BrandProfile, error)`
    - `UpsertProfile(ctx context.Context, p *BrandProfile) error`
    - `CreateDraft(ctx context.Context, d *PostDraft) error`
    - `GetDraft(ctx context.Context, userID, draftID int64) (*PostDraft, error)`
    - `ListDrafts(ctx context.Context, userID int64, limit int) ([]PostDraft, error)`
    - `UpdateDraft(ctx context.Context, d *PostDraft) error`

- [ ] **Step 1: Создать SQL-миграцию `pkg/db/migrations/postgres/00004_posts_studio.sql`**

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS brand_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL DEFAULT 'Основной',
    niche TEXT NOT NULL DEFAULT '',
    target_audience TEXT NOT NULL DEFAULT '',
    tone_of_voice VARCHAR(64) NOT NULL DEFAULT 'friendly',
    forbidden_words TEXT NOT NULL DEFAULT '',
    default_platforms JSONB NOT NULL DEFAULT '["max","telegram"]'::jsonb,
    is_default BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_brand_profiles_user_id ON brand_profiles (user_id);

CREATE TABLE IF NOT EXISTS post_drafts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_id BIGINT REFERENCES brand_profiles(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'idea',
    raw_idea TEXT NOT NULL DEFAULT '',
    source_image_url TEXT NOT NULL DEFAULT '',
    vision_summary TEXT NOT NULL DEFAULT '',
    goal VARCHAR(32) NOT NULL DEFAULT 'engagement',
    tone VARCHAR(64) NOT NULL DEFAULT 'friendly',
    target_platforms JSONB NOT NULL DEFAULT '["max","telegram"]'::jsonb,
    hooks JSONB NOT NULL DEFAULT '[]'::jsonb,
    selected_hook_id VARCHAR(64) NOT NULL DEFAULT '',
    image_prompt TEXT NOT NULL DEFAULT '',
    generated_image_url TEXT NOT NULL DEFAULT '',
    variants JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_post_drafts_user_status
ON post_drafts (user_id, status, updated_at DESC);

INSERT INTO mode_configs (mode_id, title, is_active, daily_quota)
VALUES ('posts', 'Контент-студия', TRUE, 50)
ON CONFLICT (mode_id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS post_drafts;
DROP TABLE IF EXISTS brand_profiles;
DELETE FROM mode_configs WHERE mode_id = 'posts';
```

- [ ] **Step 2: Написать failing test в `internal/modules/posts/store_test.go`**

```go
package posts

import (
	"context"
	"testing"
)

func TestInMemoryStore_DraftOwnershipAndProfile(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()

	prof := &BrandProfile{
		UserID:           10,
		Name:             "Основной",
		Niche:            "IT",
		TargetAudience:   "Разработчики",
		ToneOfVoice:      "expert",
		DefaultPlatforms: []string{"max", "telegram"},
		IsDefault:        true,
	}
	if err := st.UpsertProfile(ctx, prof); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}
	if prof.ID == 0 {
		t.Fatal("expected non-zero profile ID")
	}

	draft := &PostDraft{
		UserID:          10,
		ProfileID:       &prof.ID,
		Status:          DraftStatusIdea,
		RawIdea:         "Запуск новой фичи",
		TargetPlatforms: []string{"max", "telegram"},
	}
	if err := st.CreateDraft(ctx, draft); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	if _, err := st.GetDraft(ctx, 999, draft.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for another user, got %v", err)
	}

	got, err := st.GetDraft(ctx, 10, draft.ID)
	if err != nil || got.RawIdea != draft.RawIdea {
		t.Fatalf("GetDraft failed: %+v, err=%v", got, err)
	}
}
```

- [ ] **Step 3: Запустить тест и убедиться, что он падает**

Run: `go test ./internal/modules/posts/...`
Expected: FAIL (`undefined: NewMemoryStore`, `BrandProfile`, `PostDraft`).

- [ ] **Step 4: Реализовать `internal/modules/posts/types.go` и `internal/modules/posts/store.go`**

В `types.go` определить:
- Константы шагов FSM: `StepIdle = "idle"`, `StepIdeaInput = "idea_input"`, `StepBriefSetup = "brief_setup"`, `StepHooksSelect = "hooks_select"`, `StepGenerating = "generating"`, `StepReady = "ready"`.
- Константы статусов черновика: `DraftStatusIdea = "idea"`, `DraftStatusBrief = "brief"`, `DraftStatusHooks = "hooks"`, `DraftStatusGenerating = "generating"`, `DraftStatusReady = "ready"`, `DraftStatusArchived = "archived"`.
- Структуры `BrandProfile`, `PostDraft`, `PostHook`, `PostVariant`, `FSMStatePayload` (`ActiveDraftID int64`).
- В `store.go` определить `ErrNotFound = errors.New("posts: not found")`, интерфейс `Repository`, реализацию `PGStore` (SQL-запросы с `WHERE user_id = $1` для ownership) и `MemoryStore` (thread-safe `sync.RWMutex` для юнит-тестов без БД).

- [ ] **Step 5: Запустить тесты и убедиться, что проходят**

Run: `go test -v ./internal/modules/posts/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/db/migrations/postgres/00004_posts_studio.sql internal/modules/posts/types.go internal/modules/posts/store.go internal/modules/posts/store_test.go
git commit -m "feat(posts): add db migration, domain types, and repository"
```

---

### Task 2: Декларативный реестр площадок (`PlatformSpec`) и валидатор

**Files:**
- Create: `internal/modules/posts/platforms.go`
- Test: `internal/modules/posts/platforms_test.go`

**Interfaces:**
- Consumes: `PostVariant` (из `types.go`)
- Produces:
  - `PlatformSpec` (`ID`, `DisplayName`, `MaxChars`, `CaptionMaxChars`, `MarkupStyle`, `HashtagHint`, `AspectRatio`, `StylePrompt`)
  - `GetPlatformSpec(id string) (PlatformSpec, bool)`
  - `NormalizePlatforms(ids []string) []string`
  - `ValidateVariant(spec PlatformSpec, v *PostVariant, forbiddenWords string) []string` (возвращает список предупреждений, напр. превышение лимита или стоп-слова)

- [ ] **Step 1: Написать failing test в `internal/modules/posts/platforms_test.go`**

```go
package posts

import "testing"

func TestPlatformSpecAndValidation(t *testing.T) {
	spec, ok := GetPlatformSpec("instagram")
	if !ok || spec.MaxChars != 2200 || spec.AspectRatio != "4:5" {
		t.Fatalf("unexpected instagram spec: %+v, ok=%v", spec, ok)
	}

	normalized := NormalizePlatforms([]string{"UNKNOWN", "max", "VK", "max"})
	if len(normalized) != 2 || normalized[0] != "max" || normalized[1] != "vk" {
		t.Fatalf("unexpected normalized platforms: %v", normalized)
	}

	warnings := ValidateVariant(spec, &PostVariant{
		Text: "Успешный успех для всех!",
	}, "успешный успех, кринж")
	if len(warnings) == 0 {
		t.Fatal("expected forbidden word warning")
	}
}
```

- [ ] **Step 2: Запустить тест и убедиться, что падает**

Run: `go test ./internal/modules/posts/...`
Expected: FAIL (`undefined: GetPlatformSpec`).

- [ ] **Step 3: Реализовать `internal/modules/posts/platforms.go`**

Описать карту `platformRegistry` для `"instagram"`, `"vk"`, `"telegram"`, `"max"`, вспомогательные `GetPlatformSpec`, `AllPlatformSpecs`, `NormalizePlatforms` (дедупликация + фильтр только поддерживаемых, дефолт `["max", "telegram"]` при пустом списке), `ValidateVariant` (проверка длины в рунах `utf8.RuneCountInString` против `MaxChars` и поиск стоп-слов case-insensitive).

- [ ] **Step 4: Запустить тесты и убедиться, что проходят**

Run: `go test -v ./internal/modules/posts/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/modules/posts/platforms.go internal/modules/posts/platforms_test.go
git commit -m "feat(posts): add declarative PlatformSpec registry and variant validator"
```

---

### Task 3: AI-оркестрация пайплайна (`ai_pipeline.go`)

**Files:**
- Create: `internal/modules/posts/ai_pipeline.go`
- Test: `internal/modules/posts/ai_pipeline_test.go`

**Interfaces:**
- Consumes: `pkg/ai` (`*ai.Client`), `PlatformSpec`, `BrandProfile`, `PostDraft`
- Produces:
  - `AIGateway` интерфейс (`Chat`, `Vision`, `GenerateImage`) с адаптером `NewAIGateway(client *ai.Client) AIGateway`
  - `Pipeline` (`NewPipeline(gw AIGateway) *Pipeline`) с методами:
    - `AnalyzeImage(ctx context.Context, imageURL string) (string, error)`
    - `GenerateHooks(ctx context.Context, draft *PostDraft, profile *BrandProfile) ([]PostHook, error)`
    - `GeneratePackage(ctx context.Context, draft *PostDraft, profile *BrandProfile, specs []PlatformSpec) (map[string]PostVariant, string, error)`
    - `GenerateVisual(ctx context.Context, imagePrompt, aspectRatio string) (string, error)`
    - `RefineVariant(ctx context.Context, draft *PostDraft, profile *BrandProfile, targetPlatform, instruction string) (map[string]PostVariant, error)`

- [ ] **Step 1: Написать failing tests в `internal/modules/posts/ai_pipeline_test.go`**

Покрыть 3 ключевых кейса с моком `AIGateway`:
1. `TestPipeline_GenerateHooks_RetryOnInvalidJSON` — первый ответ LLM невалидный JSON, второй валидный -> 1 автоповтор и возврат 3 хуков.
2. `TestPipeline_GeneratePackage_Success` — парсинг вариантов для `max` и `vk` + `image_prompt`.
3. `TestPipeline_AnalyzeImage_And_GenerateVisual` — проверка вызова Vision и Image.

- [ ] **Step 2: Запустить тесты и убедиться, что падают**

Run: `go test ./internal/modules/posts/...`
Expected: FAIL (`undefined: AIGateway`, `NewPipeline`).

- [ ] **Step 3: Реализовать `internal/modules/posts/ai_pipeline.go`**

- Обёртка над `pkg/ai.Client` с таймаутами (`30 * time.Second` для Chat/Vision, `60 * time.Second` для GenerateImage).
- Хелпер `chatJSON[T any](ctx, systemPrompt, userPrompt)` с очисткой markdown-ограждений ```` ```json ```` и **1 retry** с корректирующим промптом при ошибке `json.Unmarshal`.
- Промпты для `GenerateHooks` (3 контрастных угла), `GeneratePackage` (учёт `PlatformSpec` + `BrandProfile` + `forbidden_words` + `image_prompt`), `RefineVariant` (одна площадка или `"all"`).

- [ ] **Step 4: Запустить тесты и убедиться, что проходят**

Run: `go test -v ./internal/modules/posts/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/modules/posts/ai_pipeline.go internal/modules/posts/ai_pipeline_test.go
git commit -m "feat(posts): implement AI pipeline with JSON retry and Vision/Image support"
```

---

### Task 4: Доменный сервис `posts.Service`

**Files:**
- Create: `internal/modules/posts/service.go`
- Test: `internal/modules/posts/service_test.go`

**Interfaces:**
- Consumes: `Repository`, `*Pipeline`
- Produces:
  - `Service` (`NewService(repo Repository, pipeline *Pipeline) *Service`) с методами:
    - `GetState(ctx context.Context, userID int64, activeDraftID int64) (*StudioState, error)`
    - `SaveProfile(ctx context.Context, userID int64, p *BrandProfile) (*BrandProfile, error)`
    - `StartDraft(ctx context.Context, userID int64, rawIdea, sourceImageURL string) (*PostDraft, error)`
    - `UpdateBrief(ctx context.Context, userID, draftID int64, goal, tone string, platforms []string) (*PostDraft, error)`
    - `GenerateHooks(ctx context.Context, userID, draftID int64) (*PostDraft, error)`
    - `GeneratePackage(ctx context.Context, userID, draftID int64, hookID string) (*PostDraft, error)`
    - `Refine(ctx context.Context, userID, draftID int64, platform, instruction string) (*PostDraft, error)`
    - `UpdateVariantManual(ctx context.Context, userID, draftID int64, platform string, v PostVariant) (*PostDraft, error)`
    - `RegenerateImage(ctx context.Context, userID, draftID int64, customPrompt string) (*PostDraft, error)`

- [ ] **Step 1: Написать failing test в `internal/modules/posts/service_test.go`**

Проверить полный сквозной сценарий + **graceful degradation** при ошибке генерации картинки:
- `StartDraft` (с фото -> Vision вызывается, `vision_summary` заполняется) -> `UpdateBrief` -> `GenerateHooks` -> `GeneratePackage` (картинка падает с ошибкой, но `draft.Status == DraftStatusReady` и тексты `Variants` сохранены!).

- [ ] **Step 2: Запустить тест и убедиться, что падает**

Run: `go test ./internal/modules/posts/...`
Expected: FAIL (`undefined: NewService`).

- [ ] **Step 3: Реализовать `internal/modules/posts/service.go`**

Реализовать все методы `Service`, обеспечив:
- Подтягивание `GetDefaultProfile` при `StartDraft`.
- Нормализацию площадок через `NormalizePlatforms`.
- Graceful degradation в `GeneratePackage`: ошибка `GenerateVisual` логируется и не прерывает сохранение готовых текстов в `DraftStatusReady` (возвращается draft + неблокирующее предупреждение или пустой `generated_image_url`).
- Подсчёт `CharCount` через `utf8.RuneCountInString` при генерации, правках и ручном сохранении.

- [ ] **Step 4: Запустить тесты и убедиться, что проходят**

Run: `go test -v ./internal/modules/posts/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/modules/posts/service.go internal/modules/posts/service_test.go
git commit -m "feat(posts): add unified posts.Service with graceful visual fallback"
```

---

### Task 5: Чат-модуль `posts.Module` (`BotModule`, колбэки `pst:`)

**Files:**
- Create: `internal/modules/posts/module.go`
- Test: `internal/modules/posts/module_test.go`

**Interfaces:**
- Consumes: `*Service`, `*core.State`, `*core.IncomingMessage`, `*core.IncomingCallback`
- Produces:
  - `Module` реализующий `modules.BotModule` (`New(service *Service, webAppURL string) *Module`)
  - `ID() string` (`"posts"`), `Title() string` (`"Контент-студия"`), `Service() *Service`
  - `HandleMessage(...) (*core.ModuleResult, error)`
  - `HandleCallback(...) (*core.ModuleResult, error)`

- [ ] **Step 1: Написать failing test в `internal/modules/posts/module_test.go`**

Протестировать пошаговый чат-визард:
1. На шаге `StepIdle` колбэк `pst:new` переводит `state.CurrentStep` в `StepIdeaInput`.
2. На шаге `StepIdeaInput` сообщение с текстом создаёт черновик, сохраняет `active_draft_id` в `state.Payload` и переводит в `StepBriefSetup` с кнопками площадок/целей и `[Сгенерировать 3 хука]`.
3. На шаге `StepHooksSelect` колбэк `pst:hook:<id>` генерирует пакет и переводит в `StepReady` с кнопками быстрых правок и `open_app`.

- [ ] **Step 2: Запустить тест и убедиться, что падает**

Run: `go test ./internal/modules/posts/...`
Expected: FAIL (`undefined: New`).

- [ ] **Step 3: Реализовать `internal/modules/posts/module.go`**

Реализовать маршрутизацию по `state.CurrentStep` в `HandleMessage` и по префиксам `pst:` (`pst:new`, `pst:drafts`, `pst:profile`, `pst:goal:<g>`, `pst:plat:<p>`, `pst:hooks`, `pst:hook:<id>`, `pst:refine:<preset>`, `pst:img`) в `HandleCallback`, синхронизируя `active_draft_id` в `state.Payload`.

- [ ] **Step 4: Запустить тесты и убедиться, что проходят**

Run: `go test -v ./internal/modules/posts/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/modules/posts/module.go internal/modules/posts/module_test.go
git commit -m "feat(posts): implement BotModule chat wizard with pst: callbacks"
```

---

### Task 6: REST API хэндлеры и роутинг (`handlers_posts.go`)

**Files:**
- Create: `internal/api/handlers_posts.go`
- Test: `internal/api/handlers_posts_test.go`
- Modify: `internal/api/router.go` (добавить `PostsModule *posts.Module` в `RouterConfig` и зарегистрировать роуты `/api/modules/posts/...`)

**Interfaces:**
- Consumes: `RouterConfig.PostsModule`, `UserFromContext`, `posts.Service`
- Produces:
  - `GET /api/modules/posts/state`
  - `POST /api/modules/posts/profile` и `PUT /api/modules/posts/profile/{id}`
  - `POST /api/modules/posts/drafts`
  - `PATCH /api/modules/posts/drafts/{id}/brief`
  - `POST /api/modules/posts/drafts/{id}/hooks`
  - `POST /api/modules/posts/drafts/{id}/generate`
  - `POST /api/modules/posts/drafts/{id}/refine`
  - `PATCH /api/modules/posts/drafts/{id}/variants/{platform}`
  - `POST /api/modules/posts/drafts/{id}/image`

- [ ] **Step 1: Прочитать `internal/api/handlers_reminders.go` (первые 90 строк) и место регистрации роутов в `internal/api/router.go`, чтобы точно повторить паттерн роутинга и обработки ошибок/authz**

- [ ] **Step 2: Написать failing tests в `internal/api/handlers_posts_test.go`**

Проверить:
1. Запрос без авторизации -> `401`.
2. Попытка пользователя B обратиться к `draft_id` пользователя A -> `404` / `403`.
3. Валидация пустой идеи при `POST /api/modules/posts/drafts` -> `400`.
4. Успешный сценарий создания черновика, генерации хуков и пакета от лица авторизованного пользователя.

- [ ] **Step 3: Запустить тест и убедиться, что падает**

Run: `go test ./internal/api -run TestPosts`
Expected: FAIL.

- [ ] **Step 4: Реализовать `internal/api/handlers_posts.go` и подключить в `internal/api/router.go`**

- [ ] **Step 5: Запустить тесты API и убедиться, что проходят**

Run: `go test -v ./internal/api -run TestPosts`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers_posts.go internal/api/handlers_posts_test.go internal/api/router.go
git commit -m "feat(api): add /api/modules/posts endpoints with ownership checks"
```

---

### Task 7: Подключение модуля `posts` в `cmd/showcase-bot/main.go`

**Files:**
- Modify: `cmd/showcase-bot/main.go`

**Interfaces:**
- Consumes: `posts.NewPGStore`, `posts.NewAIGateway`, `posts.NewPipeline`, `posts.NewService`, `posts.New`
- Produces: регистрация `postsMod` в `orchestrator.RegisterModule(postsMod)` и передача `PostsModule: postsMod` в `api.RouterConfig`.

- [ ] **Step 1: Прочитать строки 265–335 `cmd/showcase-bot/main.go` для точной привязки**

- [ ] **Step 2: Подключить `posts` модуль в `cmd/showcase-bot/main.go`**

Инициализировать `postsStore := posts.NewPGStore(sqlStore.DB())`, `postsPipeline := posts.NewPipeline(posts.NewAIGateway(aiClient))`, `postsService := posts.NewService(postsStore, postsPipeline)`, `postsMod := posts.New(postsService, webAppURL)`, зарегистрировать в `orchestrator` и передать в `api.RouterConfig`.

- [ ] **Step 3: Проверить сборку и все Go-тесты**

Run: `go test ./...`
Expected: PASS (всё компилируется и тесты проходят).

- [ ] **Step 4: Commit**

```bash
git add cmd/showcase-bot/main.go
git commit -m "feat(showcase-bot): wire posts module into orchestrator and API router"
```

---

### Task 8: Frontend Mini App («Контент-студия») и финальная сборка

**Files:**
- Create: `web/src/types/posts.ts`
- Create: `web/src/api/posts.ts`
- Create: `web/src/components/modes/PostsView.tsx`
- Modify: `web/src/App.tsx` (добавить `posts` в список режимов / роутинг view)

**Interfaces:**
- Consumes: REST API `/api/modules/posts/*` (через `apiClient` в `web/src/api/client.ts`)
- Produces: компонент `PostsView` с 3 вкладками (`Студия`, `История`, `Профиль бренда`), превью-карточками в стиле `MAX` / `Telegram` / `VK` / `Instagram`, счётчиком символов, копированием в один клик и панелью AI-правок.

- [ ] **Step 1: Прочитать `web/src/api/client.ts` и `web/src/components/modes/RemindersView.tsx` (первые 80 строк) для соблюдения единого стиля API-клиента и UI**

- [ ] **Step 2: Создать `web/src/types/posts.ts` и `web/src/api/posts.ts`**

Описать TypeScript-типы (`BrandProfile`, `PostDraft`, `PostHook`, `PostVariant`, `StudioState`) и типизированные методы API (`getState`, `saveProfile`, `createDraft`, `updateBrief`, `generateHooks`, `generatePackage`, `refine`, `updateVariant`, `regenerateImage`).

- [ ] **Step 3: Реализовать `web/src/components/modes/PostsView.tsx` и подключить в `web/src/App.tsx`**

- [ ] **Step 4: Собрать фронтенд и прогнать весь бэкенд-набор тестов**

Run: `cd web; npm run build; cd ..; go test ./...`
Expected: `npm run build` завершается без ошибок TS/Vite, `go test ./...` — PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/types/posts.ts web/src/api/posts.ts web/src/components/modes/PostsView.tsx web/src/App.tsx
git commit -m "feat(webapp): add Content Studio (posts) view to Mini App"
```

---

## Self-Review Check
- **Spec coverage:** Все разделы спецификации (миграция `00004_posts_studio.sql`, `PlatformSpec`, Vision + Hooks + Package + Refine + Image + retry + graceful degradation, чат-модуль `pst:`, REST API с ownership check, экран Mini App «Контент-студия», unit и API тесты) покрыты задачами 1–8.
- **No placeholders:** Все типы, методы, пути, эндпоинты и команды проверки указаны явно.
- **Type consistency:** `BrandProfile`, `PostDraft`, `PostHook`, `PostVariant`, `Repository`, `Service`, `Module` согласованы между задачами.
