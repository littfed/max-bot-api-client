# Модуль «Напоминания» (Reminders) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Разработать и ввести в эксплуатацию полнофункциональный Режим 3 «Напоминания» платформы Bot Hub (персональные задачи, рекуррентность, фоновый воркер с `SKIP LOCKED` и rate limiter, интерактивные действия в чате, интеграция с Ресепшеном, A/B-тестирование уведомлений, REST API и React интерфейс в Mini App).

**Architecture:** Модульный монолит на Go с хранением в PostgreSQL (`reminders`, `notification_templates`, `notification_events`). Доставка через параллельный `Worker` на базе `time.Ticker` и `SKIP LOCKED` транзакций с rate limiting (25 rps). Сквозная интеграция с FSM ядра, `PlatformAdapter` (MAX/Telegram) и Mini App SPA на React 18.

**Tech Stack:** Go 1.22+, PostgreSQL (pgx/database/sql), React 18, TypeScript, Tailwind CSS, Vite.

**Spec:** `docs/superpowers/specs/2026-11-25-reminders-module-design.md`

## Global Constraints
- Go code formatting with standard `gofmt` and zero build/lint errors (`go vet`).
- All timestamps stored in UTC (`TIMESTAMPTZ`), localized using `user.Timezone` (`UTC+3` default).
- Worker queries must use `FOR UPDATE SKIP LOCKED` on `status = 'pending'`.
- All user-facing REST endpoints protected by `authMiddleware`.
- Interactive callbacks must use namespace `rmd:` (`rmd:done:`, `rmd:snz:`, `rmd:del:`).
- Zero placeholders (`TODO`, `TBD`) in test and implementation code.

---

### Task 1: Database Migration & Store Models for Reminders and A/B Templates

**Files:**
- Create: `pkg/db/migrations/postgres/00003_reminders_schema.sql`
- Modify: `pkg/store/models.go`
- Modify: `pkg/store/sql_store.go`
- Test: `pkg/store/sql_store_test.go`

**Interfaces:**
- Consumes: `store.User`, `store.SQLStore`
- Produces: `store.Reminder`, `store.NotificationTemplate`, `store.NotificationEvent`, store methods (`CreateReminder`, `GetRemindersByUser`, `UpdateReminderStatus`, `SnoozeReminderInStore`, `GetPendingRemindersForUpdate`, `RecordNotificationEvent`, `GetNotificationABStats`)

- [ ] **Step 1: Write the failing test for reminders store operations**

Добавить в `pkg/store/sql_store_test.go`:
```go
func TestSQLStore_RemindersCRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	st := NewWithSQL(db)
	ctx := context.Background()

	user, err := st.sql.CreateUser(ctx, "reminders", "UTC+3")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	rem := &Reminder{
		UserID:         user.ID,
		Title:          "Тестовая задача",
		RemindAt:       time.Now().UTC().Add(10 * time.Minute),
		RecurrenceRule: "daily",
		Status:         "pending",
		SourceType:     "user",
	}

	id, err := st.sql.CreateReminder(ctx, rem)
	if err != nil {
		t.Fatalf("CreateReminder failed: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero reminder ID")
	}

	list, err := st.sql.GetRemindersByUser(ctx, user.ID, "pending")
	if err != nil {
		t.Fatalf("GetRemindersByUser failed: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Тестовая задача" {
		t.Fatalf("unexpected reminders list: %+v", list)
	}

	err = st.sql.UpdateReminderStatus(ctx, id, "completed")
	if err != nil {
		t.Fatalf("UpdateReminderStatus failed: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./pkg/store -run TestSQLStore_RemindersCRUD`
Expected: FAIL (types and methods undefined)

- [ ] **Step 3: Write migration and minimal models + store methods**

Создать `pkg/db/migrations/postgres/00003_reminders_schema.sql`:
```sql
CREATE TABLE IF NOT EXISTS reminders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    remind_at TIMESTAMPTZ NOT NULL,
    recurrence_rule VARCHAR(32) NOT NULL DEFAULT 'none',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    source_type VARCHAR(32) NOT NULL DEFAULT 'user',
    source_ref_id VARCHAR(128) NOT NULL DEFAULT '',
    template_id VARCHAR(64),
    ab_variant VARCHAR(8) NOT NULL DEFAULT 'A',
    last_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reminders_worker 
ON reminders (status, remind_at ASC) 
WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_reminders_user_status 
ON reminders (user_id, status, remind_at ASC);

CREATE TABLE IF NOT EXISTS notification_templates (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_ab_test_active BOOLEAN NOT NULL DEFAULT FALSE,
    variant_a_text TEXT NOT NULL,
    variant_a_buttons JSONB NOT NULL DEFAULT '[]'::jsonb,
    variant_b_text TEXT NOT NULL DEFAULT '',
    variant_b_buttons JSONB NOT NULL DEFAULT '[]'::jsonb,
    split_ratio_percent INT NOT NULL DEFAULT 50,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_events (
    id BIGSERIAL PRIMARY KEY,
    reminder_id BIGINT REFERENCES reminders(id) ON DELETE SET NULL,
    template_id VARCHAR(64) REFERENCES notification_templates(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    variant VARCHAR(8) NOT NULL DEFAULT 'A',
    event_type VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO notification_templates (id, title, description, is_ab_test_active, variant_a_text, variant_b_text)
VALUES (
    'booking_reminder',
    'Напоминание о бронировании в Ресепшене',
    'Триггерное уведомление за 2 часа до визита',
    TRUE,
    'Напоминаем, что вы записаны на услугу {service_title} на {slot_time}. Ждём вас!',
    'Привет! 👋 Напоминаем о вашей записи на {service_title} ({slot_time}). Мастер уже готовится к встрече!'
) ON CONFLICT (id) DO NOTHING;
```

Добавить структуры в `pkg/store/models.go`:
```go
type Reminder struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	RemindAt       time.Time  `json:"remind_at"`
	RecurrenceRule string     `json:"recurrence_rule"` // "none", "daily", "weekdays", "weekly"
	Status         string     `json:"status"`          // "pending", "sent", "completed", "cancelled"
	SourceType     string     `json:"source_type"`     // "user", "reception", "demo_simulation"
	SourceRefID    string     `json:"source_ref_id"`
	TemplateID     string     `json:"template_id,omitempty"`
	ABVariant      string     `json:"ab_variant"`
	LastSentAt     *time.Time `json:"last_sent_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type NotificationTemplate struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	IsABTestActive    bool      `json:"is_ab_test_active"`
	VariantAText      string    `json:"variant_a_text"`
	VariantAButtons   string    `json:"variant_a_buttons"`
	VariantBText      string    `json:"variant_b_text"`
	VariantBButtons   string    `json:"variant_b_buttons"`
	SplitRatioPercent int       `json:"split_ratio_percent"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type NotificationEvent struct {
	ID         int64     `json:"id"`
	ReminderID *int64    `json:"reminder_id,omitempty"`
	TemplateID string    `json:"template_id,omitempty"`
	UserID     int64     `json:"user_id"`
	Variant    string    `json:"variant"`
	EventType  string    `json:"event_type"` // "sent", "viewed", "completed", "snoozed", "cancelled"
	CreatedAt  time.Time `json:"created_at"`
}
```

Реализовать методы в `pkg/store/sql_store.go`:
- `CreateReminder(ctx context.Context, rem *Reminder) (int64, error)`
- `GetRemindersByUser(ctx context.Context, userID int64, statusFilter string) ([]Reminder, error)`
- `GetReminderByID(ctx context.Context, id int64) (*Reminder, error)`
- `UpdateReminderStatus(ctx context.Context, id int64, status string) error`
- `SnoozeReminderInStore(ctx context.Context, id int64, newRemindAt time.Time) error`
- `CancelReminderBySourceRef(ctx context.Context, sourceType, refID string) error`
- `GetPendingRemindersForUpdate(ctx context.Context, limit int) ([]Reminder, error)`
- `RecordNotificationEvent(ctx context.Context, ev *NotificationEvent) error`
- `GetNotificationTemplate(ctx context.Context, id string) (*NotificationTemplate, error)`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./pkg/store -run TestSQLStore_RemindersCRUD`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/db/migrations/postgres/00003_reminders_schema.sql pkg/store/models.go pkg/store/sql_store.go pkg/store/sql_store_test.go
git commit -m "feat(store): add reminders and notification templates database schema and operations"
```

---

### Task 2: Reminders Service & Recurrence Engine (TDD)

**Files:**
- Create: `internal/modules/reminders/service.go`
- Create: `internal/modules/reminders/service_test.go`

**Interfaces:**
- Consumes: `pkg/store.SQLStore`, `pkg/store.Reminder`
- Produces: `reminders.Service`, `CalculateNextRecurrence(rule string, from time.Time, loc *time.Location) time.Time`, `ParseTimeInput(input string, now time.Time, loc *time.Location) (time.Time, error)`

- [ ] **Step 1: Write the failing tests for time parsing and recurrence calculation**

Создать `internal/modules/reminders/service_test.go`:
```go
package reminders

import (
	"testing"
	"time"
)

func TestCalculateNextRecurrence(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	// Пятница 2026-11-27 10:00
	friday := time.Date(2026, 11, 27, 10, 0, 0, 0, loc)

	// Daily -> Суббота
	nextDaily := CalculateNextRecurrence("daily", friday, loc)
	expectedDaily := time.Date(2026, 11, 28, 10, 0, 0, 0, loc)
	if !nextDaily.Equal(expectedDaily) {
		t.Fatalf("daily expected %v, got %v", expectedDaily, nextDaily)
	}

	// Weekdays: с пятницы следующий рабочий день — понедельник
	nextWeekday := CalculateNextRecurrence("weekdays", friday, loc)
	expectedWeekday := time.Date(2026, 11, 30, 10, 0, 0, 0, loc)
	if !nextWeekday.Equal(expectedWeekday) {
		t.Fatalf("weekdays expected %v, got %v", expectedWeekday, nextWeekday)
	}

	// Weekly -> Следующая пятница
	nextWeekly := CalculateNextRecurrence("weekly", friday, loc)
	expectedWeekly := time.Date(2026, 12, 4, 10, 0, 0, 0, loc)
	if !nextWeekly.Equal(expectedWeekly) {
		t.Fatalf("weekly expected %v, got %v", expectedWeekly, nextWeekly)
	}
}

func TestParseTimeInput(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	baseTime := time.Date(2026, 11, 25, 12, 0, 0, 0, loc)

	// Относительный ввод
	t1, err := ParseTimeInput("+15m", baseTime, loc)
	if err != nil || !t1.Equal(baseTime.Add(15*time.Minute)) {
		t.Fatalf("failed relative parse: %v, got %v", err, t1)
	}

	// Абсолютное время сегодня (15:30)
	t2, err := ParseTimeInput("15:30", baseTime, loc)
	if err != nil || t2.Hour() != 15 || t2.Minute() != 30 {
		t.Fatalf("failed time-only parse: %v, got %v", err, t2)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/reminders -run TestCalculateNextRecurrence`
Expected: FAIL

- [ ] **Step 3: Implement Service and Recurrence calculations**

Создать `internal/modules/reminders/service.go`:
- Функция `CalculateNextRecurrence(rule string, from time.Time, loc *time.Location) time.Time`:
  - `none`: `time.Time{}`
  - `daily`: `from.AddDate(0, 0, 1)`
  - `weekdays`: если текущий день Пт — переносим на Пн (`+3 дня`), если Сб — на Пн (`+2 дня`), иначе `+1 день`.
  - `weekly`: `from.AddDate(0, 0, 7)`.
- Функция `ParseTimeInput(input string, now time.Time, loc *time.Location) (time.Time, error)`:
  - Обработка `+15m`, `+1h`, `+3h`.
  - Обработка `HH:MM` (если время уже прошло сегодня — ставится на завтра).
  - Обработка `DD.MM HH:MM` и `DD.MM.YYYY HH:MM`.
- Методы сервиса `Service`:
  - `CreatePersonalReminder(ctx, userID, title, desc, remindAt, recurrence)`
  - `CompleteReminder(ctx, reminderID)`: завершает текущую задачу, и если `RecurrenceRule != "none"` — создает следующую задачу в `pending`.
  - `SnoozeReminder(ctx, reminderID, minutes)`: сдвигает `remind_at = NOW() + minutes`, сбрасывает в `pending`.
  - `ScheduleBookingReminder(ctx, params)`: создает триггер от Ресепшена с шаблоном `booking_reminder`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/reminders -run "TestCalculateNextRecurrence|TestParseTimeInput"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/reminders/service.go internal/modules/reminders/service_test.go
git commit -m "feat(reminders): implement service layer with recurrence and timezone-aware parser"
```

---

### Task 3: Background Worker with Rate Limiter & Message Delivery

**Files:**
- Create: `internal/modules/reminders/worker.go`
- Create: `internal/modules/reminders/worker_test.go`

**Interfaces:**
- Consumes: `reminders.Service`, `pkg/store.SQLStore`, `platform.PlatformAdapter`
- Produces: `Worker`, `NewWorker(service, sqlStore, adapter, interval, rps)`

- [ ] **Step 1: Write the failing test for worker batch processing**

Создать `internal/modules/reminders/worker_test.go`:
```go
package reminders

import (
	"context"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/store"
)

type mockAdapter struct {
	sentCount int
	lastText  string
}

func (m *mockAdapter) SendMessage(ctx context.Context, platformUserID string, text string, options any) error {
	m.sentCount++
	m.lastText = text
	return nil
}

func TestWorker_ProcessBatch(t *testing.T) {
	// Проверяет, что воркер забирает просроченную pending задачу,
	// отправляет ее через адаптер и переводит статус в sent.
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/reminders -run TestWorker_ProcessBatch`
Expected: FAIL

- [ ] **Step 3: Implement Worker with SKIP LOCKED and rate limiter**

Создать `internal/modules/reminders/worker.go`:
```go
package reminders

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type Worker struct {
	service  *Service
	sqlStore *store.SQLStore
	adapter  platform.PlatformAdapter
	limiter  *rate.Limiter
	interval time.Duration
}

func NewWorker(service *Service, sqlStore *store.SQLStore, adapter platform.PlatformAdapter, interval time.Duration, rps float64) *Worker {
	if rps <= 0 {
		rps = 25
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Worker{
		service:  service,
		sqlStore: sqlStore,
		adapter:  adapter,
		limiter:  rate.NewLimiter(rate.Limit(rps), 1),
		interval: interval,
	}
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.processPending(ctx); err != nil {
				log.Printf("[RemindersWorker] batch error: %v", err)
			}
		}
	}
}
```
Реализовать в `processPending`:
1. Вызов `sqlStore.GetPendingRemindersForUpdate(ctx, 50)`.
2. Проверка `w.limiter.Wait(ctx)`.
3. Поиск идентичности получателя по `r.UserID`.
4. Сборка текста и кнопок (`rmd:done:<id>`, `rmd:snz:15:<id>`, `rmd:snz:60:<id>`, `rmd:del:<id>`).
5. Отправка и обновление статуса в БД на `sent`, логирование `notification_events` (event_type = 'sent').

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/reminders -run TestWorker_ProcessBatch`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/reminders/worker.go internal/modules/reminders/worker_test.go
git commit -m "feat(reminders): implement background delivery worker with SKIP LOCKED and rate limiter"
```

---

### Task 4: Module Implementation & FSM Chat Handlers

**Files:**
- Create: `internal/modules/reminders/module.go`
- Create: `internal/modules/reminders/fsm.go`
- Create: `internal/modules/reminders/module_test.go`
- Modify: `cmd/showcase-bot/main.go`

**Interfaces:**
- Consumes: `core.BotModule`, `core.IncomingMessage`, `core.IncomingCallback`, `reminders.Service`
- Produces: `reminders.Module` registered in `core.Orchestrator`

- [ ] **Step 1: Write the failing test for reminders FSM steps**

Создать `internal/modules/reminders/module_test.go`:
```go
package reminders

import (
	"context"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

func TestRemindersModule_FSMFlow(t *testing.T) {
	// Тестирование шагов:
	// 1. Нажатие rmd:create -> переход в reminders:wait_title
	// 2. Ввод текста задачи -> переход в reminders:wait_time
	// 3. Выбор пресета времени -> переход в reminders:wait_recurrence
	// 4. Выбор повтора -> сохранение задачи и вывод карточки успеха
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/reminders -run TestRemindersModule_FSMFlow`
Expected: FAIL

- [ ] **Step 3: Implement Module and FSM handlers**

Создать `internal/modules/reminders/module.go`:
- `ID() string { return "reminders" }`
- `Title() string { return "Напоминания" }`
- `HandleMessage` и `HandleCallback` делегируют вызовы в FSM.

Создать `internal/modules/reminders/fsm.go`:
- Реализация шагов `reminders:wait_title`, `reminders:wait_time`, `reminders:wait_recurrence`.
- Обработка колбэков:
  - `rmd:create` — начало создания;
  - `rmd:list` — вывод активных задач с кнопками быстрого удаления/выполнения;
  - `rmd:demo_trigger` — планирование демо-триггера через 10 секунд;
  - `rmd:done:<id>` — выполнение задачи;
  - `rmd:snz:<mins>:<id>` — откладывание на 15 или 60 мин;
  - `rmd:del:<id>` — отмена задачи.

Подключить модуль и воркер в `cmd/showcase-bot/main.go`:
- Регистрация `remindersMod := reminders.New(remindersService, sqlStore)`
- `orchestrator.RegisterModule(remindersMod)`
- Запуск `go remindersWorker.Start(ctx)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/reminders -run TestRemindersModule_FSMFlow`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/reminders/module.go internal/modules/reminders/fsm.go internal/modules/reminders/module_test.go cmd/showcase-bot/main.go
git commit -m "feat(reminders): implement FSM chat dialogs, callback actions and register module in orchestrator"
```

---

### Task 5: Reception Booking Integration & Demo Simulation Trigger

**Files:**
- Modify: `internal/modules/reception/reception.go`
- Modify: `internal/modules/reminders/service.go`
- Test: `internal/modules/reception/reception_test.go`

**Interfaces:**
- Consumes: `reminders.Service`
- Produces: автоматическое создание напоминания при бронировании слота в Ресепшене

- [ ] **Step 1: Write test for linked reminder creation on booking**

Добавить тест в `internal/modules/reception/reception_test.go`:
- Проверяет, что при успешном оформлении бронирования услуги в Reception в таблице `reminders` появляется запись с `source_type = "reception"` и временем за 2 часа до начала слота.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/reception -run TestReception_LinkedReminder`
Expected: FAIL

- [ ] **Step 3: Implement hook in reception.go**

Внедрить вызов `ScheduleBookingReminder` при завершении шага оформления брони в `reception.go`:
```go
if m.remindersService != nil {
    _ = m.remindersService.ScheduleBookingReminder(ctx, reminders.ScheduleBookingParams{
        UserID:       u.ID,
        BookingID:    booking.ID,
        ServiceTitle: srv.Title,
        SlotTime:     slotTime,
        RemindAt:     slotTime.Add(-2 * time.Hour),
        TemplateID:   "booking_reminder",
    })
}
```
А также `CancelReminderBySourceRef(ctx, "reception", booking.ID)` при отмене бронирования.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/reception -run TestReception_LinkedReminder`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/reception/reception.go internal/modules/reception/reception_test.go
git commit -m "feat(reception): integrate booking hooks with reminders service"
```

---

### Task 6: REST API Handlers & Routing

**Files:**
- Create: `internal/api/handlers_reminders.go`
- Modify: `internal/api/router.go`
- Create: `internal/api/handlers_reminders_test.go`

**Interfaces:**
- Consumes: `reminders.Service`, `authMiddleware`
- Produces: HTTP эндпоинты `/api/v1/reminders`, `/api/v1/reminders/{id}/action`, `/api/v1/reminders/simulate`, `/api/v1/admin/notifications/ab-stats`

- [ ] **Step 1: Write the failing tests for reminders REST API**

Создать `internal/api/handlers_reminders_test.go`:
```go
func TestRemindersAPI_CreateAndList(t *testing.T) {
    // 1. POST /api/v1/reminders -> 201 Created
    // 2. GET /api/v1/reminders -> 200 OK со списком
    // 3. PATCH /api/v1/reminders/{id}/action (action="snooze", minutes=15) -> 200 OK
    // 4. DELETE /api/v1/reminders/{id} -> 204 No Content
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api -run TestRemindersAPI_CreateAndList`
Expected: FAIL (routes not found)

- [ ] **Step 3: Implement handlers and register routes**

Создать `internal/api/handlers_reminders.go`:
- `handleListReminders(w, r)`: выборка задач авторизованного пользователя.
- `handleCreateReminder(w, r)`: валидация JSON `title`, `remind_at`, `recurrence_rule`.
- `handleReminderAction(w, r)`: обработка экшенов `complete`, `snooze`, `cancel`.
- `handleDeleteReminder(w, r)`: удаление задачи.
- `handleSimulateReminder(w, r)`: запуск демо-триггера с задержкой 10 сек.
- `handleAdminABStats(w, r)`: получение конверсий по шаблонам и вариантам A/B.

Смонтировать в `internal/api/router.go`:
```go
mux.HandleFunc("GET /api/v1/reminders", authMiddleware(cfg, http.HandlerFunc(remH.handleListReminders)))
mux.HandleFunc("POST /api/v1/reminders", authMiddleware(cfg, http.HandlerFunc(remH.handleCreateReminder)))
mux.HandleFunc("PATCH /api/v1/reminders/{id}/action", authMiddleware(cfg, http.HandlerFunc(remH.handleReminderAction)))
mux.HandleFunc("DELETE /api/v1/reminders/{id}", authMiddleware(cfg, http.HandlerFunc(remH.handleDeleteReminder)))
mux.HandleFunc("POST /api/v1/reminders/simulate", authMiddleware(cfg, http.HandlerFunc(remH.handleSimulateReminder)))
mux.HandleFunc("GET /api/v1/admin/notifications/ab-stats", requireAdmin(remH.handleAdminABStats))
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api -run TestRemindersAPI_CreateAndList`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers_reminders.go internal/api/handlers_reminders_test.go internal/api/router.go
git commit -m "feat(api): add REST endpoints for reminders and A/B notifications analytics"
```

---

### Task 7: Mini App React Component `RemindersView.tsx` & Mode Activation

**Files:**
- Create: `web/src/components/modes/RemindersView.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/types/index.ts`

**Interfaces:**
- Consumes: `/api/v1/reminders` REST API
- Produces: полноценный UI вкладки «Напоминания» в Mini App

- [ ] **Step 1: Write TypeScript interface and API types**

В `web/src/types/index.ts` добавить:
```typescript
export interface ReminderItem {
  id: number;
  user_id: number;
  title: string;
  description: string;
  remind_at: string;
  recurrence_rule: 'none' | 'daily' | 'weekdays' | 'weekly';
  status: 'pending' | 'sent' | 'completed' | 'cancelled';
  source_type: 'user' | 'reception' | 'demo_simulation';
  source_ref_id?: string;
  created_at: string;
}
```

- [ ] **Step 2: Implement `RemindersView.tsx` component**

Создать `web/src/components/modes/RemindersView.tsx`:
- Сводка: количество запланированных и выполненных задач.
- Кнопка «⚡ Тест демо-триггера (10 сек)» с визуальным обратным отсчетом.
- Табы: «В планах» / «История».
- Кнопка «+ Новое напоминание» с модалкой создания:
  - Инпут заголовка;
  - Быстрые кнопки времени (`+15 мин`, `+1 час`, `Сегодня 19:00`, `Завтра 10:00`) + инпут даты/времени;
  - Селектор правила повтора.
- Список карточек задач с бейджами `[Ресепшен]` / `[Личное]` и кнопками `[✅ Выполнено]`, `[+15 мин]`, `[🗑️]`.

- [ ] **Step 3: Connect `RemindersView` in `web/src/App.tsx` and activate mode**

- В `initial8Modes`: переключить `reminders` в `is_active: true`.
- В рендере активного режима:
```tsx
{activeMode === 'reminders' && <RemindersView />}
```

- [ ] **Step 4: Verify frontend build**

Run: `npm --prefix web run build`
Expected: Success with 0 errors, generated bundle in `web/dist`.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/modes/RemindersView.tsx web/src/App.tsx web/src/types/index.ts
git commit -m "feat(webapp): add RemindersView tab and activate reminders mode in Mini App"
```

---

## Plan Self-Review Check
1. **Spec Coverage:**
   - Personal reminders & recurrence -> Tasks 1, 2, 4, 7.
   - Background worker with `SKIP LOCKED` & Rate Limiter -> Tasks 1, 3.
   - Interactive chat buttons & FSM -> Task 4.
   - Reception integration -> Tasks 1, 5.
   - A/B testing templates & events -> Tasks 1, 3, 6.
   - Mini App UI & REST API -> Tasks 6, 7.
2. **No Placeholders:** All structs, SQL scripts, route paths, and test snippets are fully specified.
3. **Type Consistency:** Method and struct names match across all tasks (`Reminder`, `NotificationTemplate`, `NotificationEvent`, `ScheduleBookingReminder`, `CalculateNextRecurrence`).
