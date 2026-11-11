package gma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the public endpoint of Go Model API.
const DefaultBaseURL = "https://gma.littfed.ru"

// Default models.
const (
	DefaultChatModel   = "@scout"
	DefaultVisionModel = "@scout-vision"
	DefaultImageModel  = "flux"
)

// Client is a Go client for interacting with the Go Model API gateway.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithTimeout sets a timeout on the default http.Client.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if c.httpClient != nil {
			c.httpClient.Timeout = timeout
		}
	}
}

// NewClient initializes a new Go Model API client.
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	cleanURL := strings.TrimRight(baseURL, "/")
	if cleanURL == "" {
		cleanURL = DefaultBaseURL
	}

	c := &Client{
		baseURL: cleanURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// ChatCompletion sends a request to /v1/chat/completions.
func (c *Client) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("chat completion request cannot be nil")
	}

	endpoint := fmt.Sprintf("%s/v1/chat/completions", c.baseURL)
	var resp ChatCompletionResponse
	if err := c.postJSON(ctx, endpoint, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GenerateImage sends a request to /v1/images/generations.
func (c *Client) GenerateImage(ctx context.Context, req *ImageGenerationRequest) (*ImageGenerationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image generation request cannot be nil")
	}

	endpoint := fmt.Sprintf("%s/v1/images/generations", c.baseURL)
	var resp ImageGenerationResponse
	if err := c.postJSON(ctx, endpoint, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// postJSON marshals payload, performs an HTTP POST request, checks error codes, and unmarshals result.
func (c *Client) postJSON(ctx context.Context, endpoint string, payload any, dest any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		var apiErr ErrorResponse
		if jsonErr := json.Unmarshal(bodyBytes, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("gma api error (HTTP %d): %s", httpResp.StatusCode, apiErr.Error.Message)
		}
		return fmt.Errorf("gma api returned status HTTP %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	if dest != nil {
		if err := json.Unmarshal(bodyBytes, dest); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
