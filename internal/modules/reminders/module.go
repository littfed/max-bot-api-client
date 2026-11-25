package reminders

import (
	"context"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// Module implements the BotModule interface for the Reminders mode.
type Module struct {
	service  *Service
	sqlStore *store.SQLStore
}

// New creates a new Reminders module instance.
func New(service *Service, sqlStore *store.SQLStore) *Module {
	return &Module{
		service:  service,
		sqlStore: sqlStore,
	}
}

// ID returns the unique identifier of the mode.
func (m *Module) ID() string {
	return "reminders"
}

// Title returns the human-readable display name.
func (m *Module) Title() string {
	return "Напоминания"
}

// Service returns the underlying reminders Service.
func (m *Module) Service() *Service {
	return m.service
}

// HandleMessage handles text messages sent while in Reminders mode.
func (m *Module) HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	return m.handleMessageFSM(ctx, u, state, msg)
}

// HandleCallback handles inline button callbacks in Reminders mode.
func (m *Module) HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	return m.handleCallbackFSM(ctx, u, state, cb)
}
