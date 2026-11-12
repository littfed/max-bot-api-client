package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// Context wraps an incoming event Update with helper methods for replying and routing.
type Context struct {
	ctx    context.Context
	client *maxbot.Client
	Update *maxbot.Update
	data   map[string]any
}

// NewContext creates a new handler Context.
func NewContext(ctx context.Context, client *maxbot.Client, update *maxbot.Update) *Context {
	return &Context{
		ctx:    ctx,
		client: client,
		Update: update,
		data:   make(map[string]any),
	}
}

// Set stores a custom key-value pair in the request context.
func (c *Context) Set(key string, val any) {
	if c.data == nil {
		c.data = make(map[string]any)
	}
	c.data[key] = val
}

// Get retrieves a key-value pair from the request context.
func (c *Context) Get(key string) (any, bool) {
	if c.data == nil {
		return nil, false
	}
	val, ok := c.data[key]
	return val, ok
}

// Ctx returns the underlying context.Context.
func (c *Context) Ctx() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// Client returns the active maxbot.Client.
func (c *Context) Client() *maxbot.Client {
	return c.client
}

// Message returns the associated Message, or nil if none.
func (c *Context) Message() *maxbot.Message {
	if c.Update == nil {
		return nil
	}
	if c.Update.Message != nil {
		return c.Update.Message
	}
	if c.Update.Callback != nil && c.Update.Callback.Message != nil {
		return c.Update.Callback.Message
	}
	return nil
}

// Callback returns the associated Callback query, or nil if none.
func (c *Context) Callback() *maxbot.Callback {
	if c.Update == nil {
		return nil
	}
	return c.Update.Callback
}

// Text returns the incoming message text, or empty string.
func (c *Context) Text() string {
	msg := c.Message()
	if msg == nil {
		return ""
	}
	return msg.Body.Text
}

// Sender returns the User who triggered the update.
func (c *Context) Sender() *maxbot.User {
	if c.Update == nil {
		return nil
	}
	if c.Update.User != nil {
		return c.Update.User
	}
	if c.Update.Callback != nil && c.Update.Callback.User != nil {
		return c.Update.Callback.User
	}
	if c.Update.Message != nil && c.Update.Message.Sender != nil {
		return c.Update.Message.Sender
	}
	return nil
}

// Recipient automatically derives the target recipient to reply to.
// In group chats/channels, it targets the chat ID. In 1-on-1 dialogs, it targets the user ID.
func (c *Context) Recipient() (maxbot.Recipient, error) {
	if c.Update == nil {
		return nil, fmt.Errorf("update is nil")
	}

	// 1. If explicit chat_id is present at update level
	if c.Update.ChatID != 0 {
		return maxbot.ToChat(c.Update.ChatID), nil
	}

	// 2. From message recipient details
	if msg := c.Message(); msg != nil && msg.Recipient != nil {
		if msg.Recipient.ChatID != 0 {
			return maxbot.ToChat(msg.Recipient.ChatID), nil
		}
		if msg.Recipient.UserID != 0 {
			return maxbot.ToUser(msg.Recipient.UserID), nil
		}
	}

	// 3. Fallback to sender user_id
	if sender := c.Sender(); sender != nil && sender.UserID != 0 {
		return maxbot.ToUser(sender.UserID), nil
	}

	return nil, fmt.Errorf("cannot deduce recipient from update")
}

// Reply sends a text message back to the conversation sender (markdown formatted).
func (c *Context) Reply(text string) (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	rec, err := c.Recipient()
	if err != nil {
		return nil, err
	}
	return c.client.SendTextMessage(c.Ctx(), rec, text)
}

// ReplyKeyboard sends a text message with an inline keyboard back to the conversation.
func (c *Context) ReplyKeyboard(text string, kb *maxbot.InlineKeyboard) (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	rec, err := c.Recipient()
	if err != nil {
		return nil, err
	}
	return c.client.SendKeyboardMessage(c.Ctx(), rec, text, kb)
}

// ReplyWithAttachments sends a text message with custom attachments (e.g. media, keyboards).
func (c *Context) ReplyWithAttachments(text string, attachments ...maxbot.Attachment) (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	rec, err := c.Recipient()
	if err != nil {
		return nil, err
	}
	msg := &maxbot.SendMessageRequest{
		Text:        text,
		Format:      "markdown",
		Attachments: attachments,
	}
	return c.client.SendMessage(c.Ctx(), rec, msg)
}

// AnswerCallback acknowledges an incoming button callback.
func (c *Context) AnswerCallback(answer *maxbot.AnswerCallbackRequest) error {
	if c.client == nil {
		return nil
	}
	cb := c.Callback()
	if cb == nil {
		return fmt.Errorf("no callback in update")
	}
	return c.client.AnswerCallback(c.Ctx(), cb.CallbackID, answer)
}

// AnswerNotification shows an ephemeral toast notification in response to a callback.
func (c *Context) AnswerNotification(notification string) error {
	return c.AnswerCallback(&maxbot.AnswerCallbackRequest{
		Notification: notification,
	})
}

// SendChatAction sends a presence indicator (e.g. typing_on) to the active chat.
func (c *Context) SendChatAction(action string) error {
	if c.client == nil {
		return nil
	}
	rec, err := c.Recipient()
	if err != nil {
		return err
	}
	return c.client.SendChatAction(c.Ctx(), rec.ID(), action)
}

// Typing sends the "typing_on" action to display the typing indicator.
func (c *Context) Typing() error {
	return c.SendChatAction(maxbot.ActionTypingOn)
}

// Command parses the message text as a command (e.g. "/ping arg1 arg2").
// Returns command name without slash ("ping") and remaining arguments ("arg1 arg2").
// If the message is not a command, returns empty strings.
func (c *Context) Command() (string, string) {
	text := strings.TrimSpace(c.Text())
	if !strings.HasPrefix(text, "/") {
		return "", ""
	}

	parts := strings.SplitN(text[1:], " ", 2)
	cmd := parts[0]
	// Strip bot mention if command is in format /ping@my_bot
	if atIdx := strings.Index(cmd, "@"); atIdx != -1 {
		cmd = cmd[:atIdx]
	}

	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return strings.ToLower(cmd), args
}

// IsCommand returns true if the incoming message is a slash command.
func (c *Context) IsCommand() bool {
	cmd, _ := c.Command()
	return cmd != ""
}

// StartPayload extracts the start parameter from bot_started or "/start <payload>".
func (c *Context) StartPayload() string {
	if c.Update != nil && c.Update.Payload != "" {
		return c.Update.Payload
	}
	cmd, args := c.Command()
	if cmd == "start" {
		return args
	}
	return ""
}
