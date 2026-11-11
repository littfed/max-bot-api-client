package fsm

import (
	"context"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestWizardLifecycleAndNavigation(t *testing.T) {
	storage := NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	wizard := NewWizard("registration", WithTimeout(5*time.Second))

	var completed bool
	var cancelled bool

	wizard.Step("STEP_NAME", func(c *bot.Context) error {
		_ = FromContext(c).Set("name", c.Text())
		return wizard.Next(c)
	})
	wizard.Step("STEP_AGE", func(c *bot.Context) error {
		_ = FromContext(c).Set("age", c.Text())
		return wizard.Next(c)
	})
	wizard.Step("STEP_CONFIRM", func(c *bot.Context) error {
		completed = true
		return FromContext(c).Reset()
	})

	wizard.onCancel = func(c *bot.Context) error {
		cancelled = true
		return nil
	}

	router := bot.NewRouter()
	router.Use(Middleware(storage))
	wizard.Register(router)

	makeCtx := func(text string) *bot.Context {
		return bot.NewContext(context.Background(), nil, &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: 101},
				Recipient: &maxbot.MessageRecipient{ChatID: 202},
				Body:      maxbot.MessageBody{Text: text},
			},
		})
	}

	// 1. Start wizard
	cStart := makeCtx("")
	// Execute middleware on cStart manually to inject FSM
	mw := Middleware(storage)
	_ = mw(func(c *bot.Context) error {
		return wizard.Start(c)
	})(cStart)

	idx, st, active := wizard.CurrentStep(cStart)
	if !active || idx != 0 || st != "STEP_NAME" {
		t.Fatalf("expected step 0 STEP_NAME, got idx=%d st=%v active=%v", idx, st, active)
	}

	// 2. Submit name
	c1 := makeCtx("Alice")
	if err := router.Handle(c1); err != nil {
		t.Fatalf("Handle c1 failed: %v", err)
	}

	// Check reached STEP_AGE
	idx, st, _ = wizard.CurrentStep(c1)
	if idx != 1 || st != "STEP_AGE" {
		t.Fatalf("expected step 1 STEP_AGE, got idx=%d st=%v", idx, st)
	}

	// 3. User types "назад" (Back)
	cBack := makeCtx("назад")
	if err := router.Handle(cBack); err != nil {
		t.Fatalf("Handle cBack failed: %v", err)
	}

	// Check returned to STEP_NAME
	idx, st, _ = wizard.CurrentStep(cBack)
	if idx != 0 || st != "STEP_NAME" {
		t.Fatalf("expected back to step 0 STEP_NAME, got idx=%d st=%v", idx, st)
	}

	// 4. Re-advance to STEP_AGE
	c1Re := makeCtx("Alice")
	_ = router.Handle(c1Re)

	// 5. Submit age -> reaches STEP_CONFIRM
	c2 := makeCtx("30")
	if err := router.Handle(c2); err != nil {
		t.Fatalf("Handle c2 failed: %v", err)
	}

	idx, st, _ = wizard.CurrentStep(c2)
	if idx != 2 || st != "STEP_CONFIRM" {
		t.Fatalf("expected step 2 STEP_CONFIRM, got idx=%d st=%v", idx, st)
	}

	// Confirm completed action
	c3 := makeCtx("yes")
	if err := router.Handle(c3); err != nil {
		t.Fatalf("Handle c3 failed: %v", err)
	}
	if !completed {
		t.Fatalf("expected wizard to be completed")
	}

	// Restart and test cancel
	_ = mw(func(c *bot.Context) error {
		return wizard.Start(c)
	})(cStart)

	// 6. User types /cancel
	cCancel := makeCtx("/cancel")
	if err := router.Handle(cCancel); err != nil {
		t.Fatalf("Handle cCancel failed: %v", err)
	}

	if !cancelled {
		t.Fatalf("expected onCancel to be invoked")
	}

	if wizard.IsActive(cCancel) {
		t.Fatalf("expected wizard to be inactive after cancel")
	}
}

func TestWizardTimeoutExpiry(t *testing.T) {
	storage := NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	var timedOut bool
	wizard := NewWizard("expiring_wizard",
		WithTimeout(20*time.Millisecond),
		WithOnTimeout(func(c *bot.Context) error {
			timedOut = true
			return nil
		}),
	)
	wizard.Step("STEP_ONE", func(c *bot.Context) error {
		return nil
	})

	router := bot.NewRouter()
	router.Use(Middleware(storage))
	wizard.Register(router)

	makeCtx := func() *bot.Context {
		return bot.NewContext(context.Background(), nil, &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: 555},
				Recipient: &maxbot.MessageRecipient{ChatID: 666},
				Body:      maxbot.MessageBody{Text: "anything"},
			},
		})
	}

	// Start wizard
	c := makeCtx()
	mw := Middleware(storage)
	_ = mw(func(ctx *bot.Context) error {
		return wizard.Start(ctx)
	})(c)

	if !wizard.IsActive(c) {
		t.Fatalf("expected wizard to be active")
	}

	// Sleep past timeout
	time.Sleep(30 * time.Millisecond)

	// Dispatch update
	_ = router.Handle(makeCtx())

	if !timedOut {
		t.Fatalf("expected timeout handler to be triggered after inactivity")
	}
}
