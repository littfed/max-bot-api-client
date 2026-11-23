package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrConcurrentAction is returned when a user action is locked by another running operation.
var ErrConcurrentAction = errors.New("concurrent action locked")

// RedisClient defines the basic Redis operations required by core.
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisNXClient defines the atomic SetNX interface.
type RedisNXClient interface {
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
}

type memLockEntry struct {
	expiresAt time.Time
	token     string
}

// LockManager manages distributed locks backed by Redis or an in-memory fallback.
type LockManager struct {
	redisClient any
	mu          sync.Mutex
	memLocks    map[string]memLockEntry
}

// NewLockManager creates a new LockManager. If redisClient is nil, safe local memory mutex fallback is used.
func NewLockManager(redisClient any) *LockManager {
	return &LockManager{
		redisClient: redisClient,
		memLocks:    make(map[string]memLockEntry),
	}
}

// LockKey constructs the canonical lock key: lock:fsm:{user_id}:{mode_id}
func LockKey(userID int64, modeID string) string {
	return fmt.Sprintf("lock:fsm:%d:%s", userID, modeID)
}

// Acquire attempts to acquire a lock for the given key with a TTL.
// Returns true if the lock was acquired, false if it is currently held.
func (lm *LockManager) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}

	// 1. If Redis client is provided, use Redis SET NX EX
	if lm.redisClient != nil {
		if nxClient, ok := lm.redisClient.(RedisNXClient); ok {
			return nxClient.SetNX(ctx, key, "1", ttl)
		}
		if rc, ok := lm.redisClient.(RedisClient); ok {
			_, err := rc.Get(ctx, key)
			if err == nil {
				// Lock already held
				return false, nil
			}
			if err := rc.Set(ctx, key, "1", ttl); err != nil {
				return false, err
			}
			return true, nil
		}
	}

	// 2. Safe local memory mutex fallback (for unit tests and single-instance boot)
	lm.mu.Lock()
	defer lm.mu.Unlock()

	now := time.Now()
	if entry, exists := lm.memLocks[key]; exists {
		if now.Before(entry.expiresAt) {
			return false, nil
		}
	}

	lm.memLocks[key] = memLockEntry{
		expiresAt: now.Add(ttl),
		token:     "locked",
	}
	return true, nil
}

// Release frees the lock for the given key.
func (lm *LockManager) Release(ctx context.Context, key string) error {
	if lm.redisClient != nil {
		if rc, ok := lm.redisClient.(RedisClient); ok {
			return rc.Del(ctx, key)
		}
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()
	delete(lm.memLocks, key)
	return nil
}
