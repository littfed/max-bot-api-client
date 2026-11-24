package platform

import (
	"context"
	"time"
)

// Platform constants for supported messaging platforms.
const (
	PlatformMAX      = "max"
	PlatformTelegram = "telegram"
)

// ButtonType represents the interaction type of an inline button.
type ButtonType string

const (
	ButtonTypeCallback ButtonType = "callback"
	ButtonTypeURL      ButtonType = "url"
	ButtonTypeWebApp   ButtonType = "webapp"
)

// Button represents an interactive button in a messaging platform interface.
type Button struct {
	Text   string     `json:"text"`
	Type   ButtonType `json:"type,omitempty"`
	Action string     `json:"action,omitempty"` // Callback payload/action name
	Data   string     `json:"data,omitempty"`   // Callback payload alias
	URL    string     `json:"url,omitempty"`    // External link URL
	WebApp string     `json:"web_app,omitempty"` // Mini App / WebApp URL
}

// EffectiveType returns the resolved ButtonType based on explicit type or populated fields.
func (b Button) EffectiveType() ButtonType {
	if b.Type != "" {
		return b.Type
	}
	if b.WebApp != "" {
		return ButtonTypeWebApp
	}
	if b.URL != "" {
		return ButtonTypeURL
	}
	return ButtonTypeCallback
}

// CallbackData returns the payload used for callback queries.
func (b Button) CallbackData() string {
	if b.Action != "" {
		return b.Action
	}
	return b.Data
}

// OutMessage represents a standardized outgoing message across all platforms.
type OutMessage struct {
	Text      string     `json:"text"`
	Buttons   [][]Button `json:"buttons,omitempty"`
	ParseMode string     `json:"parse_mode,omitempty"` // e.g. "markdown", "html"
	Notify    *bool      `json:"notify,omitempty"`
}

// IncomingEvent represents a normalized incoming event from any messaging platform.
type IncomingEvent struct {
	Platform       string    `json:"platform"`
	PlatformUserID string    `json:"platform_user_id"`
	ChatID         string    `json:"chat_id,omitempty"`
	Type           string    `json:"type"` // "message", "callback", "webapp"
	MessageID      string    `json:"message_id,omitempty"`
	CallbackID     string    `json:"callback_id,omitempty"`
	Text           string    `json:"text,omitempty"`
	Data           string    `json:"data,omitempty"`
	Payload        string    `json:"payload,omitempty"`
	Username       string    `json:"username,omitempty"`
	FirstName      string    `json:"first_name,omitempty"`
	Phone          string    `json:"phone,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// PlatformAdapter defines the unified contract that any messaging platform must implement.
type PlatformAdapter interface {
	Platform() string
	SendMessage(ctx context.Context, recipientID string, body *OutMessage) (string, error)
	EditMessage(ctx context.Context, messageID string, body *OutMessage) error
	AnswerCallback(ctx context.Context, callbackID string, text string) error
	CheckMembership(ctx context.Context, chatID string, userID string) (bool, error)
	SendChatAction(ctx context.Context, recipientID string, action string) error
}
