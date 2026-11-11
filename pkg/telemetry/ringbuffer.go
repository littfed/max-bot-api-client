package telemetry

import (
	"sync"
)

// RingBuffer stores a fixed-capacity circular buffer of telemetry events.
type RingBuffer struct {
	mu       sync.RWMutex
	capacity int
	events   []*Event
	head     int
	count    int
}

// NewRingBuffer allocates a new ring buffer with the specified capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 500
	}
	return &RingBuffer{
		capacity: capacity,
		events:   make([]*Event, capacity),
	}
}

// Add inserts a new event into the buffer, overwriting the oldest event if full.
func (rb *RingBuffer) Add(e *Event) {
	if e == nil {
		return
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.events[rb.head] = e
	rb.head = (rb.head + 1) % rb.capacity
	if rb.count < rb.capacity {
		rb.count++
	}
}

// Len returns the current number of stored events.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Snapshot returns a copy of all buffered events in chronological order (oldest to newest).
func (rb *RingBuffer) Snapshot() []*Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	res := make([]*Event, rb.count)
	if rb.count == 0 {
		return res
	}

	start := (rb.head - rb.count + rb.capacity) % rb.capacity
	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.capacity
		res[i] = rb.events[idx]
	}
	return res
}

// Last returns up to n of the most recent events (newest first).
func (rb *RingBuffer) Last(n int) []*Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || rb.count == 0 {
		return nil
	}
	if n > rb.count {
		n = rb.count
	}

	res := make([]*Event, n)
	for i := 0; i < n; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		res[i] = rb.events[idx]
	}
	return res
}

// Filter returns all buffered events that satisfy the given predicate.
func (rb *RingBuffer) Filter(fn func(e *Event) bool) []*Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var res []*Event
	start := (rb.head - rb.count + rb.capacity) % rb.capacity
	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.capacity
		ev := rb.events[idx]
		if fn(ev) {
			res = append(res, ev)
		}
	}
	return res
}

// Clear removes all stored events from the buffer.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.events = make([]*Event, rb.capacity)
	rb.head = 0
	rb.count = 0
}
