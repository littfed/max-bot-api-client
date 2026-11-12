package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Option configures an AI Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithDefaultChatModel sets the fallback chat completion model.
func WithDefaultChatModel(model string) Option {
	return func(c *Client) {
		c.defaultChatModel = model
	}
}

// WithDefaultImageModel sets the fallback image generation model.
func WithDefaultImageModel(model string) Option {
	return func(c *Client) {
		c.defaultImageModel = model
	}
}

// WithDefaultAudioModel sets the fallback audio transcription model.
func WithDefaultAudioModel(model string) Option {
	return func(c *Client) {
		c.defaultAudioModel = model
	}
}

// WithTimeout sets a custom request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// Client is a lightweight, zero-dependency client for OpenAI/Anthropic compatible AI gateways (like go-model-api).
type Client struct {
	endpoint          string
	apiKey            string
	httpClient        *http.Client
	defaultChatModel  string
	defaultImageModel string
	defaultAudioModel string
}

// NewClient initializes a new AI gateway client.
func NewClient(endpoint string, apiKey string, opts ...Option) *Client {
	cleanEndpoint := strings.TrimRight(endpoint, "/")
	if cleanEndpoint == "" {
		cleanEndpoint = "http://localhost:8080/v1"
	}

	c := &Client{
		endpoint:          cleanEndpoint,
		apiKey:            apiKey,
		httpClient:        &http.Client{Timeout: 90 * time.Second},
		defaultChatModel:  "gpt-4o-mini",
		defaultImageModel: "flux",
		defaultAudioModel: "whisper-1",
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Endpoint returns the configured base endpoint URL.
func (c *Client) Endpoint() string {
	return c.endpoint
}

// Ping checks gateway health and connectivity.
func (c *Client) Ping(ctx context.Context) error {
	pingURL := fmt.Sprintf("%s/models", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ai: gateway unreachable at %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("ai: gateway returned status %d", resp.StatusCode)
	}
	return nil
}

// Chat performs a chat completion request against /chat/completions.
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil || len(req.Messages) == 0 {
		return nil, errors.New("ai: messages cannot be empty")
	}

	model := req.Model
	if model == "" {
		model = c.defaultChatModel
	}

	payload := map[string]any{
		"model":    model,
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		payload["max_tokens"] = *req.MaxTokens
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal error: %w", err)
	}

	apiURL := fmt.Sprintf("%s/chat/completions", c.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai: gateway error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var rawResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &rawResp); err != nil {
		return nil, fmt.Errorf("ai: unmarshal response failed: %w", err)
	}

	content := ""
	if len(rawResp.Choices) > 0 {
		content = rawResp.Choices[0].Message.Content
	}

	return &ChatResponse{
		ID:               rawResp.ID,
		Model:            rawResp.Model,
		Content:          content,
		PromptTokens:     rawResp.Usage.PromptTokens,
		CompletionTokens: rawResp.Usage.CompletionTokens,
		TotalTokens:      rawResp.Usage.TotalTokens,
		Latency:          time.Since(start),
	}, nil
}

// GenerateImage generates an image from a prompt via /images/generations.
func (c *Client) GenerateImage(ctx context.Context, req *ImageRequest) (*ImageResponse, error) {
	if req == nil || strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("ai: prompt cannot be empty")
	}

	model := req.Model
	if model == "" {
		model = c.defaultImageModel
	}
	size := req.Size
	if size == "" {
		size = "512x512"
	}
	respFormat := req.ResponseFormat
	if respFormat == "" {
		respFormat = "b64_json"
	}

	payload := map[string]any{
		"prompt":          req.Prompt,
		"model":           model,
		"size":            size,
		"response_format": respFormat,
		"n":               1,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal error: %w", err)
	}

	apiURL := fmt.Sprintf("%s/images/generations", c.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai: gateway error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var rawResp struct {
		Created int64 `json:"created"`
		Data    []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &rawResp); err != nil {
		return nil, fmt.Errorf("ai: unmarshal image response failed: %w", err)
	}

	if len(rawResp.Data) == 0 {
		return nil, errors.New("ai: empty data in image response")
	}

	var imgBytes []byte
	imgURL := rawResp.Data[0].URL

	if rawResp.Data[0].B64JSON != "" {
		b, err := base64.StdEncoding.DecodeString(rawResp.Data[0].B64JSON)
		if err != nil {
			return nil, fmt.Errorf("ai: failed to decode b64_json: %w", err)
		}
		imgBytes = b
	} else if imgURL != "" {
		// Download image bytes from URL
		downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
		if err == nil {
			dlResp, dlErr := c.httpClient.Do(downloadReq)
			if dlErr == nil && dlResp.StatusCode == http.StatusOK {
				imgBytes, _ = io.ReadAll(dlResp.Body)
				_ = dlResp.Body.Close()
			}
		}
	}

	return &ImageResponse{
		Data:    imgBytes,
		URL:     imgURL,
		Model:   model,
		Latency: time.Since(start),
	}, nil
}

// TranscribeAudio sends audio bytes to /audio/transcriptions (Whisper STT).
func (c *Client) TranscribeAudio(ctx context.Context, req *AudioRequest) (*AudioResponse, error) {
	if req == nil || len(req.AudioBytes) == 0 {
		return nil, errors.New("ai: audio bytes cannot be empty")
	}

	model := req.Model
	if model == "" {
		model = c.defaultAudioModel
	}
	filename := req.Filename
	if filename == "" {
		filename = "audio.wav"
	}

	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("ai: create form file failed: %w", err)
	}
	if _, err := part.Write(req.AudioBytes); err != nil {
		return nil, fmt.Errorf("ai: write audio failed: %w", err)
	}

	_ = writer.WriteField("model", model)
	if req.Language != "" {
		_ = writer.WriteField("language", req.Language)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("ai: close writer failed: %w", err)
	}

	apiURL := fmt.Sprintf("%s/audio/transcriptions", c.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bodyBuf)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai: gateway error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var rawResp struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &rawResp); err != nil {
		return nil, fmt.Errorf("ai: unmarshal audio response failed: %w", err)
	}

	return &AudioResponse{
		Text:    rawResp.Text,
		Model:   model,
		Latency: time.Since(start),
	}, nil
}
