package store

import "time"

// Channel represents metadata and permission status for a MAX channel or group.
type Channel struct {
	ChatID      int64     `json:"chat_id"`
	Title       string    `json:"title"`
	Type        string    `json:"type"` // "channel", "chat", "dialog"
	IsAdmin     bool      `json:"is_admin"`
	Permissions []string  `json:"permissions"`
	LastUpdated time.Time `json:"last_updated"`
}

// OrderItem represents a line item in a customer order.
type OrderItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Price    int    `json:"price"`
	Quantity int    `json:"quantity"`
}

// Order represents an e-commerce order submitted through the Mini App.
type Order struct {
	ID           string      `json:"id"`
	UserID       int64       `json:"user_id"`
	UserName     string      `json:"user_name"`
	Items        []OrderItem `json:"items"`
	Total        int         `json:"total"`
	DeliveryType string      `json:"delivery_type"`
	Address      string      `json:"address,omitempty"`
	Status       string      `json:"status"` // "accepted", "processing", "delivered", "cancelled"
	Verified     bool        `json:"verified"`
	CreatedAt    time.Time   `json:"created_at"`
}

// Reaction represents interactive emoji reaction telemetry on a channel post.
type Reaction struct {
	PostMID   string           `json:"post_mid"`
	ChatID    int64            `json:"chat_id"`
	Counts    map[string]int   `json:"counts"`     // emoji -> total count
	UserVotes map[int64]string `json:"user_votes"` // userID -> emoji
}

// Poll represents an interactive poll published into a channel or chat.
type Poll struct {
	ID         string         `json:"id"`
	ChatID     int64          `json:"chat_id"`
	MessageMID string         `json:"message_mid,omitempty"`
	Question   string         `json:"question"`
	Options    []string       `json:"options"`
	Votes      map[int64]int  `json:"votes"` // userID -> optionIdx
	CreatedAt  time.Time      `json:"created_at"`
}

// Giveaway represents a channel contest / giveaway with subscription check.
type Giveaway struct {
	ID              string           `json:"id"`
	ChatID          int64            `json:"chat_id"`
	Title           string           `json:"title"`
	PromoCode       string           `json:"promo_code"`
	DiscountPercent int              `json:"discount_percent"`
	Participants    map[int64]string `json:"participants"` // userID -> userName
	Winners         []int64          `json:"winners,omitempty"`
	Active          bool             `json:"active"`
	CreatedAt       time.Time        `json:"created_at"`
}

// MediaArtifact represents a persistent multimodal artifact (AI generated image, STT voice transcript, document analysis).
type MediaArtifact struct {
	ID           int64          `json:"id"`
	UserID       int64          `json:"user_id"`
	ArtifactType string         `json:"artifact_type"` // "image", "stt", "doc"
	Title        string         `json:"title"`
	Prompt       string         `json:"prompt,omitempty"`
	ContentText  string         `json:"content_text,omitempty"`
	MediaURL     string         `json:"media_url,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}
