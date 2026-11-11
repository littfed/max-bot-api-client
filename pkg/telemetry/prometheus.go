package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// PrometheusExporter aggregates telemetry events into Prometheus text-format metrics.
type PrometheusExporter struct {
	mu                   sync.RWMutex
	buttonClicks         map[string]uint64  // "action|screen" -> count
	commands             map[string]uint64  // command -> count
	commandDurationSum   map[string]float64 // command -> sum seconds
	commandDurationCount map[string]uint64  // command -> count
	apiCalls             map[string]uint64  // method -> count
	apiDurationSum       map[string]float64 // method -> sum seconds
	apiDurationCount     map[string]uint64  // method -> count
	errors               map[string]uint64  // action -> count
	activeUsers          map[int64]time.Time
	server               *http.Server
}

// NewPrometheusExporter creates and initializes an exporter.
func NewPrometheusExporter() *PrometheusExporter {
	return &PrometheusExporter{
		buttonClicks:         make(map[string]uint64),
		commands:             make(map[string]uint64),
		commandDurationSum:   make(map[string]float64),
		commandDurationCount: make(map[string]uint64),
		apiCalls:             make(map[string]uint64),
		apiDurationSum:       make(map[string]float64),
		apiDurationCount:     make(map[string]uint64),
		errors:               make(map[string]uint64),
		activeUsers:          make(map[int64]time.Time),
	}
}

// Emit implements EventSink, consuming events to update internal metric aggregates.
func (pe *PrometheusExporter) Emit(ctx context.Context, e *Event) error {
	if e == nil {
		return nil
	}
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Track active user
	if e.UserID != 0 {
		pe.activeUsers[e.UserID] = e.Timestamp
	}

	switch e.Type {
	case EventButtonClick:
		key := fmt.Sprintf("%s|%s", e.Action, e.Screen)
		pe.buttonClicks[key]++

	case EventCommand:
		pe.commands[e.Action]++
		if e.Duration > 0 {
			pe.commandDurationSum[e.Action] += e.Duration.Seconds()
			pe.commandDurationCount[e.Action]++
		}

	case EventApiCall:
		pe.apiCalls[e.Action]++
		if e.Duration > 0 {
			pe.apiDurationSum[e.Action] += e.Duration.Seconds()
			pe.apiDurationCount[e.Action]++
		}

	case EventError:
		pe.errors[e.Action]++
	}

	return nil
}

// FormatMetrics generates the Prometheus exposition text format string.
func (pe *PrometheusExporter) FormatMetrics() string {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var sb strings.Builder

	// 1. Button Clicks
	sb.WriteString("# HELP maxbot_button_clicks_total Total number of inline button clicks\n")
	sb.WriteString("# TYPE maxbot_button_clicks_total counter\n")
	sortedClickKeys := sortedKeys(pe.buttonClicks)
	for _, k := range sortedClickKeys {
		parts := strings.Split(k, "|")
		action, screen := parts[0], ""
		if len(parts) > 1 {
			screen = parts[1]
		}
		sb.WriteString(fmt.Sprintf("maxbot_button_clicks_total{action=%q,screen=%q} %d\n",
			action, screen, pe.buttonClicks[k]))
	}

	// 2. Commands Total
	sb.WriteString("\n# HELP maxbot_commands_total Total number of bot commands executed\n")
	sb.WriteString("# TYPE maxbot_commands_total counter\n")
	sortedCmdKeys := sortedKeys(pe.commands)
	for _, cmd := range sortedCmdKeys {
		sb.WriteString(fmt.Sprintf("maxbot_commands_total{command=%q} %d\n", cmd, pe.commands[cmd]))
	}

	// 3. Command Durations (Histogram/Summary representation)
	if len(pe.commandDurationCount) > 0 {
		sb.WriteString("\n# HELP maxbot_command_duration_seconds Total execution time of bot commands\n")
		sb.WriteString("# TYPE maxbot_command_duration_seconds summary\n")
		for _, cmd := range sortedCmdKeys {
			if count, ok := pe.commandDurationCount[cmd]; ok && count > 0 {
				sb.WriteString(fmt.Sprintf("maxbot_command_duration_seconds_sum{command=%q} %.6f\n",
					cmd, pe.commandDurationSum[cmd]))
				sb.WriteString(fmt.Sprintf("maxbot_command_duration_seconds_count{command=%q} %d\n",
					cmd, count))
			}
		}
	}

	// 4. API Calls Total & Durations
	if len(pe.apiCalls) > 0 {
		sb.WriteString("\n# HELP maxbot_api_calls_total Outbound MAX Bot API calls made\n")
		sb.WriteString("# TYPE maxbot_api_calls_total counter\n")
		for _, method := range sortedKeys(pe.apiCalls) {
			sb.WriteString(fmt.Sprintf("maxbot_api_calls_total{method=%q} %d\n", method, pe.apiCalls[method]))
		}
	}

	// 5. Errors Total
	sb.WriteString("\n# HELP maxbot_errors_total Total errors encountered\n")
	sb.WriteString("# TYPE maxbot_errors_total counter\n")
	for _, act := range sortedKeys(pe.errors) {
		sb.WriteString(fmt.Sprintf("maxbot_errors_total{action=%q} %d\n", act, pe.errors[act]))
	}

	// 6. Active Users (Gauge)
	sb.WriteString("\n# HELP maxbot_active_users Unique active users seen by bot\n")
	sb.WriteString("# TYPE maxbot_active_users gauge\n")
	sb.WriteString(fmt.Sprintf("maxbot_active_users %d\n", len(pe.activeUsers)))

	return sb.String()
}

// Handler returns an http.HandlerFunc that renders the metrics on HTTP GET.
func (pe *PrometheusExporter) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.04; charset=utf-8")
		_, _ = w.Write([]byte(pe.FormatMetrics()))
	}
}

// StartServer starts a dedicated HTTP server listening for Prometheus scraping on addr.
func (pe *PrometheusExporter) StartServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", pe.Handler())

	pe.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := pe.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Prometheus server error: %v\n", err)
		}
	}()
	return nil
}

// Close stops the HTTP server if running.
func (pe *PrometheusExporter) Close() error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if pe.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pe.server.Shutdown(ctx)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
