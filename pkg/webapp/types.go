package webapp

import (
	"encoding/json"
	"time"
)

// InitDataUser represents the authenticated user payload unpacked from WebApp initData.
type InitDataUser struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
	IsPremium    bool   `json:"is_premium,omitempty"`
}

// InitData represents the decoded and cryptographically verified WebApp launch context.
type InitData struct {
	QueryID      string        `json:"query_id,omitempty"`
	User         *InitDataUser `json:"user,omitempty"`
	AuthDate     time.Time     `json:"auth_date"`
	Hash         string        `json:"hash"`
	StartParam   string        `json:"start_param,omitempty"`
	ChatType     string        `json:"chat_type,omitempty"`
	ChatInstance string        `json:"chat_instance,omitempty"`
	Raw          string        `json:"raw"`
}

// ActionRequest represents an action or event sent from the Mini App webview to the bot backend.
type ActionRequest struct {
	InitData string          `json:"init_data"`
	Action   string          `json:"action"`
	Payload  json.RawMessage `json:"payload"`
}

// ActionResponse represents the standard response returned back to the webview.
type ActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
