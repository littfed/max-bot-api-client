package fsm

import (
	"context"
	"sync"
	"time"
)

type memoryRecord struct {
	state     State
	data      map[string]any
	updatedAt time.Time
}

// MemoryStorage implements thread-safe in-memory session storage with TTL expiration.
type MemoryStorage struct {
	mu       sync.RWMutex
	sessions map[SessionKey]*memoryRecord
	ttl      time.Duration
	stopGC   chan struct{}
}

// NewMemoryStorage creates a new MemoryStorage with the given session TTL.
// If ttl <= 0, sessions never expire automatically.
func NewMemoryStorage(ttl time.Duration) *MemoryStorage {
	ms := &MemoryStorage{
		sessions: make(map[SessionKey]*memoryRecord),
		ttl:      ttl,
		stopGC:   make(chan struct{}),
	}

	if ttl > 0 {
		go ms.gcLoop(ttl)
	}

	return ms
}

func (ms *MemoryStorage) gcLoop(ttl time.Duration) {
	ticker := time.NewTicker(ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.cleanupExpired()
		case <-ms.stopGC:
			return
		}
	}
}

func (ms *MemoryStorage) cleanupExpired() {
	if ms.ttl <= 0 {
		return
	}
	now := time.Now()
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for k, rec := range ms.sessions {
		if now.Sub(rec.updatedAt) > ms.ttl {
			delete(ms.sessions, k)
		}
	}
}

func (ms *MemoryStorage) getRecord(key SessionKey) *memoryRecord {
	rec, ok := ms.sessions[key]
	if !ok {
		rec = &memoryRecord{
			state:     DefaultState,
			data:      make(map[string]any),
			updatedAt: time.Now(),
		}
		ms.sessions[key] = rec
		return rec
	}

	// Check expiration on read
	if ms.ttl > 0 && time.Since(rec.updatedAt) > ms.ttl {
		rec.state = DefaultState
		rec.data = make(map[string]any)
	}
	rec.updatedAt = time.Now()
	return rec
}

// SetState updates the state of a session.
func (ms *MemoryStorage) SetState(ctx context.Context, key SessionKey, state State) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	rec := ms.getRecord(key)
	rec.state = state
	rec.updatedAt = time.Now()
	return nil
}

// GetState retrieves the active state of a session.
func (ms *MemoryStorage) GetState(ctx context.Context, key SessionKey) (State, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	rec := ms.getRecord(key)
	return rec.state, nil
}

// SetData stores payload data for the session.
func (ms *MemoryStorage) SetData(ctx context.Context, key SessionKey, data map[string]any) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	rec := ms.getRecord(key)
	copyData := make(map[string]any, len(data))
	for k, v := range data {
		copyData[k] = v
	}
	rec.data = copyData
	rec.updatedAt = time.Now()
	return nil
}

// GetData retrieves a copy of the payload data.
func (ms *MemoryStorage) GetData(ctx context.Context, key SessionKey) (map[string]any, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	rec := ms.getRecord(key)
	copyData := make(map[string]any, len(rec.data))
	for k, v := range rec.data {
		copyData[k] = v
	}
	return copyData, nil
}

// UpdateData mutates the session payload safely under a lock.
func (ms *MemoryStorage) UpdateData(ctx context.Context, key SessionKey, fn func(data map[string]any) error) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	rec := ms.getRecord(key)
	if err := fn(rec.data); err != nil {
		return err
	}
	rec.updatedAt = time.Now()
	return nil
}

// Reset clears the state and data of a session.
func (ms *MemoryStorage) Reset(ctx context.Context, key SessionKey) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.sessions, key)
	return nil
}

// Close stops the background GC worker.
func (ms *MemoryStorage) Close() error {
	select {
	case <-ms.stopGC:
	default:
		close(ms.stopGC)
	}
	return nil
}
