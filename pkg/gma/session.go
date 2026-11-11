package gma

import (
	"sync"
	"time"
)

// DefaultSessionTTL is how long an idle chat session is kept before resetting.
const DefaultSessionTTL = 30 * time.Minute

// DefaultMaxHistory is the maximum number of recent messages kept per user.
const DefaultMaxHistory = 12

// UserSession stores conversation context and user-specific model preferences.
type UserSession struct {
	UserID      int64
	Messages    []ChatMessage
	ChatModel   string
	VisionModel string
	ImageModel  string
	UpdatedAt   time.Time
}

// SessionManager manages thread-safe user conversation histories with ring-buffer semantics and TTL expiration.
type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[int64]*UserSession
	maxMessages int
	ttl         time.Duration
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(maxMessages int, ttl time.Duration) *SessionManager {
	if maxMessages <= 0 {
		maxMessages = DefaultMaxHistory
	}
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionManager{
		sessions:    make(map[int64]*UserSession),
		maxMessages: maxMessages,
		ttl:         ttl,
	}
}

// getOrCreate returns the session for userID, resetting it if expired.
// Must be called with lock held.
func (sm *SessionManager) getOrCreate(userID int64) *UserSession {
	s, ok := sm.sessions[userID]
	now := time.Now()
	if !ok {
		s = &UserSession{
			UserID:      userID,
			Messages:    make([]ChatMessage, 0, sm.maxMessages),
			ChatModel:   DefaultChatModel,
			VisionModel: DefaultVisionModel,
			ImageModel:  DefaultImageModel,
			UpdatedAt:   now,
		}
		sm.sessions[userID] = s
		return s
	}

	if sm.ttl > 0 && now.Sub(s.UpdatedAt) > sm.ttl {
		// Session expired - clear messages but keep preferred models
		s.Messages = make([]ChatMessage, 0, sm.maxMessages)
	}
	s.UpdatedAt = now
	return s
}

// AddMessage appends a message to the user's history, trimming the oldest if exceeding maxMessages.
func (sm *SessionManager) AddMessage(userID int64, msg ChatMessage) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s := sm.getOrCreate(userID)
	s.Messages = append(s.Messages, msg)
	if len(s.Messages) > sm.maxMessages {
		// Ring-buffer sliding window: drop the oldest message
		s.Messages = s.Messages[len(s.Messages)-sm.maxMessages:]
	}
}

// AddUserMessage adds a user message to the user's history.
func (sm *SessionManager) AddUserMessage(userID int64, text string) {
	sm.AddMessage(userID, NewTextMessage(RoleUser, text))
}

// AddAssistantMessage adds an assistant message to the user's history.
func (sm *SessionManager) AddAssistantMessage(userID int64, text string) {
	sm.AddMessage(userID, NewTextMessage(RoleAssistant, text))
}

// GetHistory returns a copy of the conversation history for userID.
func (sm *SessionManager) GetHistory(userID int64) []ChatMessage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.sessions[userID]
	if !ok {
		return nil
	}

	if sm.ttl > 0 && time.Since(s.UpdatedAt) > sm.ttl {
		return nil
	}

	history := make([]ChatMessage, len(s.Messages))
	copy(history, s.Messages)
	return history
}

// Clear wipes the conversation history for userID.
func (sm *SessionManager) Clear(userID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.sessions[userID]; ok {
		s.Messages = make([]ChatMessage, 0, sm.maxMessages)
		s.UpdatedAt = time.Now()
	}
}

// GetModels returns the active chat, vision, and image models for userID.
func (sm *SessionManager) GetModels(userID int64) (chatModel, visionModel, imageModel string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.sessions[userID]
	if !ok {
		return DefaultChatModel, DefaultVisionModel, DefaultImageModel
	}

	cm := s.ChatModel
	if cm == "" {
		cm = DefaultChatModel
	}
	vm := s.VisionModel
	if vm == "" {
		vm = DefaultVisionModel
	}
	im := s.ImageModel
	if im == "" {
		im = DefaultImageModel
	}
	return cm, vm, im
}

// SetChatModel updates the active chat model for userID.
func (sm *SessionManager) SetChatModel(userID int64, model string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s := sm.getOrCreate(userID)
	s.ChatModel = model
}

// SetVisionModel updates the active vision model for userID.
func (sm *SessionManager) SetVisionModel(userID int64, model string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s := sm.getOrCreate(userID)
	s.VisionModel = model
}

// SetImageModel updates the active image model for userID.
func (sm *SessionManager) SetImageModel(userID int64, model string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s := sm.getOrCreate(userID)
	s.ImageModel = model
}
