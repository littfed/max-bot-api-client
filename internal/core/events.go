package core

import (
	"context"
	"encoding/json"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/store"
)

// IncomingMessage represents an incoming chat message from any platform.
type IncomingMessage struct {
	ID             string    `json:"id"`
	ChatID         int64     `json:"chat_id"`
	Platform       string    `json:"platform"`
	PlatformUserID string    `json:"platform_user_id"`
	Username       string    `json:"username,omitempty"`
	FirstName      string    `json:"first_name,omitempty"`
	Phone          string    `json:"phone,omitempty"`
	Text           string    `json:"text"`
	Command        string    `json:"command,omitempty"`
	Payload        string    `json:"payload,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// CallbackAction represents an inline button callback action from any platform.
type CallbackAction struct {
	ID             string `json:"id"`
	ChatID         int64  `json:"chat_id"`
	MessageID      string `json:"message_id"`
	Platform       string `json:"platform"`
	PlatformUserID string `json:"platform_user_id"`
	Data           string `json:"data"`
}

// IncomingCallback is an alias for CallbackAction.
type IncomingCallback = CallbackAction

// WebAppAction represents an event or action originating from Mini App SPA.
type WebAppAction struct {
	Endpoint       string          `json:"endpoint"`
	Action         string          `json:"action"`
	Platform       string          `json:"platform"`
	PlatformUserID string          `json:"platform_user_id"`
	Payload        json.RawMessage `json:"payload"`
}

// DomainEventType enumerates supported event categories.
type DomainEventType string

const (
	EventTypeMessage  DomainEventType = "message"
	EventTypeCallback DomainEventType = "callback"
	EventTypeWebApp   DomainEventType = "webapp"
)

// DomainEvent is the unified domain event ingested by the Core Orchestrator.
type DomainEvent struct {
	Type           DomainEventType  `json:"type"`
	Platform       string           `json:"platform"`
	PlatformUserID string           `json:"platform_user_id"`
	Username       string           `json:"username,omitempty"`
	FirstName      string           `json:"first_name,omitempty"`
	Phone          string           `json:"phone,omitempty"`
	Message        *IncomingMessage `json:"message,omitempty"`
	Callback       *CallbackAction  `json:"callback,omitempty"`
	WebApp         *WebAppAction    `json:"webapp,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// ResponseButton represents an inline or keyboard button in a response.
type ResponseButton struct {
	Text   string `json:"text"`
	Data   string `json:"data,omitempty"`
	URL    string `json:"url,omitempty"`
	WebApp string `json:"web_app,omitempty"`
}

// ModuleResult is the result returned by a BotModule when processing an event.
type ModuleResult struct {
	Text       string             `json:"text,omitempty"`
	Buttons    [][]ResponseButton `json:"buttons,omitempty"`
	NewStep    string             `json:"new_step,omitempty"`
	NewPayload json.RawMessage    `json:"new_payload,omitempty"`
	Handled    bool               `json:"handled"`
}

// DomainResponse represents the final response produced by Core Orchestrator.
type DomainResponse struct {
	UserID             int64              `json:"user_id"`
	ActiveModeID       string             `json:"active_mode_id"`
	Text               string             `json:"text,omitempty"`
	Buttons            [][]ResponseButton `json:"buttons,omitempty"`
	Handled            bool               `json:"handled"`
	IsMaintenance      bool               `json:"is_maintenance,omitempty"`
	MaintenanceMessage string             `json:"maintenance_message,omitempty"`
	Error              error              `json:"-"`
}

// BotModule defines the interface that all Bot Hub mode modules implement.
type BotModule interface {
	ID() string
	Title() string
	HandleMessage(ctx context.Context, u *store.User, state *State, msg *IncomingMessage) (*ModuleResult, error)
	HandleCallback(ctx context.Context, u *store.User, state *State, cb *IncomingCallback) (*ModuleResult, error)
}
