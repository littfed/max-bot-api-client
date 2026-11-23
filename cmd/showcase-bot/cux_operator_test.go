package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestOperatorEscalationAndPauseMode(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())
	RegisterMenuRoutes(botInstance, hood)

	userID := int64(777)
	chatID := int64(888)
	key := fsm.SessionKey{UserID: userID, ChatID: chatID}

	makeCtx := func(text string, payload ...string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender: &maxbot.User{
					UserID:    userID,
					FirstName: "Алексей",
					LastName:  "Тестовый",
					Username:  "alex_test",
				},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: text},
			},
		}
		if len(payload) > 0 {
			u.Callback = &maxbot.Callback{
				CallbackID: "cb_operator_test",
				Payload:    payload[0],
				User: &maxbot.User{
					UserID:    userID,
					FirstName: "Алексей",
					LastName:  "Тестовый",
					Username:  "alex_test",
				},
			}
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	// 1. Test /operator command transitions to OPERATOR_PAUSED
	cOperatorCmd := makeCtx("/operator")
	if err := botInstance.Router().Handle(cOperatorCmd); err != nil {
		t.Fatalf("operator command failed: %v", err)
	}

	st, err := storage.GetState(context.Background(), key)
	if err != nil || st != "OPERATOR_PAUSED" {
		t.Fatalf("expected state OPERATOR_PAUSED, got %s (err: %v)", st, err)
	}

	data, err := storage.GetData(context.Background(), key)
	if err != nil {
		t.Fatalf("get data failed: %v", err)
	}
	if _, ok := data["operator_paused_at"]; !ok {
		t.Fatalf("expected operator_paused_at timestamp to be stored in FSM")
	}

	// 2. Incoming messages while paused are silenced (bot does not crash or transition state)
	cUserMsg := makeCtx("Алло, есть кто живой?")
	if err := botInstance.Router().Handle(cUserMsg); err != nil {
		t.Fatalf("paused message handling returned error: %v", err)
	}

	st, _ = storage.GetState(context.Background(), key)
	if st != "OPERATOR_PAUSED" {
		t.Fatalf("expected state to remain OPERATOR_PAUSED, got %s", st)
	}

	// 3. Test /resume restores automated mode
	cResume := makeCtx("/resume")
	if err := botInstance.Router().Handle(cResume); err != nil {
		t.Fatalf("resume command failed: %v", err)
	}

	st, _ = storage.GetState(context.Background(), key)
	if st != fsm.StateEmpty {
		t.Fatalf("expected empty state after resume, got %s", st)
	}

	// 4. Test nav:operator callback
	cOperatorCb := makeCtx("", "nav:operator")
	if err := botInstance.Router().Handle(cOperatorCb); err != nil {
		t.Fatalf("nav:operator callback failed: %v", err)
	}

	st, _ = storage.GetState(context.Background(), key)
	if st != "OPERATOR_PAUSED" {
		t.Fatalf("expected state OPERATOR_PAUSED from callback, got %s", st)
	}

	// 5. Test nav:resume callback
	cResumeCb := makeCtx("", "nav:resume")
	if err := botInstance.Router().Handle(cResumeCb); err != nil {
		t.Fatalf("nav:resume callback failed: %v", err)
	}

	st, _ = storage.GetState(context.Background(), key)
	if st != fsm.StateEmpty {
		t.Fatalf("expected empty state after nav:resume callback, got %s", st)
	}
}

func TestOperatorWatchdogTimeout(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())
	RegisterMenuRoutes(botInstance, hood)

	userID := int64(999)
	chatID := int64(888)
	key := fsm.SessionKey{UserID: userID, ChatID: chatID}

	// Manually set session in OPERATOR_PAUSED state with expired timestamp (> 30 min ago)
	_ = storage.SetState(context.Background(), key, "OPERATOR_PAUSED")
	expiredTime := time.Now().Unix() - 1900 // 31 minutes ago
	_ = storage.UpdateData(context.Background(), key, func(data map[string]any) error {
		data["operator_paused_at"] = float64(expiredTime)
		return nil
	})

	u := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender:    &maxbot.User{UserID: userID, FirstName: "ТаймаутТест"},
			Recipient: &maxbot.MessageRecipient{ChatID: chatID},
			Body:      maxbot.MessageBody{Text: "Привет, я жду уже полчаса!"},
		},
	}
	ctx := bot.NewContext(context.Background(), nil, u)

	// Sending a message after watchdog expiration should trigger auto-resume
	if err := botInstance.Router().Handle(ctx); err != nil {
		t.Fatalf("watchdog message handle failed: %v", err)
	}

	st, _ := storage.GetState(context.Background(), key)
	if st != fsm.StateEmpty {
		t.Fatalf("expected state to auto-reset to empty after watchdog timeout, got %s", st)
	}
}

func TestBuildLeadDossier(t *testing.T) {
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	userID := int64(12345)
	chatID := int64(67890)
	key := fsm.SessionKey{UserID: userID, ChatID: chatID}

	_ = storage.SetState(context.Background(), key, "ECOM_STEP_CHECKOUT")
	_ = storage.UpdateData(context.Background(), key, func(data map[string]any) error {
		data["prod_id"] = "prod_coffee"
		data["order_id"] = "ORD-999"
		data["delivery"] = "ул. Арбат, 10"
		data["service_id"] = "srv_cut"
		data["slot"] = "Завтра, 14:30"
		data["phone"] = "+7 (999) 777-88-99"
		data["quiz_score"] = float64(3)
		return nil
	})

	u := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{
				UserID:    userID,
				FirstName: "Дмитрий",
				LastName:  "Петров",
				Username:  "dmitry_p",
			},
			Recipient: &maxbot.MessageRecipient{ChatID: chatID},
			Body:      maxbot.MessageBody{Text: "/operator"},
		},
	}
	c := bot.NewContext(context.Background(), nil, u)
	fsmCtx := fsm.NewFSMContext(context.Background(), storage, key)
	c.Set("fsm_context", fsmCtx)

	dossier := BuildLeadDossier(c)

	if !strings.Contains(dossier, "Дмитрий Петров") {
		t.Errorf("expected customer name in dossier, got: %s", dossier)
	}
	if !strings.Contains(dossier, "12345") {
		t.Errorf("expected user ID in dossier, got: %s", dossier)
	}
	if !strings.Contains(dossier, "@dmitry_p") {
		t.Errorf("expected username in dossier, got: %s", dossier)
	}
	if !strings.Contains(dossier, "ORD-999") {
		t.Errorf("expected order ID in dossier, got: %s", dossier)
	}
	if !strings.Contains(dossier, "Кофемашина") {
		t.Errorf("expected product name in dossier, got: %s", dossier)
	}
	if !strings.Contains(dossier, "+7 (999) 777-88-99") {
		t.Errorf("expected phone in dossier, got: %s", dossier)
	}
	if !strings.Contains(dossier, "3/3") {
		t.Errorf("expected quiz score in dossier, got: %s", dossier)
	}
}
