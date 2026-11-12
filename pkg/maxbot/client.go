package maxbot

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/certs"
)

// DefaultBaseURL is the active production endpoint for MAX Bot API.
const DefaultBaseURL = "https://platform-api2.max.ru"

// Client is the primary interface for communicating with MAX Bot API.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*clientConfig) error

type clientConfig struct {
	baseURL         string
	httpClient      *http.Client
	timeout         time.Duration
	useRussianCerts bool
	insecureTLS     bool
}

// WithBaseURL overrides the default base URL.
func WithBaseURL(rawURL string) Option {
	return func(c *clientConfig) error {
		c.baseURL = strings.TrimRight(rawURL, "/")
		return nil
	}
}

// WithHTTPClient provides a custom http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) error {
		c.httpClient = client
		return nil
	}
}

// WithTimeout sets a custom HTTP request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) error {
		c.timeout = timeout
		return nil
	}
}

// WithRussianCerts enables or disables the embedded Russian Root CA certificate bundle.
// Enabled by default.
func WithRussianCerts(enabled bool) Option {
	return func(c *clientConfig) error {
		c.useRussianCerts = enabled
		return nil
	}
}

// WithInsecureTLS enables InsecureSkipVerify in the TLS configuration (for debugging only).
func WithInsecureTLS(insecure bool) Option {
	return func(c *clientConfig) error {
		c.insecureTLS = insecure
		return nil
	}
}

// NewClient creates a new Client configured to communicate with platform-api2.max.ru.
// Automatically bundles Russian Trusted Root CA certificates by default.
func NewClient(token string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("bot token cannot be empty")
	}

	cfg := &clientConfig{
		baseURL:         DefaultBaseURL,
		timeout:         60 * time.Second,
		useRussianCerts: true,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("failed to apply client option: %w", err)
		}
	}

	if cfg.httpClient == nil {
		var tlsConfig *tls.Config
		if cfg.insecureTLS {
			tlsConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		} else if cfg.useRussianCerts {
			var err error
			tlsConfig, err = certs.NewTLSConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to initialize Russian Root CA TLS configuration: %w", err)
			}
		}

		cfg.httpClient = &http.Client{
			Timeout: cfg.timeout,
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
	}

	return &Client{
		token:      strings.TrimSpace(token),
		baseURL:    cfg.baseURL,
		httpClient: cfg.httpClient,
	}, nil
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, query url.Values, body any) (*http.Request, error) {
	u := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	if len(query) > 0 {
		u = fmt.Sprintf("%s?%s", u, query.Encode())
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}

	// MAX Bot API requires the raw token in the Authorization header (no Bearer prefix).
	req.Header.Set("Authorization", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "max-bot-api-client-go/1.0")

	return req, nil
}

func (c *Client) execute(req *http.Request, target any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			RawBody:    string(respData),
		}
		_ = json.Unmarshal(respData, apiErr)
		return apiErr
	}

	if target != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, target); err != nil {
			return fmt.Errorf("failed to decode response JSON: %w (raw: %s)", err, string(respData))
		}
	}

	return nil
}

// GetMe retrieves the authenticated bot's profile.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/me", nil, nil)
	if err != nil {
		return nil, err
	}
	var user User
	if err := c.execute(req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// SendMessage sends a structured message to a recipient (user or chat).
func (c *Client) SendMessage(ctx context.Context, recipient Recipient, msg *SendMessageRequest) (*Message, error) {
	if recipient == nil {
		return nil, fmt.Errorf("recipient cannot be nil")
	}
	if msg == nil {
		return nil, fmt.Errorf("message payload cannot be nil")
	}

	q := make(url.Values)
	recipient.ApplyRecipient(q)

	req, err := c.newRequest(ctx, http.MethodPost, "/messages", q, msg)
	if err != nil {
		return nil, err
	}

	// MAX Bot API wraps the returned message inside a "message" key: {"message": {...}}
	var rawJSON map[string]json.RawMessage
	if err := c.execute(req, &rawJSON); err != nil {
		return nil, err
	}

	var result Message
	if msgData, ok := rawJSON["message"]; ok {
		_ = json.Unmarshal(msgData, &result)
	} else {
		// Fallback if returned unwrapped
		fullData, _ := json.Marshal(rawJSON)
		_ = json.Unmarshal(fullData, &result)
	}

	if result.MessageID == "" && result.Body.MID != "" {
		result.MessageID = result.Body.MID
	}
	return &result, nil
}

// SendTextMessage sends a plain text or markdown message to a recipient.
func (c *Client) SendTextMessage(ctx context.Context, recipient Recipient, text string) (*Message, error) {
	return c.SendMessage(ctx, recipient, &SendMessageRequest{
		Text:   text,
		Format: "markdown",
	})
}

// SendKeyboardMessage sends a message with an attached inline keyboard.
func (c *Client) SendKeyboardMessage(ctx context.Context, recipient Recipient, text string, kb *InlineKeyboard) (*Message, error) {
	msg := &SendMessageRequest{
		Text:   text,
		Format: "markdown",
	}
	if kb != nil {
		msg.Attachments = append(msg.Attachments, kb.ToAttachment())
	}
	return c.SendMessage(ctx, recipient, msg)
}

// EditMessage edits an existing message sent by the bot.
func (c *Client) EditMessage(ctx context.Context, messageID string, msg *EditMessageRequest) (*Message, error) {
	if strings.TrimSpace(messageID) == "" {
		return nil, fmt.Errorf("messageID cannot be empty")
	}
	q := make(url.Values)
	q.Set("message_id", messageID)

	req, err := c.newRequest(ctx, http.MethodPut, "/messages", q, msg)
	if err != nil {
		return nil, err
	}

	var rawJSON map[string]json.RawMessage
	if err := c.execute(req, &rawJSON); err != nil {
		return nil, err
	}

	var result Message
	if msgData, ok := rawJSON["message"]; ok {
		_ = json.Unmarshal(msgData, &result)
	} else {
		fullData, _ := json.Marshal(rawJSON)
		_ = json.Unmarshal(fullData, &result)
	}

	if result.MessageID == "" && result.Body.MID != "" {
		result.MessageID = result.Body.MID
	}
	return &result, nil
}

// DeleteMessage deletes a message by its ID.
func (c *Client) DeleteMessage(ctx context.Context, messageID string) error {
	if strings.TrimSpace(messageID) == "" {
		return fmt.Errorf("messageID cannot be empty")
	}
	q := make(url.Values)
	q.Set("message_id", messageID)

	req, err := c.newRequest(ctx, http.MethodDelete, "/messages", q, nil)
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// AnswerCallback acknowledges an interactive button click event.
func (c *Client) AnswerCallback(ctx context.Context, callbackID string, answer *AnswerCallbackRequest) error {
	if strings.TrimSpace(callbackID) == "" {
		return fmt.Errorf("callbackID cannot be empty")
	}
	q := make(url.Values)
	q.Set("callback_id", callbackID)

	req, err := c.newRequest(ctx, http.MethodPost, "/answers", q, answer)
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// GetUpdates fetches events sequentially via Long Polling.
func (c *Client) GetUpdates(ctx context.Context, opts *GetUpdatesOptions) (*UpdatesResponse, error) {
	q := make(url.Values)
	if opts != nil {
		if opts.Marker > 0 {
			q.Set("marker", strconv.FormatInt(opts.Marker, 10))
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Timeout > 0 {
			q.Set("timeout", strconv.Itoa(opts.Timeout))
		}
		if len(opts.Types) > 0 {
			q.Set("types", strings.Join(opts.Types, ","))
		}
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/updates", q, nil)
	if err != nil {
		return nil, err
	}

	var result UpdatesResponse
	if err := c.execute(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSubscriptions lists all active webhook subscriptions.
func (c *Client) GetSubscriptions(ctx context.Context) ([]Subscription, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/subscriptions", nil, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Subscriptions []Subscription `json:"subscriptions"`
	}
	if err := c.execute(req, &result); err != nil {
		return nil, err
	}
	return result.Subscriptions, nil
}

// SetSubscription registers a new webhook subscription.
func (c *Client) SetSubscription(ctx context.Context, sub *SetSubscriptionRequest) (*Subscription, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/subscriptions", nil, sub)
	if err != nil {
		return nil, err
	}

	var result Subscription
	if err := c.execute(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteSubscription removes an existing webhook subscription by URL.
func (c *Client) DeleteSubscription(ctx context.Context, webhookURL string) error {
	q := make(url.Values)
	q.Set("url", webhookURL)

	req, err := c.newRequest(ctx, http.MethodDelete, "/subscriptions", q, nil)
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// EnsureWebhook reconciles webhook subscriptions by pruning old ones and setting the new one cleanly.
func (c *Client) EnsureWebhook(ctx context.Context, webhookURL, secret string) error {
	existing, err := c.GetSubscriptions(ctx)
	if err == nil {
		for _, s := range existing {
			_ = c.DeleteSubscription(ctx, s.URL)
		}
	}

	_, err = c.SetSubscription(ctx, &SetSubscriptionRequest{
		URL:    webhookURL,
		Secret: secret,
	})
	return err
}

// SendChatAction sends a presence indicator (e.g. ActionTypingOn) to a chat or dialog.
func (c *Client) SendChatAction(ctx context.Context, chatID int64, action string) error {
	endpoint := fmt.Sprintf("/chats/%d/actions", chatID)
	req, err := c.newRequest(ctx, http.MethodPost, endpoint, nil, &ChatActionRequest{Action: action})
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// GetChat retrieves metadata for a chat or channel by ID.
func (c *Client) GetChat(ctx context.Context, chatID int64) (*Chat, error) {
	endpoint := fmt.Sprintf("/chats/%d", chatID)
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var chat Chat
	if err := c.execute(req, &chat); err != nil {
		return nil, err
	}
	return &chat, nil
}

// PinMessage pins a specific message in a chat or channel.
func (c *Client) PinMessage(ctx context.Context, chatID int64, messageID string, notify bool) error {
	endpoint := fmt.Sprintf("/chats/%d/pin", chatID)
	req, err := c.newRequest(ctx, http.MethodPut, endpoint, nil, &PinMessageRequest{
		MessageID: messageID,
		Notify:    notify,
	})
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// UnpinMessage removes the pinned message in a chat or channel.
func (c *Client) UnpinMessage(ctx context.Context, chatID int64) error {
	endpoint := fmt.Sprintf("/chats/%d/pin", chatID)
	req, err := c.newRequest(ctx, http.MethodDelete, endpoint, nil, nil)
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// GetChatMembers lists members of a group chat or channel.
func (c *Client) GetChatMembers(ctx context.Context, chatID int64, marker ...int64) (*ChatMembersResponse, error) {
	endpoint := fmt.Sprintf("/chats/%d/members", chatID)
	q := url.Values{}
	if len(marker) > 0 && marker[0] > 0 {
		q.Set("marker", strconv.FormatInt(marker[0], 10))
	}
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, q, nil)
	if err != nil {
		return nil, err
	}
	var resp ChatMembersResponse
	if err := c.execute(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// KickChatMember removes a participant from a group chat or channel.
func (c *Client) KickChatMember(ctx context.Context, chatID int64, userID int64) error {
	endpoint := fmt.Sprintf("/chats/%d/members", chatID)
	q := url.Values{}
	q.Set("user_id", strconv.FormatInt(userID, 10))
	req, err := c.newRequest(ctx, http.MethodDelete, endpoint, q, nil)
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// LeaveChat removes the bot itself from a group chat or channel.
func (c *Client) LeaveChat(ctx context.Context, chatID int64) error {
	endpoint := fmt.Sprintf("/chats/%d/members/me", chatID)
	req, err := c.newRequest(ctx, http.MethodDelete, endpoint, nil, nil)
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// UploadMedia executes the two-phase media upload flow.
func (c *Client) UploadMedia(ctx context.Context, mediaType, filename string, r io.Reader) (*UploadURLResponse, error) {
	q := make(url.Values)
	q.Set("type", mediaType)

	// Step 1: Request upload URL and token
	req, err := c.newRequest(ctx, http.MethodPost, "/uploads", q, nil)
	if err != nil {
		return nil, err
	}

	var uploadResp UploadURLResponse
	if err := c.execute(req, &uploadResp); err != nil {
		return nil, fmt.Errorf("failed to get upload URL: %w", err)
	}

	// Step 2: Upload file via multipart/form-data
	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart form file: %w", err)
	}
	if _, err := io.Copy(part, r); err != nil {
		return nil, fmt.Errorf("failed to copy file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadResp.URL, bodyBuf)
	if err != nil {
		return nil, err
	}
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(uploadReq)
	if err != nil {
		return nil, fmt.Errorf("failed to upload media to upload URL: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload server returned error status %d: %s", resp.StatusCode, string(respBody))
	}

	// Step 2 returns JSON containing the media token:
	// For photos: {"photos": {"0": {"token": "..."}}}
	// For files/videos: {"token": "..."}
	var step2JSON map[string]any
	if err := json.Unmarshal(respBody, &step2JSON); err == nil {
		if photos, ok := step2JSON["photos"].(map[string]any); ok {
			for _, v := range photos {
				if photoObj, ok := v.(map[string]any); ok {
					if tok, ok := photoObj["token"].(string); ok && tok != "" {
						uploadResp.Token = tok
						break
					}
				}
			}
		}
		if uploadResp.Token == "" {
			if tok, ok := step2JSON["token"].(string); ok && tok != "" {
				uploadResp.Token = tok
			}
		}
	}

	return &uploadResp, nil
}

// SetCommands configures the bot's command suggestion menu.
func (c *Client) SetCommands(ctx context.Context, commands []BotCommand) error {
	req, err := c.newRequest(ctx, http.MethodPatch, "/me/commands", nil, &SetCommandsRequest{Commands: commands})
	if err != nil {
		return err
	}
	return c.execute(req, nil)
}

// HTTPClient returns the internal http.Client configured with Russian CA certificates and timeout.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// DownloadFile downloads binary data from a URL using the client's TLS configuration.
func (c *Client) DownloadFile(ctx context.Context, fileURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code %d downloading %s", resp.StatusCode, fileURL)
	}
	return io.ReadAll(resp.Body)
}
