package fsm

import (
	"context"
)

// FSMContext provides high-level dialog state and data manipulation bound to a specific user and chat.
type FSMContext struct {
	storage Storage
	key     SessionKey
	ctx     context.Context
}

// NewFSMContext constructs an FSMContext instance.
func NewFSMContext(ctx context.Context, storage Storage, key SessionKey) *FSMContext {
	if ctx == nil {
		ctx = context.Background()
	}
	return &FSMContext{
		storage: storage,
		key:     key,
		ctx:     ctx,
	}
}

// Key returns the session identifier.
func (f *FSMContext) Key() SessionKey {
	return f.key
}

// SetState transitions the session to a new state.
func (f *FSMContext) SetState(state State) error {
	return f.storage.SetState(f.ctx, f.key, state)
}

// GetState returns the current state as a string, fulfilling the router's StateGetter interface.
func (f *FSMContext) GetState() (string, error) {
	st, err := f.storage.GetState(f.ctx, f.key)
	return string(st), err
}

// Set stores a single key-value pair in the session's data payload.
func (f *FSMContext) Set(key string, val any) error {
	return f.storage.UpdateData(f.ctx, f.key, func(data map[string]any) error {
		data[key] = val
		return nil
	})
}

// Get retrieves a single value from the session's data payload.
func (f *FSMContext) Get(key string) (any, bool) {
	data, err := f.storage.GetData(f.ctx, f.key)
	if err != nil || data == nil {
		return nil, false
	}
	val, ok := data[key]
	return val, ok
}

// SetData overwrites the entire session data payload.
func (f *FSMContext) SetData(data map[string]any) error {
	return f.storage.SetData(f.ctx, f.key, data)
}

// GetData returns a copy of all stored session data.
func (f *FSMContext) GetData() (map[string]any, error) {
	return f.storage.GetData(f.ctx, f.key)
}

// Reset clears the state and all stored session data.
func (f *FSMContext) Reset() error {
	return f.storage.Reset(f.ctx, f.key)
}
