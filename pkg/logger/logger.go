package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	bearerRegex = regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9_\-\.]{6,})`)
	skKeyRegex  = regexp.MustCompile(`\b(sk-[a-zA-Z0-9_\-]{6,})\b`)
	botTokRegex = regexp.MustCompile(`\b(\d{6,}:[a-zA-Z0-9_\-]{10,})\b`)
)

// sensitiveKeySet contains lowercased key names that hold sensitive credentials.
var sensitiveKeySet = map[string]bool{
	"token":         true,
	"bot_token":     true,
	"has_bot_token": false, // boolean flags shouldn't be masked
	"has_gma_key":   false,
	"gma_key":       true,
	"api_key":       true,
	"apikey":        true,
	"password":      true,
	"pass":          true,
	"secret":        true,
	"secret_key":    true,
	"terminal_key":  true,
	"terminalpass":  true,
	"authorization": true,
	"auth":          true,
	"cookie":        true,
	"set-cookie":    true,
}

// MaskSecret masks sensitive values, preserving edge characters for debugging.
func MaskSecret(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	l := len(val)
	if l <= 6 {
		return "***"
	}
	if l <= 12 {
		return val[:2] + "***" + val[l-2:]
	}
	return val[:4] + "***" + val[l-3:]
}

// SanitizeString cleans secrets inside strings (e.g. Bearer tokens, sk- keys, bot tokens).
func SanitizeString(s string) string {
	s = bearerRegex.ReplaceAllStringFunc(s, func(m string) string {
		parts := bearerRegex.FindStringSubmatch(m)
		if len(parts) == 3 {
			return parts[1] + MaskSecret(parts[2])
		}
		return m
	})
	s = skKeyRegex.ReplaceAllStringFunc(s, func(m string) string {
		return MaskSecret(m)
	})
	s = botTokRegex.ReplaceAllStringFunc(s, func(m string) string {
		return MaskSecret(m)
	})
	return s
}

// ParseLevel parses a string log level into slog.Level.
// Supported: debug, info, warn, warning, error.
// Defaults to slog.LevelDebug if empty or unrecognized when in test/dev mode.
func ParseLevel(lvl string, defaultLevel slog.Level) slog.Level {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return defaultLevel
	}
}

// Options configures the structured slog JSON logger.
type Options struct {
	Level  slog.Level
	Output io.Writer
}

// Option modifies logger Options.
type Option func(*Options)

// WithLevel sets the minimum log level.
func WithLevel(lvl slog.Level) Option {
	return func(o *Options) {
		o.Level = lvl
	}
}

// WithOutput sets the output writer (defaults to os.Stdout).
func WithOutput(w io.Writer) Option {
	return func(o *Options) {
		o.Output = w
	}
}

// New creates a new structured JSON slog.Logger.
func New(opts ...Option) *slog.Logger {
	defaultLvl := slog.LevelDebug
	if envLvl := os.Getenv("LOG_LEVEL"); envLvl != "" {
		defaultLvl = ParseLevel(envLvl, slog.LevelDebug)
	}

	cfg := Options{
		Level:  defaultLvl,
		Output: os.Stdout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	replaceAttr := func(groups []string, a slog.Attr) slog.Attr {
		keyLower := strings.ToLower(a.Key)

		// 1. Check if the key itself indicates a sensitive value
		if mask, ok := sensitiveKeySet[keyLower]; ok && mask {
			if strVal, ok := a.Value.Any().(string); ok {
				return slog.String(a.Key, MaskSecret(strVal))
			}
			return slog.String(a.Key, "***")
		}

		// 2. Format source: show short file:line
		if a.Key == slog.SourceKey {
			if source, ok := a.Value.Any().(*slog.Source); ok && source != nil {
				source.File = filepath.Base(source.File)
				return slog.Any(a.Key, source)
			}
		}

		// 3. Sanitize string values for embedded secrets
		if a.Value.Kind() == slog.KindString {
			sanitized := SanitizeString(a.Value.String())
			if sanitized != a.Value.String() {
				return slog.String(a.Key, sanitized)
			}
		}

		return a
	}

	handlerOpts := &slog.HandlerOptions{
		Level:       cfg.Level,
		AddSource:   true,
		ReplaceAttr: replaceAttr,
	}

	jsonHandler := slog.NewJSONHandler(cfg.Output, handlerOpts)
	return slog.New(jsonHandler)
}

// InitGlobal configures the default slog logger using New().
func InitGlobal(opts ...Option) *slog.Logger {
	l := New(opts...)
	slog.SetDefault(l)
	return l
}

// ContextWithLogger stores a logger in a context.
type contextKey struct{}

var loggerKey = contextKey{}

// WithContext returns a new context containing the logger.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from context or returns slog.Default().
func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}
