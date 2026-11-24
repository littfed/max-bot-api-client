package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/store"
)

// OrchestratorStore abstracts store operations required by Core Orchestrator.
type OrchestratorStore interface {
	ResolveUserIdentity(ctx context.Context, platform, platformUserID, username, firstName, phone string) (*store.User, error)
	GetUser(ctx context.Context, id int64) (*store.User, error)
	SetUserActiveMode(ctx context.Context, userID int64, modeID string) error
	GetModeConfigs(ctx context.Context) ([]store.ModeConfig, error)
	SaveLead(ctx context.Context, lead *store.Lead) (int64, error)
}

// Orchestrator coordinates the 7-step domain event processing pipeline.
type Orchestrator struct {
	store       OrchestratorStore
	fsm         *CompositeFSM
	lockManager *LockManager
	guard       *MaintenanceGuard
	mu          sync.RWMutex
	modules     map[string]BotModule
	quotasMu    sync.Mutex
	memQuotas   map[string]int
}

// NewOrchestrator creates a new Core Orchestrator.
func NewOrchestrator(
	st OrchestratorStore,
	fsm *CompositeFSM,
	lockManager *LockManager,
	guard *MaintenanceGuard,
) *Orchestrator {
	return &Orchestrator{
		store:       st,
		fsm:         fsm,
		lockManager: lockManager,
		guard:       guard,
		modules:     make(map[string]BotModule),
		memQuotas:   make(map[string]int),
	}
}

// RegisterModule registers a BotModule for an active mode ID.
func (o *Orchestrator) RegisterModule(mod BotModule) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.modules[mod.ID()] = mod
}

// ProcessEvent executes the complete 7-step Bot Hub event pipeline.
func (o *Orchestrator) ProcessEvent(ctx context.Context, event *DomainEvent) (*DomainResponse, error) {
	if event == nil {
		return nil, errors.New("orchestrator: event cannot be nil")
	}

	// 1. Identity Resolver: maps (platform, platform_user_id) to persistent User
	var user *store.User
	if o.store != nil {
		var err error
		user, err = o.store.ResolveUserIdentity(
			ctx,
			event.Platform,
			event.PlatformUserID,
			event.Username,
			event.FirstName,
			event.Phone,
		)
		if err != nil {
			return nil, fmt.Errorf("orchestrator: failed to resolve user identity: %w", err)
		}
	} else {
		user = &store.User{
			ID:           1,
			ActiveModeID: "shop",
			Role:         "user",
		}
	}

	// 2. Global Command Interceptor: handles /start, /change_mode, /lead, /help
	if event.Message != nil {
		text := strings.TrimSpace(event.Message.Text)
		if strings.HasPrefix(text, "/") {
			parts := strings.Fields(text)
			cmd := strings.ToLower(parts[0])
			switch cmd {
			case "/start":
				return o.handleStart(ctx, user, event.Message)
			case "/hub":
				return o.handleListModes(ctx, user)
			case "/change_mode", "/mode":
				if len(parts) > 1 {
					return o.handleChangeMode(ctx, user, parts[1])
				}
				return o.handleListModes(ctx, user)
			case "/lead":
				comment := strings.Join(parts[1:], " ")
				return o.handleLead(ctx, user, event.Platform, comment)
			case "/help":
				return o.handleHelp(ctx, user)
			}
		}
	}

	// Intercept mode selection callbacks: "mode:select:<mode_id>"
	if event.Callback != nil && strings.HasPrefix(event.Callback.Data, "mode:select:") {
		targetMode := strings.TrimPrefix(event.Callback.Data, "mode:select:")
		return o.handleChangeMode(ctx, user, targetMode)
	}

	activeMode := user.ActiveModeID
	if activeMode == "" {
		activeMode = "shop"
	}

	// 3. Redis Lock Acquisition: lock:fsm:{user_id}:{active_mode_id}
	lockKey := LockKey(user.ID, activeMode)
	acquired, err := o.lockManager.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: failed to acquire lock: %w", err)
	}
	if !acquired {
		return &DomainResponse{
			UserID:       user.ID,
			ActiveModeID: activeMode,
			Text:         "Пожалуйста, подождите, предыдущее действие еще обрабатывается.",
			Handled:      false,
			Error:        ErrConcurrentAction,
		}, nil
	}
	defer func() {
		_ = o.lockManager.Release(ctx, lockKey)
	}()

	// 4. Maintenance Check: check if mode is under maintenance
	if o.guard != nil {
		isActive, maintMsg, err := o.guard.IsActive(ctx, activeMode)
		if err != nil {
			return nil, fmt.Errorf("orchestrator: maintenance check failed: %w", err)
		}
		if !isActive {
			buttons := o.activeModeButtons(ctx)
			return &DomainResponse{
				UserID:             user.ID,
				ActiveModeID:       activeMode,
				Text:               maintMsg,
				Buttons:            buttons,
				Handled:            true,
				IsMaintenance:      true,
				MaintenanceMessage: maintMsg,
			}, nil
		}
	}

	// 5. Quota Check: daily quota protection per user and mode
	dailyQuota := 100
	if o.store != nil {
		configs, err := o.store.GetModeConfigs(ctx)
		if err == nil {
			for _, cfg := range configs {
				if cfg.ModeID == activeMode {
					if cfg.DailyQuota > 0 {
						dailyQuota = cfg.DailyQuota
					}
					break
				}
			}
		}
	}
	quotaKey := fmt.Sprintf("bothub:quota:%d:%s:%s", user.ID, activeMode, time.Now().Format("20060102"))
	allowed, err := o.checkAndIncrementQuota(ctx, quotaKey, dailyQuota)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: quota check failed: %w", err)
	}
	if !allowed {
		return &DomainResponse{
			UserID:       user.ID,
			ActiveModeID: activeMode,
			Text:         "Суточный лимит операций в этом режиме исчерпан. Оставьте заявку: /lead",
			Handled:      true,
		}, nil
	}

	// 6. Module Dispatch: delegate event to registered virtual bot module
	o.mu.RLock()
	module, exists := o.modules[activeMode]
	o.mu.RUnlock()

	if !exists {
		return &DomainResponse{
			UserID:       user.ID,
			ActiveModeID: activeMode,
			Text:         fmt.Sprintf("Режим '%s' временно недоступен", activeMode),
			Handled:      false,
		}, nil
	}

	// Load composite FSM state for (user.ID, activeMode)
	state, err := o.fsm.Get(ctx, user.ID, activeMode)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: failed to get fsm state: %w", err)
	}

	var modResult *ModuleResult
	if event.Message != nil {
		modResult, err = module.HandleMessage(ctx, user, state, event.Message)
	} else if event.Callback != nil {
		modResult, err = module.HandleCallback(ctx, user, state, event.Callback)
	}

	if err != nil {
		return nil, fmt.Errorf("orchestrator: module %s handler error: %w", activeMode, err)
	}

	if modResult == nil {
		return &DomainResponse{
			UserID:       user.ID,
			ActiveModeID: activeMode,
			Handled:      false,
		}, nil
	}

	// 7. Unlock & Persistence: persist updated FSM step and payload or reset
	if modResult.ResetState {
		if err := o.fsm.Reset(ctx, user.ID, activeMode); err != nil {
			return nil, fmt.Errorf("orchestrator: failed to reset fsm state: %w", err)
		}
	} else if modResult.NewStep != "" || len(modResult.NewPayload) > 0 {
		newStep := modResult.NewStep
		if newStep == "" {
			newStep = state.CurrentStep
		}
		newPayload := modResult.NewPayload
		if len(newPayload) == 0 {
			newPayload = state.Payload
		}
		if err := o.fsm.Set(ctx, user.ID, activeMode, newStep, newPayload); err != nil {
			return nil, fmt.Errorf("orchestrator: failed to persist fsm state: %w", err)
		}
	}

	return &DomainResponse{
		UserID:       user.ID,
		ActiveModeID: activeMode,
		Text:         modResult.Text,
		Buttons:      modResult.Buttons,
		Handled:      modResult.Handled,
	}, nil
}

func (o *Orchestrator) handleStart(ctx context.Context, u *store.User, msg *IncomingMessage) (*DomainResponse, error) {
	activeMode := u.ActiveModeID
	if activeMode == "" {
		activeMode = "shop"
	}
	text := fmt.Sprintf("👋 Привет! Добро пожаловать в «Bot Hub» — мультирежимную платформу ботов.\n\nТекущий активный режим: *%s*.\nИспользуйте меню или Mini App для навигации.", activeMode)
	buttons := o.activeModeButtons(ctx)
	return &DomainResponse{
		UserID:       u.ID,
		ActiveModeID: activeMode,
		Text:         text,
		Buttons:      buttons,
		Handled:      true,
	}, nil
}

func (o *Orchestrator) handleChangeMode(ctx context.Context, u *store.User, targetMode string) (*DomainResponse, error) {
	targetMode = strings.ToLower(strings.TrimSpace(targetMode))
	if o.store != nil {
		if err := o.store.SetUserActiveMode(ctx, u.ID, targetMode); err != nil {
			return nil, fmt.Errorf("failed to change active mode: %w", err)
		}
	}
	u.ActiveModeID = targetMode

	text := fmt.Sprintf("✅ Активный режим переключен на: *%s*", targetMode)
	return &DomainResponse{
		UserID:       u.ID,
		ActiveModeID: targetMode,
		Text:         text,
		Handled:      true,
	}, nil
}

func (o *Orchestrator) handleListModes(ctx context.Context, u *store.User) (*DomainResponse, error) {
	text := "Выберите интересующий режим платформы «Bot Hub»:"
	buttons := o.activeModeButtons(ctx)
	return &DomainResponse{
		UserID:       u.ID,
		ActiveModeID: u.ActiveModeID,
		Text:         text,
		Buttons:      buttons,
		Handled:      true,
	}, nil
}

func (o *Orchestrator) handleLead(ctx context.Context, u *store.User, platform, comment string) (*DomainResponse, error) {
	if o.store != nil {
		lead := &store.Lead{
			UserID:      u.ID,
			Platform:    platform,
			SourceMode:  u.ActiveModeID,
			TestedModes: json.RawMessage(fmt.Sprintf(`["%s"]`, u.ActiveModeID)),
			Contact:     fmt.Sprintf("User ID: %d", u.ID),
			Comment:     comment,
			Status:      "new",
			CreatedAt:   time.Now(),
		}
		_, _ = o.store.SaveLead(ctx, lead)
	}

	return &DomainResponse{
		UserID:       u.ID,
		ActiveModeID: u.ActiveModeID,
		Text:         "🚀 Спасибо за интерес! Ваша заявка «Хочу такого бота» зарегистрирована. Наш специалист свяжется с вами.",
		Handled:      true,
	}, nil
}

func (o *Orchestrator) handleHelp(ctx context.Context, u *store.User) (*DomainResponse, error) {
	text := "📌 Команды платформы Bot Hub:\n" +
		"/start — Главное меню и перезапуск диалога\n" +
		"/mode [имя] — Переключение активного режима\n" +
		"/lead — Заявка «Хочу такого бота»\n" +
		"/help — Справка"
	return &DomainResponse{
		UserID:       u.ID,
		ActiveModeID: u.ActiveModeID,
		Text:         text,
		Handled:      true,
	}, nil
}

func (o *Orchestrator) activeModeButtons(ctx context.Context) [][]ResponseButton {
	var buttons [][]ResponseButton
	if o.store != nil {
		configs, err := o.store.GetModeConfigs(ctx)
		if err == nil {
			var row []ResponseButton
			for _, cfg := range configs {
				if cfg.IsActive {
					row = append(row, ResponseButton{
						Text: cfg.Title,
						Data: "mode:select:" + cfg.ModeID,
					})
					if len(row) == 2 {
						buttons = append(buttons, row)
						row = nil
					}
				}
			}
			if len(row) > 0 {
				buttons = append(buttons, row)
			}
			return buttons
		}
	}

	// Default buttons if store not available
	return [][]ResponseButton{
		{
			{Text: "🛒 Магазин", Data: "mode:select:shop"},
			{Text: "🏨 Ресепшен", Data: "mode:select:reception"},
		},
		{
			{Text: "🎧 Поддержка", Data: "mode:select:support"},
			{Text: "💬 Общение", Data: "mode:select:chat"},
		},
	}
}

func (o *Orchestrator) checkAndIncrementQuota(ctx context.Context, key string, limit int) (bool, error) {
	// 1. If Redis is available, check / increment via Redis
	if o.lockManager != nil && o.lockManager.redisClient != nil {
		if rc, ok := o.lockManager.redisClient.(interface {
			Incr(ctx context.Context, key string) (int64, error)
		}); ok {
			count, err := rc.Incr(ctx, key)
			if err == nil {
				return count <= int64(limit), nil
			}
		}
		if rc, ok := o.lockManager.redisClient.(RedisClient); ok {
			val, _ := rc.Get(ctx, key)
			curr := 0
			if val != "" {
				curr, _ = strconv.Atoi(val)
			}
			if curr >= limit {
				return false, nil
			}
			_ = rc.Set(ctx, key, strconv.Itoa(curr+1), 24*time.Hour)
			return true, nil
		}
	}

	// 2. Local memory counter
	o.quotasMu.Lock()
	defer o.quotasMu.Unlock()

	curr := o.memQuotas[key]
	if curr >= limit {
		return false, nil
	}
	o.memQuotas[key] = curr + 1
	return true, nil
}
