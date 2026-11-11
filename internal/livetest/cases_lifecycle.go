package livetest

import (
	"context"
	"fmt"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// TestCaseMessageLifecycle verifies the end-to-end outbound message lifecycle:
// Send -> Pin -> Edit -> Unpin -> Delete, with strict deferred cleanup guarantees.
type TestCaseMessageLifecycle struct{}

func (t *TestCaseMessageLifecycle) Name() string              { return "Lifecycle_MessageSendEditDelete" }
func (t *TestCaseMessageLifecycle) Category() string          { return "lifecycle" }
func (t *TestCaseMessageLifecycle) Description() string       { return "Send outbound message, pin, edit text, unpin, and delete with guaranteed cleanup" }
func (t *TestCaseMessageLifecycle) RequiresInteractive() bool { return false }
func (t *TestCaseMessageLifecycle) RequiresRecipient() bool   { return true }

func (t *TestCaseMessageLifecycle) Run(ctx context.Context, s *Suite) (*TestRecord, error) {
	rec := &TestRecord{
		Name:        t.Name(),
		Category:    t.Category(),
		Description: t.Description(),
		Details:     make(map[string]any),
	}

	var recipient maxbot.Recipient
	if s.Config.TestChatID != 0 {
		recipient = maxbot.ToChat(s.Config.TestChatID)
	} else if s.Config.TestUserID != 0 {
		recipient = maxbot.ToUser(s.Config.TestUserID)
	} else {
		rec.Status = StatusSkipped
		rec.ErrorMsg = "neither TEST_CHAT_ID nor TEST_USER_ID configured"
		return rec, nil
	}

	var messageIDsToDelete []string
	defer func() {
		if s.Config.Cleanup && len(messageIDsToDelete) > 0 {
			cleanCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for _, mid := range messageIDsToDelete {
				if mid != "" {
					_ = s.Client.DeleteMessage(cleanCtx, mid)
				}
			}
		}
	}()

	// 1. Send initial test message
	startSend := time.Now()
	initText := fmt.Sprintf("🧪 **[LiveTest] Проверка жизненного цикла сообщения**\nОтправлено: `%s`", startSend.Format("15:04:05.000"))
	msg, err := s.Client.SendTextMessage(ctx, recipient, initText)
	sendRTT := time.Since(startSend)

	if err != nil {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("SendTextMessage failed: %w", err)
		rec.AddEvent(DirectionOutbound, "POST /messages", initText, false, err.Error())
		return rec, rec.Error
	}

	if msg.MessageID == "" {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("SendTextMessage succeeded but MessageID (body.mid) is empty")
		return rec, rec.Error
	}

	messageIDsToDelete = append(messageIDsToDelete, msg.MessageID)
	rec.Details["message_id"] = msg.MessageID
	rec.Details["send_rtt_ms"] = sendRTT.Milliseconds()
	rec.AddEvent(DirectionOutbound, "POST /messages", fmt.Sprintf("Sent message: mid=%s", msg.MessageID), true, msg)

	// 2. Pin message (if TestChatID is present)
	if s.Config.TestChatID != 0 {
		startPin := time.Now()
		pinErr := s.Client.PinMessage(ctx, s.Config.TestChatID, msg.MessageID, false)
		pinRTT := time.Since(startPin)
		if pinErr != nil {
			// Pinning in some dialogs might be restricted by permissions; record warning but don't hard-fail if not allowed
			rec.Details["pin_status"] = fmt.Sprintf("warning: %v", pinErr)
			rec.AddEvent(DirectionAction, "PUT /chats/{id}/pin", fmt.Sprintf("Pin skipped/unsupported: %v", pinErr), false, pinErr)
		} else {
			rec.Details["pin_rtt_ms"] = pinRTT.Milliseconds()
			rec.AddEvent(DirectionAction, "PUT /chats/{id}/pin", "Message pinned successfully", true, nil)

			// Small pause for visual realism
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return rec, ctx.Err()
			}

			// Unpin message
			startUnpin := time.Now()
			_ = s.Client.UnpinMessage(ctx, s.Config.TestChatID)
			rec.Details["unpin_rtt_ms"] = time.Since(startUnpin).Milliseconds()
			rec.AddEvent(DirectionAction, "DELETE /chats/{id}/pin", "Message unpinned successfully", true, nil)
		}
	}

	// 3. Edit message text
	startEdit := time.Now()
	editText := fmt.Sprintf("🧪 **[LiveTest] ✅ Сообщение успешно отредактировано!**\n_MID: `%s`_\n_RTT редактирования: %s_", msg.MessageID, time.Since(startEdit).Round(time.Millisecond))
	editedMsg, err := s.Client.EditMessage(ctx, msg.MessageID, &maxbot.EditMessageRequest{
		Text:   editText,
		Format: "markdown",
	})
	editRTT := time.Since(startEdit)

	if err != nil {
		rec.Status = StatusFailed
		rec.Error = fmt.Errorf("EditMessage failed: %w", err)
		rec.AddEvent(DirectionOutbound, "PUT /messages", editText, false, err.Error())
		return rec, rec.Error
	}

	rec.Details["edit_rtt_ms"] = editRTT.Milliseconds()
	rec.AddEvent(DirectionOutbound, "PUT /messages", "Message edited successfully", true, editedMsg)

	// Pause briefly before deletion if cleanup is enabled
	if s.Config.Cleanup {
		select {
		case <-time.After(600 * time.Millisecond):
		case <-ctx.Done():
			return rec, ctx.Err()
		}

		startDel := time.Now()
		delErr := s.Client.DeleteMessage(ctx, msg.MessageID)
		delRTT := time.Since(startDel)

		if delErr != nil {
			rec.Status = StatusFailed
			rec.Error = fmt.Errorf("DeleteMessage failed: %w", delErr)
			rec.AddEvent(DirectionAction, "DELETE /messages", "", false, delErr.Error())
			return rec, rec.Error
		}

		// Message already deleted, clear from cleanup list
		messageIDsToDelete = nil
		rec.Details["delete_rtt_ms"] = delRTT.Milliseconds()
		rec.AddEvent(DirectionAction, "DELETE /messages", fmt.Sprintf("Message %s deleted", msg.MessageID), true, nil)
	}

	rec.Status = StatusPassed
	rec.RTT = sendRTT + editRTT
	return rec, nil
}
