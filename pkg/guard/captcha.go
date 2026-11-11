package guard

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// CaptchaChallenge tracks an active verification challenge for a joined member.
type CaptchaChallenge struct {
	UserID         int64
	ChatID         int64
	ChallengeMID   string
	ExpiresAt      time.Time
	timer          *time.Timer
	ExpectedAnswer string
}

// CaptchaGate intercepts new chat members and enforces interactive captcha verification.
type CaptchaGate struct {
	client     *maxbot.Client
	timeout    time.Duration
	challenges map[string]*CaptchaChallenge // key: "chatID:userID"
	onVerified func(ctx context.Context, chatID, userID int64)
	onKicked   func(ctx context.Context, chatID, userID int64)
	mu         sync.Mutex
}

// CaptchaOption configures a CaptchaGate instance.
type CaptchaOption func(*CaptchaGate)

// WithTimeout sets the time limit for solving the captcha before the user is kicked.
func WithTimeout(d time.Duration) CaptchaOption {
	return func(g *CaptchaGate) {
		g.timeout = d
	}
}

// WithOnVerified registers a callback executed when a user successfully passes the captcha.
func WithOnVerified(fn func(ctx context.Context, chatID, userID int64)) CaptchaOption {
	return func(g *CaptchaGate) {
		g.onVerified = fn
	}
}

// WithOnKicked registers a callback executed when a user fails the captcha and is kicked.
func WithOnKicked(fn func(ctx context.Context, chatID, userID int64)) CaptchaOption {
	return func(g *CaptchaGate) {
		g.onKicked = fn
	}
}

// NewCaptchaGate creates a new interactive anti-spam gate.
func NewCaptchaGate(client *maxbot.Client, opts ...CaptchaOption) *CaptchaGate {
	g := &CaptchaGate{
		client:     client,
		timeout:    60 * time.Second,
		challenges: make(map[string]*CaptchaChallenge),
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

func (g *CaptchaGate) key(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

// HandleNewMember issues a verification challenge to the new member.
func (g *CaptchaGate) HandleNewMember(ctx context.Context, chatID, userID int64, userName string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := g.key(chatID, userID)
	if existing, ok := g.challenges[key]; ok {
		if existing.timer != nil {
			existing.timer.Stop()
		}
	}

	payloadVerify := fmt.Sprintf("guard:v:%d:%d", chatID, userID)
	kb := maxbot.NewInlineKeyboard().Row(
		maxbot.NewIntentCallbackButton("✅ Я человек (нажмите)", payloadVerify, maxbot.IntentPositive),
	)

	text := fmt.Sprintf("👋 Добро пожаловать, %s!\n\nПожалуйста, подтвердите, что вы человек, нажав на кнопку ниже в течение %d сек.",
		userName, int(g.timeout.Seconds()))

	var mid string
	if g.client != nil {
		resp, err := g.client.SendKeyboardMessage(ctx, maxbot.ToChat(chatID), text, kb)
		if err != nil {
			return fmt.Errorf("guard: failed to send captcha challenge: %w", err)
		}
		if resp != nil {
			mid = resp.ID()
		}
	}

	challenge := &CaptchaChallenge{
		UserID:         userID,
		ChatID:         chatID,
		ChallengeMID:   mid,
		ExpiresAt:      time.Now().Add(g.timeout),
		ExpectedAnswer: payloadVerify,
	}

	challenge.timer = time.AfterFunc(g.timeout, func() {
		g.onTimeout(chatID, userID)
	})

	g.challenges[key] = challenge
	return nil
}

func (g *CaptchaGate) onTimeout(chatID, userID int64) {
	g.mu.Lock()
	key := g.key(chatID, userID)
	challenge, ok := g.challenges[key]
	if !ok {
		g.mu.Unlock()
		return
	}
	delete(g.challenges, key)
	g.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if g.client != nil {
		// 1. Kick the spammer/unverified user
		_ = g.client.KickChatMember(ctx, chatID, userID)

		// 2. Remove challenge message if mid is known
		if challenge.ChallengeMID != "" {
			_ = g.client.DeleteMessage(ctx, challenge.ChallengeMID)
		}
	}

	if g.onKicked != nil {
		g.onKicked(ctx, chatID, userID)
	}
}

// VerifyCallback processes an incoming callback query for captcha resolution.
// Returns (handled bool, err error).
func (g *CaptchaGate) VerifyCallback(c *bot.Context) (bool, error) {
	cb := c.Callback()
	if cb == nil || !strings.HasPrefix(cb.Payload, "guard:v:") {
		return false, nil
	}

	parts := strings.Split(cb.Payload, ":")
	if len(parts) != 4 {
		return false, nil
	}

	var chatID, userID int64
	fmt.Sscanf(parts[2], "%d", &chatID)
	fmt.Sscanf(parts[3], "%d", &userID)

	clickerID := int64(0)
	if cb.User != nil {
		clickerID = cb.User.UserID
	}

	// Reject if clicked by someone else
	if clickerID != userID {
		_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
			Notification: "⚠️ Эта проверка предназначена для другого пользователя!",
		})
		return true, nil
	}

	g.mu.Lock()
	key := g.key(chatID, userID)
	challenge, ok := g.challenges[key]
	if !ok {
		g.mu.Unlock()
		_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
			Notification: "Проверка уже завершена или истекла.",
		})
		return true, nil
	}

	if challenge.timer != nil {
		challenge.timer.Stop()
	}
	delete(g.challenges, key)
	g.mu.Unlock()

	// Verification succeeded
	_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
		Notification: "✅ Проверка успешно пройдена! Добро пожаловать!",
	})

	if g.client != nil && challenge.ChallengeMID != "" {
		_ = g.client.DeleteMessage(c.Ctx(), challenge.ChallengeMID)
	}

	if g.onVerified != nil {
		g.onVerified(c.Ctx(), chatID, userID)
	}

	return true, nil
}

// Register installs the captcha callback interceptor onto a bot.Router.
func (g *CaptchaGate) Register(router *bot.Router) {
	router.OnCallbackPrefix("guard:v:", func(c *bot.Context) error {
		_, err := g.VerifyCallback(c)
		return err
	})
}
