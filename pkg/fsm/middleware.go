package fsm

import (
	"github.com/littfed/max-bot-api-client/pkg/bot"
)

// ContextKeyFSM is the key used to store the FSMContext in bot.Context.
const ContextKeyFSM = "fsm_context"

// Middleware returns a bot.MiddlewareFunc that injects an FSMContext into every incoming request.
func Middleware(storage Storage) bot.MiddlewareFunc {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(c *bot.Context) error {
			if storage == nil {
				return next(c)
			}

			var userID int64
			var chatID int64
			if sender := c.Sender(); sender != nil {
				userID = sender.UserID
			}
			if rec, err := c.Recipient(); err == nil && rec != nil {
				chatID = rec.ID()
			}

			key := SessionKey{
				UserID: userID,
				ChatID: chatID,
			}

			fsmCtx := NewFSMContext(c.Ctx(), storage, key)
			c.Set(ContextKeyFSM, fsmCtx)

			return next(c)
		}
	}
}

// FromContext extracts the active FSMContext from a bot.Context, or returns nil if FSM middleware is not active.
func FromContext(c *bot.Context) *FSMContext {
	if c == nil {
		return nil
	}
	val, ok := c.Get(ContextKeyFSM)
	if !ok || val == nil {
		return nil
	}
	fsmCtx, ok := val.(*FSMContext)
	if !ok {
		return nil
	}
	return fsmCtx
}
