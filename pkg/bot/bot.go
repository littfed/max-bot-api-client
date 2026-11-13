package bot

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// Bot provides high-level event routing and Long Polling update consumption.
type Bot struct {
	client          *maxbot.Client
	router          *Router
	logger          *slog.Logger
	pollingTimeout  int
	pollingInterval time.Duration
	marker          int64
	recentChats     sync.Map
}

// BotOption configures a Bot instance.
type BotOption func(*Bot)

// WithLogger configures a structured slog.Logger.
func WithLogger(logger *slog.Logger) BotOption {
	return func(b *Bot) {
		b.logger = logger
	}
}

// WithPollingTimeout sets the Long Polling server wait timeout in seconds (default: 30s).
func WithPollingTimeout(seconds int) BotOption {
	return func(b *Bot) {
		b.pollingTimeout = seconds
	}
}

// WithPollingInterval sets a sleep interval between consecutive poll requests (default: 0).
func WithPollingInterval(interval time.Duration) BotOption {
	return func(b *Bot) {
		b.pollingInterval = interval
	}
}

// WithInitialMarker sets the starting marker for update pagination.
func WithInitialMarker(marker int64) BotOption {
	return func(b *Bot) {
		b.marker = marker
	}
}

// New creates a new Bot from a bot token.
func New(token string, opts ...BotOption) (*Bot, error) {
	client, err := maxbot.NewClient(token)
	if err != nil {
		return nil, err
	}
	return NewWithClient(client, opts...), nil
}

// NewWithClient creates a Bot using an existing maxbot.Client.
func NewWithClient(client *maxbot.Client, opts ...BotOption) *Bot {
	b := &Bot{
		client:         client,
		router:         NewRouter(),
		logger:         slog.Default(),
		pollingTimeout: 30,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Client returns the underlying maxbot.Client.
func (b *Bot) Client() *maxbot.Client {
	return b.client
}

// Router returns the bot's internal Router.
func (b *Bot) Router() *Router {
	return b.router
}

// Use adds middlewares to the bot router.
func (b *Bot) Use(mw ...MiddlewareFunc) {
	b.router.Use(mw...)
}

// OnCommand registers a handler for a specific command (e.g. "/ping").
func (b *Bot) OnCommand(command string, handler HandlerFunc) {
	b.router.OnCommand(command, handler)
}

// OnText registers a handler for exact text matching (case-insensitive).
func (b *Bot) OnText(text string, handler HandlerFunc) {
	b.router.OnText(text, handler)
}

// OnTextRegex registers a handler matching message text against a regex.
func (b *Bot) OnTextRegex(re *regexp.Regexp, handler HandlerFunc) {
	b.router.OnTextRegex(re, handler)
}

// OnStart registers a handler for bot launch (both `bot_started` and `/start`).
func (b *Bot) OnStart(handler HandlerFunc) {
	b.router.OnStart(handler)
}

// OnMessage registers a handler for any incoming message.
func (b *Bot) OnMessage(handler HandlerFunc) {
	b.router.OnMessage(handler)
}

// OnIdleMessage registers a handler for plain incoming messages when no FSM state is active.
func (b *Bot) OnIdleMessage(handler HandlerFunc) {
	b.router.OnIdleMessage(handler)
}

// OnCallback registers a handler for any callback query.
func (b *Bot) OnCallback(handler HandlerFunc) {
	b.router.OnCallback(handler)
}

// OnCallbackPayload registers a handler for a callback matching an exact payload.
func (b *Bot) OnCallbackPayload(payload string, handler HandlerFunc) {
	b.router.OnCallbackPayload(payload, handler)
}

// OnCallbackPrefix registers a handler for a callback starting with a prefix.
func (b *Bot) OnCallbackPrefix(prefix string, handler HandlerFunc) {
	b.router.OnCallbackPrefix(prefix, handler)
}

// OnAction registers a handler matching a callback action (plain or tracked).
func (b *Bot) OnAction(action string, handler HandlerFunc) {
	b.router.OnAction(action, handler)
}

// OnState registers a handler executed when the session matches the given FSM state.
func (b *Bot) OnState(state string, handler HandlerFunc) {
	b.router.OnState(state, handler)
}

// OnStateCallback registers a handler executed when in a specific FSM state and receiving a matching callback payload.
func (b *Bot) OnStateCallback(state string, payload string, handler HandlerFunc) {
	b.router.OnStateCallback(state, payload, handler)
}

// OnBotAdded registers a handler invoked when the bot is added to a group chat or channel.
func (b *Bot) OnBotAdded(handler HandlerFunc) {
	b.router.OnBotAdded(handler)
}

// OnBotRemoved registers a handler invoked when the bot is removed from a group chat or channel.
func (b *Bot) OnBotRemoved(handler HandlerFunc) {
	b.router.OnBotRemoved(handler)
}

// OnAudio registers a handler for incoming audio/voice messages.
func (b *Bot) OnAudio(handler HandlerFunc) {
	b.router.OnAudio(handler)
}

// OnVoiceNoteAttempt registers a handler invoked when a user records a voice note via the MAX mobile app microphone.
func (b *Bot) OnVoiceNoteAttempt(handler HandlerFunc) {
	b.router.OnVoiceNoteAttempt(handler)
}

// OnAny registers a fallback handler matching any incoming update.
func (b *Bot) OnAny(handler HandlerFunc) {
	b.router.OnAny(handler)
}

// RecordActiveChat registers an active chat and user ID for correlating bare events.
func (b *Bot) RecordActiveChat(chatID, userID int64) {
	if chatID != 0 {
		b.recentChats.Store(chatID, userID)
	}
}

// ProcessUpdate handles a single update (useful for both Long Polling and Webhook endpoints).
func (b *Bot) ProcessUpdate(ctx context.Context, u *maxbot.Update) error {
	raw := u.RawJSON
	if raw == "" {
		bData, _ := json.Marshal(u)
		raw = string(bData)
	}
	b.logger.Info("incoming update", slog.String("type", u.UpdateType), slog.String("raw", raw))

	// Track active chat from incoming messages or callbacks
	if u.Message != nil {
		var chatID int64
		if u.Message.Recipient != nil {
			chatID = u.Message.Recipient.ChatID
		}
		var userID int64
		if u.Message.Sender != nil {
			userID = u.Message.Sender.UserID
		}
		if chatID != 0 {
			b.RecordActiveChat(chatID, userID)
		}
	} else if u.Callback != nil {
		var chatID int64
		if u.Callback.Message != nil && u.Callback.Message.Recipient != nil {
			chatID = u.Callback.Message.Recipient.ChatID
		}
		var userID int64
		if u.Callback.User != nil {
			userID = u.Callback.User.UserID
		}
		if chatID != 0 {
			b.RecordActiveChat(chatID, userID)
		}
	}

	// Handle bare message_created updates (omitted body when recording voice note via mobile microphone)
	if u.UpdateType == maxbot.UpdateTypeMessageCreated && u.Message == nil {
		b.logger.Warn("bare message_created received without message body - resolving correlated chat",
			slog.Int64("timestamp", u.Timestamp),
		)

		// Brief pause to allow platform backend CDN/indexing sync
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		var targetChatID int64
		var targetUserID int64

		b.recentChats.Range(func(key, value any) bool {
			cid := key.(int64)
			uid := value.(int64)
			chat, err := b.client.GetChat(ctx, cid)
			if err == nil && chat != nil {
				diff := chat.LastEventTime - u.Timestamp
				if diff < 0 {
					diff = -diff
				}
				if diff <= 5000 {
					targetChatID = cid
					targetUserID = uid
					return false
				}
			}
			return true
		})

		if targetChatID == 0 {
			b.recentChats.Range(func(key, value any) bool {
				targetChatID = key.(int64)
				targetUserID = value.(int64)
				return false
			})
		}

		if targetChatID != 0 {
			// Query latest messages to check if audio was delayed
			msgs, err := b.client.GetMessages(ctx, targetChatID, 3)
			if err == nil && len(msgs) > 0 {
				latest := msgs[0]
				diff := latest.Timestamp - u.Timestamp
				if diff < 0 {
					diff = -diff
				}
				if diff <= 5000 && len(latest.Body.Attachments) > 0 {
					u.Message = &latest
					c := NewContext(ctx, b.client, u)
					return b.router.Handle(c)
				}
			}

			// MAX server omitted voice payload. Construct synthetic message so bot can inform the user.
			synthMsg := &maxbot.Message{
				Timestamp: u.Timestamp,
				Recipient: &maxbot.MessageRecipient{
					ChatID:   targetChatID,
					ChatType: "dialog",
					UserID:   targetUserID,
				},
				Sender: &maxbot.User{
					UserID: targetUserID,
				},
				Body: maxbot.MessageBody{
					Text: "[голос с микрофона]",
					Attachments: []maxbot.Attachment{
						{
							Type: "voice_note_attempt",
						},
					},
				},
			}
			u.Message = synthMsg
		}
	}

	c := NewContext(ctx, b.client, u)
	return b.router.Handle(c)
}

// StartLongPolling begins sequentially consuming events from platform-api2.max.ru.
// Blocks until ctx is cancelled or an unrecoverable error occurs.
func (b *Bot) StartLongPolling(ctx context.Context) error {
	// 1. Verify credentials and connectivity on startup
	me, err := b.client.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify bot token on startup: %w", err)
	}

	b.logger.Info("connected to MAX Bot API",
		slog.String("bot_name", me.Name),
		slog.String("bot_username", me.Username),
		slog.Int64("bot_id", me.UserID),
	)

	// Long Polling loop
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("long polling stopped gracefully by context cancellation")
			return ctx.Err()
		default:
		}

		opts := &maxbot.GetUpdatesOptions{
			Marker:  b.marker,
			Timeout: b.pollingTimeout,
			Limit:   100,
		}

		resp, err := b.client.GetUpdates(ctx, opts)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}

			// If rate limited, backoff
			if maxbot.IsTooManyRequests(err) {
				b.logger.Warn("rate limited by MAX API (429), backing off for 2 seconds")
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(2 * time.Second):
					continue
				}
			}

			b.logger.Error("failed to fetch updates via long polling", slog.Any("error", err))
			// Transient network error backoff
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(1 * time.Second):
				continue
			}
		}

		if resp != nil {
			if resp.Marker > 0 {
				b.marker = resp.Marker
			}

			for _, update := range resp.Updates {
				u := update
				go func(item *maxbot.Update) {
					if err := b.ProcessUpdate(ctx, item); err != nil {
						b.logger.Error("handler error processing update",
							slog.String("type", item.UpdateType),
							slog.Any("error", err),
						)
					}
				}(&u)
			}
		}

		if b.pollingInterval > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(b.pollingInterval):
			}
		}
	}
}

// WebhookHandler returns an http.HandlerFunc that decodes incoming webhooks and dispatches them.
func (b *Bot) WebhookHandler(secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if secret != "" {
			reqSecret := r.Header.Get("X-Max-Bot-Api-Secret")
			if subtle.ConstantTimeCompare([]byte(reqSecret), []byte(secret)) != 1 {
				http.Error(w, "Unauthorized secret", http.StatusUnauthorized)
				return
			}
		}

		var update maxbot.Update
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = b.ProcessUpdate(r.Context(), &update)
	}
}
