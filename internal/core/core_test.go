package core_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
	_ "modernc.org/sqlite"
)

func setupTestStore(t *testing.T) *store.SQLStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	s, err := store.NewSQLStore(db, "sqlite")
	if err != nil {
		t.Fatalf("failed to create SQLStore: %v", err)
	}
	return s
}

func setupTestFSM(t *testing.T) *core.CompositeFSM {
	t.Helper()
	s := setupTestStore(t)
	return core.NewCompositeFSM(s, nil)
}

func setupTestLockManager(t *testing.T) *core.LockManager {
	t.Helper()
	return core.NewLockManager(nil)
}

// mockModule implements core.BotModule for testing dispatch
type mockModule struct {
	id          string
	title       string
	handledMsg  *core.IncomingMessage
	handledCb   *core.IncomingCallback
	replyResult *core.ModuleResult
	replyErr    error
}

func (m *mockModule) ID() string    { return m.id }
func (m *mockModule) Title() string { return m.title }
func (m *mockModule) HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	m.handledMsg = msg
	if m.replyResult != nil {
		return m.replyResult, m.replyErr
	}
	return &core.ModuleResult{
		Text:    "Processed in " + m.id,
		Handled: true,
	}, m.replyErr
}
func (m *mockModule) HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	m.handledCb = cb
	if m.replyResult != nil {
		return m.replyResult, m.replyErr
	}
	return &core.ModuleResult{
		Text:    "Callback processed in " + m.id,
		Handled: true,
	}, m.replyErr
}

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

func TestMaintenanceGuard_StatusAndInvalidation(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	guard := core.NewMaintenanceGuard(s, nil)

	// Mode 1 (shop) is active by default
	active, _, err := guard.IsActive(ctx, "shop")
	if err != nil {
		t.Fatalf("IsActive shop failed: %v", err)
	}
	if !active {
		t.Errorf("Expected shop to be active")
	}

	// Mode 3 (reminders) is in maintenance (inactive) by default in migration 00002
	activeReminders, maintMsg, err := guard.IsActive(ctx, "reminders")
	if err != nil {
		t.Fatalf("IsActive reminders failed: %v", err)
	}
	if activeReminders {
		t.Errorf("Expected reminders to be inactive/maintenance")
	}
	if maintMsg == "" {
		t.Errorf("Expected maintenance message for reminders")
	}

	// Toggle shop to inactive
	if err := guard.SetModeActive(ctx, "shop", false, "Технические работы в магазине"); err != nil {
		t.Fatalf("SetModeActive failed: %v", err)
	}

	activeAfter, msgAfter, err := guard.IsActive(ctx, "shop")
	if err != nil {
		t.Fatalf("IsActive after toggle failed: %v", err)
	}
	if activeAfter {
		t.Errorf("Expected shop to be inactive after toggle")
	}
	if msgAfter != "Технические работы в магазине" {
		t.Errorf("Unexpected maintenance message: %s", msgAfter)
	}
}

func TestOrchestrator_PipelineDispatch(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	fsm := core.NewCompositeFSM(s, nil)
	lockMgr := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(s, nil)
	orch := core.NewOrchestrator(s, fsm, lockMgr, guard)

	shopMod := &mockModule{
		id:    "shop",
		title: "Магазин",
		replyResult: &core.ModuleResult{
			Text:       "Добро пожаловать в каталог!",
			NewStep:    "catalog_view",
			NewPayload: json.RawMessage(`{"last_action":"open"}`),
			Handled:    true,
		},
	}
	orch.RegisterModule(shopMod)

	event := &core.DomainEvent{
		Type:           core.EventTypeMessage,
		Platform:       "max",
		PlatformUserID: "user_777",
		Username:       "john_doe",
		FirstName:      "John",
		Message: &core.IncomingMessage{
			ID:             "msg_1",
			ChatID:         777,
			Platform:       "max",
			PlatformUserID: "user_777",
			Text:           "Покажи товары",
			CreatedAt:      time.Now(),
		},
	}

	resp, err := orch.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}
	if resp == nil || !resp.Handled {
		t.Fatalf("Expected handled response, got %+v", resp)
	}
	if resp.Text != "Добро пожаловать в каталог!" {
		t.Errorf("Unexpected text: %s", resp.Text)
	}

	// Verify state was persisted in composite FSM
	st, err := fsm.Get(ctx, resp.UserID, "shop")
	if err != nil {
		t.Fatalf("fsm.Get failed: %v", err)
	}
	if st.CurrentStep != "catalog_view" {
		t.Errorf("Expected current step 'catalog_view', got %s", st.CurrentStep)
	}
}

func TestOrchestrator_MaintenanceRejection(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	fsm := core.NewCompositeFSM(s, nil)
	lockMgr := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(s, nil)
	orch := core.NewOrchestrator(s, fsm, lockMgr, guard)

	remindersMod := &mockModule{
		id:    "reminders",
		title: "Напоминания",
	}
	orch.RegisterModule(remindersMod)

	// Resolve user identity and set active mode to reminders
	u, err := s.ResolveUserIdentity(ctx, "max", "user_reminders", "test", "User", "")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}
	if err := s.SetUserActiveMode(ctx, u.ID, "reminders"); err != nil {
		t.Fatalf("SetUserActiveMode failed: %v", err)
	}

	event := &core.DomainEvent{
		Type:           core.EventTypeMessage,
		Platform:       "max",
		PlatformUserID: "user_reminders",
		Message: &core.IncomingMessage{
			ID:             "msg_2",
			ChatID:         888,
			Platform:       "max",
			PlatformUserID: "user_reminders",
			Text:           "Создай напоминание",
			CreatedAt:      time.Now(),
		},
	}

	resp, err := orch.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}
	if !resp.IsMaintenance {
		t.Errorf("Expected IsMaintenance true, got false")
	}
	if remindersMod.handledMsg != nil {
		t.Errorf("Module should NOT have received event during maintenance!")
	}
}

func TestOrchestrator_GlobalCommands(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	fsm := core.NewCompositeFSM(s, nil)
	lockMgr := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(s, nil)
	orch := core.NewOrchestrator(s, fsm, lockMgr, guard)

	event := &core.DomainEvent{
		Type:           core.EventTypeMessage,
		Platform:       "max",
		PlatformUserID: "user_global",
		Message: &core.IncomingMessage{
			ID:             "msg_start",
			ChatID:         999,
			Platform:       "max",
			PlatformUserID: "user_global",
			Text:           "/start",
			CreatedAt:      time.Now(),
		},
	}

	resp, err := orch.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent /start failed: %v", err)
	}
	if !resp.Handled {
		t.Errorf("Expected /start to be handled")
	}
}

type mockRedis struct {
	sync.Mutex
	data map[string]string
	ttls map[string]time.Duration
}

func newMockRedis() *mockRedis {
	return &mockRedis{
		data: make(map[string]string),
		ttls: make(map[string]time.Duration),
	}
}

func (m *mockRedis) Get(ctx context.Context, key string) (string, error) {
	m.Lock()
	defer m.Unlock()
	val, ok := m.data[key]
	if !ok {
		return "", errors.New("redis: nil")
	}
	return val, nil
}

func (m *mockRedis) Set(ctx context.Context, key string, val string, ttl time.Duration) error {
	m.Lock()
	defer m.Unlock()
	m.data[key] = val
	m.ttls[key] = ttl
	return nil
}

func (m *mockRedis) Del(ctx context.Context, keys ...string) error {
	m.Lock()
	defer m.Unlock()
	for _, k := range keys {
		delete(m.data, k)
		delete(m.ttls, k)
	}
	return nil
}

func (m *mockRedis) SetNX(ctx context.Context, key string, val string, ttl time.Duration) (bool, error) {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = val
	m.ttls[key] = ttl
	return true, nil
}

func TestCompositeFSM_RedisCaching(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	mr := newMockRedis()
	fsm := core.NewCompositeFSM(s, mr)

	userID := int64(1001)
	err := fsm.Set(ctx, userID, "shop", "checkout", []byte(`{"sum":1500}`))
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify key in mock redis
	rKey := "fsm:state:1001:shop"
	val, err := mr.Get(ctx, rKey)
	if err != nil || val == "" {
		t.Fatalf("Expected key %s in mock redis, got err: %v", rKey, err)
	}

	// Read via Get
	st, err := fsm.Get(ctx, userID, "shop")
	if err != nil || st.CurrentStep != "checkout" {
		t.Fatalf("Get returned unexpected state: %+v, err: %v", st, err)
	}
}

func TestRedisLock_Expiration(t *testing.T) {
	ctx := context.Background()
	lockMgr := core.NewLockManager(nil)

	key := "lock:expire:test"
	acquired, err := lockMgr.Acquire(ctx, key, 50*time.Millisecond)
	if err != nil || !acquired {
		t.Fatalf("First lock should succeed")
	}

	// Immediately re-acquire fails
	acquired2, _ := lockMgr.Acquire(ctx, key, 50*time.Millisecond)
	if acquired2 {
		t.Fatalf("Immediate second lock should fail")
	}

	// Wait for expiration
	time.Sleep(70 * time.Millisecond)

	acquired3, err := lockMgr.Acquire(ctx, key, 50*time.Millisecond)
	if err != nil || !acquired3 {
		t.Fatalf("Lock re-acquire should succeed after TTL expiration")
	}
}

func TestLockManager_WithMockRedis(t *testing.T) {
	ctx := context.Background()
	mr := newMockRedis()
	lockMgr := core.NewLockManager(mr)

	key := "lock:fsm:500:shop"
	ok1, err := lockMgr.Acquire(ctx, key, 2*time.Second)
	if err != nil || !ok1 {
		t.Fatalf("Lock acquire via mock redis should succeed")
	}

	ok2, err := lockMgr.Acquire(ctx, key, 2*time.Second)
	if err != nil || ok2 {
		t.Fatalf("Second lock acquire via mock redis should fail")
	}

	_ = lockMgr.Release(ctx, key)
	ok3, err := lockMgr.Acquire(ctx, key, 2*time.Second)
	if err != nil || !ok3 {
		t.Fatalf("Lock re-acquire via mock redis should succeed after release")
	}
}

func TestOrchestrator_ConcurrentLockError(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	fsm := core.NewCompositeFSM(s, nil)
	lockMgr := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(s, nil)
	orch := core.NewOrchestrator(s, fsm, lockMgr, guard)

	shopMod := &mockModule{id: "shop", title: "Магазин"}
	orch.RegisterModule(shopMod)

	// Pre-acquire lock for user 777 in shop
	lockKey := core.LockKey(1, "shop")
	_, _ = lockMgr.Acquire(ctx, lockKey, 5*time.Second)

	event := &core.DomainEvent{
		Type:           core.EventTypeMessage,
		Platform:       "max",
		PlatformUserID: "user_concurrent",
		Message: &core.IncomingMessage{
			ID:             "msg_c",
			ChatID:         123,
			Platform:       "max",
			PlatformUserID: "user_concurrent",
			Text:           "test message",
			CreatedAt:      time.Now(),
		},
	}

	resp, err := orch.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent should not fail with hard error: %v", err)
	}
	if resp.Handled {
		t.Errorf("Response should not be marked handled when locked")
	}
	if !errors.Is(resp.Error, core.ErrConcurrentAction) {
		t.Errorf("Expected ErrConcurrentAction, got %v", resp.Error)
	}
}

func TestOrchestrator_CallbackHandling(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	fsm := core.NewCompositeFSM(s, nil)
	lockMgr := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(s, nil)
	orch := core.NewOrchestrator(s, fsm, lockMgr, guard)

	shopMod := &mockModule{
		id:    "shop",
		title: "Магазин",
		replyResult: &core.ModuleResult{
			Text:    "Товар добавлен в корзину",
			NewStep: "cart_updated",
			Handled: true,
		},
	}
	orch.RegisterModule(shopMod)

	event := &core.DomainEvent{
		Type:           core.EventTypeCallback,
		Platform:       "max",
		PlatformUserID: "user_cb",
		Callback: &core.CallbackAction{
			ID:             "cb_1",
			ChatID:         555,
			MessageID:      "m_1",
			Platform:       "max",
			PlatformUserID: "user_cb",
			Data:           "shop:add:item_123",
		},
	}

	resp, err := orch.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent callback failed: %v", err)
	}
	if resp == nil || !resp.Handled {
		t.Fatalf("Expected handled response, got %+v", resp)
	}
	if resp.Text != "Товар добавлен в корзину" {
		t.Errorf("Unexpected text: %s", resp.Text)
	}
}

type mockErrStore struct {
	err error
}

func (m *mockErrStore) GetUserModeState(ctx context.Context, userID int64, modeID string) (*store.UserModeState, error) {
	return nil, m.err
}

func (m *mockErrStore) SaveUserModeState(ctx context.Context, state *store.UserModeState) error {
	return m.err
}

func TestCompositeFSM_DBError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("connection reset by peer")
	fsm := core.NewCompositeFSM(&mockErrStore{err: expectedErr}, nil)

	st, err := fsm.Get(ctx, 42, "shop")
	if err == nil {
		t.Fatalf("Expected error when DB fails, got state: %+v", st)
	}
	if !errors.Is(err, expectedErr) && !strings.Contains(err.Error(), "connection reset by peer") {
		t.Errorf("Expected db error to be wrapped, got %v", err)
	}
}

func TestOrchestrator_ResetState(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	fsm := core.NewCompositeFSM(s, nil)
	lockMgr := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(s, nil)
	orch := core.NewOrchestrator(s, fsm, lockMgr, guard)

	// Pre-populate state
	_ = fsm.Set(ctx, 1, "shop", "checkout", []byte(`{"order_id":100}`))

	shopMod := &mockModule{
		id:    "shop",
		title: "Магазин",
		replyResult: &core.ModuleResult{
			Text:       "Заказ оформлен!",
			ResetState: true,
			Handled:    true,
		},
	}
	orch.RegisterModule(shopMod)

	event := &core.DomainEvent{
		Type:           core.EventTypeMessage,
		Platform:       "max",
		PlatformUserID: "user_reset",
		Message: &core.IncomingMessage{
			ID:             "msg_r",
			ChatID:         777,
			Platform:       "max",
			PlatformUserID: "user_reset",
			Text:           "Завершить заказ",
			CreatedAt:      time.Now(),
		},
	}

	resp, err := orch.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}
	if !resp.Handled {
		t.Errorf("Expected handled true")
	}

	// Verify state is reset
	st, err := fsm.Get(ctx, resp.UserID, "shop")
	if err != nil {
		t.Fatalf("fsm.Get failed: %v", err)
	}
	if st.CurrentStep != "" || string(st.Payload) != "{}" {
		t.Errorf("Expected reset state with empty step and {}, got step=%q payload=%s", st.CurrentStep, string(st.Payload))
	}
}

func TestOrchestrator_QuotaLimit(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	fsm := core.NewCompositeFSM(s, nil)
	lockMgr := core.NewLockManager(nil)
	guard := core.NewMaintenanceGuard(s, nil)
	orch := core.NewOrchestrator(s, fsm, lockMgr, guard)

	shopMod := &mockModule{
		id:    "shop",
		title: "Магазин",
		replyResult: &core.ModuleResult{
			Text:    "OK",
			Handled: true,
		},
	}
	orch.RegisterModule(shopMod)

	u, err := s.ResolveUserIdentity(ctx, "max", "user_quota", "user", "Quota", "")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}
	_ = u

	event := &core.DomainEvent{
		Type:           core.EventTypeMessage,
		Platform:       "max",
		PlatformUserID: "user_quota",
		Message: &core.IncomingMessage{
			ID:             "msg_q",
			ChatID:         111,
			Platform:       "max",
			PlatformUserID: "user_quota",
			Text:           "запрос",
			CreatedAt:      time.Now(),
		},
	}

	// Loop 100 times to exhaust quota
	for i := 0; i < 100; i++ {
		r, err := orch.ProcessEvent(ctx, event)
		if err != nil {
			t.Fatalf("ProcessEvent request %d failed: %v", i, err)
		}
		if r.Text == "Суточный лимит операций в этом режиме исчерпан. Оставьте заявку: /lead" {
			t.Fatalf("Quota should not be reached at iteration %d", i)
		}
	}

	// 101st request must hit quota limit
	resp, err := orch.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent 101 failed: %v", err)
	}
	if !strings.Contains(resp.Text, "Суточный лимит операций в этом режиме исчерпан") {
		t.Errorf("Expected quota message, got: %q", resp.Text)
	}
}
