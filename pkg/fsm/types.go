package fsm

import (
	"context"
	"fmt"
)

// State represents the current dialog step/status of a user session.
type State string

// DefaultState represents an uninitialized or idle dialog state.
const DefaultState State = ""

// StateEmpty is an alias for DefaultState.
const StateEmpty = DefaultState

// SessionKey uniquely identifies a user within a specific chat/dialog.
type SessionKey struct {
	UserID int64 `json:"user_id"`
	ChatID int64 `json:"chat_id"`
}

// String returns a composite string identifier "userID:chatID".
func (k SessionKey) String() string {
	return fmt.Sprintf("%d:%d", k.UserID, k.ChatID)
}

// Storage is the persistent or in-memory backend for storing FSM states and payloads.
type Storage interface {
	SetState(ctx context.Context, key SessionKey, state State) error
	GetState(ctx context.Context, key SessionKey) (State, error)
	SetData(ctx context.Context, key SessionKey, data map[string]any) error
	GetData(ctx context.Context, key SessionKey) (map[string]any, error)
	UpdateData(ctx context.Context, key SessionKey, fn func(data map[string]any) error) error
	Reset(ctx context.Context, key SessionKey) error
	Close() error
}
