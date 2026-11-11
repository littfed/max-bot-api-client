package payments

import (
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusSucceeded PaymentStatus = "succeeded"
	StatusCanceled  PaymentStatus = "canceled"
	StatusRefunded  PaymentStatus = "refunded"
)

// Payment represents a normalized transaction across payment providers.
type Payment struct {
	ID          string            `json:"id"`
	Gateway     string            `json:"gateway"`
	Amount      float64           `json:"amount"`
	Currency    string            `json:"currency"`
	Status      PaymentStatus     `json:"status"`
	UserID      int64             `json:"user_id"`
	ChatID      int64             `json:"chat_id"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
}

// PaymentButton creates an attractive payment link button with positive intent for inline keyboards.
func PaymentButton(text string, checkoutURL string) maxbot.Button {
	return maxbot.Button{
		Type:   maxbot.ButtonTypeLink,
		Text:   text,
		URL:    checkoutURL,
		Intent: maxbot.IntentPositive,
	}
}
