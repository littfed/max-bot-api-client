package telemetry

import (
	"fmt"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// ContextKeyCollector is the context key for storing the collector in bot.Context.
const ContextKeyCollector = "telemetry_collector"

// Middleware creates a bot.MiddlewareFunc that automatically captures telemetry for commands,
// button clicks, messages, latencies, and execution errors.
func Middleware(collector *Collector) bot.MiddlewareFunc {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(c *bot.Context) (err error) {
			if collector == nil {
				return next(c)
			}

			// Store collector in request context for Track(...) and helper access
			c.Set(ContextKeyCollector, collector)

			start := time.Now()
			u := c.Update

			var userID int64
			var chatID int64
			if sender := c.Sender(); sender != nil {
				userID = sender.UserID
			}
			if rec, recErr := c.Recipient(); recErr == nil && rec != nil {
				chatID = rec.ID()
			}

			// 1. Intercept button clicks (message_callback)
			if u != nil && u.UpdateType == maxbot.UpdateTypeMessageCallback {
				if metrics, ok := ExtractClickMetrics(u); ok {
					clickEv := NewButtonClickEvent(
						metrics.UserID,
						metrics.ChatID,
						metrics.Action,
						metrics.Screen,
						metrics.TimeDelta,
					)
					if metrics.TraceID != "" {
						clickEv.Properties["trace_id"] = metrics.TraceID
					}
					clickEv.Properties["is_tracked"] = metrics.IsTracked
					collector.Record(c.Ctx(), clickEv)
				}
			}

			// 2. Panic recovery with error telemetry recording
			defer func() {
				duration := time.Since(start)

				if r := recover(); r != nil {
					panicErr := fmt.Errorf("panic in handler: %v", r)
					collector.Record(c.Ctx(), NewErrorEvent(userID, chatID, "panic", panicErr))
					panic(r) // Re-panic so bot.Recovery() can catch it if registered
				}

				if err != nil {
					collector.Record(c.Ctx(), NewErrorEvent(userID, chatID, "handler_error", err))
				}

				// If command was executed, log command duration
				if cmd, _ := c.Command(); cmd != "" {
					cmdEv := NewCommandEvent(userID, chatID, "/"+cmd, duration, err)
					collector.Record(c.Ctx(), cmdEv)
				}
			}()

			return next(c)
		}
	}
}

// Track records a custom business analytics event via the collector stored in bot.Context.
func Track(c *bot.Context, name string, properties ...map[string]any) {
	if c == nil {
		return
	}
	colVal, ok := c.Get(ContextKeyCollector)
	if !ok || colVal == nil {
		return
	}
	collector, ok := colVal.(*Collector)
	if !ok || collector == nil {
		return
	}

	var userID int64
	var chatID int64
	if sender := c.Sender(); sender != nil {
		userID = sender.UserID
	}
	if rec, err := c.Recipient(); err == nil && rec != nil {
		chatID = rec.ID()
	}

	var props map[string]any
	if len(properties) > 0 && properties[0] != nil {
		props = properties[0]
	}

	collector.Record(c.Ctx(), NewCustomEvent(userID, chatID, name, props))
}

// NewContextTrackedKeyboard returns a TrackedKeyboard initialized with the sender's user ID and screen.
func NewContextTrackedKeyboard(c *bot.Context, screen string) *TrackedKeyboard {
	var userID int64
	if c != nil {
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
		}
	}
	return NewTrackedKeyboard(screen, userID)
}
