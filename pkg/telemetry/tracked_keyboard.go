package telemetry

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

const (
	payloadPrefix = "trk:"
	separator     = "|"
)

// ButtonMetadata holds decoded tracking tags embedded in button payloads.
type ButtonMetadata struct {
	Action    string `json:"action"`
	Screen    string `json:"screen,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	IsTracked bool   `json:"is_tracked"`
}

// FormatTrackedPayload encodes an action, screen name, and trace ID into a compact payload.
func FormatTrackedPayload(action, screen, traceID string) string {
	action = strings.ReplaceAll(action, separator, "_")
	screen = strings.ReplaceAll(screen, separator, "_")
	traceID = strings.ReplaceAll(traceID, separator, "_")

	if traceID == "" {
		traceID = strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
	}

	return fmt.Sprintf("%s%s%s%s%s%s", payloadPrefix, action, separator, screen, separator, traceID)
}

// ParseTrackedPayload decodes a button payload into ButtonMetadata.
// If the payload does not contain tracking prefix, it gracefully returns the payload as Action with IsTracked=false.
func ParseTrackedPayload(payload string) *ButtonMetadata {
	if !strings.HasPrefix(payload, payloadPrefix) {
		return &ButtonMetadata{
			Action:    payload,
			IsTracked: false,
		}
	}

	body := strings.TrimPrefix(payload, payloadPrefix)
	parts := strings.Split(body, separator)

	meta := &ButtonMetadata{
		Action:    parts[0],
		IsTracked: true,
	}
	if len(parts) > 1 {
		meta.Screen = parts[1]
	}
	if len(parts) > 2 {
		meta.TraceID = parts[2]
	}
	return meta
}

// TrackedLinkURL decorates an external URL with telemetry parameters (max_uid, utm_source, utm_campaign, ref_trk).
func TrackedLinkURL(rawURL string, userID int64, screen, traceID string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	q := parsed.Query()
	q.Set("utm_source", "max_bot")
	if userID != 0 {
		q.Set("max_uid", strconv.FormatInt(userID, 10))
	}
	if screen != "" {
		q.Set("utm_campaign", screen)
	}
	if traceID != "" {
		q.Set("ref_trk", traceID)
	}

	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// TrackedKeyboard provides a fluent builder for inline keyboards with automatic telemetry instrumentation.
type TrackedKeyboard struct {
	kb      *maxbot.InlineKeyboard
	screen  string
	userID  int64
	traceID string
	curRow  []maxbot.Button
}

// NewTrackedKeyboard initializes a tracked keyboard builder scoped to a screen and user.
func NewTrackedKeyboard(screen string, userID int64) *TrackedKeyboard {
	return &TrackedKeyboard{
		kb:      maxbot.NewInlineKeyboard(),
		screen:  screen,
		userID:  userID,
		traceID: strconv.FormatInt(time.Now().UnixNano()%1000000, 10),
	}
}

// Callback adds an instrumented callback button to the current row.
func (tk *TrackedKeyboard) Callback(text, action string, intent ...string) *TrackedKeyboard {
	payload := FormatTrackedPayload(action, tk.screen, tk.traceID)
	btnIntent := maxbot.IntentDefault
	if len(intent) > 0 && intent[0] != "" {
		btnIntent = intent[0]
	}
	tk.curRow = append(tk.curRow, maxbot.NewIntentCallbackButton(text, payload, btnIntent))
	return tk
}

// Link adds an instrumented external link button to the current row.
func (tk *TrackedKeyboard) Link(text, rawURL string) *TrackedKeyboard {
	trackedURL := TrackedLinkURL(rawURL, tk.userID, tk.screen, tk.traceID)
	tk.curRow = append(tk.curRow, maxbot.NewLinkButton(text, trackedURL))
	return tk
}

// Clipboard adds a button that copies text to clipboard.
func (tk *TrackedKeyboard) Clipboard(text, textToCopy string) *TrackedKeyboard {
	tk.curRow = append(tk.curRow, maxbot.NewClipboardButton(text, textToCopy))
	return tk
}

// Row flushes the current row of buttons and starts a new row.
func (tk *TrackedKeyboard) Row(buttons ...maxbot.Button) *TrackedKeyboard {
	if len(tk.curRow) > 0 {
		tk.kb.Row(tk.curRow...)
		tk.curRow = nil
	}
	if len(buttons) > 0 {
		tk.kb.Row(buttons...)
	}
	return tk
}

// Build finalizes and returns the underlying *maxbot.InlineKeyboard.
func (tk *TrackedKeyboard) Build() *maxbot.InlineKeyboard {
	if len(tk.curRow) > 0 {
		tk.kb.Row(tk.curRow...)
		tk.curRow = nil
	}
	return tk.kb
}

// ClickMetrics holds the analytics extracted from an inbound button callback update.
type ClickMetrics struct {
	UserID     int64
	UserName   string
	ChatID     int64
	MessageID  string
	Action     string
	Screen     string
	TraceID    string
	TimeDelta  time.Duration // Time to Click (TTK)
	IsTracked  bool
	RawPayload string
}

// ExtractClickMetrics parses a MAX update and extracts comprehensive button click metrics.
func ExtractClickMetrics(u *maxbot.Update) (*ClickMetrics, bool) {
	if u == nil || u.UpdateType != maxbot.UpdateTypeMessageCallback || u.Callback == nil {
		return nil, false
	}

	cb := u.Callback
	meta := ParseTrackedPayload(cb.Payload)

	metrics := &ClickMetrics{
		Action:     meta.Action,
		Screen:     meta.Screen,
		TraceID:    meta.TraceID,
		IsTracked:  meta.IsTracked,
		RawPayload: cb.Payload,
	}

	if cb.User != nil {
		metrics.UserID = cb.User.UserID
		metrics.UserName = cb.User.Name
	}

	if u.Message != nil {
		metrics.MessageID = u.Message.MessageID
		if metrics.MessageID == "" && u.Message.Body.MID != "" {
			metrics.MessageID = u.Message.Body.MID
		}
		if u.Message.Recipient != nil {
			metrics.ChatID = u.Message.Recipient.ChatID
		}

		// Calculate Time to Click (TTK)
		if u.Message.Timestamp > 0 && cb.Timestamp > u.Message.Timestamp {
			metrics.TimeDelta = time.Duration(cb.Timestamp-u.Message.Timestamp) * time.Millisecond
		}
	}

	return metrics, true
}
