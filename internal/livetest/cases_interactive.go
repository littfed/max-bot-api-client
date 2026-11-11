package livetest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// TestCaseInteractiveCallback tests human-in-the-loop button callback interception and RTT.
type TestCaseInteractiveCallback struct {
	Timeout time.Duration
}

func (t *TestCaseInteractiveCallback) Name() string              { return "HITL_InlineButtonCallback" }
func (t *TestCaseInteractiveCallback) Category() string          { return "hitl" }
func (t *TestCaseInteractiveCallback) Description() string       { return "Send interactive inline buttons, await user click, verify callback, and measure RTT" }
func (t *TestCaseInteractiveCallback) RequiresInteractive() bool { return true }
func (t *TestCaseInteractiveCallback) RequiresRecipient() bool   { return true }

func (t *TestCaseInteractiveCallback) Run(ctx context.Context, s *Suite) (*TestRecord, error) {
	rec := &TestRecord{
		Name:        t.Name(),
		Category:    t.Category(),
		Description: t.Description(),
		Details:     make(map[string]any),
	}

	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	matcher, err := NewMatcher(testCtx, s.Client)
	if err != nil {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("failed to initialize matcher: %w", err)
		return rec, rec.Error
	}

	var recipient maxbot.Recipient
	if s.Config.TestChatID != 0 {
		recipient = maxbot.ToChat(s.Config.TestChatID)
	} else {
		recipient = maxbot.ToUser(s.Config.TestUserID)
	}

	targetPayload := fmt.Sprintf("hitl_btn_%d", time.Now().UnixNano()%100000)

	kb := maxbot.NewInlineKeyboard().
		Row(
			maxbot.NewIntentCallbackButton("✅ Нажми для проверки", targetPayload, maxbot.IntentPositive),
			maxbot.NewIntentCallbackButton("⭐ Primary", "hitl_primary", maxbot.IntentPrimary),
		)

	promptText := fmt.Sprintf("🧪 **[LiveTest HITL] Интерактивная проверка кнопок**\n\nПожалуйста, нажмите зеленую кнопку **«Нажми для проверки»** в течение %s.\n\nТест проверит доставку события callback на сервер и измерит задержку (RTT).", timeout.Round(time.Second))

	var promptMID string
	defer func() {
		if s.Config.Cleanup && promptMID != "" {
			cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanCancel()
			_ = s.Client.DeleteMessage(cleanCtx, promptMID)
		}
	}()

	sendStart := time.Now()
	msg, err := s.Client.SendKeyboardMessage(testCtx, recipient, promptText, kb)
	sendRTT := time.Since(sendStart)
	if err != nil {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("failed to send interactive test message: %w", err)
		rec.AddEvent(DirectionOutbound, "POST /messages", promptText, false, err.Error())
		return rec, rec.Error
	}
	promptMID = msg.MessageID
	rec.Details["prompt_send_rtt_ms"] = sendRTT.Milliseconds()
	rec.AddEvent(DirectionOutbound, "POST /messages", fmt.Sprintf("Prompt sent: mid=%s", promptMID), true, msg)

	// Wait for user interaction
	update, rtt, err := matcher.WaitFor(testCtx, CallbackPayloadMatcher(targetPayload, s.Config.TestUserID))
	rec.RTT = rtt
	rec.Details["click_rtt_ms"] = rtt.Milliseconds()

	if err != nil {
		if errors.Is(err, ErrMatcherTimeout) || errors.Is(err, context.DeadlineExceeded) {
			rec.Status = StatusTimeout
			rec.Error = fmt.Errorf("interactive click timed out after %s", timeout)
			rec.AddEvent(DirectionInbound, "Timeout", fmt.Sprintf("No click received within %s", timeout), false, nil)
			return rec, nil
		}
		rec.Status = StatusFailed
		rec.Error = err
		return rec, err
	}

	// Answer callback with notification toast
	if update.Callback != nil {
		rec.Details["callback_id"] = update.Callback.CallbackID
		rec.Details["user_name"] = update.Callback.User.Name
		rec.AddEvent(DirectionInbound, "message_callback", fmt.Sprintf("Callback received from %s (payload=%s)", update.Callback.User.Name, targetPayload), true, update.Callback)

		ansErr := s.Client.AnswerCallback(testCtx, update.Callback.CallbackID, &maxbot.AnswerCallbackRequest{
			Notification: fmt.Sprintf("✅ Тест успешно пройден! (RTT: %s)", rtt.Round(time.Millisecond)),
		})
		if ansErr != nil {
			rec.Details["answer_warning"] = ansErr.Error()
		} else {
			rec.AddEvent(DirectionAction, "POST /answers", "Toast notification displayed to user", true, nil)
		}
	}

	rec.Status = StatusPassed
	return rec, nil
}
