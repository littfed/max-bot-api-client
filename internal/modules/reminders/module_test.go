package reminders

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

func TestModule_FSMFlow(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "module_fsm_test.db")

	s, err := store.NewFromEnv("file:"+dbPath, "")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	sqlStore := s.SQL()
	ctx := context.Background()

	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "user_mod_fsm_1", "fsm_user", "FSMTester", "+79996665544")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}

	service := NewService(sqlStore)
	mod := New(service, sqlStore)

	if mod.ID() != "reminders" || mod.Title() != "Напоминания" {
		t.Fatalf("unexpected mod ID/Title: %s / %s", mod.ID(), mod.Title())
	}

	// 1. Initial /menu
	res0, err := mod.HandleMessage(ctx, user, nil, &core.IncomingMessage{Text: "/start"})
	if err != nil {
		t.Fatalf("initial message failed: %v", err)
	}
	if !strings.Contains(res0.Text, "Режим «Напоминания»") {
		t.Errorf("expected welcome text, got: %s", res0.Text)
	}

	// 2. Click [➕ Новое напоминание]
	res1, err := mod.HandleCallback(ctx, user, nil, &core.IncomingCallback{Data: "rmd:create"})
	if err != nil {
		t.Fatalf("callback rmd:create failed: %v", err)
	}
	if res1.NewStep != "wait_title" {
		t.Errorf("expected step 'wait_title', got %q", res1.NewStep)
	}

	// 3. User sends title text
	state1 := &core.State{CurrentStep: "wait_title"}
	res2, err := mod.HandleMessage(ctx, user, state1, &core.IncomingMessage{Text: "Позвонить партнёру"})
	if err != nil {
		t.Fatalf("message title failed: %v", err)
	}
	if res2.NewStep != "wait_time" {
		t.Errorf("expected step 'wait_time', got %q", res2.NewStep)
	}

	// 4. User clicks preset [+15 мин]
	state2 := &core.State{CurrentStep: "wait_time", Payload: res2.NewPayload}
	res3, err := mod.HandleCallback(ctx, user, state2, &core.IncomingCallback{Data: "rmd:time:+15m"})
	if err != nil {
		t.Fatalf("callback time failed: %v", err)
	}
	if res3.NewStep != "wait_recurrence" {
		t.Errorf("expected step 'wait_recurrence', got %q", res3.NewStep)
	}

	// 5. User chooses recurrence [🔁 Каждый день]
	state3 := &core.State{CurrentStep: "wait_recurrence", Payload: res3.NewPayload}
	res4, err := mod.HandleCallback(ctx, user, state3, &core.IncomingCallback{Data: "rmd:rec:daily"})
	if err != nil {
		t.Fatalf("callback rec failed: %v", err)
	}
	if !res4.ResetState {
		t.Errorf("expected ResetState to be true")
	}
	if !strings.Contains(res4.Text, "Позвонить партнёру") || !strings.Contains(res4.Text, "Каждый день") {
		t.Errorf("expected confirmation text, got: %s", res4.Text)
	}

	// 6. View list of reminders
	resList, err := mod.HandleCallback(ctx, user, nil, &core.IncomingCallback{Data: "rmd:list"})
	if err != nil {
		t.Fatalf("callback rmd:list failed: %v", err)
	}
	if !strings.Contains(resList.Text, "Позвонить партнёру") {
		t.Errorf("expected list to contain reminder, got: %s", resList.Text)
	}

	// 7. Test Demo trigger callback
	resDemo, err := mod.HandleCallback(ctx, user, nil, &core.IncomingCallback{Data: "rmd:demo_trigger"})
	if err != nil {
		t.Fatalf("callback rmd:demo_trigger failed: %v", err)
	}
	if !strings.Contains(resDemo.Text, "Демо-триггер запущен") {
		t.Errorf("expected demo trigger text, got: %s", resDemo.Text)
	}
}
