package livetest

import (
	"fmt"
	"time"
)

// TestStatus represents the outcome of a test case.
type TestStatus string

const (
	StatusPassed  TestStatus = "PASSED"
	StatusFailed  TestStatus = "FAILED"
	StatusSkipped TestStatus = "SKIPPED"
	StatusTimeout TestStatus = "TIMEOUT"
)

// EventDirection indicates whether an event is outbound (bot to user/API) or inbound (user to bot).
type EventDirection string

const (
	DirectionOutbound EventDirection = "OUTBOUND"
	DirectionInbound  EventDirection = "INBOUND"
	DirectionAction   EventDirection = "ACTION"
	DirectionAssert   EventDirection = "ASSERT"
)

// TranscriptEvent records an individual interaction step during a test run.
type TranscriptEvent struct {
	Timestamp time.Time      `json:"timestamp"`
	Direction EventDirection `json:"direction"`
	Title     string         `json:"title"`
	Text      string         `json:"text,omitempty"`
	Payload   any            `json:"payload,omitempty"`
	RTT       time.Duration  `json:"rtt,omitempty"`
	Passed    bool           `json:"passed"`
}

// TestRecord contains the detailed execution metrics and transcript of a single test case.
type TestRecord struct {
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Status      TestStatus        `json:"status"`
	Duration    time.Duration     `json:"duration"`
	RTT         time.Duration     `json:"rtt,omitempty"`
	Error       error             `json:"-"`
	ErrorMsg    string            `json:"error,omitempty"`
	Events      []TranscriptEvent `json:"events,omitempty"`
	Details     map[string]any    `json:"details,omitempty"`
}

// AddEvent appends a transcript event to the record.
func (r *TestRecord) AddEvent(dir EventDirection, title, text string, passed bool, payload any) {
	r.Events = append(r.Events, TranscriptEvent{
		Timestamp: time.Now(),
		Direction: dir,
		Title:     title,
		Text:      text,
		Passed:    passed,
		Payload:   payload,
	})
}

// SuiteSummary aggregates the overall statistics of a test run.
type SuiteSummary struct {
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Skipped   int           `json:"skipped"`
	Timeout   int           `json:"timeout"`
	Duration  time.Duration `json:"duration"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
}

// String returns a human-readable summary line.
func (s SuiteSummary) String() string {
	return fmt.Sprintf("Total: %d | Passed: %d | Failed: %d | Skipped: %d | Timeout: %d | Time: %s",
		s.Total, s.Passed, s.Failed, s.Skipped, s.Timeout, s.Duration.Round(time.Millisecond))
}
