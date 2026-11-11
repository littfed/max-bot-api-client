package livetest

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// TestCaseGetMe verifies bot credentials and retrieves bot profile metadata.
type TestCaseGetMe struct{}

func (t *TestCaseGetMe) Name() string                { return "API_GetMe" }
func (t *TestCaseGetMe) Category() string            { return "api" }
func (t *TestCaseGetMe) Description() string         { return "Verify bot token validity and fetch bot profile metadata" }
func (t *TestCaseGetMe) RequiresInteractive() bool   { return false }
func (t *TestCaseGetMe) RequiresRecipient() bool     { return false }

func (t *TestCaseGetMe) Run(ctx context.Context, s *Suite) (*TestRecord, error) {
	rec := &TestRecord{
		Name:        t.Name(),
		Category:    t.Category(),
		Description: t.Description(),
		Details:     make(map[string]any),
	}

	start := time.Now()
	me, err := s.Client.GetMe(ctx)
	rtt := time.Since(start)
	rec.RTT = rtt

	if err != nil {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("GetMe request failed: %w", err)
		rec.AddEvent(DirectionOutbound, "GET /me", "", false, err.Error())
		return rec, rec.Error
	}

	if me.UserID == 0 {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("bot user ID is zero")
		return rec, rec.Error
	}

	s.BotUser = me
	rec.Status = StatusPassed
	rec.Details["bot_id"] = me.UserID
	rec.Details["bot_name"] = me.Name
	rec.Details["username"] = me.Username
	rec.Details["is_bot"] = me.IsBot

	rec.AddEvent(DirectionOutbound, "GET /me", fmt.Sprintf("Bot: %s (@%s, ID=%d)", me.Name, me.Username, me.UserID), true, me)
	return rec, nil
}

// TestCaseSetCommands registers the bot suggestion commands menu on MAX API.
type TestCaseSetCommands struct{}

func (t *TestCaseSetCommands) Name() string                { return "API_SetCommands" }
func (t *TestCaseSetCommands) Category() string            { return "api" }
func (t *TestCaseSetCommands) Description() string         { return "Register client commands menu on MAX API gateway" }
func (t *TestCaseSetCommands) RequiresInteractive() bool   { return false }
func (t *TestCaseSetCommands) RequiresRecipient() bool     { return false }

func (t *TestCaseSetCommands) Run(ctx context.Context, s *Suite) (*TestRecord, error) {
	rec := &TestRecord{
		Name:        t.Name(),
		Category:    t.Category(),
		Description: t.Description(),
		Details:     make(map[string]any),
	}

	commands := []maxbot.BotCommand{
		{Name: "start", Description: "Главное меню и приветствие"},
		{Name: "ping", Description: "Проверка доступности бота"},
		{Name: "livetest", Description: "Запуск диагностических тестов"},
	}

	start := time.Now()
	err := s.Client.SetCommands(ctx, commands)
	rtt := time.Since(start)
	rec.RTT = rtt

	if err != nil {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("SetCommands request failed: %w", err)
		rec.AddEvent(DirectionOutbound, "PATCH /me/commands", "", false, err.Error())
		return rec, rec.Error
	}

	rec.Status = StatusPassed
	rec.Details["commands_count"] = len(commands)
	rec.AddEvent(DirectionOutbound, "PATCH /me/commands", fmt.Sprintf("Registered %d commands", len(commands)), true, commands)
	return rec, nil
}

// TestCaseUploadMedia tests the 2-step media upload flow against iu.oneme.ru.
type TestCaseUploadMedia struct{}

func (t *TestCaseUploadMedia) Name() string                { return "API_UploadMedia" }
func (t *TestCaseUploadMedia) Category() string            { return "api" }
func (t *TestCaseUploadMedia) Description() string         { return "Synthesize PNG image and perform 2-step upload to iu.oneme.ru" }
func (t *TestCaseUploadMedia) RequiresInteractive() bool   { return false }
func (t *TestCaseUploadMedia) RequiresRecipient() bool     { return false }

func (t *TestCaseUploadMedia) Run(ctx context.Context, s *Suite) (*TestRecord, error) {
	rec := &TestRecord{
		Name:        t.Name(),
		Category:    t.Category(),
		Description: t.Description(),
		Details:     make(map[string]any),
	}

	// Synthesize a 32x32 test pattern image in-memory
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.RGBA{R: 50, G: 120, B: 240, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("failed to encode synthetic PNG: %w", err)
		return rec, rec.Error
	}

	start := time.Now()
	res, err := s.Client.UploadMedia(ctx, maxbot.UploadTypeImage, "synthetic_test.png", bytes.NewReader(buf.Bytes()))
	rtt := time.Since(start)
	rec.RTT = rtt

	if err != nil {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("UploadMedia failed: %w", err)
		rec.AddEvent(DirectionOutbound, "POST /uploads + iu.oneme.ru", "", false, err.Error())
		return rec, rec.Error
	}

	if res.Token == "" {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("upload succeeded but returned empty media token")
		return rec, rec.Error
	}

	rec.Status = StatusPassed
	rec.Details["token_length"] = len(res.Token)
	rec.Details["token_prefix"] = res.Token[:min(len(res.Token), 16)] + "..."
	rec.AddEvent(DirectionOutbound, "UploadMedia (2-step)", fmt.Sprintf("Acquired token: len=%d", len(res.Token)), true, res)
	return rec, nil
}
