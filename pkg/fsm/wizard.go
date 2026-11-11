package fsm

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
)

var (
	// ErrWizardComplete is returned when Next() is called on the final step.
	ErrWizardComplete = errors.New("fsm: wizard is already at the final step")
	// ErrWizardNotActive is returned when navigation is attempted on a session not in the wizard.
	ErrWizardNotActive = errors.New("fsm: session is not in this wizard")
	// ErrWizardExpired is returned when the session exceeds the configured inactivity timeout.
	ErrWizardExpired = errors.New("fsm: wizard session has expired")
)

const (
	wizardKeyStepIndex    = "_wizard_step_index"
	wizardKeyLastActivity = "_wizard_last_activity"
)

// WizardStep represents a single step in a multi-step conversation.
type WizardStep struct {
	State   State
	Handler bot.HandlerFunc
}

// Wizard manages sequential multi-step forms, dialogs, and checkout flows.
type Wizard struct {
	name       string
	steps      []WizardStep
	stepMap    map[State]int
	timeout    time.Duration
	onComplete bot.HandlerFunc
	onCancel   bot.HandlerFunc
	onTimeout  bot.HandlerFunc
	mu         sync.RWMutex
}

// WizardOption configures a Wizard instance.
type WizardOption func(*Wizard)

// WithTimeout sets the inactivity timeout for the wizard.
func WithTimeout(d time.Duration) WizardOption {
	return func(w *Wizard) {
		w.timeout = d
	}
}

// WithOnComplete sets a callback handler executed when the wizard finishes its last step.
func WithOnComplete(h bot.HandlerFunc) WizardOption {
	return func(w *Wizard) {
		w.onComplete = h
	}
}

// WithOnCancel sets a callback handler executed when the wizard is cancelled.
func WithOnCancel(h bot.HandlerFunc) WizardOption {
	return func(w *Wizard) {
		w.onCancel = h
	}
}

// WithOnTimeout sets a callback handler executed when the wizard expires due to inactivity.
func WithOnTimeout(h bot.HandlerFunc) WizardOption {
	return func(w *Wizard) {
		w.onTimeout = h
	}
}

// NewWizard creates a new multi-step Wizard manager.
func NewWizard(name string, opts ...WizardOption) *Wizard {
	w := &Wizard{
		name:    name,
		steps:   make([]WizardStep, 0),
		stepMap: make(map[State]int),
		timeout: 30 * time.Minute,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Step appends a state and its update handler to the wizard sequence.
func (w *Wizard) Step(state State, handler bot.HandlerFunc) *Wizard {
	w.mu.Lock()
	defer w.mu.Unlock()

	idx := len(w.steps)
	w.steps = append(w.steps, WizardStep{
		State:   state,
		Handler: handler,
	})
	w.stepMap[state] = idx
	return w
}

// Start begins the wizard by transitioning the user into the first step.
func (w *Wizard) Start(c *bot.Context) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if len(w.steps) == 0 {
		return fmt.Errorf("fsm: wizard %q has no registered steps", w.name)
	}

	fsmCtx := FromContext(c)
	if fsmCtx == nil {
		return fmt.Errorf("fsm: no FSM context found in bot.Context")
	}

	firstStep := w.steps[0]
	if err := fsmCtx.SetState(firstStep.State); err != nil {
		return err
	}

	_ = fsmCtx.Set(wizardKeyStepIndex, float64(0))
	_ = fsmCtx.Set(wizardKeyLastActivity, float64(time.Now().Unix()))

	return nil
}

// CurrentStep returns the 0-based index and State of the active step for the session.
func (w *Wizard) CurrentStep(c *bot.Context) (int, State, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	fsmCtx := FromContext(c)
	if fsmCtx == nil {
		return -1, StateEmpty, false
	}

	stStr, err := fsmCtx.GetState()
	st := State(stStr)
	if err != nil || st == StateEmpty {
		return -1, StateEmpty, false
	}

	idx, ok := w.stepMap[st]
	if !ok {
		return -1, StateEmpty, false
	}
	return idx, st, true
}

// IsActive returns true if the user's active FSM state belongs to this wizard.
func (w *Wizard) IsActive(c *bot.Context) bool {
	_, _, active := w.CurrentStep(c)
	return active
}

// Next advances the wizard to the next step.
func (w *Wizard) Next(c *bot.Context) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	fsmCtx := FromContext(c)
	if fsmCtx == nil {
		return ErrWizardNotActive
	}

	idx, _, active := w.CurrentStep(c)
	if !active {
		return ErrWizardNotActive
	}

	if idx+1 >= len(w.steps) {
		if w.onComplete != nil {
			return w.onComplete(c)
		}
		return ErrWizardComplete
	}

	nextStep := w.steps[idx+1]
	if err := fsmCtx.SetState(nextStep.State); err != nil {
		return err
	}

	_ = fsmCtx.Set(wizardKeyStepIndex, float64(idx+1))
	_ = fsmCtx.Set(wizardKeyLastActivity, float64(time.Now().Unix()))

	return nil
}

// Back returns the wizard to the previous step.
func (w *Wizard) Back(c *bot.Context) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	fsmCtx := FromContext(c)
	if fsmCtx == nil {
		return ErrWizardNotActive
	}

	idx, _, active := w.CurrentStep(c)
	if !active {
		return ErrWizardNotActive
	}

	if idx <= 0 {
		// Already at the first step
		return nil
	}

	prevStep := w.steps[idx-1]
	if err := fsmCtx.SetState(prevStep.State); err != nil {
		return err
	}

	_ = fsmCtx.Set(wizardKeyStepIndex, float64(idx-1))
	_ = fsmCtx.Set(wizardKeyLastActivity, float64(time.Now().Unix()))

	return nil
}

// Cancel terminates the wizard flow and resets the FSM session.
func (w *Wizard) Cancel(c *bot.Context) error {
	fsmCtx := FromContext(c)
	if fsmCtx != nil {
		_ = fsmCtx.Reset()
	}
	if w.onCancel != nil {
		return w.onCancel(c)
	}
	return nil
}

// CheckExpiry checks if the session has expired due to inactivity.
// If expired, it triggers onTimeout (if configured) and resets the session.
func (w *Wizard) CheckExpiry(c *bot.Context) (bool, error) {
	if w.timeout <= 0 {
		return false, nil
	}

	fsmCtx := FromContext(c)
	if fsmCtx == nil {
		return false, nil
	}

	data, err := fsmCtx.GetData()
	if err != nil {
		return false, err
	}

	lastActivityVal, ok := data[wizardKeyLastActivity]
	if !ok {
		return false, nil
	}

	var lastActivityTime time.Time
	switch v := lastActivityVal.(type) {
	case float64:
		lastActivityTime = time.Unix(int64(v), 0)
	case int64:
		lastActivityTime = time.Unix(v, 0)
	default:
		return false, nil
	}

	if time.Since(lastActivityTime) > w.timeout {
		_ = fsmCtx.Reset()
		if w.onTimeout != nil {
			_ = w.onTimeout(c)
		}
		return true, ErrWizardExpired
	}

	// Update last activity timestamp
	_ = fsmCtx.Set(wizardKeyLastActivity, float64(time.Now().Unix()))
	return false, nil
}

// Register registers all wizard step handlers, cancel/back commands, and callback handlers onto router.
func (w *Wizard) Register(router *bot.Router) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	router.OnCallbackPayload("wizard:cancel", func(c *bot.Context) error {
		return w.Cancel(c)
	})
	router.OnCallbackPayload("wizard:back", func(c *bot.Context) error {
		return w.Back(c)
	})

	for _, step := range w.steps {
		s := step
		router.OnState(string(s.State), func(c *bot.Context) error {
			text := strings.TrimSpace(strings.ToLower(c.Text()))
			if text == "/cancel" || text == "отмена" {
				return w.Cancel(c)
			}
			if text == "/back" || text == "назад" {
				return w.Back(c)
			}

			// Check session expiration
			expired, err := w.CheckExpiry(c)
			if expired {
				return err
			}

			if s.Handler != nil {
				return s.Handler(c)
			}
			return nil
		})
	}
}
