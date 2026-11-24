package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/internal/platform"
)

const (
	// DefaultBaseURL is the standard Telegram Bot API base URL.
	DefaultBaseURL = "https://api.telegram.org"
)

// Option configures an Adapter.
type Option func(*Adapter)

// WithBaseURL overrides the Telegram Bot API base URL (useful for self-hosted instances or tests).
func WithBaseURL(url string) Option {
	return func(a *Adapter) {
		a.baseURL = strings.TrimRight(url, "/")
	}
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(a *Adapter) {
		a.httpClient = client
	}
}

// WithTimeout sets a custom HTTP request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(a *Adapter) {
		a.timeout = timeout
	}
}

// Adapter implements platform.PlatformAdapter for Telegram Bot API.
type Adapter struct {
	token      string
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// Ensure Adapter implements platform.PlatformAdapter at compile time.
var _ platform.PlatformAdapter = (*Adapter)(nil)

// NewAdapter creates a new Telegram platform adapter with the provided bot token.
func NewAdapter(token string, opts ...Option) *Adapter {
	a := &Adapter{
		token:   token,
		baseURL: DefaultBaseURL,
		timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.httpClient == nil {
		a.httpClient = &http.Client{
			Timeout: a.timeout,
		}
	}
	return a
}

// Platform returns the unique platform identifier.
func (a *Adapter) Platform() string {
	return platform.PlatformTelegram
}

// InlineKeyboardButton represents a Telegram inline keyboard button.
type InlineKeyboardButton struct {
	Text         string          `json:"text"`
	CallbackData string          `json:"callback_data,omitempty"`
	URL          string          `json:"url,omitempty"`
	WebApp       *WebAppInfo     `json:"web_app,omitempty"`
}

// WebAppInfo describes a Web App for Telegram.
type WebAppInfo struct {
	URL string `json:"url"`
}

// InlineKeyboardMarkup represents a Telegram inline keyboard markup.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// SendMessage delivers a message to a chat or user on Telegram.
func (a *Adapter) SendMessage(ctx context.Context, recipientID string, body *platform.OutMessage) error {
	if body == nil {
		return fmt.Errorf("message body cannot be nil")
	}

	chatID := cleanRecipientID(recipientID)
	if chatID == "" {
		return fmt.Errorf("recipientID cannot be empty")
	}

	payload := map[string]any{
		"chat_id": chatID,
		"text":    body.Text,
	}

	if body.ParseMode != "" {
		payload["parse_mode"] = body.ParseMode
	}
	if body.Notify != nil && !*body.Notify {
		payload["disable_notification"] = true
	}

	if len(body.Buttons) > 0 {
		payload["reply_markup"] = buildInlineKeyboardMarkup(body.Buttons)
	}

	return a.call(ctx, "sendMessage", payload, nil)
}

// EditMessage edits an existing message on Telegram.
// messageID can be provided as "chatID:messageID" or as a single message ID (or inline message ID).
func (a *Adapter) EditMessage(ctx context.Context, messageID string, body *platform.OutMessage) error {
	if strings.TrimSpace(messageID) == "" {
		return fmt.Errorf("messageID cannot be empty")
	}
	if body == nil {
		return fmt.Errorf("message body cannot be nil")
	}

	payload := map[string]any{
		"text": body.Text,
	}

	if body.ParseMode != "" {
		payload["parse_mode"] = body.ParseMode
	}
	if len(body.Buttons) > 0 {
		payload["reply_markup"] = buildInlineKeyboardMarkup(body.Buttons)
	}

	// Handle composite "chatID:messageID" format
	if strings.Contains(messageID, ":") {
		parts := strings.SplitN(messageID, ":", 2)
		payload["chat_id"] = cleanRecipientID(parts[0])
		if mid, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			payload["message_id"] = mid
		} else {
			payload["message_id"] = parts[1]
		}
	} else if _, err := strconv.ParseInt(messageID, 10, 64); err == nil {
		return errors.New("telegram: chat message edit requires composite 'chatID:messageID' format or inline_message_id")
	} else {
		payload["inline_message_id"] = messageID
	}

	return a.call(ctx, "editMessageText", payload, nil)
}

// AnswerCallback acknowledges a Telegram callback query with optional notification text.
func (a *Adapter) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	if strings.TrimSpace(callbackID) == "" {
		return fmt.Errorf("callbackID cannot be empty")
	}

	payload := map[string]any{
		"callback_query_id": callbackID,
	}
	if text != "" {
		payload["text"] = text
	}

	return a.call(ctx, "answerCallbackQuery", payload, nil)
}

// CheckMembership checks if a user is currently a member/admin/creator of a Telegram chat.
func (a *Adapter) CheckMembership(ctx context.Context, chatID string, userID string) (bool, error) {
	cID := cleanRecipientID(chatID)
	if cID == "" {
		return false, fmt.Errorf("chatID cannot be empty")
	}
	uID, err := strconv.ParseInt(cleanRecipientID(userID), 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid userID %q: %w", userID, err)
	}

	payload := map[string]any{
		"chat_id": cID,
		"user_id": uID,
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := a.call(ctx, "getChatMember", payload, &resp); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "user not found") ||
			strings.Contains(errStr, "PARTICIPANT_ID_INVALID") ||
			strings.Contains(errStr, "chat not found") {
			return false, nil
		}
		return false, err
	}

	// Status can be: "creator", "administrator", "member", "restricted", "left", "kicked"
	switch resp.Status {
	case "creator", "administrator", "member", "restricted":
		return true, nil
	default:
		return false, nil
	}
}

// SendChatAction sends a chat presence action (e.g. typing) to a chat in Telegram.
func (a *Adapter) SendChatAction(ctx context.Context, recipientID string, action string) error {
	cID := cleanRecipientID(recipientID)
	if cID == "" {
		return fmt.Errorf("recipientID cannot be empty")
	}
	tgAction := action
	if tgAction == "" || tgAction == "typing_on" {
		tgAction = "typing"
	}
	payload := map[string]any{
		"chat_id": cID,
		"action":  tgAction,
	}
	var resp bool
	return a.call(ctx, "sendChatAction", payload, &resp)
}

// cleanRecipientID strips "user_", "chat_", "user:", "chat:" prefixes.
func cleanRecipientID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "user_") || strings.HasPrefix(lower, "user:") ||
		strings.HasPrefix(lower, "chat_") || strings.HasPrefix(lower, "chat:") {
		return trimmed[5:]
	}
	return trimmed
}

func buildInlineKeyboardMarkup(matrix [][]platform.Button) InlineKeyboardMarkup {
	var rows [][]InlineKeyboardButton
	for _, row := range matrix {
		var tgRow []InlineKeyboardButton
		for _, b := range row {
			btn := InlineKeyboardButton{Text: b.Text}
			switch b.EffectiveType() {
			case platform.ButtonTypeWebApp:
				btn.WebApp = &WebAppInfo{URL: b.WebApp}
			case platform.ButtonTypeURL:
				btn.URL = b.URL
			default:
				btn.CallbackData = b.CallbackData()
			}
			tgRow = append(tgRow, btn)
		}
		if len(tgRow) > 0 {
			rows = append(rows, tgRow)
		}
	}
	return InlineKeyboardMarkup{InlineKeyboard: rows}
}

// call sends an HTTP request to Telegram Bot API.
func (a *Adapter) call(ctx context.Context, method string, reqPayload any, target any) error {
	endpoint := fmt.Sprintf("%s/bot%s/%s", a.baseURL, a.token, method)

	var reqBody io.Reader
	if reqPayload != nil {
		data, err := json.Marshal(reqPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal telegram request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reqBody)
	if err != nil {
		return err
	}
	if reqPayload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram api request %s failed: %w", method, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read telegram response: %w", err)
	}

	var tgEnvelope struct {
		OK          bool            `json:"ok"`
		ErrorCode   int             `json:"error_code,omitempty"`
		Description string          `json:"description,omitempty"`
		Result      json.RawMessage `json:"result,omitempty"`
	}

	if err := json.Unmarshal(bodyBytes, &tgEnvelope); err != nil {
		return fmt.Errorf("failed to parse telegram response: %w (raw: %s)", err, string(bodyBytes))
	}

	if !tgEnvelope.OK {
		return fmt.Errorf("telegram api error (%d): %s", tgEnvelope.ErrorCode, tgEnvelope.Description)
	}

	if target != nil && len(tgEnvelope.Result) > 0 {
		if err := json.Unmarshal(tgEnvelope.Result, target); err != nil {
			return fmt.Errorf("failed to decode telegram result: %w", err)
		}
	}

	return nil
}
