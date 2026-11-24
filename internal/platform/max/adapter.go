package max

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// BotClient defines the minimal interface required from the MAX Bot client.
// This interface is satisfied by *maxbot.Client.
type BotClient interface {
	SendMessage(ctx context.Context, recipient maxbot.Recipient, msg *maxbot.SendMessageRequest) (*maxbot.Message, error)
	EditMessage(ctx context.Context, messageID string, msg *maxbot.EditMessageRequest) (*maxbot.Message, error)
	AnswerCallback(ctx context.Context, callbackID string, answer *maxbot.AnswerCallbackRequest) error
	GetChatMembers(ctx context.Context, chatID int64, marker ...int64) (*maxbot.ChatMembersResponse, error)
	SendChatAction(ctx context.Context, chatID int64, action string) error
}

// Adapter implements platform.PlatformAdapter for MAX messenger.
type Adapter struct {
	client BotClient
}

// Ensure Adapter implements platform.PlatformAdapter at compile time.
var _ platform.PlatformAdapter = (*Adapter)(nil)

// NewAdapter creates a new MAX PlatformAdapter wrapping a BotClient.
func NewAdapter(client BotClient) *Adapter {
	return &Adapter{
		client: client,
	}
}

// Platform returns the unique platform identifier.
func (a *Adapter) Platform() string {
	return platform.PlatformMAX
}

// SendMessage delivers a message to a recipient (user or chat) via MAX Bot API.
func (a *Adapter) SendMessage(ctx context.Context, recipientID string, body *platform.OutMessage) error {
	if a.client == nil {
		return fmt.Errorf("max client is not configured")
	}
	if body == nil {
		return fmt.Errorf("message body cannot be nil")
	}

	recipient, err := parseRecipient(recipientID)
	if err != nil {
		return fmt.Errorf("failed to parse recipient ID %q: %w", recipientID, err)
	}

	req := &maxbot.SendMessageRequest{
		Text:   body.Text,
		Format: body.ParseMode,
		Notify: body.Notify,
	}
	if req.Format == "" {
		req.Format = "markdown"
	}

	if len(body.Buttons) > 0 {
		kb := buildKeyboard(body.Buttons)
		req.Attachments = append(req.Attachments, kb.ToAttachment())
	}

	_, err = a.client.SendMessage(ctx, recipient, req)
	return err
}

// EditMessage modifies an existing message's text and/or buttons.
func (a *Adapter) EditMessage(ctx context.Context, messageID string, body *platform.OutMessage) error {
	if a.client == nil {
		return fmt.Errorf("max client is not configured")
	}
	if strings.TrimSpace(messageID) == "" {
		return fmt.Errorf("messageID cannot be empty")
	}
	if body == nil {
		return fmt.Errorf("message body cannot be nil")
	}

	req := &maxbot.EditMessageRequest{
		Text:   body.Text,
		Format: body.ParseMode,
	}
	if req.Format == "" {
		req.Format = "markdown"
	}

	if len(body.Buttons) > 0 {
		kb := buildKeyboard(body.Buttons)
		req.Attachments = append(req.Attachments, kb.ToAttachment())
	}

	_, err := a.client.EditMessage(ctx, messageID, req)
	return err
}

// AnswerCallback acknowledges a button click callback query.
func (a *Adapter) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	if a.client == nil {
		return fmt.Errorf("max client is not configured")
	}
	if strings.TrimSpace(callbackID) == "" {
		return fmt.Errorf("callbackID cannot be empty")
	}

	return a.client.AnswerCallback(ctx, callbackID, &maxbot.AnswerCallbackRequest{
		Notification: text,
	})
}

// CheckMembership checks if a user is a participant of the specified chat.
func (a *Adapter) CheckMembership(ctx context.Context, chatID string, userID string) (bool, error) {
	if a.client == nil {
		return false, fmt.Errorf("max client is not configured")
	}

	cID, err := parseNumericID(chatID)
	if err != nil {
		return false, fmt.Errorf("invalid chatID %q: %w", chatID, err)
	}
	uID, err := parseNumericID(userID)
	if err != nil {
		return false, fmt.Errorf("invalid userID %q: %w", userID, err)
	}

	// MAX Bot API GET /chats/{chat_id}/members supports pagination via marker.
	var marker int64
	for {
		resp, err := a.client.GetChatMembers(ctx, cID, marker)
		if err != nil {
			return false, err
		}
		for _, member := range resp.Members {
			if member.UserID == uID {
				return true, nil
			}
		}
		if resp.Marker == 0 || resp.Marker == marker || len(resp.Members) == 0 {
			break
		}
		marker = resp.Marker
	}

	return false, nil
}

// SendChatAction sends a presence action (e.g. ActionTypingOn) to a recipient in MAX.
func (a *Adapter) SendChatAction(ctx context.Context, recipientID string, action string) error {
	if a.client == nil {
		return fmt.Errorf("max client is not configured")
	}
	rec, err := parseRecipient(recipientID)
	if err != nil {
		return fmt.Errorf("failed to parse recipient ID %q: %w", recipientID, err)
	}
	if action == "" {
		action = maxbot.ActionTypingOn
	}
	return a.client.SendChatAction(ctx, rec.ID(), action)
}

// parseRecipient resolves string identifiers like "user_42", "user:42", "chat_100", "chat:100", "-100", "42".
func parseRecipient(recipientID string) (maxbot.Recipient, error) {
	trimmed := strings.TrimSpace(recipientID)
	if trimmed == "" {
		return nil, fmt.Errorf("recipient ID cannot be empty")
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "user_") || strings.HasPrefix(lower, "user:") {
		idStr := trimmed[5:]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user id in %q: %w", recipientID, err)
		}
		return maxbot.ToUser(id), nil
	}

	if strings.HasPrefix(lower, "chat_") || strings.HasPrefix(lower, "chat:") {
		idStr := trimmed[5:]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid chat id in %q: %w", recipientID, err)
		}
		return maxbot.ToChat(id), nil
	}

	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse recipient ID %q: %w", recipientID, err)
	}

	// In messenger conventions, negative IDs denote group chats/channels.
	if id < 0 {
		return maxbot.ToChat(id), nil
	}
	return maxbot.ToUser(id), nil
}

func parseNumericID(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "user_") || strings.HasPrefix(lower, "user:") ||
		strings.HasPrefix(lower, "chat_") || strings.HasPrefix(lower, "chat:") {
		trimmed = trimmed[5:]
	}
	return strconv.ParseInt(trimmed, 10, 64)
}

func buildKeyboard(matrix [][]platform.Button) *maxbot.InlineKeyboard {
	kb := maxbot.NewInlineKeyboard()
	for _, row := range matrix {
		var rowBtns []maxbot.Button
		for _, b := range row {
			switch b.EffectiveType() {
			case platform.ButtonTypeWebApp:
				rowBtns = append(rowBtns, maxbot.NewOpenAppButton(b.Text, b.WebApp))
			case platform.ButtonTypeURL:
				rowBtns = append(rowBtns, maxbot.NewLinkButton(b.Text, b.URL))
			default:
				rowBtns = append(rowBtns, maxbot.NewCallbackButton(b.Text, b.CallbackData()))
			}
		}
		if len(rowBtns) > 0 {
			kb.Row(rowBtns...)
		}
	}
	return kb
}
