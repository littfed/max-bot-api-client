package bot

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

// Recovery returns a middleware that recovers from panics in downstream handlers.
func Recovery(customLogger *slog.Logger) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					err = fmt.Errorf("panic in handler: %v", r)
					log := c.Logger()
					if customLogger != nil {
						log = customLogger.With(
							slog.String("trace_id", c.TraceID()),
						)
					}
					log.Error("recovered from handler panic",
						slog.Any("panic", r),
						slog.String("stack_trace", stack),
						slog.String("text", c.Text()),
					)
				}
			}()
			return next(c)
		}
	}
}

// Logger returns a middleware that logs update execution metrics.
func Logger(customLogger *slog.Logger) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			start := time.Now()
			log := c.Logger()
			if customLogger != nil {
				log = customLogger.With(
					slog.String("trace_id", c.TraceID()),
				)
			}

			cbPayload := ""
			if cb := c.Callback(); cb != nil {
				cbPayload = cb.Payload
			}

			log.Debug("update execution started",
				slog.String("text", c.Text()),
				slog.String("payload", cbPayload),
			)

			err := next(c)
			duration := time.Since(start)

			if err != nil {
				log.Error("update handled with error",
					slog.String("text", c.Text()),
					slog.String("payload", cbPayload),
					slog.Duration("duration", duration),
					slog.Int64("duration_ms", duration.Milliseconds()),
					slog.Any("error", err),
				)
			} else {
				log.Info("update handled",
					slog.String("text", c.Text()),
					slog.String("payload", cbPayload),
					slog.Duration("duration", duration),
					slog.Int64("duration_ms", duration.Milliseconds()),
				)
			}
			return err
		}
	}
}
