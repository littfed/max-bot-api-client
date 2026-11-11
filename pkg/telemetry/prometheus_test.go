package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrometheusExporterFormatAndHandler(t *testing.T) {
	exp := NewPrometheusExporter()
	ctx := context.Background()

	// Emit test events
	_ = exp.Emit(ctx, NewButtonClickEvent(101, 501, "buy_vip", "catalog", 350*time.Millisecond))
	_ = exp.Emit(ctx, NewButtonClickEvent(102, 501, "buy_vip", "catalog", 200*time.Millisecond))
	_ = exp.Emit(ctx, NewButtonClickEvent(101, 501, "cancel", "checkout", 100*time.Millisecond))
	_ = exp.Emit(ctx, NewCommandEvent(101, 501, "/start", 15*time.Millisecond, nil))
	_ = exp.Emit(ctx, NewCommandEvent(103, 502, "/start", 25*time.Millisecond, nil))
	_ = exp.Emit(ctx, NewErrorEvent(103, 502, "payment_failed", nil))

	// Verify text output
	metricsText := exp.FormatMetrics()

	if !strings.Contains(metricsText, `maxbot_button_clicks_total{action="buy_vip",screen="catalog"} 2`) {
		t.Errorf("missing or incorrect buy_vip counter: %s", metricsText)
	}
	if !strings.Contains(metricsText, `maxbot_button_clicks_total{action="cancel",screen="checkout"} 1`) {
		t.Errorf("missing or incorrect cancel counter: %s", metricsText)
	}
	if !strings.Contains(metricsText, `maxbot_commands_total{command="/start"} 2`) {
		t.Errorf("missing or incorrect /start counter: %s", metricsText)
	}
	if !strings.Contains(metricsText, `maxbot_command_duration_seconds_count{command="/start"} 2`) {
		t.Errorf("missing /start duration count: %s", metricsText)
	}
	if !strings.Contains(metricsText, `maxbot_errors_total{action="payment_failed"} 1`) {
		t.Errorf("missing payment_failed error count: %s", metricsText)
	}
	if !strings.Contains(metricsText, `maxbot_active_users 3`) { // Users: 101, 102, 103
		t.Errorf("expected 3 active users, got: %s", metricsText)
	}

	// Test HTTP handler
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	exp.Handler()(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /metrics, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/plain") {
		t.Errorf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}
}

func TestPrometheusExporterWithCollector(t *testing.T) {
	exp := NewPrometheusExporter()
	collector := NewCollector(WithSink(exp))
	defer collector.Close()

	ctx := context.Background()
	collector.Record(ctx, NewButtonClickEvent(999, 111, "click_test", "home", 0))

	metrics := exp.FormatMetrics()
	if !strings.Contains(metrics, `maxbot_button_clicks_total{action="click_test",screen="home"} 1`) {
		t.Errorf("collector failed to pipe event to PrometheusExporter: %s", metrics)
	}
}
