package maxbot

import (
	"fmt"
	"net/url"
	"strconv"
)

// Recipient specifies where an outbound message or action should be routed.
// MAX Bot API differentiates between 1-on-1 private user dialogs (?user_id=...)
// and group chats / channels (?chat_id=...).
type Recipient interface {
	ApplyRecipient(values url.Values)
	String() string
	ID() int64
	IsUser() bool
}

type userRecipient struct {
	userID int64
}

// ToUser returns a Recipient targeting a private 1-on-1 dialog with a user.
func ToUser(userID int64) Recipient {
	return userRecipient{userID: userID}
}

func (u userRecipient) ApplyRecipient(values url.Values) {
	values.Set("user_id", strconv.FormatInt(u.userID, 10))
}

func (u userRecipient) String() string {
	return fmt.Sprintf("user:%d", u.userID)
}

func (u userRecipient) ID() int64 {
	return u.userID
}

func (u userRecipient) IsUser() bool {
	return true
}

type chatRecipient struct {
	chatID int64
}

// ToChat returns a Recipient targeting a group chat or channel.
func ToChat(chatID int64) Recipient {
	return chatRecipient{chatID: chatID}
}

func (c chatRecipient) ApplyRecipient(values url.Values) {
	values.Set("chat_id", strconv.FormatInt(c.chatID, 10))
}

func (c chatRecipient) String() string {
	return fmt.Sprintf("chat:%d", c.chatID)
}

func (c chatRecipient) ID() int64 {
	return c.chatID
}

func (c chatRecipient) IsUser() bool {
	return false
}

// AsRecipient returns a Recipient targeting this user.
func (u *User) AsRecipient() Recipient {
	return ToUser(u.UserID)
}

// AsRecipient returns a Recipient targeting this chat.
func (c *Chat) AsRecipient() Recipient {
	return ToChat(c.ChatID)
}
