package telemetry_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/telemetry"
)

func TestEndToEndTelemetryPipeline(t *testing.T) {
	prom := telemetry.NewPrometheusExporter()
	collector := telemetry.NewCollector(
		telemetry.WithBufferSize(100),
		telemetry.WithSink(prom),
	)
	defer collector.Close()

	router := bot.NewRouter()
	router.Use(telemetry.Middleware(collector))

	// Register test routes
	startInvoked := false
	router.OnStart(func(c *bot.Context) error {
		startInvoked = true
		return nil
	})

	actionInvoked := false
	router.OnAction("select_pro", func(c *bot.Context) error {
		actionInvoked = true
		telemetry.Track(c, "funnel_tariff_selected", map[string]any{"plan": "pro"})
		return nil
	})

	// 1. Dispatch /start command update
	u1 := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{UserID: 42, Name: "Tester"},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 100,
			},
			Body: maxbot.MessageBody{
				Text: "/start",
			},
		},
	}
	c1 := bot.NewContext(context.Background(), nil, u1)
	if err := router.Handle(c1); err != nil {
		t.Fatalf("dispatch /start failed: %v", err)
	}
	if !startInvoked {
		t.Errorf("/start handler was not invoked")
	}

	// 2. Dispatch tracked button click
	sendTime := time.Now().Add(-1500 * time.Millisecond).UnixMilli()
	clickTime := time.Now().UnixMilli()
	u2 := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Timestamp:  clickTime,
		Message: &maxbot.Message{
			Timestamp: sendTime,
			Recipient: &maxbot.MessageRecipient{ChatID: 100},
		},
		Callback: &maxbot.Callback{
			CallbackID: "cb_pro_1",
			Payload:    telemetry.FormatTrackedPayload("select_pro", "onboarding", "trace_1"),
			Timestamp:  clickTime,
			User:       &maxbot.User{UserID: 42, Name: "Tester"},
		},
	}
	c2 := bot.NewContext(context.Background(), nil, u2)
	if err := router.Handle(c2); err != nil {
		t.Fatalf("dispatch button click failed: %v", err)
	}
	if !actionInvoked {
		t.Errorf("OnAction(select_pro) handler was not invoked")
	}

	// 3. Verify in-memory Ring Buffer
	events := collector.Events()
	if len(events) != 3 {
		t.Fatalf("expected 3 events (1 command + 1 click + 1 custom), got %d", len(events))
	}

	if events[0].Type != telemetry.EventCommand || events[0].Action != "/start" {
		t.Errorf("expected first event to be command /start, got %+v", events[0])
	}
	if events[1].Type != telemetry.EventButtonClick || events[1].Action != "select_pro" || events[1].Screen != "onboarding" {
		t.Errorf("expected second event to be button click select_pro, got %+v", events[1])
	}
	if events[2].Type != telemetry.EventCustom || events[2].Action != "funnel_tariff_selected" {
		t.Errorf("expected third event to be custom funnel_tariff_selected, got %+v", events[2])
	}

	// 4. Verify Prometheus text format
	metrics := prom.FormatMetrics()
	if !strings.Contains(metrics, `maxbot_commands_total{command="/start"} 1`) {
		t.Errorf("missing command counter in prometheus output: %s", metrics)
	}
	if !strings.Contains(metrics, `maxbot_button_clicks_total{action="select_pro",screen="onboarding"} 1`) {
		t.Errorf("missing button click counter in prometheus output: %s", metrics)
	}
	if !strings.Contains(metrics, `maxbot_active_users 1`) {
		t.Errorf("expected 1 active user in prometheus output: %s", metrics)
	}
}
