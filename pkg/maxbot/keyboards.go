package maxbot

import (
	"encoding/json"
	"fmt"
)

// Button types supported by MAX Messenger inline keyboards.
const (
	ButtonTypeCallback        = "callback"
	ButtonTypeLink            = "link"
	ButtonTypeOpenApp         = "open_app"
	ButtonTypeClipboard       = "clipboard"
	ButtonTypeRequestContact  = "request_contact"
	ButtonTypeRequestLocation = "request_geo_location"
	ButtonTypeMessage         = "message"
)

// Button visual intent color styles.
const (
	IntentDefault  = "default"
	IntentPositive = "positive" // Green
	IntentNegative = "negative" // Red
	IntentPrimary  = "primary"  // Blue / Accent
)

// Button represents an interactive element in an InlineKeyboard.
type Button struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Payload   string `json:"payload,omitempty"`
	URL       string `json:"url,omitempty"`
	WebApp    string `json:"webApp,omitempty"`
	Data      string `json:"data,omitempty"`
	Intent    string `json:"intent,omitempty"`
	Quick     bool   `json:"quick,omitempty"`
	ContactID int64  `json:"contactId,omitempty"`
}

// WithIntent sets the visual color style of the button (positive, negative, primary, default).
func (b Button) WithIntent(intent string) Button {
	b.Intent = intent
	return b
}

// NewCallbackButton creates a button that sends an event to the bot when clicked.
func NewCallbackButton(text, payload string) Button {
	return Button{
		Type:    ButtonTypeCallback,
		Text:    text,
		Payload: payload,
	}
}

// NewIntentCallbackButton creates a callback button with an explicit visual color intent.
func NewIntentCallbackButton(text, payload, intent string) Button {
	return Button{
		Type:    ButtonTypeCallback,
		Text:    text,
		Payload: payload,
		Intent:  intent,
	}
}

// NewLinkButton creates a button that opens an external URL.
func NewLinkButton(text, url string) Button {
	return Button{
		Type: ButtonTypeLink,
		Text: text,
		URL:  url,
	}
}

// NewOpenAppButton creates a button that launches a MAX Mini App in an embedded WebView.
func NewOpenAppButton(text, webAppURL string) Button {
	return Button{
		Type:   ButtonTypeOpenApp,
		Text:   text,
		WebApp: webAppURL,
	}
}

// NewClipboardButton creates a button that copies text into the user's OS clipboard.
// Note: MAX Bot API requires the text to copy to be in the 'payload' field.
func NewClipboardButton(text, textToCopy string) Button {
	return Button{
		Type:    ButtonTypeClipboard,
		Text:    text,
		Payload: textToCopy,
		Data:    textToCopy,
	}
}

// NewContactButton creates a button requesting the user's verified phone number.
func NewContactButton(text string) Button {
	return Button{
		Type: ButtonTypeRequestContact,
		Text: text,
	}
}

// NewLocationButton creates a button requesting the user's current GPS coordinates.
func NewLocationButton(text string, quick bool) Button {
	return Button{
		Type:  ButtonTypeRequestLocation,
		Text:  text,
		Quick: quick,
	}
}

// InlineKeyboard represents an interactive keyboard matrix.
type InlineKeyboard struct {
	rows [][]Button
}

// NewInlineKeyboard constructs a new empty InlineKeyboard builder.
func NewInlineKeyboard() *InlineKeyboard {
	return &InlineKeyboard{
		rows: make([][]Button, 0),
	}
}

// Row adds a row of buttons to the keyboard.
func (ik *InlineKeyboard) Row(buttons ...Button) *InlineKeyboard {
	if len(buttons) > 0 {
		ik.rows = append(ik.rows, buttons)
	}
	return ik
}

// Rows returns the raw button matrix.
func (ik *InlineKeyboard) Rows() [][]Button {
	return ik.rows
}

// Payload represents the JSON payload of an inline_keyboard attachment.
type KeyboardPayload struct {
	Buttons [][]Button `json:"buttons"`
}

// ToAttachment converts the InlineKeyboard into a MAX Attachment.
func (ik *InlineKeyboard) ToAttachment() Attachment {
	return Attachment{
		Type: "inline_keyboard",
		Payload: KeyboardPayload{
			Buttons: ik.rows,
		},
	}
}

// MarshalJSON marshals the keyboard directly into its attachment JSON format.
func (ik *InlineKeyboard) MarshalJSON() ([]byte, error) {
	return json.Marshal(ik.ToAttachment())
}

// ParseKeyboardPayload decodes an attachment payload into a KeyboardPayload struct.
func ParseKeyboardPayload(att Attachment) (*KeyboardPayload, error) {
	if att.Type != "inline_keyboard" {
		return nil, fmt.Errorf("expected attachment type 'inline_keyboard', got '%s'", att.Type)
	}
	return DecodeAttachmentPayload[KeyboardPayload](att)
}
