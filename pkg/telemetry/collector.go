package telemetry

import (
	"context"
	"sync"
)

// Option configures a Collector instance.
type Option func(*Collector)

// WithBufferSize sets the ring buffer capacity.
func WithBufferSize(capacity int) Option {
	return func(c *Collector) {
		c.buffer = NewRingBuffer(capacity)
	}
}

// WithSink registers an EventSink during collector initialization.
func WithSink(sink EventSink) Option {
	return func(c *Collector) {
		if sink != nil {
			c.sinks = append(c.sinks, sink)
		}
	}
}

// Collector is the central telemetry orchestrator.
type Collector struct {
	mu     sync.RWMutex
	buffer *RingBuffer
	sinks  []EventSink
}

// NewCollector constructs a Collector with the given options.
func NewCollector(opts ...Option) *Collector {
	c := &Collector{
		buffer: NewRingBuffer(1000), // Default 1000 items
		sinks:  make([]EventSink, 0),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Record stores an event in the ring buffer and dispatches it to all sinks.
func (c *Collector) Record(ctx context.Context, e *Event) {
	if e == nil {
		return
	}

	// 1. Store in memory buffer
	c.buffer.Add(e)

	// 2. Dispatch to sinks
	c.mu.RLock()
	sinks := c.sinks
	c.mu.RUnlock()

	for _, s := range sinks {
		_ = s.Emit(ctx, e)
	}
}

// AddSink registers an additional EventSink.
func (c *Collector) AddSink(sink EventSink) {
	if sink == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sinks = append(c.sinks, sink)
}

// Events returns a snapshot of all events in the ring buffer.
func (c *Collector) Events() []*Event {
	return c.buffer.Snapshot()
}

// Last returns the most recent n events from the buffer.
func (c *Collector) Last(n int) []*Event {
	return c.buffer.Last(n)
}

// Filter queries events from the buffer using a predicate.
func (c *Collector) Filter(fn func(e *Event) bool) []*Event {
	return c.buffer.Filter(fn)
}

// Close closes all registered sinks.
func (c *Collector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error
	for _, s := range c.sinks {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
