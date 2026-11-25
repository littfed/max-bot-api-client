package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/pkg/logger"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestContextTraceIDAndLogger(t *testing.T) {
	var buf bytes.Buffer
	baseLogger := logger.New(
		logger.WithLevel(slog.LevelDebug),
		logger.WithOutput(&buf),
	)

	ctx := logger.WithContext(context.Background(), baseLogger)

	update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{
				UserID:   777888,
				Username: "tester",
			},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 999111,
			},
			Body: maxbot.MessageBody{
				Text: "hello world",
			},
		},
	}

	c := NewContext(ctx, nil, update)

	traceID := c.TraceID()
	if traceID == "" || !strings.HasPrefix(traceID, "tr_") {
		t.Fatalf("expected trace_id with 'tr_' prefix, got %q", traceID)
	}

	c.Logger().Info("processing message", slog.String("action", "test"))

	out := strings.TrimSpace(buf.String())
	var logData map[string]interface{}
	if err := json.Unmarshal([]byte(out), &logData); err != nil {
		t.Fatalf("failed to unmarshal log JSON: %v, raw: %s", err, out)
	}

	if logData["trace_id"] != traceID {
		t.Errorf("expected trace_id %q, got %v", traceID, logData["trace_id"])
	}
	if logData["user_id"] != float64(777888) {
		t.Errorf("expected user_id 777888, got %v", logData["user_id"])
	}
	if logData["chat_id"] != float64(999111) {
		t.Errorf("expected chat_id 999111, got %v", logData["chat_id"])
	}
	if logData["update_type"] != maxbot.UpdateTypeMessageCreated {
		t.Errorf("expected update_type %q, got %v", maxbot.UpdateTypeMessageCreated, logData["update_type"])
	}
}

func TestAttachmentClassification(t *testing.T) {
	// 1. Text file (3418 bytes) must be classified as FileAttachment, NOT AudioAttachment
	txtData := strings.Repeat("A", 3418)
	txtUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Attachments: []maxbot.Attachment{
					{
						Type: maxbot.UploadTypeFile,
						Payload: map[string]any{
							"title":        "notes.txt",
							"data":         txtData,
							"content_type": "text/plain; charset=utf-8",
						},
					},
				},
			},
		},
	}
	cTxt := NewContext(context.Background(), nil, txtUpdate)
	if _, isAudio := cTxt.AudioAttachment(); isAudio {
		t.Errorf("expected text file NOT to be classified as AudioAttachment")
	}
	if _, isFile := cTxt.FileAttachment(); !isFile {
		t.Errorf("expected text file to be classified as FileAttachment")
	}

	// 2. Audio file (.ogg) with generic "file" type must be classified as AudioAttachment
	audioUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Attachments: []maxbot.Attachment{
					{
						Type: "file",
						Payload: map[string]any{
							"name":         "voice.ogg",
							"data":         "T2dnUwACAAAAAAAAA...",
							"content_type": "audio/ogg",
						},
					},
				},
			},
		},
	}
	cAudio := NewContext(context.Background(), nil, audioUpdate)
	if _, isAudio := cAudio.AudioAttachment(); !isAudio {
		t.Errorf("expected .ogg file to be classified as AudioAttachment")
	}
	if _, isFile := cAudio.FileAttachment(); isFile {
		t.Errorf("expected .ogg file NOT to be classified as FileAttachment")
	}

	// 3. Image file (.webp) with generic "file" type must be classified as ImageAttachment
	webpUpdate := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Attachments: []maxbot.Attachment{
					{
						Type: "file",
						Payload: map[string]any{
							"filename":     "photo.webp",
							"data":         "UklGR...",
							"content_type": "image/webp",
						},
					},
				},
			},
		},
	}
	cWebp := NewContext(context.Background(), nil, webpUpdate)
	if _, isImg := cWebp.ImageAttachment(); !isImg {
		t.Errorf("expected .webp file to be classified as ImageAttachment")
	}
	if _, isFile := cWebp.FileAttachment(); isFile {
		t.Errorf("expected .webp file NOT to be classified as FileAttachment")
	}

	// 4. Video file (.mp4) with Filename on Attachment struct directly
	mp4Update := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Body: maxbot.MessageBody{
				Attachments: []maxbot.Attachment{
					{
						Type:     "file",
						Filename: "slovami-story.mp4",
						Size:     1102532,
						Payload: map[string]any{
							"fileId": 5857870395,
							"url":    "https://fd.oneme.ru/getfile?rq=test",
						},
					},
				},
			},
		},
	}
	cMp4 := NewContext(context.Background(), nil, mp4Update)
	att, isFile := cMp4.FileAttachment()
	if !isFile || att == nil {
		t.Fatalf("expected .mp4 to be returned as FileAttachment for inspection")
	}
	if att.GetFilename() != "slovami-story.mp4" {
		t.Errorf("expected filename 'slovami-story.mp4', got %q", att.GetFilename())
	}
	if att.GetSize() != 1102532 {
		t.Errorf("expected size 1102532, got %d", att.GetSize())
	}
	if _, isAudio := cMp4.AudioAttachment(); isAudio {
		t.Errorf("expected .mp4 NOT to be AudioAttachment")
	}
	if _, isImg := cMp4.ImageAttachment(); isImg {
		t.Errorf("expected .mp4 NOT to be ImageAttachment")
	}
}

func TestContextCommandParsing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		input       string
		expectedCmd string
		expectedArg string
	}{
		{name: "simple_command", input: "/ping", expectedCmd: "ping", expectedArg: ""},
		{name: "command_with_bot_mention", input: "/ping@my_bot", expectedCmd: "ping", expectedArg: ""},
		{name: "command_with_args", input: "/echo hello world", expectedCmd: "echo", expectedArg: "hello world"},
		{name: "start_with_ref", input: "/start ref_12345", expectedCmd: "start", expectedArg: "ref_12345"},
		{name: "plain_text", input: "hello without slash", expectedCmd: "", expectedArg: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &Context{
				Update: &maxbot.Update{
					Message: &maxbot.Message{
						Body: maxbot.MessageBody{Text: tc.input},
					},
				},
			}

			cmd, args := c.Command()
			if cmd != tc.expectedCmd || args != tc.expectedArg {
				t.Errorf("input %q: expected cmd=%q arg=%q, got cmd=%q arg=%q",
					tc.input, tc.expectedCmd, tc.expectedArg, cmd, args)
			}
		})
	}
}

func TestContextStartPayload(t *testing.T) {
	t.Parallel()
	c1 := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeBotStarted,
			Payload:    "ref_u999",
		},
	}
	if p := c1.StartPayload(); p != "ref_u999" {
		t.Errorf("expected ref_u999, got %s", p)
	}

	c2 := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{Text: "/start ref_u888"},
			},
		},
	}
	if p := c2.StartPayload(); p != "ref_u888" {
		t.Errorf("expected ref_u888, got %s", p)
	}
}

func TestContextRecipientDerivation(t *testing.T) {
	t.Parallel()
	c1 := &Context{
		Update: &maxbot.Update{
			Message: &maxbot.Message{
				Sender: &maxbot.User{UserID: 555},
			},
		},
	}
	r1, err := c1.Recipient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r1.IsUser() || r1.ID() != 555 {
		t.Errorf("expected user:555, got %s", r1)
	}

	c2 := &Context{
		Update: &maxbot.Update{
			ChatID: 999,
			Message: &maxbot.Message{
				Sender: &maxbot.User{UserID: 555},
			},
		},
	}
	r2, err := c2.Recipient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2.IsUser() || r2.ID() != 999 {
		t.Errorf("expected chat:999, got %s", r2)
	}
}

func TestContextVoiceNoteAttempt(t *testing.T) {
	t.Parallel()
	c := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{
					Text: "[голос с микрофона]",
					Attachments: []maxbot.Attachment{
						{Type: "voice_note_attempt"},
					},
				},
			},
		},
	}

	if !c.IsVoiceNoteAttempt() {
		t.Error("expected IsVoiceNoteAttempt to be true")
	}

	router := NewRouter()
	called := false
	router.OnVoiceNoteAttempt(func(ctx *Context) error {
		called = true
		return nil
	})

	if err := router.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected OnVoiceNoteAttempt handler to be called")
	}
}

func TestContextAudioAttachmentFile(t *testing.T) {
	t.Parallel()
	c := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type: "file",
							Payload: map[string]any{
								"name": "voice_recording.ogg",
								"url":  "https://example.com/audio.ogg",
							},
						},
					},
				},
			},
		},
	}

	att, hasAudio := c.AudioAttachment()
	if !hasAudio || att == nil {
		t.Fatal("expected AudioAttachment to detect .ogg file attachment")
	}
}

func TestContextEditingMethods(t *testing.T) {
	t.Parallel()
	var lastMethod string
	var lastPath string
	var lastBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&lastBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"body": map[string]any{
					"mid":  "mid_123",
					"text": "updated",
				},
			},
		})
	}))
	defer srv.Close()

	client, err := maxbot.NewClient("test-token", maxbot.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	c := &Context{
		client: client,
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCallback,
			Callback: &maxbot.Callback{
				CallbackID: "cb_999",
				Message: &maxbot.Message{
					Body: maxbot.MessageBody{
						MID:  "mid_123",
						Text: "initial",
					},
				},
			},
		},
	}

	kb := maxbot.NewInlineKeyboard().Row(maxbot.NewCallbackButton("btn", "payload"))
	msg, err := c.EditKeyboard("new text", kb)
	if err != nil {
		t.Fatalf("EditKeyboard failed: %v", err)
	}
	if msg == nil || msg.Body.MID != "mid_123" {
		t.Errorf("expected mid_123, got %+v", msg)
	}
	if lastMethod != http.MethodPut || lastPath != "/messages" {
		t.Errorf("expected PUT /messages, got %s %s", lastMethod, lastPath)
	}

	msg, err = c.EditOrReplyKeyboard("edited in place", kb)
	if err != nil {
		t.Fatalf("EditOrReplyKeyboard failed: %v", err)
	}
	if msg == nil || msg.Body.MID != "mid_123" {
		t.Errorf("expected mid_123, got %+v", msg)
	}
	if lastMethod != http.MethodPut || lastPath != "/messages" {
		t.Errorf("expected PUT /messages for callback, got %s %s", lastMethod, lastPath)
	}

	cUserMsg := &Context{
		client: client,
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Sender: &maxbot.User{UserID: 456},
				Recipient: &maxbot.MessageRecipient{
					ChatID: 789,
				},
				Body: maxbot.MessageBody{
					MID:  "user_mid_999",
					Text: "/menu",
				},
			},
		},
	}
	msg, err = cUserMsg.EditOrReplyKeyboard("reply message", kb)
	if err != nil {
		t.Fatalf("EditOrReplyKeyboard on user message failed: %v", err)
	}
	if lastMethod != http.MethodPost || lastPath != "/messages" {
		t.Errorf("expected POST /messages for user command, got %s %s", lastMethod, lastPath)
	}

	msg, err = c.ClearKeyboard()
	if err != nil {
		t.Fatalf("ClearKeyboard failed: %v", err)
	}
	if msg == nil {
		t.Errorf("expected valid message from ClearKeyboard")
	}
}

func TestContextImageAttachmentAndRouting(t *testing.T) {
	t.Parallel()
	c1 := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{
					Text: "Распознай картинку",
					Attachments: []maxbot.Attachment{
						{
							Type: maxbot.UploadTypeImage,
							Payload: map[string]any{
								"url": "https://example.com/photo.jpg",
							},
						},
					},
				},
			},
		},
	}

	att1, hasImg1 := c1.ImageAttachment()
	if !hasImg1 || att1 == nil {
		t.Fatal("expected ImageAttachment to detect image attachment")
	}
	u1, ok1 := c1.ImageURL()
	if !ok1 || u1 != "https://example.com/photo.jpg" {
		t.Fatalf("expected url https://example.com/photo.jpg, got %s", u1)
	}

	c2 := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type: "photo",
							Payload: map[string]any{
								"photos": []any{
									map[string]any{"url": "https://example.com/thumb.jpg"},
									map[string]any{"url": "https://example.com/large.jpg"},
								},
							},
						},
					},
				},
			},
		},
	}
	att2, hasImg2 := c2.ImageAttachment()
	if !hasImg2 || att2 == nil {
		t.Fatal("expected ImageAttachment to detect photo attachment")
	}
	u2, ok2 := c2.ImageURL()
	if !ok2 || u2 != "https://example.com/large.jpg" {
		t.Fatalf("expected highest resolution url https://example.com/large.jpg, got %s", u2)
	}

	c3 := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type: "file",
							Payload: map[string]any{
								"name": "screenshot.png",
								"url":  "https://example.com/screenshot.png",
							},
						},
					},
				},
			},
		},
	}
	att3, hasImg3 := c3.ImageAttachment()
	if !hasImg3 || att3 == nil {
		t.Fatal("expected ImageAttachment to detect .png file as image")
	}

	c4 := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{
					Attachments: []maxbot.Attachment{
						{
							Type: "image",
							Payload: map[string]any{
								"b64_json": "aGVsbG8gd29ybGQ=",
							},
						},
					},
				},
			},
		},
	}
	bytes4, _, err4 := c4.ImageBytes()
	if err4 != nil || string(bytes4) != "hello world" {
		t.Fatalf("expected 'hello world', got %s, err: %v", string(bytes4), err4)
	}

	b := NewWithClient(nil)
	var handledImage bool
	b.OnImage(func(c *Context) error {
		handledImage = true
		return nil
	})

	err := b.Router().Handle(c1)
	if err != nil {
		t.Fatalf("router handle failed: %v", err)
	}
	if !handledImage {
		t.Fatal("expected OnImage handler to be invoked")
	}

	testFormats := []struct {
		name    string
		attType string
		payload map[string]any
		isImage bool
	}{
		{name: "webp_file", attType: "file", payload: map[string]any{"name": "photo.webp"}, isImage: true},
		{name: "avif_file", attType: "file", payload: map[string]any{"name": "banner.avif"}, isImage: true},
		{name: "jfif_file", attType: "file", payload: map[string]any{"name": "camera.jfif"}, isImage: true},
		{name: "heif_document", attType: "document", payload: map[string]any{"name": "shot.heif"}, isImage: true},
		{name: "svg_document", attType: "document", payload: map[string]any{"name": "icon.svg"}, isImage: true},
		{name: "dng_raw_document", attType: "document", payload: map[string]any{"name": "raw.dng"}, isImage: true},
		{name: "sticker_type", attType: "sticker", payload: map[string]any{"url": "https://example.com/sticker"}, isImage: true},
		{name: "document_mime_webp", attType: "document", payload: map[string]any{"mime_type": "image/webp"}, isImage: true},
		{name: "url_with_query_webp", attType: "file", payload: map[string]any{"url": "https://example.com/img.webp?token=xyz"}, isImage: true},
		{name: "text_file_not_image", attType: "file", payload: map[string]any{"name": "readme.txt"}, isImage: false},
	}

	for _, tc := range testFormats {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testCtx := &Context{
				Update: &maxbot.Update{
					UpdateType: maxbot.UpdateTypeMessageCreated,
					Message: &maxbot.Message{
						Body: maxbot.MessageBody{
							Attachments: []maxbot.Attachment{
								{
									Type:    tc.attType,
									Payload: tc.payload,
								},
							},
						},
					},
				},
			}
			_, detected := testCtx.ImageAttachment()
			if detected != tc.isImage {
				t.Errorf("expected ImageAttachment=%v, got %v", tc.isImage, detected)
			}
		})
	}
}
