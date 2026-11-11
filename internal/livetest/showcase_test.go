package livetest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/payments"
	"github.com/littfed/max-bot-api-client/pkg/telemetry"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
)

func TestShowcaseBotUnifiedEndToEndJourney(t *testing.T) {
	storage := fsm.NewMemoryStorage(15 * time.Minute)
	defer storage.Close()

	collector := telemetry.NewCollector(telemetry.WithBufferSize(200))
	promExporter := telemetry.NewPrometheusExporter()
	collector.AddSink(promExporter)

	b := bot.NewWithClient(nil)
	b.Use(fsm.Middleware(storage))
	b.Use(telemetry.Middleware(collector))

	// 1. Core navigation & start
	b.OnStart(func(c *bot.Context) error {
		return c.AnswerNotification("Меню открыто")
	})

	var devModeActive bool
	b.OnCommand("dev", func(c *bot.Context) error {
		devModeActive = !devModeActive
		return nil
	})

	userID := int64(9999)
	chatID := int64(8888)

	makeCtx := func(text string, payload ...string) *bot.Context {
		u := &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender:    &maxbot.User{UserID: userID, FirstName: "Алексей"},
				Recipient: &maxbot.MessageRecipient{ChatID: chatID},
				Body:      maxbot.MessageBody{Text: text},
			},
		}
		if len(payload) > 0 {
			u.Callback = &maxbot.Callback{
				CallbackID: "cb_tour",
				Payload:    payload[0],
				User:       &maxbot.User{UserID: userID},
			}
		}
		return bot.NewContext(context.Background(), nil, u)
	}

	// 1. Send /start
	if err := b.Router().Handle(makeCtx("/start")); err != nil {
		t.Fatalf("/start failed: %v", err)
	}

	// 2. Toggle /dev mode
	if err := b.Router().Handle(makeCtx("/dev")); err != nil {
		t.Fatalf("/dev toggle failed: %v", err)
	}
	if !devModeActive {
		t.Fatal("expected developer mode to be active")
	}

	// 3. Test SBP payment button generation
	sbpLink := payments.GenerateSBPLink("ORD-TOUR-1", payments.SBPOptions{
		Type:   payments.SBPTypeDynamic,
		Amount: 2490.00,
	})
	if !strings.Contains(sbpLink, "qr.nspk.ru") || !strings.Contains(sbpLink, "sum=249000") {
		t.Fatalf("unexpected SBP link: %s", sbpLink)
	}

	// 4. Test Mini App OpenApp button & return link
	appBtn, err := webapp.OpenAppButton("Открыть магазин", "showcase_bot")
	if err != nil || appBtn.Type != "open_app" {
		t.Fatalf("OpenAppButton error: %v", err)
	}

	returnURL := webapp.GenerateReturnLink("showcase_bot", "token_123")
	if !strings.Contains(returnURL, "max.ru/showcase_bot?start=token_123") {
		t.Fatalf("unexpected return URL: %s", returnURL)
	}

	// 5. Verify telemetry events collected
	events := collector.Events()
	if len(events) == 0 {
		t.Fatal("expected telemetry events to be recorded")
	}

	// 6. Test FSM session key isolation and storage
	key := fsm.SessionKey{UserID: userID, ChatID: chatID}
	_ = storage.SetState(context.Background(), key, "TOUR_COMPLETED")
	st, err := storage.GetState(context.Background(), key)
	if err != nil || st != "TOUR_COMPLETED" {
		t.Fatalf("expected state TOUR_COMPLETED, got %s", st)
	}

	_ = storage.Reset(context.Background(), key)
	st, _ = storage.GetState(context.Background(), key)
	if st != fsm.StateEmpty {
		t.Fatalf("expected empty state after reset, got %s", st)
	}
}
