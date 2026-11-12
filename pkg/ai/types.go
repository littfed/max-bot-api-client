package ai

import (
	"time"
)

// Role constants for ChatMessage.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// ChatMessage represents a single message turn in a chat completion request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the payload sent to /chat/completions.
type ChatRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

// ChatResponse represents the result of a chat completion query.
type ChatResponse struct {
	ID               string        `json:"id"`
	Model            string        `json:"model"`
	Content          string        `json:"content"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalTokens      int           `json:"total_tokens"`
	Latency          time.Duration `json:"latency"`
}

// ImageRequest represents the payload sent to /images/generations.
type ImageRequest struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model,omitempty"`
	Size           string `json:"size,omitempty"` // e.g. "512x512", "1024x1024"
	ResponseFormat string `json:"response_format,omitempty"` // "b64_json" or "url"
}

// ImageResponse represents the generated image bytes and metadata.
type ImageResponse struct {
	Data    []byte        `json:"data"`
	URL     string        `json:"url,omitempty"`
	Model   string        `json:"model"`
	Latency time.Duration `json:"latency"`
}

// AudioRequest represents an audio transcription (STT) request.
type AudioRequest struct {
	AudioBytes []byte `json:"-"`
	Filename   string `json:"filename,omitempty"`
	Model      string `json:"model,omitempty"`
	Language   string `json:"language,omitempty"`
}

// AudioResponse represents the transcribed text result.
type AudioResponse struct {
	Text    string        `json:"text"`
	Model   string        `json:"model"`
	Latency time.Duration `json:"latency"`
}
