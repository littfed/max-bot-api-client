package telemetry

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestPayloadEncodingDecoding(t *testing.T) {
	payload := FormatTrackedPayload("buy_subscription", "pricing_screen", "trace_123")
	if !strings.HasPrefix(payload, "trk:") {
		t.Fatalf("expected payload prefix trk:, got %s", payload)
	}

	meta := ParseTrackedPayload(payload)
	if !meta.IsTracked {
		t.Errorf("expected IsTracked=true")
	}
	if meta.Action != "buy_subscription" {
		t.Errorf("expected action buy_subscription, got %s", meta.Action)
	}
	if meta.Screen != "pricing_screen" {
		t.Errorf("expected screen pricing_screen, got %s", meta.Screen)
	}
	if meta.TraceID != "trace_123" {
		t.Errorf("expected trace_123, got %s", meta.TraceID)
	}

	// Legacy untracked payload fallback
	legacy := ParseTrackedPayload("menu_catalog")
	if legacy.IsTracked {
		t.Errorf("expected IsTracked=false for legacy payload")
	}
	if legacy.Action != "menu_catalog" {
		t.Errorf("expected action menu_catalog, got %s", legacy.Action)
	}
}

func TestTrackedLinkURL(t *testing.T) {
	raw := "https://example.com/checkout?item=book"
	decorated := TrackedLinkURL(raw, 396393759, "checkout_step", "tr_99")

	parsed, err := url.Parse(decorated)
	if err != nil {
		t.Fatalf("failed to parse decorated url: %v", err)
	}

	q := parsed.Query()
	if q.Get("utm_source") != "max_bot" {
		t.Errorf("expected utm_source max_bot, got %s", q.Get("utm_source"))
	}
	if q.Get("max_uid") != "396393759" {
		t.Errorf("expected max_uid 396393759, got %s", q.Get("max_uid"))
	}
	if q.Get("utm_campaign") != "checkout_step" {
		t.Errorf("expected utm_campaign checkout_step, got %s", q.Get("utm_campaign"))
	}
	if q.Get("ref_trk") != "tr_99" {
		t.Errorf("expected ref_trk tr_99, got %s", q.Get("ref_trk"))
	}
	if q.Get("item") != "book" {
		t.Errorf("original query param item lost, got %s", q.Get("item"))
	}
}

func TestTrackedKeyboardBuilder(t *testing.T) {
	tk := NewTrackedKeyboard("main_menu", 12345)
	tk.Callback("⭐ Купить VIP", "buy_vip", maxbot.IntentPositive).
		Callback("❌ Отмена", "cancel", maxbot.IntentNegative).
		Row().
		Link("🌐 Официальный сайт", "https://example.com").
		Clipboard("📋 Промокод", "VIP2026").
		Row()

	kb := tk.Build()
	att := kb.ToAttachment()
	if att.Type != "inline_keyboard" {
		t.Fatalf("expected inline_keyboard, got %s", att.Type)
	}

	rows := kb.Rows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if len(rows[0]) != 2 || len(rows[1]) != 2 {
		t.Errorf("unexpected row lengths: %d, %d", len(rows[0]), len(rows[1]))
	}

	// Verify button payload format
	btnVIP := rows[0][0]
	if !strings.HasPrefix(btnVIP.Payload, "trk:buy_vip|main_menu|") {
		t.Errorf("unexpected tracked payload for VIP button: %s", btnVIP.Payload)
	}
}

func TestExtractClickMetrics(t *testing.T) {
	msgTime := time.Now().Add(-5 * time.Second).UnixMilli()
	cbTime := time.Now().UnixMilli()

	u := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCallback,
		Timestamp:  cbTime,
		Message: &maxbot.Message{
			MessageID: "mid.12345",
			Timestamp: msgTime,
			Recipient: &maxbot.MessageRecipient{
				ChatID: 8888,
			},
			Body: maxbot.MessageBody{
				MID: "mid.12345",
			},
		},
		Callback: &maxbot.Callback{
			CallbackID: "cb_999",
			Payload:    FormatTrackedPayload("confirm_order", "cart", "tr_abc"),
			Timestamp:  cbTime,
			User: &maxbot.User{
				UserID: 777,
				Name:   "Иван",
			},
		},
	}

	metrics, ok := ExtractClickMetrics(u)
	if !ok || metrics == nil {
		t.Fatalf("expected valid click metrics")
	}

	if metrics.UserID != 777 || metrics.UserName != "Иван" {
		t.Errorf("user mismatch: %d, %s", metrics.UserID, metrics.UserName)
	}
	if metrics.Action != "confirm_order" || metrics.Screen != "cart" || metrics.TraceID != "tr_abc" {
		t.Errorf("payload mismatch: %+v", metrics)
	}
	if !metrics.IsTracked {
		t.Errorf("expected IsTracked=true")
	}
	if metrics.TimeDelta < 4*time.Second || metrics.TimeDelta > 6*time.Second {
		t.Errorf("unexpected TimeDelta (TTK): %v", metrics.TimeDelta)
	}
}
