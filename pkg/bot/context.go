package bot

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/littfed/max-bot-api-client/pkg/logger"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// Context wraps an incoming event Update with helper methods for replying and routing.
type Context struct {
	ctx     context.Context
	client  *maxbot.Client
	Update  *maxbot.Update
	data    map[string]any
	traceID string
	logger  *slog.Logger
}

// GenerateTraceID generates a random 16-character hex trace ID with "tr_" prefix.
func GenerateTraceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("tr_%d", time.Now().UnixNano())
	}
	return "tr_" + hex.EncodeToString(b)
}

// NewContext creates a new handler Context with an attached trace_id and contextual logger.
func NewContext(ctx context.Context, client *maxbot.Client, update *maxbot.Update) *Context {
	if ctx == nil {
		ctx = context.Background()
	}

	traceID := GenerateTraceID()

	var senderID int64
	var chatID int64
	var updateType string

	if update != nil {
		updateType = update.UpdateType
		if update.Callback != nil {
			if update.Callback.User != nil {
				senderID = update.Callback.User.UserID
			}
			if update.Callback.Message != nil && update.Callback.Message.Recipient != nil {
				chatID = update.Callback.Message.Recipient.ChatID
			} else if update.Message != nil && update.Message.Recipient != nil {
				chatID = update.Message.Recipient.ChatID
			}
		} else if update.Message != nil {
			if update.Message.Sender != nil {
				senderID = update.Message.Sender.UserID
			}
			if update.Message.Recipient != nil {
				chatID = update.Message.Recipient.ChatID
			}
		}
	}

	baseLogger := logger.FromContext(ctx)
	contextLogger := baseLogger.With(
		slog.String("trace_id", traceID),
		slog.Int64("user_id", senderID),
		slog.Int64("chat_id", chatID),
		slog.String("update_type", updateType),
	)

	ctx = logger.WithContext(ctx, contextLogger)

	return &Context{
		ctx:     ctx,
		client:  client,
		Update:  update,
		data:    make(map[string]any),
		traceID: traceID,
		logger:  contextLogger,
	}
}

// TraceID returns the unique trace identifier for this request lifecycle.
func (c *Context) TraceID() string {
	if c.traceID == "" {
		c.traceID = GenerateTraceID()
	}
	return c.traceID
}

// Logger returns the contextual structured logger pre-filled with trace_id, user_id, chat_id.
func (c *Context) Logger() *slog.Logger {
	if c.logger == nil {
		c.logger = logger.FromContext(c.ctx)
	}
	return c.logger
}

// WithLogger replaces the contextual logger for this context.
func (c *Context) WithLogger(l *slog.Logger) {
	c.logger = l
	if c.ctx != nil {
		c.ctx = logger.WithContext(c.ctx, l)
	}
}

// Set stores a custom key-value pair in the request context.
func (c *Context) Set(key string, val any) {
	if c.data == nil {
		c.data = make(map[string]any)
	}
	c.data[key] = val
}

// Get retrieves a key-value pair from the request context.
func (c *Context) Get(key string) (any, bool) {
	if c.data == nil {
		return nil, false
	}
	val, ok := c.data[key]
	return val, ok
}

// Ctx returns the underlying context.Context.
func (c *Context) Ctx() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// Client returns the active maxbot.Client.
func (c *Context) Client() *maxbot.Client {
	return c.client
}

// Message returns the associated Message, or nil if none.
func (c *Context) Message() *maxbot.Message {
	if c.Update == nil {
		return nil
	}
	if c.Update.Message != nil {
		return c.Update.Message
	}
	if c.Update.Callback != nil && c.Update.Callback.Message != nil {
		return c.Update.Callback.Message
	}
	return nil
}

// Callback returns the associated Callback query, or nil if none.
func (c *Context) Callback() *maxbot.Callback {
	if c.Update == nil {
		return nil
	}
	return c.Update.Callback
}

// Text returns the incoming message text, or empty string.
func (c *Context) Text() string {
	msg := c.Message()
	if msg == nil {
		return ""
	}
	return msg.Body.Text
}

// ExtractAttachmentName extracts the original file name from an attachment payload.
func ExtractAttachmentName(pMap map[string]any) string {
	if pMap == nil {
		return ""
	}
	for _, k := range []string{"name", "filename", "file_name", "title", "caption"} {
		if n, ok := pMap[k].(string); ok && strings.TrimSpace(n) != "" {
			return strings.TrimSpace(n)
		}
	}
	for _, k := range []string{"url", "download_url", "link", "file_url", "src"} {
		if u, ok := pMap[k].(string); ok && u != "" {
			cleanU := u
			if qIdx := strings.Index(cleanU, "?"); qIdx != -1 {
				cleanU = cleanU[:qIdx]
			}
			base := filepath.Base(cleanU)
			if base != "" && base != "." && base != "/" && strings.Contains(base, ".") {
				return base
			}
		}
	}
	return ""
}

func extractAttachmentName(pMap map[string]any) string {
	return ExtractAttachmentName(pMap)
}

// AudioAttachment returns the first audio attachment from the message, if present.
func (c *Context) AudioAttachment() (*maxbot.Attachment, bool) {
	msg := c.Message()
	if msg == nil {
		return nil, false
	}
	for _, att := range msg.Body.Attachments {
		if att.Type == maxbot.UploadTypeAudio || att.Type == "voice" || att.Type == "audio" {
			return &att, true
		}
		if att.Type == maxbot.UploadTypeFile || att.Type == "file" {
			name := att.GetFilename()
			if isAudioExtension(strings.ToLower(name)) {
				return &att, true
			}
			if pMap, ok := att.Payload.(map[string]any); ok {
				for _, mimeKey := range []string{"content_type", "mime_type", "type", "mime"} {
					if mimeVal, ok := pMap[mimeKey].(string); ok && mimeVal != "" {
						mimeLower := strings.ToLower(mimeVal)
						if strings.HasPrefix(mimeLower, "audio/") || mimeLower == "application/ogg" || mimeLower == "application/x-ogg" {
							return &att, true
						}
					}
				}
			}
		}
	}
	return nil, false
}

// AudioBytes downloads or decodes the binary audio payload from an audio attachment.
func (c *Context) AudioBytes() ([]byte, string, error) {
	att, ok := c.AudioAttachment()
	if !ok || att == nil {
		return nil, "", fmt.Errorf("no audio attachment found in message")
	}

	filename := att.GetFilename()
	if filename == "" {
		filename = "voice.ogg"
	}
	if pMap, ok := att.Payload.(map[string]any); ok {
		for _, b64Key := range []string{"b64_json", "base64", "data"} {
			if b64Str, ok := pMap[b64Key].(string); ok && b64Str != "" {
				cleanB64 := b64Str
				if commaIdx := strings.Index(cleanB64, ","); commaIdx != -1 && strings.Contains(cleanB64[:commaIdx], "base64") {
					cleanB64 = cleanB64[commaIdx+1:]
				}
				data, err := base64.StdEncoding.DecodeString(cleanB64)
				if err == nil && len(data) > 0 {
					return data, filename, nil
				}
			}
		}
		for _, bKey := range []string{"bytes", "raw", "raw_data"} {
			if bData, ok := pMap[bKey].([]byte); ok && len(bData) > 0 {
				return bData, filename, nil
			}
		}
	}

	u := extractURLFromAttachmentPayload(att.Payload)
	if u == "" {
		return nil, "", fmt.Errorf("cannot extract audio url or data from payload")
	}

	if c.client != nil {
		data, err := c.client.DownloadFile(c.Ctx(), u)
		if err == nil && len(data) > 0 {
			return data, filename, nil
		}
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(c.Ctx(), http.MethodGet, u, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create http request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("audio download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read audio body: %w", err)
	}
	return data, filename, nil
}

// ImageAttachment returns the first image, photo, or picture attachment from the message, if present.
func (c *Context) ImageAttachment() (*maxbot.Attachment, bool) {
	msg := c.Message()
	if msg == nil {
		return nil, false
	}
	for _, att := range msg.Body.Attachments {
		// 1. Direct image, photo, picture, or sticker types
		if att.Type == maxbot.UploadTypeImage || att.Type == "photo" || att.Type == "image" ||
			att.Type == "picture" || att.Type == "sticker" {
			return &att, true
		}
		// 2. File / Document attachments: check filename, mime-type, or URL
		if att.Type == maxbot.UploadTypeFile || att.Type == "file" || att.Type == "document" {
			name := att.GetFilename()
			if isImageExtension(strings.ToLower(name)) {
				return &att, true
			}
			if pMap, ok := att.Payload.(map[string]any); ok {
				// Check content-type / mime-type
				for _, mimeKey := range []string{"content_type", "mime_type", "type", "mime"} {
					if mimeVal, ok := pMap[mimeKey].(string); ok && mimeVal != "" {
						if strings.HasPrefix(strings.ToLower(mimeVal), "image/") {
							return &att, true
						}
					}
				}
				// Check url extension or data:image/
				for _, urlKey := range []string{"url", "file_url", "src", "download_url", "link"} {
					if u, ok := pMap[urlKey].(string); ok && u != "" {
						if strings.HasPrefix(u, "data:image/") {
							return &att, true
						}
						cleanURL := strings.ToLower(u)
						if qIdx := strings.Index(cleanURL, "?"); qIdx != -1 {
							cleanURL = cleanURL[:qIdx]
						}
						if isImageExtension(cleanURL) {
							return &att, true
						}
					}
				}
			}
		}
	}
	return nil, false
}

// ImageURL returns the direct download or preview URL of the attached image, if available.
func (c *Context) ImageURL() (string, bool) {
	att, ok := c.ImageAttachment()
	if !ok || att == nil {
		return "", false
	}
	u := extractURLFromAttachmentPayload(att.Payload)
	if u != "" {
		return u, true
	}
	return "", false
}

// ImageBytes downloads or decodes the binary image payload from the message attachments.
func (c *Context) ImageBytes() ([]byte, string, error) {
	att, ok := c.ImageAttachment()
	if !ok || att == nil {
		return nil, "", fmt.Errorf("no image attachment found in message")
	}

	filename := att.GetFilename()
	if filename == "" {
		filename = "image.jpg"
	}
	if pMap, ok := att.Payload.(map[string]any); ok {
		// If base64 payload is already present
		for _, b64Key := range []string{"b64_json", "base64", "data"} {
			if b64Str, ok := pMap[b64Key].(string); ok && b64Str != "" {
				cleanB64 := b64Str
				if commaIdx := strings.Index(cleanB64, ","); commaIdx != -1 && strings.Contains(cleanB64[:commaIdx], "base64") {
					cleanB64 = cleanB64[commaIdx+1:]
				}
				data, err := base64.StdEncoding.DecodeString(cleanB64)
				if err == nil && len(data) > 0 {
					return data, filename, nil
				}
			}
		}
		for _, bKey := range []string{"bytes", "raw", "raw_data"} {
			if bData, ok := pMap[bKey].([]byte); ok && len(bData) > 0 {
				return bData, filename, nil
			}
		}
	}

	u := extractURLFromAttachmentPayload(att.Payload)
	if u == "" {
		return nil, "", fmt.Errorf("cannot extract image url or data from payload")
	}

	// If data URI
	if strings.HasPrefix(u, "data:") {
		commaIdx := strings.Index(u, ",")
		if commaIdx != -1 {
			cleanB64 := u[commaIdx+1:]
			data, err := base64.StdEncoding.DecodeString(cleanB64)
			if err == nil && len(data) > 0 {
				return data, filename, nil
			}
		}
	}

	// Try using bot client DownloadFile first (handles Russian TLS certificates and bot auth)
	if c.client != nil {
		data, err := c.client.DownloadFile(c.Ctx(), u)
		if err == nil && len(data) > 0 {
			slog.Info("image downloaded via bot client", slog.String("url", u), slog.Int("bytes", len(data)))
			return data, filename, nil
		}
		slog.Warn("bot client DownloadFile failed, trying direct http", slog.String("url", u), slog.Any("error", err))
	}

	// Fallback to standard http.Get with timeout
	httpClient := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequestWithContext(c.Ctx(), http.MethodGet, u, nil)
	if err != nil {
		slog.Error("failed to create http request for image download", slog.String("url", u), slog.Any("error", err))
		return nil, "", fmt.Errorf("failed to create http request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("direct http image download failed", slog.String("url", u), slog.Any("error", err))
		return nil, "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("image download returned bad status", slog.String("url", u), slog.Int("status", resp.StatusCode))
		return nil, "", fmt.Errorf("image download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read image body", slog.String("url", u), slog.Any("error", err))
		return nil, "", fmt.Errorf("failed to read image body: %w", err)
	}
	slog.Info("image downloaded via direct http", slog.String("url", u), slog.Int("bytes", len(data)))
	return data, filename, nil
}

// FileAttachment returns the first non-image, non-audio document attachment from the message, if present.
func (c *Context) FileAttachment() (*maxbot.Attachment, bool) {
	msg := c.Message()
	if msg == nil {
		return nil, false
	}
	for _, att := range msg.Body.Attachments {
		if att.Type == maxbot.UploadTypeFile || att.Type == "file" || att.Type == "document" {
			name := att.GetFilename()
			nameLower := strings.ToLower(name)
			if isAudioExtension(nameLower) || isImageExtension(nameLower) {
				continue
			}
			if pMap, ok := att.Payload.(map[string]any); ok {
				// Skip if mime_type or content_type is image or audio
				for _, mimeKey := range []string{"content_type", "mime_type", "type", "mime"} {
					if mimeVal, ok := pMap[mimeKey].(string); ok && mimeVal != "" {
						mimeLower := strings.ToLower(mimeVal)
						if strings.HasPrefix(mimeLower, "image/") || strings.HasPrefix(mimeLower, "audio/") || mimeLower == "application/ogg" || mimeLower == "application/x-ogg" {
							goto skip
						}
					}
				}
			}
			return &att, true
		}
	skip:
	}
	return nil, false
}

// FileURL returns the direct download URL of the attached document, if available.
func (c *Context) FileURL() (string, bool) {
	att, ok := c.FileAttachment()
	if !ok || att == nil {
		return "", false
	}
	u := extractURLFromAttachmentPayload(att.Payload)
	if u != "" {
		return u, true
	}
	return "", false
}

// FileBytes downloads or decodes the binary file payload from document attachments.
func (c *Context) FileBytes() ([]byte, string, error) {
	att, ok := c.FileAttachment()
	if !ok || att == nil {
		return nil, "", fmt.Errorf("no file attachment found in message")
	}

	filename := att.GetFilename()
	if filename == "" {
		filename = "document.txt"
	}
	if pMap, ok := att.Payload.(map[string]any); ok {
		for _, b64Key := range []string{"b64_json", "base64", "data"} {
			if b64Str, ok := pMap[b64Key].(string); ok && b64Str != "" {
				cleanB64 := b64Str
				if commaIdx := strings.Index(cleanB64, ","); commaIdx != -1 && strings.Contains(cleanB64[:commaIdx], "base64") {
					cleanB64 = cleanB64[commaIdx+1:]
				}
				data, err := base64.StdEncoding.DecodeString(cleanB64)
				if err == nil && len(data) > 0 {
					return data, filename, nil
				}
				if utf8.ValidString(cleanB64) && len(cleanB64) > 0 {
					return []byte(cleanB64), filename, nil
				}
			}
		}
		for _, bKey := range []string{"bytes", "raw", "raw_data"} {
			if bData, ok := pMap[bKey].([]byte); ok && len(bData) > 0 {
				return bData, filename, nil
			}
		}
	}

	u := extractURLFromAttachmentPayload(att.Payload)
	if u == "" {
		return nil, "", fmt.Errorf("cannot extract file url or data from payload")
	}

	if c.client != nil {
		data, err := c.client.DownloadFile(c.Ctx(), u)
		if err == nil && len(data) > 0 {
			return data, filename, nil
		}
	}

	httpClient := &http.Client{Timeout: 25 * time.Second}
	req, err := http.NewRequestWithContext(c.Ctx(), http.MethodGet, u, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create http request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("file download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file body: %w", err)
	}
	return data, filename, nil
}

func isAudioExtension(nameLower string) bool {
	if qIdx := strings.Index(nameLower, "?"); qIdx != -1 {
		nameLower = nameLower[:qIdx]
	}
	if fIdx := strings.Index(nameLower, "#"); fIdx != -1 {
		nameLower = nameLower[:fIdx]
	}
	return strings.HasSuffix(nameLower, ".ogg") || strings.HasSuffix(nameLower, ".mp3") ||
		strings.HasSuffix(nameLower, ".wav") || strings.HasSuffix(nameLower, ".m4a") ||
		strings.HasSuffix(nameLower, ".aac") || strings.HasSuffix(nameLower, ".opus") ||
		strings.HasSuffix(nameLower, ".flac") || strings.HasSuffix(nameLower, ".wma") ||
		strings.HasSuffix(nameLower, ".aiff") || strings.HasSuffix(nameLower, ".alac")
}

func isImageExtension(nameLower string) bool {
	if qIdx := strings.Index(nameLower, "?"); qIdx != -1 {
		nameLower = nameLower[:qIdx]
	}
	if fIdx := strings.Index(nameLower, "#"); fIdx != -1 {
		nameLower = nameLower[:fIdx]
	}

	extensions := []string{
		// JPEG family
		".jpg", ".jpeg", ".jfif", ".jpe", ".jif", ".jfi", ".pjpeg", ".pjp",
		// Web / Modern formats
		".webp", ".png", ".apng", ".gif", ".avif", ".avis", ".svg", ".svgz",
		// Apple / Mobile high efficiency formats
		".heic", ".heif", ".hif",
		// Windows Bitmaps & Icons
		".bmp", ".dib", ".ico", ".cur",
		// TIFF
		".tiff", ".tif",
		// Camera RAW formats
		".raw", ".dng", ".cr2", ".cr3", ".nef", ".nrw", ".arw", ".srf", ".sr2", ".rw2", ".orf", ".pef", ".raf",
		// Graphics editing & raster formats
		".psd", ".psb", ".ai", ".eps", ".tga", ".dds", ".hdr", ".exr", ".xcf",
	}

	for _, ext := range extensions {
		if strings.HasSuffix(nameLower, ext) {
			return true
		}
	}
	return false
}

// extractURLFromAttachmentPayload extracts an image URL from an attachment payload.
func extractURLFromAttachmentPayload(payload any) string {
	if payload == nil {
		return ""
	}
	switch v := payload.(type) {
	case string:
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "data:image/") {
			return v
		}
	case map[string]any:
		for _, key := range []string{"url", "file_url", "src", "download_url", "link", "preview_url", "full_url"} {
			if u, ok := v[key].(string); ok && u != "" {
				return u
			}
		}
		// Check "photos" as []any
		if photosSlice, ok := v["photos"].([]any); ok {
			for i := len(photosSlice) - 1; i >= 0; i-- {
				if pMap, ok := photosSlice[i].(map[string]any); ok {
					for _, key := range []string{"url", "file_url", "src", "link"} {
						if u, ok := pMap[key].(string); ok && u != "" {
							return u
						}
					}
				}
			}
		}
		// Check "photos" as map[string]any
		if photosMap, ok := v["photos"].(map[string]any); ok {
			for _, val := range photosMap {
				if pMap, ok := val.(map[string]any); ok {
					for _, key := range []string{"url", "file_url", "src", "link"} {
						if u, ok := pMap[key].(string); ok && u != "" {
							return u
						}
					}
				}
			}
		}
		// Check "photo" as map[string]any
		if pMap, ok := v["photo"].(map[string]any); ok {
			for _, key := range []string{"url", "file_url", "src", "link"} {
				if u, ok := pMap[key].(string); ok && u != "" {
					return u
				}
			}
		}
		// Check "file" as map[string]any
		if fMap, ok := v["file"].(map[string]any); ok {
			for _, key := range []string{"url", "file_url", "src", "download_url"} {
				if u, ok := fMap[key].(string); ok && u != "" {
					return u
				}
			}
		}
	}
	return ""
}

// IsVoiceNoteAttempt returns true if this update represents a microphone voice note event from MAX Messenger.
func (c *Context) IsVoiceNoteAttempt() bool {
	if c.Message() == nil {
		return false
	}
	for _, att := range c.Message().Body.Attachments {
		if att.Type == "voice_note_attempt" {
			return true
		}
	}
	return c.Text() == "[голос с микрофона]"
}

// Sender returns the User who triggered the update.
func (c *Context) Sender() *maxbot.User {
	if c.Update == nil {
		return nil
	}
	if c.Update.Callback != nil && c.Update.Callback.User != nil && c.Update.Callback.User.UserID != 0 {
		return c.Update.Callback.User
	}
	if c.Update.Message != nil && c.Update.Message.Sender != nil && c.Update.Message.Sender.UserID != 0 {
		return c.Update.Message.Sender
	}
	if c.Update.User != nil && c.Update.User.UserID != 0 {
		return c.Update.User
	}
	if c.Update.Callback != nil && c.Update.Callback.User != nil {
		return c.Update.Callback.User
	}
	if c.Update.Message != nil && c.Update.Message.Sender != nil {
		return c.Update.Message.Sender
	}
	return c.Update.User
}

// Recipient automatically derives the target recipient to reply to.
// In group chats/channels, it targets the chat ID. In 1-on-1 dialogs, it targets the user ID of the sender.
func (c *Context) Recipient() (maxbot.Recipient, error) {
	if c.Update == nil {
		return nil, fmt.Errorf("update is nil")
	}

	// 1. If explicit chat_id is present at update level
	if c.Update.ChatID != 0 {
		return maxbot.ToChat(c.Update.ChatID), nil
	}

	// 2. If message was sent in a group chat or channel, reply to that chat
	if msg := c.Message(); msg != nil && msg.Recipient != nil && msg.Recipient.ChatID != 0 {
		return maxbot.ToChat(msg.Recipient.ChatID), nil
	}

	// 3. In 1-on-1 dialogs, always reply to the sender
	if sender := c.Sender(); sender != nil && sender.UserID != 0 {
		return maxbot.ToUser(sender.UserID), nil
	}

	// 4. From Callback user
	if cb := c.Callback(); cb != nil && cb.User != nil && cb.User.UserID != 0 {
		return maxbot.ToUser(cb.User.UserID), nil
	}

	// 5. Fallback from message recipient if sender is omitted
	if msg := c.Message(); msg != nil && msg.Recipient != nil && msg.Recipient.UserID != 0 {
		return maxbot.ToUser(msg.Recipient.UserID), nil
	}

	return nil, fmt.Errorf("cannot deduce recipient from update")
}

// Reply sends a text message back to the conversation sender (markdown formatted).
func (c *Context) Reply(text string) (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	rec, err := c.Recipient()
	if err != nil {
		return nil, err
	}
	return c.client.SendTextMessage(c.Ctx(), rec, text)
}

// ReplyKeyboard sends a text message with an inline keyboard back to the conversation.
func (c *Context) ReplyKeyboard(text string, kb *maxbot.InlineKeyboard) (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	rec, err := c.Recipient()
	if err != nil {
		return nil, err
	}
	return c.client.SendKeyboardMessage(c.Ctx(), rec, text, kb)
}

// ReplyWithAttachments sends a text message with custom attachments (e.g. media, keyboards).
func (c *Context) ReplyWithAttachments(text string, attachments ...maxbot.Attachment) (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	rec, err := c.Recipient()
	if err != nil {
		return nil, err
	}
	msg := &maxbot.SendMessageRequest{
		Text:        text,
		Format:      "markdown",
		Attachments: attachments,
	}
	return c.client.SendMessage(c.Ctx(), rec, msg)
}

// AnswerCallback acknowledges an incoming button callback.
func (c *Context) AnswerCallback(answer *maxbot.AnswerCallbackRequest) error {
	if c.client == nil {
		return nil
	}
	cb := c.Callback()
	if cb == nil {
		return fmt.Errorf("no callback in update")
	}
	return c.client.AnswerCallback(c.Ctx(), cb.CallbackID, answer)
}

// AnswerNotification shows an ephemeral toast notification in response to a callback.
func (c *Context) AnswerNotification(notification string) error {
	return c.AnswerCallback(&maxbot.AnswerCallbackRequest{
		Notification: notification,
	})
}

// AnswerCallbackAlert shows an ephemeral alert or notification in response to an obsolete or rejected callback.
func (c *Context) AnswerCallbackAlert(alert string) error {
	return c.AnswerNotification(alert)
}

// SendChatAction sends a presence indicator (e.g. typing_on) to the active chat.
func (c *Context) SendChatAction(action string) error {
	if c.client == nil {
		return nil
	}
	rec, err := c.Recipient()
	if err != nil {
		return err
	}
	return c.client.SendChatAction(c.Ctx(), rec.ID(), action)
}

// Typing sends the "typing_on" action to display the typing indicator.
func (c *Context) Typing() error {
	return c.SendChatAction(maxbot.ActionTypingOn)
}

// EditMessage edits the message using an explicit EditMessageRequest.
func (c *Context) EditMessage(req *maxbot.EditMessageRequest) (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	msg := c.Message()
	if msg == nil || msg.Body.MID == "" {
		return nil, fmt.Errorf("cannot edit message: message ID is empty")
	}
	return c.client.EditMessage(c.Ctx(), msg.Body.MID, req)
}

// Edit edits the text of the current message (e.g. from an incoming button callback).
func (c *Context) Edit(text string) (*maxbot.Message, error) {
	return c.EditKeyboard(text, nil)
}

// EditKeyboard edits the text and inline keyboard of the current message in place.
func (c *Context) EditKeyboard(text string, kb *maxbot.InlineKeyboard) (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	msg := c.Message()
	if msg == nil || msg.Body.MID == "" {
		return nil, fmt.Errorf("cannot edit message: message ID is empty")
	}
	req := &maxbot.EditMessageRequest{
		Text:   text,
		Format: "markdown",
	}
	if kb != nil {
		req.Attachments = []maxbot.Attachment{kb.ToAttachment()}
	}
	return c.client.EditMessage(c.Ctx(), msg.Body.MID, req)
}

// EditOrReplyKeyboard attempts to edit the current message in place if a message ID is available
// from an incoming button callback. If editing is not applicable (e.g. initial command /start or /menu)
// or fails, it falls back to sending a new message with ReplyKeyboard.
func (c *Context) EditOrReplyKeyboard(text string, kb *maxbot.InlineKeyboard) (*maxbot.Message, error) {
	if c.Callback() != nil {
		if msg := c.Message(); msg != nil && msg.Body.MID != "" {
			res, err := c.EditKeyboard(text, kb)
			if err == nil {
				return res, nil
			}
		}
	}
	return c.ReplyKeyboard(text, kb)
}

// ClearKeyboard removes the inline keyboard from the current message, keeping its existing text.
func (c *Context) ClearKeyboard() (*maxbot.Message, error) {
	if c.client == nil {
		return &maxbot.Message{}, nil
	}
	msg := c.Message()
	if msg == nil || msg.Body.MID == "" {
		return nil, fmt.Errorf("cannot clear keyboard: message ID is empty")
	}
	req := &maxbot.EditMessageRequest{
		Text:   msg.Body.Text,
		Format: msg.Body.Format,
	}
	if req.Format == "" {
		req.Format = "markdown"
	}
	return c.client.EditMessage(c.Ctx(), msg.Body.MID, req)
}

// Command parses the message text as a command (e.g. "/ping arg1 arg2").
// Returns command name without slash ("ping") and remaining arguments ("arg1 arg2").
// If the message is not a command, returns empty strings.
func (c *Context) Command() (string, string) {
	text := strings.TrimSpace(c.Text())
	if !strings.HasPrefix(text, "/") {
		return "", ""
	}

	parts := strings.SplitN(text[1:], " ", 2)
	cmd := parts[0]
	// Strip bot mention if command is in format /ping@my_bot
	if atIdx := strings.Index(cmd, "@"); atIdx != -1 {
		cmd = cmd[:atIdx]
	}

	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return strings.ToLower(cmd), args
}

// IsCommand returns true if the incoming message is a slash command.
func (c *Context) IsCommand() bool {
	cmd, _ := c.Command()
	return cmd != ""
}

// CommandArgs returns the arguments string following the command name, or empty string.
func (c *Context) CommandArgs() string {
	_, args := c.Command()
	return args
}

// StartPayload extracts the start parameter from bot_started or "/start <payload>".
func (c *Context) StartPayload() string {
	if c.Update != nil && c.Update.Payload != "" {
		return c.Update.Payload
	}
	cmd, args := c.Command()
	if cmd == "start" {
		return args
	}
	return ""
}
