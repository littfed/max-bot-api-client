package bot

import (
	"fmt"
	"log/slog"
	"time"
)

// Recovery returns a middleware that recovers from panics in downstream handlers.
func Recovery(logger *slog.Logger) MiddlewareFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in handler: %v", r)
					logger.Error("recovered from handler panic",
						slog.Any("panic", r),
						slog.String("text", c.Text()),
					)
				}
			}()
			return next(c)
		}
	}
}

// Logger returns a middleware that logs update execution metrics.
func Logger(logger *slog.Logger) MiddlewareFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			start := time.Now()
			sender := c.Sender()
			var senderID int64
			var username string
			if sender != nil {
				senderID = sender.UserID
				username = sender.Username
			}

			err := next(c)
			duration := time.Since(start)

			if err != nil {
				logger.Error("update handled with error",
					slog.Int64("user_id", senderID),
					slog.String("username", username),
					slog.String("text", c.Text()),
					slog.Duration("duration", duration),
					slog.Any("error", err),
				)
			} else {
				logger.Info("update handled",
					slog.Int64("user_id", senderID),
					slog.String("username", username),
					slog.String("text", c.Text()),
					slog.Duration("duration", duration),
				)
			}
			return err
		}
	}
}
