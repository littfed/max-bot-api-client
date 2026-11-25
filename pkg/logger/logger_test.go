package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		fallback slog.Level
		expected slog.Level
	}{
		{"upper_debug", "DEBUG", slog.LevelInfo, slog.LevelDebug},
		{"lower_debug", "debug", slog.LevelInfo, slog.LevelDebug},
		{"upper_info", "INFO", slog.LevelDebug, slog.LevelInfo},
		{"lower_warn", "warn", slog.LevelInfo, slog.LevelWarn},
		{"warning_alias", "warning", slog.LevelInfo, slog.LevelWarn},
		{"lower_error", "error", slog.LevelInfo, slog.LevelError},
		{"unknown_fallback", "unknown", slog.LevelDebug, slog.LevelDebug},
		{"empty_fallback", "", slog.LevelWarn, slog.LevelWarn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseLevel(tt.input, tt.fallback)
			if got != tt.expected {
				t.Errorf("ParseLevel(%q, %v) = %v, want %v", tt.input, tt.fallback, got, tt.expected)
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"short", "12345", "***"},
		{"exact_8", "12345678", "12***78"},
		{"api_key", "sk-1234567890abcdef", "sk-1***def"},
		{"bot_token", "536754869:AAFn_abc123xyz_987", "5367***987"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MaskSecret(tt.input)
			if got != tt.expected {
				t.Errorf("MaskSecret(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeString(t *testing.T) {
	raw := "Authorization: Bearer secretToken12345 and key sk-998877665544332211"
	sanitized := SanitizeString(raw)

	if strings.Contains(sanitized, "secretToken12345") {
		t.Errorf("SanitizeString failed to mask bearer token: %s", sanitized)
	}
	if strings.Contains(sanitized, "sk-998877665544332211") {
		t.Errorf("SanitizeString failed to mask sk- key: %s", sanitized)
	}
	if !strings.Contains(sanitized, "Bearer ") {
		t.Errorf("SanitizeString corrupted Bearer prefix: %s", sanitized)
	}
}

func TestLoggerJSONOutputAndMasking(t *testing.T) {
	var buf bytes.Buffer
	l := New(
		WithLevel(slog.LevelDebug),
		WithOutput(&buf),
	)

	l.Info("user action",
		slog.String("user_id", "12345"),
		slog.String("token", "secretBotToken123456"),
		slog.String("gma_key", "sk-realGmaKey999888777"),
		slog.String("prompt", "draw a red cat with Authorization: Bearer mySecretToken"),
		slog.Bool("has_bot_token", true),
	)

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line JSON output, got %d lines: %s", len(lines), out)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		t.Fatalf("output is not valid JSON: %v, raw: %s", err, lines[0])
	}

	if data["level"] != "INFO" {
		t.Errorf("expected level INFO, got %v", data["level"])
	}
	if data["msg"] != "user action" {
		t.Errorf("expected msg 'user action', got %v", data["msg"])
	}

	// Verify token masking
	tokenVal, ok := data["token"].(string)
	if !ok || strings.Contains(tokenVal, "secretBotToken123456") {
		t.Errorf("token was not masked: %v", tokenVal)
	}

	// Verify gma_key masking
	gmaVal, ok := data["gma_key"].(string)
	if !ok || strings.Contains(gmaVal, "sk-realGmaKey999888777") {
		t.Errorf("gma_key was not masked: %v", gmaVal)
	}

	// Verify embedded token in prompt string is masked
	promptVal, ok := data["prompt"].(string)
	if !ok || strings.Contains(promptVal, "mySecretToken") {
		t.Errorf("embedded secret in prompt was not masked: %v", promptVal)
	}

	// Verify has_bot_token boolean was preserved
	hasTok, ok := data["has_bot_token"].(bool)
	if !ok || !hasTok {
		t.Errorf("has_bot_token boolean flag corrupted: %v", data["has_bot_token"])
	}

	// Verify source attribution exists
	if _, ok := data["source"]; !ok {
		t.Errorf("source attribute missing in JSON output")
	}
}

func TestLoggerEnvLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "warn")
	var buf bytes.Buffer
	l := New(WithOutput(&buf))

	l.Debug("debug message")
	l.Info("info message")
	l.Warn("warn message")

	out := buf.String()
	if strings.Contains(out, "debug message") {
		t.Errorf("debug message should have been filtered out: %s", out)
	}
	if strings.Contains(out, "info message") {
		t.Errorf("info message should have been filtered out: %s", out)
	}
	if !strings.Contains(out, "warn message") {
		t.Errorf("warn message should have been logged: %s", out)
	}
}
