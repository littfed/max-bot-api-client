package maxbot

import "encoding/json"

// Standard update types dispatched by MAX Bot API.
const (
	UpdateTypeMessageCreated     = "message_created"
	UpdateTypeMessageCallback    = "message_callback"
	UpdateTypeMessageEdited      = "message_edited"
	UpdateTypeMessageRemoved     = "message_removed"
	UpdateTypeBotStarted         = "bot_started"
	UpdateTypeBotStopped         = "bot_stopped"
	UpdateTypeBotAdded           = "bot_added"
	UpdateTypeBotRemoved         = "bot_removed"
	UpdateTypeUserAdded          = "user_added"
	UpdateTypeUserRemoved        = "user_removed"
	UpdateTypeChatTitleChanged   = "chat_title_changed"
)

// Supported presence chat action types for POST /chats/{chatId}/actions.
const (
	ActionTypingOn     = "typing_on"
	ActionSendingPhoto = "sending_photo"
	ActionSendingVideo = "sending_video"
	ActionSendingAudio = "sending_audio"
	ActionSendingFile  = "sending_file"
)

// Supported media types for POST /uploads?type={type}.
const (
	UploadTypeImage = "image"
	UploadTypeVideo = "video"
	UploadTypeAudio = "audio"
	UploadTypeFile  = "file"
)

// ChatActionRequest represents the payload for POST /chats/{chatId}/actions.
type ChatActionRequest struct {
	Action string `json:"action"`
}

// PinMessageRequest represents the payload for PUT /chats/{chatId}/pin.
type PinMessageRequest struct {
	MessageID string `json:"message_id"`
	Notify    bool   `json:"notify,omitempty"`
}

// User represents a user or bot in MAX.
type User struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Name      string `json:"name,omitempty"`
	Username  string `json:"username,omitempty"`
	IsBot     bool   `json:"is_bot,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Chat represents a dialogue, group chat, or channel in MAX.
type Chat struct {
	ChatID      int64  `json:"chat_id"`
	Type        string `json:"type"` // "dialog", "chat", "channel"
	Status      string `json:"status,omitempty"`
	Title       string `json:"title,omitempty"`
	Icon        string `json:"icon,omitempty"`
	LastEventTime int64 `json:"last_event_time,omitempty"`
	ParticipantsCount int `json:"participants_count,omitempty"`
	Description string `json:"description,omitempty"`
}

// Message represents a message entity in MAX.
type Message struct {
	MessageID string            `json:"message_id,omitempty"`
	Timestamp int64             `json:"timestamp"`
	Sender    *User             `json:"sender,omitempty"`
	Recipient *MessageRecipient `json:"recipient,omitempty"`
	Body      MessageBody       `json:"body"`
}

// ID returns the canonical unique message identifier (mid from body or message_id).
func (m *Message) ID() string {
	if m == nil {
		return ""
	}
	if m.Body.MID != "" {
		return m.Body.MID
	}
	return m.MessageID
}

// MessageResponse represents the top-level envelope returned by POST and PUT /messages.
type MessageResponse struct {
	Message Message `json:"message"`
}

// BotCommand represents a bot command item for the command suggestion menu.
type BotCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ChatMembersResponse represents the response returned by GET /chats/{chat_id}/members.
type ChatMembersResponse struct {
	Members []ChatMember `json:"members"`
	Marker  int64        `json:"marker,omitempty"`
}

// ChatMember represents a participant in a chat or channel.
type ChatMember struct {
	UserID          int64  `json:"user_id"`
	FirstName       string `json:"first_name,omitempty"`
	LastName        string `json:"last_name,omitempty"`
	Name            string `json:"name,omitempty"`
	Username        string `json:"username,omitempty"`
	IsBot           bool   `json:"is_bot,omitempty"`
	AvatarURL       string `json:"avatar_url,omitempty"`
	IsAdmin         bool   `json:"is_admin,omitempty"`
	CanSendMessages bool   `json:"can_send_messages,omitempty"`
}

// SetCommandsRequest represents the payload for PATCH /me/commands.
type SetCommandsRequest struct {
	Commands []BotCommand `json:"commands"`
}

// MessageRecipient represents the recipient details within a Message.
type MessageRecipient struct {
	ChatID   int64  `json:"chat_id,omitempty"`
	ChatType string `json:"chat_type,omitempty"`
	UserID   int64  `json:"user_id,omitempty"`
}

// MessageBody represents the content and attachments of a message.
type MessageBody struct {
	MID         string       `json:"mid,omitempty"`
	Seq         int64        `json:"seq,omitempty"`
	Text        string       `json:"text"`
	Format      string       `json:"format,omitempty"` // "markdown" or "html"
	Attachments []Attachment `json:"attachments,omitempty"`
	Link        *MessageLink `json:"link,omitempty"`
}

// MessageLink represents a forwarded or replied-to message link.
type MessageLink struct {
	Type    string   `json:"type"` // "forward", "reply"
	Message *Message `json:"message,omitempty"`
}

// Attachment represents an interactive or media attachment.
type Attachment struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Callback represents an interactive inline button click event.
type Callback struct {
	CallbackID string   `json:"callback_id"`
	Payload    string   `json:"payload"`
	User       *User    `json:"user"`
	Message    *Message `json:"message,omitempty"`
	Timestamp  int64    `json:"timestamp"`
}

// Update represents an event received via Long Polling or Webhook.
type Update struct {
	UpdateType string    `json:"update_type"`
	Timestamp  int64     `json:"timestamp"`
	Message    *Message  `json:"message,omitempty"`
	Callback   *Callback `json:"callback,omitempty"`
	ChatID     int64     `json:"chat_id,omitempty"`
	User       *User     `json:"user,omitempty"`
	Payload    string    `json:"payload,omitempty"` // For bot_started
	Marker     int64     `json:"marker,omitempty"`
}

// UpdatesResponse is the container returned by GET /updates.
type UpdatesResponse struct {
	Updates []Update `json:"updates"`
	Marker  int64    `json:"marker"`
}

// SendMessageRequest represents the payload for POST /messages.
type SendMessageRequest struct {
	Text        string       `json:"text"`
	Format      string       `json:"format,omitempty"` // "markdown" or "html"
	Attachments []Attachment `json:"attachments,omitempty"`
	Link        *MessageLink `json:"link,omitempty"`
	Notify      *bool        `json:"notify,omitempty"`
}

// EditMessageRequest represents the payload for PUT /messages.
type EditMessageRequest struct {
	Text        string       `json:"text,omitempty"`
	Format      string       `json:"format,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Link        *MessageLink `json:"link,omitempty"`
}

// AnswerCallbackRequest represents the payload for POST /answers.
type AnswerCallbackRequest struct {
	Notification string       `json:"notification,omitempty"`
	Message      *MessageBody `json:"message,omitempty"`
}

// Subscription represents an active webhook registration.
type Subscription struct {
	URL    string `json:"url"`
	Secret string `json:"secret,omitempty"`
	Time   int64  `json:"time,omitempty"`
}

// SetSubscriptionRequest represents the payload for POST /subscriptions.
type SetSubscriptionRequest struct {
	URL        string   `json:"url"`
	Secret     string   `json:"secret,omitempty"`
	EventTypes []string `json:"event_types,omitempty"`
}

// UploadURLResponse represents the response from POST /uploads?type=...
type UploadURLResponse struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// GetUpdatesOptions contains query parameters for GET /updates.
type GetUpdatesOptions struct {
	Marker  int64    `json:"marker,omitempty"`
	Limit   int      `json:"limit,omitempty"`   // 1 to 1000
	Timeout int      `json:"timeout,omitempty"` // 0 to 90 seconds
	Types   []string `json:"types,omitempty"`
}

// Helper to deserialize an Attachment Payload into a typed struct.
func DecodeAttachmentPayload[T any](att Attachment) (*T, error) {
	data, err := json.Marshal(att.Payload)
	if err != nil {
		return nil, err
	}
	var target T
	if err := json.Unmarshal(data, &target); err != nil {
		return nil, err
	}
	return &target, nil
}
