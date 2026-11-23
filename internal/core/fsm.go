package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/store"
)

// State represents the active step and persistent draft payload for a user within a mode.
type State struct {
	UserID      int64           `json:"user_id"`
	ModeID      string          `json:"mode_id"`
	CurrentStep string          `json:"current_step"`
	Payload     json.RawMessage `json:"payload"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// FSMStore abstracts database operations needed by CompositeFSM.
type FSMStore interface {
	GetUserModeState(ctx context.Context, userID int64, modeID string) (*store.UserModeState, error)
	SaveUserModeState(ctx context.Context, state *store.UserModeState) error
}

// CompositeFSM manages multi-mode state isolation combining Redis caching and PostgreSQL persistence.
type CompositeFSM struct {
	store       FSMStore
	redisClient any
	mu          sync.RWMutex
	memStates   map[string]*State
}

// NewCompositeFSM creates a CompositeFSM.
func NewCompositeFSM(st FSMStore, redisClient any) *CompositeFSM {
	return &CompositeFSM{
		store:       st,
		redisClient: redisClient,
		memStates:   make(map[string]*State),
	}
}

func (f *CompositeFSM) cacheKey(userID int64, modeID string) string {
	return fmt.Sprintf("fsm:state:%d:%s", userID, modeID)
}

// Get retrieves state by composite key (userID, modeID).
func (f *CompositeFSM) Get(ctx context.Context, userID int64, modeID string) (*State, error) {
	key := f.cacheKey(userID, modeID)

	// 1. Try Redis cache
	if f.redisClient != nil {
		if rc, ok := f.redisClient.(RedisClient); ok {
			val, err := rc.Get(ctx, key)
			if err == nil && val != "" {
				var st State
				if err := json.Unmarshal([]byte(val), &st); err == nil {
					return &st, nil
				}
			}
		}
	}

	// 2. Try SQLStore
	if f.store != nil {
		dbState, err := f.store.GetUserModeState(ctx, userID, modeID)
		if err == nil && dbState != nil {
			st := &State{
				UserID:      dbState.UserID,
				ModeID:      dbState.ModeID,
				CurrentStep: dbState.CurrentStep,
				Payload:     dbState.Payload,
				UpdatedAt:   dbState.UpdatedAt,
			}
			// Warm cache
			if f.redisClient != nil {
				if rc, ok := f.redisClient.(RedisClient); ok {
					if raw, err := json.Marshal(st); err == nil {
						_ = rc.Set(ctx, key, string(raw), 24*time.Hour)
					}
				}
			}
			return st, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("fsm: failed to get user mode state: %w", err)
		}
	}

	// 3. Fallback to memory map
	f.mu.RLock()
	st, exists := f.memStates[key]
	f.mu.RUnlock()
	if exists && st != nil {
		// Return copy
		copied := *st
		return &copied, nil
	}

	// 4. Default fresh state if nothing exists yet
	return &State{
		UserID:      userID,
		ModeID:      modeID,
		CurrentStep: "",
		Payload:     json.RawMessage("{}"),
		UpdatedAt:   time.Now(),
	}, nil
}

// Set updates step and payload for the composite key (userID, modeID).
func (f *CompositeFSM) Set(ctx context.Context, userID int64, modeID string, step string, payload []byte) error {
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	if !json.Valid(payload) {
		return errors.New("fsm: invalid json payload")
	}

	st := &State{
		UserID:      userID,
		ModeID:      modeID,
		CurrentStep: step,
		Payload:     json.RawMessage(payload),
		UpdatedAt:   time.Now(),
	}

	// 1. Persist to SQLStore
	if f.store != nil {
		dbState := &store.UserModeState{
			UserID:      userID,
			ModeID:      modeID,
			CurrentStep: step,
			Payload:     st.Payload,
			UpdatedAt:   st.UpdatedAt,
		}
		if err := f.store.SaveUserModeState(ctx, dbState); err != nil {
			return fmt.Errorf("fsm: failed to save state to store: %w", err)
		}
	}

	// 2. Cache in Redis (TTL 24h)
	key := f.cacheKey(userID, modeID)
	if f.redisClient != nil {
		if rc, ok := f.redisClient.(RedisClient); ok {
			if raw, err := json.Marshal(st); err == nil {
				_ = rc.Set(ctx, key, string(raw), 24*time.Hour)
			}
		}
	}

	// 3. Save in local memory map
	f.mu.Lock()
	f.memStates[key] = st
	f.mu.Unlock()

	return nil
}

// Reset clears the state back to empty initial step and payload.
func (f *CompositeFSM) Reset(ctx context.Context, userID int64, modeID string) error {
	return f.Set(ctx, userID, modeID, "", []byte("{}"))
}
