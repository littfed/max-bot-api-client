package store

import (
	"encoding/json"
	"time"
)

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

// User represents a Bot Hub user profile across platforms.
type User struct {
	ID           int64     `json:"id"`
	ActiveModeID string    `json:"active_mode_id"`
	Role         string    `json:"role"`
	Timezone     string    `json:"timezone"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserIdentity represents a platform-specific identifier linked to a User.
type UserIdentity struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Platform       string    `json:"platform"`
	PlatformUserID string    `json:"platform_user_id"`
	Username       string    `json:"username,omitempty"`
	FirstName      string    `json:"first_name,omitempty"`
	Phone          string    `json:"phone,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// UserModeState represents the composite FSM state for a user within a specific mode.
type UserModeState struct {
	UserID      int64           `json:"user_id"`
	ModeID      string          `json:"mode_id"`
	CurrentStep string          `json:"current_step"`
	Payload     json.RawMessage `json:"payload"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ModeConfig represents the configuration and dynamic maintenance switch for a Bot Hub mode.
type ModeConfig struct {
	ModeID             string    `json:"mode_id"`
	Title              string    `json:"title"`
	IsActive           bool      `json:"is_active"`
	MaintenanceMessage string    `json:"maintenance_message"`
	DailyQuota         int       `json:"daily_quota"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Lead represents a potential customer lead captured via the "Want such a bot" button.
type Lead struct {
	ID          int64           `json:"id"`
	UserID      int64           `json:"user_id"`
	Platform    string          `json:"platform"`
	SourceMode  string          `json:"source_mode"`
	TestedModes json.RawMessage `json:"tested_modes"`
	Contact     string          `json:"contact"`
	Comment     string          `json:"comment"`
	Status      string          `json:"status"` // 'new', 'in_progress', 'closed'
	CreatedAt   time.Time       `json:"created_at"`
}

// MascotPhrase represents an interactive phrase uttered by the central mascot.
type MascotPhrase struct {
	ID          int64  `json:"id"`
	Priority    int    `json:"priority"` // 1: urgent, 2: progress, 3: time, 4: tap
	TriggerType string `json:"trigger_type"`
	Text        string `json:"text"`
	IsActive    bool   `json:"is_active"`
}

// Product represents an item or subscription in the Shop catalog.
type Product struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Price          int    `json:"price"`
	Category       string `json:"category"`
	ItemType       string `json:"item_type"` // "digital", "subscription"
	ContentPayload string `json:"content_payload"`
	IsActive       bool   `json:"is_active"`
}

// Service represents a bookable service in the Reception module.
type Service struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	DurationMinutes int    `json:"duration_minutes"`
	Price           int    `json:"price"`
	Description     string `json:"description"`
}

// TimeSlot represents a time slot for a service booking.
type TimeSlot struct {
	ID             string    `json:"id"`
	ServiceID      string    `json:"service_id"`
	SlotTime       time.Time `json:"slot_time"`
	IsBooked       bool      `json:"is_booked"`
	BookedByUserID *int64    `json:"booked_by_user_id,omitempty"`
}

// Booking represents a confirmed service booking.
type Booking struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	ServiceID string    `json:"service_id"`
	SlotTime  time.Time `json:"slot_time"`
	Status    string    `json:"status"` // "confirmed", "cancelled"
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Event represents an event in the Reception module.
type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	EventTime   time.Time `json:"event_time"`
	TotalSeats  int       `json:"total_seats"`
	BookedSeats int       `json:"booked_seats"`
}

// EventRegistration represents a user's registration for an event.
type EventRegistration struct {
	ID           string    `json:"id"`
	EventID      string    `json:"event_id"`
	UserID       int64     `json:"user_id"`
	TicketQR     string    `json:"ticket_qr"`
	RegisteredAt time.Time `json:"registered_at"`
}

// OnboardingSubmission represents a user's completed questionnaire.
type OnboardingSubmission struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"user_id"`
	Answers   json.RawMessage `json:"answers"`
	CreatedAt time.Time       `json:"created_at"`
}

// KnowledgeChunk represents a chunk in the knowledge base for RAG.
type KnowledgeChunk struct {
	ID        int64  `json:"id"`
	DocID     int64  `json:"doc_id"`
	ModeScope string `json:"mode_scope"`
	ChunkText string `json:"chunk_text"`
}

// SupportTicket represents a customer support ticket.
type SupportTicket struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Platform  string    `json:"platform"`
	ModeScope string    `json:"mode_scope"`
	Subject   string    `json:"subject"`
	Status    string    `json:"status"` // "open", "answered", "closed"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SupportMessage represents a message within a support ticket.
type SupportMessage struct {
	ID          int64     `json:"id"`
	TicketID    int64     `json:"ticket_id"`
	SenderType  string    `json:"sender_type"` // "user", "operator", "bot"
	MessageText string    `json:"message_text"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatSession represents an LLM conversation session with history.
type ChatSession struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"user_id"`
	RoleID    int             `json:"role_id"`
	ToneID    int             `json:"tone_id"`
	Messages  json.RawMessage `json:"messages"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// AdminAuditLog represents an audit log entry for actions performed in the admin panel.
type AdminAuditLog struct {
	ID          int64           `json:"id"`
	AdminID     int64           `json:"admin_id"`
	Action      string          `json:"action"`
	TargetMode  string          `json:"target_mode,omitempty"`
	PayloadDiff json.RawMessage `json:"payload_diff,omitempty"`
	IP          string          `json:"ip,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}


