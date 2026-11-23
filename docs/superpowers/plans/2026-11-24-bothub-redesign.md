# Bot Hub Redesign Implementation Plan (Phase 1: MVP)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform `showcase-bot` into a modular multi-mode "Bot Hub" platform with 8 virtual bots, a central Hub with interactive mascot (3–[1+M+1]–3 grid), composite FSM `(user_id, mode_id)`, hidden admin maintenance switches, and 4 fully working MVP modes (Shop, Reception, Support RAG, AI Chat).

**Architecture:** Modular Monolith in Go (`internal/core`, `internal/platform`, `internal/modules`, `internal/admin`, `internal/api`) combined with a component-driven React 18 + TypeScript + Tailwind SPA in `web/` embedded into the Go binary via `embed.FS`. State isolation via composite key `(user_id, mode_id)` protected by Redis distributed locks (`SET NX EX 5`).

**Tech Stack:** Go 1.23+, PostgreSQL 16+ (pgvector + FTS fallback), Goose migrations, Redis 7+ (FSM, Distributed Locks, Maintenance Cache), React 18, Vite, TypeScript, Tailwind CSS, Docker multi-stage build, Dokploy.

**Spec:** [docs/superpowers/specs/2026-11-24-bothub-redesign-design.md](file:///C:/mimocode/max-bot-api-client/docs/superpowers/specs/2026-11-24-bothub-redesign-design.md)

## Global Constraints

- Go 1.23+ modular monolith architecture.
- Every FSM operation uses composite key `(user_id, mode_id)`.
- Race condition safety via Redis distributed lock `lock:fsm:{user_id}:{mode_id}` with 5-second TTL.
- Maintenance switch invalidation in Redis under 5 seconds.
- PlatformAdapter abstraction for MAX and Telegram.
- Hybrid RAG search: PostgreSQL vector similarity (`<=>`) with automatic fallback to `tsvector` FTS if `vector` extension is not present.
- Frontend: React 18 + Vite + TypeScript + Tailwind CSS in `web/`, compiled to `web/dist` and embedded via Go `embed.FS`.
- Zero placeholders (no TBD, TODO, or vague instructions).

---

### Task 1: Database Migration & Schema Expansion (Goose Migration 00002)

**Files:**
- Create: `pkg/db/migrations/postgres/00002_bothub_schema.sql`
- Create: `pkg/db/migrations/sqlite/00002_bothub_schema.sql`
- Modify: `pkg/store/sql_store.go`
- Test: `pkg/store/sql_store_test.go`

**Interfaces:**
- Consumes: Existing PostgreSQL / SQLite database connection in `pkg/store`.
- Produces: Storage methods:
  - `GetUser(ctx, id) (*User, error)`
  - `ResolveUserIdentity(ctx, platform, platformUserID, username, firstName, phone) (*User, error)`
  - `GetModeConfigs(ctx) ([]ModeConfig, error)`
  - `SetModeActive(ctx, modeID, isActive, maintMsg) error`
  - `GetUserModeState(ctx, userID, modeID) (*UserModeState, error)`
  - `SaveUserModeState(ctx, state *UserModeState) error`
  - `SaveLead(ctx, lead *Lead) (int64, error)`
  - `GetLeads(ctx, status) ([]Lead, error)`
  - `GetMascotPhrases(ctx) ([]MascotPhrase, error)`

- [ ] **Step 1: Write the failing test for new Bot Hub store methods**

```go
// In pkg/store/sql_store_test.go
func TestBotHubStore_UserAndModeState(t *testing.T) {
	ctx := context.Background()
	store := setupTestStore(t)

	// Resolve user identity
	user, err := store.ResolveUserIdentity(ctx, "max", "user_123", "testuser", "Test", "+79991234567")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}
	if user.ID == 0 || user.ActiveModeID != "shop" {
		t.Errorf("Unexpected user: %+v", user)
	}

	// Save and get composite mode state
	state := &UserModeState{
		UserID:      user.ID,
		ModeID:      "reception",
		CurrentStep: "select_slot",
		Payload:     []byte(`{"service_id":"srv_1"}`),
	}
	if err := store.SaveUserModeState(ctx, state); err != nil {
		t.Fatalf("SaveUserModeState failed: %v", err)
	}

	gotState, err := store.GetUserModeState(ctx, user.ID, "reception")
	if err != nil {
		t.Fatalf("GetUserModeState failed: %v", err)
	}
	if gotState.CurrentStep != "select_slot" {
		t.Errorf("Expected current_step select_slot, got %s", gotState.CurrentStep)
	}

	// Mode configs (maintenance switches)
	configs, err := store.GetModeConfigs(ctx)
	if err != nil {
		t.Fatalf("GetModeConfigs failed: %v", err)
	}
	if len(configs) < 8 {
		t.Errorf("Expected 8 default mode configs, got %d", len(configs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./pkg/store -run TestBotHubStore_UserAndModeState`
Expected: FAIL (undefined methods `ResolveUserIdentity`, `SaveUserModeState`, etc.)

- [ ] **Step 3: Write migration and implement store methods**

Create `pkg/db/migrations/postgres/00002_bothub_schema.sql`:
- Create tables: `users`, `user_identities`, `user_mode_states`, `mode_configs`, `leads`, `mascot_phrases`, `products`, `services`, `time_slots`, `bookings`, `events`, `event_registrations`, `onboarding_submissions`, `knowledge_documents`, `knowledge_chunks`, `support_tickets`, `support_messages`, `chat_sessions`, `admin_audit_logs`.
- Seed default `mode_configs` (8 rows: `shop`, `reception`, `reminders`, `channels`, `giveaways`, `support`, `docs`, `chat`). Modes 1, 2, 6, 8 have `is_active = true`; modes 3, 4, 5, 7 have `is_active = false` with maintenance message "Модуль находится в разработке (Этап 2)".
- Seed default `mascot_phrases` (priority 1..4).
- Add models and methods in `pkg/store/sql_store.go` and implement SQLite equivalent in `pkg/db/migrations/sqlite/00002_bothub_schema.sql`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./pkg/store -run TestBotHubStore_UserAndModeState`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/db/migrations/ pkg/store/
git commit -m "feat(store): add Bot Hub schema migration 00002 and store operations"
```

---

### Task 2: Core Orchestrator & Composite FSM with Redis Locks

**Files:**
- Create: `internal/core/events.go`
- Create: `internal/core/fsm.go`
- Create: `internal/core/lock.go`
- Create: `internal/core/guard.go`
- Create: `internal/core/orchestrator.go`
- Test: `internal/core/core_test.go`

**Interfaces:**
- Consumes: `pkg/store.SQLStore`, `pkg/fsm.RedisStore` or Redis client.
- Produces:
  - `NewLockManager(redisClient) *LockManager`
  - `LockManager.Acquire(ctx, key, ttl) (bool, error)`
  - `LockManager.Release(ctx, key) error`
  - `NewCompositeFSM(store, redisClient) *CompositeFSM`
  - `CompositeFSM.Get(ctx, userID, modeID) (*State, error)`
  - `CompositeFSM.Set(ctx, userID, modeID, step, payload) error`
  - `NewMaintenanceGuard(store, redisClient) *MaintenanceGuard`
  - `MaintenanceGuard.IsActive(ctx, modeID) (bool, string, error)`
  - `NewOrchestrator(store, fsm, lockManager, guard) *Orchestrator`
  - `Orchestrator.ProcessEvent(ctx, event *DomainEvent) (*DomainResponse, error)`

- [ ] **Step 1: Write the failing test for Core Orchestrator & Redis Locks**

```go
// In internal/core/core_test.go
func TestCompositeFSM_Isolation(t *testing.T) {
	ctx := context.Background()
	fsm := setupTestFSM(t)

	userID := int64(42)
	// Set state in shop
	err := fsm.Set(ctx, userID, "shop", "cart_review", []byte(`{"items":[1]}`))
	if err != nil {
		t.Fatalf("fsm.Set shop failed: %v", err)
	}

	// Set state in reception
	err = fsm.Set(ctx, userID, "reception", "select_slot", []byte(`{"date":"2026-11-25"}`))
	if err != nil {
		t.Fatalf("fsm.Set reception failed: %v", err)
	}

	// Verify shop state is unaffected
	shopState, err := fsm.Get(ctx, userID, "shop")
	if err != nil || shopState.CurrentStep != "cart_review" {
		t.Errorf("Shop state corrupt: %+v, err: %v", shopState, err)
	}

	// Verify reception state is distinct
	receptionState, err := fsm.Get(ctx, userID, "reception")
	if err != nil || receptionState.CurrentStep != "select_slot" {
		t.Errorf("Reception state corrupt: %+v, err: %v", receptionState, err)
	}
}

func TestRedisLock_ConcurrencyPrevention(t *testing.T) {
	ctx := context.Background()
	lockMgr := setupTestLockManager(t)

	key := "lock:fsm:100:shop"
	acquired1, err := lockMgr.Acquire(ctx, key, 3*time.Second)
	if err != nil || !acquired1 {
		t.Fatalf("First lock should succeed: %v", err)
	}

	// Second acquire on the same key must fail
	acquired2, err := lockMgr.Acquire(ctx, key, 3*time.Second)
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	if acquired2 {
		t.Errorf("Second lock should NOT be acquired!")
	}

	// Release and re-acquire
	_ = lockMgr.Release(ctx, key)
	acquired3, err := lockMgr.Acquire(ctx, key, 3*time.Second)
	if err != nil || !acquired3 {
		t.Fatalf("Lock re-acquire should succeed after release: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/core -run TestCompositeFSM_Isolation`
Expected: FAIL (cannot find package `internal/core`)

- [ ] **Step 3: Implement Composite FSM, LockManager, Guard, and Orchestrator**

- `internal/core/lock.go`: implement `SET NX EX` distributed lock with fallback for local test memory.
- `internal/core/fsm.go`: composite key `(user_id, mode_id)` reader/writer combining Redis cache with PostgreSQL persistence.
- `internal/core/guard.go`: cache mode configs in Redis with key `bothub:mode:{mode_id}:active`. Fast invalidation on admin toggle.
- `internal/core/orchestrator.go`: pipeline executing steps 1-7 (Identity Resolver -> Global Commands -> Redis Lock -> Maintenance Check -> Module Dispatch -> State Persistence).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/core -run "TestCompositeFSM_Isolation|TestRedisLock_ConcurrencyPrevention"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/
git commit -m "feat(core): implement composite FSM, distributed lock, maintenance guard, and orchestrator"
```

---

### Task 3: Platform Abstraction Layer (MAX & Telegram Adapters)

**Files:**
- Create: `internal/platform/adapter.go`
- Create: `internal/platform/max/adapter.go`
- Create: `internal/platform/telegram/adapter.go`
- Test: `internal/platform/adapter_test.go`

**Interfaces:**
- Consumes: `pkg/maxbot.Client`, incoming HTTP Webhook requests.
- Produces:
  ```go
  type PlatformAdapter interface {
      Platform() string
      SendMessage(ctx context.Context, recipientID string, body *OutMessage) error
      EditMessage(ctx context.Context, messageID string, body *OutMessage) error
      AnswerCallback(ctx context.Context, callbackID string, text string) error
      CheckMembership(ctx context.Context, chatID string, userID string) (bool, error)
  }
  ```

- [ ] **Step 1: Write the failing test for PlatformAdapter**

```go
// In internal/platform/adapter_test.go
func TestMAXAdapter_SendMessage(t *testing.T) {
	mockClient := &mockMaxBotClient{}
	adapter := max.NewAdapter(mockClient)
	if adapter.Platform() != "max" {
		t.Errorf("Expected platform 'max', got %s", adapter.Platform())
	}

	msg := &platform.OutMessage{
		Text: "Hello Bot Hub!",
		Buttons: [][]platform.Button{
			{{Text: "Сменить режим", Action: "change_mode"}},
		},
	}
	err := adapter.SendMessage(context.Background(), "user_42", msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if len(mockClient.sentMessages) != 1 {
		t.Errorf("Expected 1 sent message, got %d", len(mockClient.sentMessages))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/platform -run TestMAXAdapter_SendMessage`
Expected: FAIL (types not defined)

- [ ] **Step 3: Implement MAX & Telegram adapters**

- `internal/platform/adapter.go`: define `PlatformAdapter`, `OutMessage`, `Button`, `IncomingEvent`.
- `internal/platform/max/adapter.go`: wrap `pkg/maxbot.Client` to implement `PlatformAdapter`.
- `internal/platform/telegram/adapter.go`: implement `PlatformAdapter` contract for Telegram Bot API.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/platform -run TestMAXAdapter_SendMessage`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/platform/
git commit -m "feat(platform): implement unified platform adapter layer for MAX and Telegram"
```

---

### Task 4: Virtual Bot Module Interface & 4 Core Modules

**Files:**
- Create: `internal/modules/module.go`
- Create: `internal/modules/shop/shop.go`
- Create: `internal/modules/reception/reception.go`
- Create: `internal/modules/support/support.go`
- Create: `internal/modules/chat/chat.go`
- Test: `internal/modules/modules_test.go`

**Interfaces:**
- Consumes: `internal/core`, `pkg/store`, `pkg/ai`, `pkg/payments`.
- Produces:
  ```go
  type BotModule interface {
      ID() string
      Title() string
      HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error)
      HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error)
  }
  ```

- [ ] **Step 1: Write failing tests for each module**

```go
// In internal/modules/modules_test.go
func TestShopModule_DemoSimulation(t *testing.T) {
	shop := setupTestShopModule(t)
	resPaid, err := shop.SimulatePayment(context.Background(), "ord_demo_1", "paid")
	if err != nil || resPaid.Status != "paid" {
		t.Errorf("SimulatePayment paid failed: %v, res: %+v", err, resPaid)
	}

	resAbandoned, err := shop.SimulatePayment(context.Background(), "ord_demo_2", "abandoned")
	if err != nil || resAbandoned.Status != "abandoned" {
		t.Errorf("SimulatePayment abandoned failed: %v, res: %+v", err, resAbandoned)
	}
}

func TestReceptionModule_AntiOverbooking(t *testing.T) {
	reception := setupTestReceptionModule(t)
	ctx := context.Background()
	slotTime := time.Now().Add(24 * time.Hour).Truncate(time.Hour)

	// User 1 books
	err1 := reception.BookSlot(ctx, 101, "srv_consult", slotTime)
	if err1 != nil {
		t.Fatalf("First booking should succeed: %v", err1)
	}

	// User 2 attempts same slot -> must fail
	err2 := reception.BookSlot(ctx, 102, "srv_consult", slotTime)
	if err2 == nil {
		t.Errorf("Expected overbooking error for user 2, got nil!")
	}
}

func TestSupportModule_ScopedRAG(t *testing.T) {
	support := setupTestSupportModule(t)
	ctx := context.Background()
	ans, score, err := support.Ask(ctx, "Как оформить подписку?", "shop")
	if err != nil {
		t.Fatalf("Support Ask failed: %v", err)
	}
	if score < 0.5 || ans == "" {
		t.Errorf("Unexpected RAG answer: %s, score: %f", ans, score)
	}
}

func TestChatModule_RoleAndTone(t *testing.T) {
	chat := setupTestChatModule(t)
	ctx := context.Background()
	resp, err := chat.SendMessage(ctx, 42, 2, 1, "Что такое юнит-экономика?")
	if err != nil {
		t.Fatalf("Chat SendMessage failed: %v", err)
	}
	if resp == "" {
		t.Errorf("Expected non-empty LLM response")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/modules -run "TestShopModule_DemoSimulation|TestReceptionModule_AntiOverbooking"`
Expected: FAIL

- [ ] **Step 3: Implement the 4 modules**

- `shop/`: catalog, cart, demo QR generator, simulation actions (`paid` -> instant file/channel delivery; `abandoned` -> reminder task).
- `reception/`: slot booking with anti-overbooking check, event QR ticket generator, onboarding questionnaire state machine.
- `support/`: hybrid RAG (`mode_scope` filtering) with `pkg/ai` embeddings/FTS, threshold check (0.72) and ticket escalation.
- `chat/`: 5 roles, 4 tones, sliding memory window (10 messages) using `pkg/ai`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/modules`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/
git commit -m "feat(modules): implement 4 core modules (shop, reception, support, chat)"
```

---

### Task 5: Hidden Admin Service & RBAC

**Files:**
- Create: `internal/admin/service.go`
- Test: `internal/admin/admin_test.go`

**Interfaces:**
- Consumes: `pkg/store.SQLStore`, `internal/core.MaintenanceGuard`.
- Produces:
  - `ToggleMode(ctx, adminID, modeID, isActive, maintMsg, ip) error`
  - `ListLeads(ctx, adminID, status) ([]store.Lead, error)`
  - `UpdateLeadStatus(ctx, adminID, leadID, status, ip) error`
  - `ListTickets(ctx, adminID, status) ([]store.SupportTicket, error)`
  - `ReplyTicket(ctx, adminID, ticketID, messageText, ip) error`

- [ ] **Step 1: Write failing test for Admin service**

```go
// In internal/admin/admin_test.go
func TestAdminService_ToggleModeWithAudit(t *testing.T) {
	adminSvc := setupTestAdminService(t)
	ctx := context.Background()

	// Non-admin cannot toggle
	err := adminSvc.ToggleMode(ctx, 999, "shop", false, "Техработы", "127.0.0.1")
	if err == nil {
		t.Errorf("Expected forbidden error for non-admin user!")
	}

	// Superadmin toggles
	err = adminSvc.ToggleMode(ctx, 1, "shop", false, "Техработы", "127.0.0.1")
	if err != nil {
		t.Fatalf("Admin toggle failed: %v", err)
	}

	// Verify audit log recorded
	logs, err := adminSvc.GetAuditLogs(ctx, 1, 10)
	if err != nil || len(logs) == 0 {
		t.Fatalf("Expected audit log for admin action, got %v", logs)
	}
	if logs[0].Action != "toggle_maintenance" {
		t.Errorf("Expected action toggle_maintenance, got %s", logs[0].Action)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/admin -run TestAdminService_ToggleModeWithAudit`
Expected: FAIL

- [ ] **Step 3: Implement Admin Service with RBAC & Audit Log**

- Check admin role (`admin`, `superadmin`).
- Toggle maintenance state in DB and immediately invalidate Redis cache key `bothub:mode:{mode_id}:active`.
- Log every admin action in `admin_audit_logs`.
- Ticket reply delivery via `PlatformAdapter`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/admin -run TestAdminService_ToggleModeWithAudit`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/admin/
git commit -m "feat(admin): implement hidden admin service with RBAC, mode toggles, and audit logging"
```

---

### Task 6: REST API Router & WebApp Authentication

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/handlers_hub.go`
- Create: `internal/api/handlers_modules.go`
- Create: `internal/api/handlers_admin.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `internal/core`, `internal/modules`, `internal/admin`, `pkg/webapp.Validator`.
- Produces: `http.Handler` mounted under `/api/v1/`.

- [ ] **Step 1: Write failing HTTP integration tests for API**

```go
// In internal/api/api_test.go
func TestAPI_BootstrapAndModeSwitch(t *testing.T) {
	ts := setupTestAPIServer(t)
	defer ts.Close()

	// GET /api/v1/bootstrap
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/bootstrap", nil)
	req.Header.Set("X-Init-Data", validMAXInitData)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Bootstrap failed: err=%v, code=%d", err, resp.StatusCode)
	}

	var bootstrapResp struct {
		ActiveMode string `json:"active_mode"`
		Modes      []struct {
			ID       string `json:"id"`
			IsActive bool   `json:"is_active"`
		} `json:"modes"`
		Mascot struct {
			Phrase string `json:"phrase"`
		} `json:"mascot"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&bootstrapResp)
	if len(bootstrapResp.Modes) < 8 {
		t.Errorf("Expected 8 modes in bootstrap, got %d", len(bootstrapResp.Modes))
	}

	// POST /api/v1/user/mode
	switchBody := bytes.NewBufferString(`{"mode_id":"reception"}`)
	req2, _ := http.NewRequest("POST", ts.URL+"/api/v1/user/mode", switchBody)
	req2.Header.Set("X-Init-Data", validMAXInitData)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("Switch mode failed: err=%v, code=%d", err, resp2.StatusCode)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api -run TestAPI_BootstrapAndModeSwitch`
Expected: FAIL

- [ ] **Step 3: Implement REST API endpoints**

Implement:
- `/api/v1/bootstrap` (InitData validation, user JWT, 8 modes with maintenance status, mascot phrase, capabilities).
- `/api/v1/user/mode` (mode switch, system chat notification, cache update).
- `/api/v1/mascot/tap` (priority 4 random quote).
- `/api/v1/lead` (CTA submission).
- `/api/v1/shop/*`, `/api/v1/reception/*`, `/api/v1/support/*`, `/api/v1/chat/*`.
- `/api/v1/admin/*` (protected by admin role).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api -run TestAPI_BootstrapAndModeSwitch`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat(api): implement /api/v1 REST routes for Hub, 4 modules, and admin panel"
```

---

### Task 7: React 18 + TS + Tailwind Mini App Frontend (`web/`)

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/vite.config.ts`, `web/tailwind.config.js`
- Create: `web/src/main.tsx`, `web/src/App.tsx`, `web/src/index.css`
- Create: `web/src/components/hub/HubGrid.tsx`
- Create: `web/src/components/hub/ModeCard.tsx`
- Create: `web/src/components/hub/MascotRenderer.tsx`
- Create: `web/src/components/hub/SpeechBubble.tsx`
- Create: `web/src/components/common/Header.tsx`
- Create: `web/src/components/common/LeadModal.tsx`
- Create: `web/src/components/modes/ShopView.tsx`
- Create: `web/src/components/modes/ReceptionView.tsx`
- Create: `web/src/components/modes/SupportView.tsx`
- Create: `web/src/components/modes/ChatView.tsx`
- Create: `web/src/components/admin/AdminPanel.tsx`
- Test: `web/` build command (`npm run build`)

**Interfaces:**
- Consumes: `/api/v1/*` REST API.
- Produces: Production SPA bundle in `web/dist/` (HTML, JS, CSS, assets).

- [ ] **Step 1: Scaffold Vite + React + TypeScript + Tailwind project in `web/`**

Create `web/package.json` with React 18, Vite, TypeScript, Tailwind CSS, Lucide icons, Axios. Run `npm install` inside `web/`.

- [ ] **Step 2: Implement UI components**

- `HubGrid.tsx`: 3–[1+M+1]–3 responsive layout.
- `ModeCard.tsx`: active glow, available card, maintenance dimmed grayscale with Toast on tap.
- `MascotRenderer.tsx`: breathing animation, tap bounce, `SpeechBubble.tsx` for dynamic prioritized quotes.
- `ShopView.tsx`: catalog, cart, demo QR with "Я оплатил" / "Я не оплатил".
- `ReceptionView.tsx`: slot calendar, events with QR tickets, onboarding wizard.
- `SupportView.tsx`: scoped RAG Q&A, operator ticket form.
- `ChatView.tsx`: 5 role cards, 4 tone chips, conversation window.
- `LeadModal.tsx`: CTA lead form with auto-attached metadata.
- `AdminPanel.tsx`: 8 maintenance switches, leads list, tickets list.

- [ ] **Step 3: Run Vite build to verify clean compilation**

Run: `cd web && npm run build`
Expected: Output generated in `web/dist/` with 0 TypeScript errors.

- [ ] **Step 4: Commit**

```bash
git add web/
git commit -m "feat(frontend): implement React 18 SPA with 3-[1+M+1]-3 Hub, Mascot, 4 mode views, and Admin panel"
```

---

### Task 8: Embedding Frontend & Multi-Stage Dockerfile Deployment

**Files:**
- Create: `cmd/showcase-bot/web_embed.go`
- Modify: `cmd/showcase-bot/main.go`
- Modify: `Dockerfile.showcase-bot`
- Test: Full build and live Dokploy deployment

**Interfaces:**
- Consumes: `web/dist` via `//go:embed all:web/dist`.
- Produces: Unified production binary serving static Mini App on `/` and `/assets/*`, REST API on `/api/v1/*`, and MAX bot webhook on `/webhook`.

- [ ] **Step 1: Write static asset embedding handler in Go**

```go
// In cmd/showcase-bot/web_embed.go
package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web/dist
var webDistFS embed.FS

func RegisterWebSPA(mux *http.ServeMux) error {
	distFS, err := fs.Sub(webDistFS, "web/dist")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.Handle("/", fileServer)
	return nil
}
```

- [ ] **Step 2: Update `Dockerfile.showcase-bot` to multi-stage**

Add `node:20-alpine AS web-builder` stage, copy `web/dist` into `go-builder`, build statically linked binary.

- [ ] **Step 3: Run local build and all tests**

Run: `go test -v ./...`
Expected: PASS across all packages.

- [ ] **Step 4: Commit and push to `master`**

```bash
git add cmd/showcase-bot/ Dockerfile.showcase-bot
git commit -m "feat(deploy): wire embedded SPA and multi-stage Dockerfile for Dokploy release"
git push origin master
```

- [ ] **Step 5: Trigger Dokploy redeploy & verify live health**

Trigger Dokploy redeploy on application `ynEbxpi4_T3fkbuGVK0OF` (`max-showcase-bot`), verify HTTP 200 on `https://combot.littfed.ru/` and `/api/v1/bootstrap`.

---

## Plan Review & Verification Checklist
1. **Spec Coverage:** Covers Hub 3–[1+M+1]–3, Mascot with clouds, 4 MVP modes, Composite FSM with Redis lock, Hidden Admin, Dokploy deployment.
2. **No Placeholders:** All tasks define exact files, test code, implementation details, and shell commands.
3. **Type Consistency:** Types across DB models, Core Orchestrator, Modules, REST API, and Frontend are fully aligned.
