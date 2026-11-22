package ai

import (
	"encoding/base64"
	"fmt"
	"time"
)

// Role constants for ChatMessage.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// ContentPartType constants for multimodal messages.
const (
	ContentTypeText     = "text"
	ContentTypeImageURL = "image_url"
)

// ImageURL holds an image location or data URI.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", "auto"
}

// ChatContentPart represents a multimodal part in user or system message content.
type ChatContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ChatMessage represents a single message turn in a chat completion request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []ChatContentPart
}

// TextContent returns the text portion of Content if available.
func (m ChatMessage) TextContent() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []ChatContentPart:
		for _, part := range v {
			if part.Type == ContentTypeText && part.Text != "" {
				return part.Text
			}
		}
	}
	return ""
}

// NewTextMessage creates a text-only chat message.
func NewTextMessage(role, text string) ChatMessage {
	return ChatMessage{
		Role:    role,
		Content: text,
	}
}

// NewMultimodalMessage creates a user message containing text and an image URL or data URI.
func NewMultimodalMessage(text, imageURL string) ChatMessage {
	parts := make([]ChatContentPart, 0, 2)
	if text != "" {
		parts = append(parts, ChatContentPart{
			Type: ContentTypeText,
			Text: text,
		})
	}
	if imageURL != "" {
		parts = append(parts, ChatContentPart{
			Type: ContentTypeImageURL,
			ImageURL: &ImageURL{
				URL: imageURL,
			},
		})
	}
	return ChatMessage{
		Role:    RoleUser,
		Content: parts,
	}
}

// NewMultimodalMessageWithBytes creates a user message containing text and inlined base64 image data.
func NewMultimodalMessageWithBytes(text string, mimeType string, imgBytes []byte) ChatMessage {
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgBytes))
	return NewMultimodalMessage(text, dataURI)
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

// VisionRequest represents a request to analyze or recognize an image.
type VisionRequest struct {
	Model       string   `json:"model,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
	ImageBytes  []byte   `json:"-"`
	MimeType    string   `json:"-"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
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
