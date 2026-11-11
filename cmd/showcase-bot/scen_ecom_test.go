package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/payments"
)

func TestEcomScenarioWorkflow(t *testing.T) {
	hood := NewHoodEngine()
	storage := fsm.NewMemoryStorage(10 * time.Minute)
	defer storage.Close()

	router := bot.NewRouter()
	router.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(router)

	// Test helper to create bot.Bot with router
	botInstance := bot.NewWithClient(nil)
	botInstance.Use(fsm.Middleware(storage))
	hood.RegisterRoutes(botInstance.Router())
	RegisterEcomScenario(botInstance, hood, payments.NewRouter())

	userID := int64(555)
	chatID := int64(666)

	makeCtx := func(text string, payload ...string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: text},
			},
		}
		if len(payload) > 0 {
			u.Callback = &maxbot.Callback{
				CallbackID: "cb_test",
				Payload:    payload[0],
				User:       &maxbot.User{UserID: userID},
			}
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	// 1. Open catalog
	cCatalog := makeCtx("", "scen:ecom")
	if err := botInstance.Router().Handle(cCatalog); err != nil {
		t.Fatalf("catalog handle failed: %v", err)
	}

	// 2. Choose product (Coffee machine)
	cChoose := makeCtx("", "ecom:choose:prod_coffee")
	if err := botInstance.Router().Handle(cChoose); err != nil {
		t.Fatalf("choose product failed: %v", err)
	}

	// Verify state transitioned to ECOM_STEP_DELIVERY
	key := fsm.SessionKey{UserID: userID, ChatID: chatID}
	st, err := storage.GetState(context.Background(), key)
	if err != nil || st != "ECOM_STEP_DELIVERY" {
		t.Fatalf("expected state ECOM_STEP_DELIVERY, got %s (err: %v)", st, err)
	}

	// 3. User submits delivery address
	cAddress := makeCtx("г. Санкт-Петербург, Невский проспект, 28")
	if err := botInstance.Router().Handle(cAddress); err != nil {
		t.Fatalf("address step failed: %v", err)
	}

	// Verify state transitioned to ECOM_STEP_CHECKOUT
	st, _ = storage.GetState(context.Background(), key)
	if st != "ECOM_STEP_CHECKOUT" {
		t.Fatalf("expected state ECOM_STEP_CHECKOUT, got %s", st)
	}

	// 4. Test Sandbox payment simulator callback
	cPay := makeCtx("", "ecom:sandbox:ORD-777:4990:666:555")
	if err := botInstance.Router().Handle(cPay); err != nil {
		t.Fatalf("sandbox pay callback failed: %v", err)
	}

	// Verify session reset after payment
	st, _ = storage.GetState(context.Background(), key)
	if st != fsm.StateEmpty {
		t.Fatalf("expected empty state after payment completion, got %s", st)
	}
}

func TestFindProduct(t *testing.T) {
	p := findProduct("prod_headphones")
	if p == nil || !strings.Contains(p.Name, "наушники") {
		t.Fatalf("expected headphones product, got %+v", p)
	}

	pNil := findProduct("non_existent")
	if pNil != nil {
		t.Fatalf("expected nil for non-existent product")
	}
}
