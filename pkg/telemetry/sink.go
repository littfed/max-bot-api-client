package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"sync"
)

// EventSink consumes emitted telemetry events.
type EventSink interface {
	Emit(ctx context.Context, e *Event) error
	Close() error
}

// JSONLogSink outputs newline-delimited JSON events to an io.Writer.
type JSONLogSink struct {
	mu     sync.Mutex
	writer io.Writer
	enc    *json.Encoder
}

// NewJSONLogSink creates an EventSink that serializes each event to JSON on writer.
func NewJSONLogSink(w io.Writer) *JSONLogSink {
	return &JSONLogSink{
		writer: w,
		enc:    json.NewEncoder(w),
	}
}

// Emit writes the JSON representation of an event.
func (s *JSONLogSink) Emit(ctx context.Context, e *Event) error {
	if e == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(e)
}

// Close flushes if writer implements io.Closer.
func (s *JSONLogSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.writer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// FuncSink adapts a plain function into an EventSink.
type FuncSink struct {
	fn func(ctx context.Context, e *Event) error
}

// NewFuncSink wraps a handler function into an EventSink.
func NewFuncSink(fn func(ctx context.Context, e *Event) error) *FuncSink {
	return &FuncSink{fn: fn}
}

func (s *FuncSink) Emit(ctx context.Context, e *Event) error {
	if s.fn != nil {
		return s.fn(ctx, e)
	}
	return nil
}

func (s *FuncSink) Close() error {
	return nil
}
