package gma

// Role constants for OpenAI-compatible chat messages.
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

// ChatMessage represents a single turn in a chat conversation.
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

// NewMultimodalMessage creates a user message containing text and an image URL.
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

// ChatCompletionRequest represents a request to /v1/chat/completions.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatChoice represents a single completion choice returned by the model.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionResponse represents the response from /v1/chat/completions.
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

// Text returns the text content of the first choice, or empty string.
func (r *ChatCompletionResponse) Text() string {
	if r == nil || len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.TextContent()
}

// ImageGenerationRequest represents a request to /v1/images/generations.
type ImageGenerationRequest struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model,omitempty"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"` // "url" or "b64_json"
}

// ImageData represents an individual generated image in the response.
type ImageData struct {
	URL     string `json:"url,omitempty"`
	B64JSON string `json:"b64_json,omitempty"`
}

// ImageGenerationResponse represents the response from /v1/images/generations.
type ImageGenerationResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

// ErrorResponse represents an API error from the gateway.
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}
