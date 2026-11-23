package modules

import (
	"context"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// BotModule defines the common interface that all virtual bot modules implement.
type BotModule interface {
	ID() string
	Title() string
	HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error)
	HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error)
}
