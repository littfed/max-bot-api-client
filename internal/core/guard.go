package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/store"
)

// ModeConfigReader abstracts reading and updating mode configs.
type ModeConfigReader interface {
	GetModeConfigs(ctx context.Context) ([]store.ModeConfig, error)
	SetModeActive(ctx context.Context, modeID string, isActive bool, maintMsg string) error
}

type guardCacheEntry struct {
	Active    bool      `json:"active"`
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expires_at"`
}

// MaintenanceGuard caches mode statuses with under 5s invalidation.
type MaintenanceGuard struct {
	store       ModeConfigReader
	redisClient any
	mu          sync.RWMutex
	memCache    map[string]guardCacheEntry
	cacheTTL    time.Duration
}

// NewMaintenanceGuard creates a MaintenanceGuard.
func NewMaintenanceGuard(st ModeConfigReader, redisClient any) *MaintenanceGuard {
	return &MaintenanceGuard{
		store:       st,
		redisClient: redisClient,
		memCache:    make(map[string]guardCacheEntry),
		cacheTTL:    4 * time.Second, // under 5s invalidation window
	}
}

func (g *MaintenanceGuard) redisKey(modeID string) string {
	return fmt.Sprintf("bothub:mode:%s:active", modeID)
}

// IsActive checks if the given mode is currently active.
// Returns (isActive, maintenanceMessage, error).
func (g *MaintenanceGuard) IsActive(ctx context.Context, modeID string) (bool, string, error) {
	rKey := g.redisKey(modeID)

	// 1. Try Redis cache
	if g.redisClient != nil {
		if rc, ok := g.redisClient.(RedisClient); ok {
			val, err := rc.Get(ctx, rKey)
			if err == nil && val != "" {
				var entry guardCacheEntry
				if err := json.Unmarshal([]byte(val), &entry); err == nil {
					return entry.Active, entry.Message, nil
				}
			}
		}
	}

	// 2. Try in-memory cache
	g.mu.RLock()
	entry, exists := g.memCache[modeID]
	g.mu.RUnlock()
	if exists && time.Now().Before(entry.ExpiresAt) {
		return entry.Active, entry.Message, nil
	}

	// 3. Query DB
	if g.store != nil {
		configs, err := g.store.GetModeConfigs(ctx)
		if err != nil {
			return false, "", fmt.Errorf("guard: failed to query mode configs: %w", err)
		}

		for _, cfg := range configs {
			if cfg.ModeID == modeID {
				newEntry := guardCacheEntry{
					Active:    cfg.IsActive,
					Message:   cfg.MaintenanceMessage,
					ExpiresAt: time.Now().Add(g.cacheTTL),
				}

				// Update memory cache
				g.mu.Lock()
				g.memCache[modeID] = newEntry
				g.mu.Unlock()

				// Update Redis cache
				if g.redisClient != nil {
					if rc, ok := g.redisClient.(RedisClient); ok {
						if raw, err := json.Marshal(newEntry); err == nil {
							_ = rc.Set(ctx, rKey, string(raw), g.cacheTTL)
						}
					}
				}

				return cfg.IsActive, cfg.MaintenanceMessage, nil
			}
		}
		// If mode was not found in mode_configs
		return false, "Режим не найден", nil
	}

	// Safe fallback if store is nil
	return true, "", nil
}

// Invalidate purges cache for the given modeID across Redis and memory.
func (g *MaintenanceGuard) Invalidate(ctx context.Context, modeID string) error {
	rKey := g.redisKey(modeID)
	if g.redisClient != nil {
		if rc, ok := g.redisClient.(RedisClient); ok {
			_ = rc.Del(ctx, rKey)
		}
	}

	g.mu.Lock()
	delete(g.memCache, modeID)
	g.mu.Unlock()
	return nil
}

// SetModeActive toggles a mode's active status and invalidates cache immediately.
func (g *MaintenanceGuard) SetModeActive(ctx context.Context, modeID string, isActive bool, maintMsg string) error {
	if g.store != nil {
		if err := g.store.SetModeActive(ctx, modeID, isActive, maintMsg); err != nil {
			return err
		}
	}
	return g.Invalidate(ctx, modeID)
}
