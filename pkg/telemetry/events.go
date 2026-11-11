package telemetry

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Global atomic counter for generating lightweight unique event IDs
var eventCounter uint64

// EventType classifies the category of a telemetry event.
type EventType string

const (
	EventButtonClick EventType = "button_click"
	EventCommand     EventType = "command"
	EventMessage     EventType = "message"
	EventApiCall     EventType = "api_call"
	EventError       EventType = "error"
	EventCustom      EventType = "custom"
)

// Event represents an immutable, strongly-typed telemetry record.
type Event struct {
	ID         string         `json:"id"`
	Type       EventType      `json:"type"`
	Timestamp  time.Time      `json:"timestamp"`
	UserID     int64          `json:"user_id,omitempty"`
	ChatID     int64          `json:"chat_id,omitempty"`
	Action     string         `json:"action,omitempty"`
	Screen     string         `json:"screen,omitempty"`
	Duration   time.Duration  `json:"duration,omitempty"`   // Execution or RTT latency
	TimeDelta  time.Duration  `json:"time_delta,omitempty"` // E.g. Time to click (TTK)
	Error      string         `json:"error,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

func nextEventID() string {
	val := atomic.AddUint64(&eventCounter, 1)
	return fmt.Sprintf("evt_%d_%d", time.Now().UnixNano()%1000000, val)
}

// NewButtonClickEvent creates an event representing an inline button press.
func NewButtonClickEvent(userID, chatID int64, action, screen string, ttk time.Duration) *Event {
	return &Event{
		ID:         nextEventID(),
		Type:       EventButtonClick,
		Timestamp:  time.Now(),
		UserID:     userID,
		ChatID:     chatID,
		Action:     action,
		Screen:     screen,
		TimeDelta:  ttk,
		Properties: make(map[string]any),
	}
}

// NewCommandEvent creates an event representing a bot command execution.
func NewCommandEvent(userID, chatID int64, cmd string, duration time.Duration, err error) *Event {
	ev := &Event{
		ID:         nextEventID(),
		Type:       EventCommand,
		Timestamp:  time.Now(),
		UserID:     userID,
		ChatID:     chatID,
		Action:     cmd,
		Duration:   duration,
		Properties: make(map[string]any),
	}
	if err != nil {
		ev.Error = err.Error()
	}
	return ev
}

// NewApiCallEvent creates an event representing an outbound MAX API call.
func NewApiCallEvent(method string, rtt time.Duration, err error) *Event {
	ev := &Event{
		ID:         nextEventID(),
		Type:       EventApiCall,
		Timestamp:  time.Now(),
		Action:     method,
		Duration:   rtt,
		Properties: make(map[string]any),
	}
	if err != nil {
		ev.Error = err.Error()
	}
	return ev
}

// NewErrorEvent creates a system or business error event.
func NewErrorEvent(userID, chatID int64, action string, err error) *Event {
	ev := &Event{
		ID:         nextEventID(),
		Type:       EventError,
		Timestamp:  time.Now(),
		UserID:     userID,
		ChatID:     chatID,
		Action:     action,
		Properties: make(map[string]any),
	}
	if err != nil {
		ev.Error = err.Error()
	}
	return ev
}

// NewCustomEvent creates an arbitrary business analytics event.
func NewCustomEvent(userID, chatID int64, name string, props map[string]any) *Event {
	if props == nil {
		props = make(map[string]any)
	}
	return &Event{
		ID:         nextEventID(),
		Type:       EventCustom,
		Timestamp:  time.Now(),
		UserID:     userID,
		ChatID:     chatID,
		Action:     name,
		Properties: props,
	}
}
