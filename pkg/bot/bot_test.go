package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestContextCommandParsing(t *testing.T) {
	cases := []struct {
		input       string
		expectedCmd string
		expectedArg string
	}{
		{"/ping", "ping", ""},
		{"/ping@my_bot", "ping", ""},
		{"/echo hello world", "echo", "hello world"},
		{"/start ref_12345", "start", "ref_12345"},
		{"hello without slash", "", ""},
	}

	for _, tc := range cases {
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
	}
}

func TestContextStartPayload(t *testing.T) {
	// Case 1: bot_started update with payload field
	c1 := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeBotStarted,
			Payload:    "ref_u999",
		},
	}
	if p := c1.StartPayload(); p != "ref_u999" {
		t.Errorf("expected ref_u999, got %s", p)
	}

	// Case 2: message_created update with /start command
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

func TestRouterDispatchAndMiddlewares(t *testing.T) {
	router := NewRouter()

	var order []string
	router.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			order = append(order, "mw1_before")
			err := next(c)
			order = append(order, "mw1_after")
			return err
		}
	})

	var pingHandled bool
	router.OnCommand("ping", func(c *Context) error {
		pingHandled = true
		order = append(order, "handler_ping")
		return nil
	})

	c := &Context{
		Update: &maxbot.Update{
			UpdateType: maxbot.UpdateTypeMessageCreated,
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{Text: "/ping"},
			},
		},
	}

	err := router.Handle(c)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !pingHandled {
		t.Fatal("expected ping handler to be called")
	}

	expectedOrder := []string{"mw1_before", "handler_ping", "mw1_after"}
	if len(order) != len(expectedOrder) {
		t.Fatalf("unexpected execution order: %v", order)
	}
	for i := range order {
		if order[i] != expectedOrder[i] {
			t.Errorf("at index %d: expected %s, got %s", i, expectedOrder[i], order[i])
		}
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	router := NewRouter()
	router.Use(Recovery(nil))

	router.OnCommand("crash", func(c *Context) error {
		panic("fatal boom!")
	})

	c := &Context{
		Update: &maxbot.Update{
			Message: &maxbot.Message{
				Body: maxbot.MessageBody{Text: "/crash"},
			},
		},
	}

	err := router.Handle(c)
	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
}

func TestBotLongPollingCycle(t *testing.T) {
	var pollCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me":
			_ = json.NewEncoder(w).Encode(maxbot.User{
				UserID:   1001,
				Name:     "PingBot",
				Username: "ping_bot",
				IsBot:    true,
			})
		case "/updates":
			count := atomic.AddInt32(&pollCount, 1)
			if count == 1 {
				_ = json.NewEncoder(w).Encode(maxbot.UpdatesResponse{
					Marker: 10,
					Updates: []maxbot.Update{
						{
							UpdateType: maxbot.UpdateTypeMessageCreated,
							Message: &maxbot.Message{
								MessageID: "msg-1",
								Body:      maxbot.MessageBody{Text: "/ping"},
								Sender:    &maxbot.User{UserID: 777},
							},
						},
					},
				})
			} else {
				// Empty response on subsequent polls
				_ = json.NewEncoder(w).Encode(maxbot.UpdatesResponse{
					Marker:  11,
					Updates: []maxbot.Update{},
				})
			}
		case "/messages":
			_ = json.NewEncoder(w).Encode(maxbot.Message{
				MessageID: "msg-reply",
				Body:      maxbot.MessageBody{Text: "pong! 🏓"},
			})
		}
	}))
	defer ts.Close()

	client, err := maxbot.NewClient("fake-token",
		maxbot.WithBaseURL(ts.URL),
		maxbot.WithRussianCerts(false),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	b := NewWithClient(client, WithPollingTimeout(1))

	pingReplied := make(chan struct{}, 1)
	b.OnCommand("ping", func(c *Context) error {
		_, err := c.Reply("pong! 🏓")
		if err != nil {
			t.Errorf("Reply failed: %v", err)
		}
		pingReplied <- struct{}{}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = b.StartLongPolling(ctx)
	}()

	select {
	case <-pingReplied:
		// Ping command was dispatched and replied!
		cancel()
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("timed out waiting for ping command to be processed")
	}
}

func TestContextRecipientDerivation(t *testing.T) {
	// Case 1: Private message from sender
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

	// Case 2: Group chat
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

	// 1. Test EditKeyboard
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

	// 2. Test EditOrReplyKeyboard in-place
	msg, err = c.EditOrReplyKeyboard("edited in place", kb)
	if err != nil {
		t.Fatalf("EditOrReplyKeyboard failed: %v", err)
	}
	if msg == nil || msg.Body.MID != "mid_123" {
		t.Errorf("expected mid_123, got %+v", msg)
	}

	// 3. Test ClearKeyboard
	msg, err = c.ClearKeyboard()
	if err != nil {
		t.Fatalf("ClearKeyboard failed: %v", err)
	}
	if msg == nil {
		t.Errorf("expected valid message from ClearKeyboard")
	}
}

func TestContextImageAttachmentAndRouting(t *testing.T) {
	// 1. Direct image upload attachment
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

	// 2. Photos array variant
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

	// 3. File attachment with image extension (.png)
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

	// 4. Base64 payload decoding via ImageBytes
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

	// 5. Router OnImage matching
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

	// 6. Test various image formats (.webp, .avif, .jfif, .svg, .heif, .dng)
	testFormats := []struct {
		name     string
		attType  string
		payload  map[string]any
		isImage  bool
	}{
		{name: "webp file", attType: "file", payload: map[string]any{"name": "photo.webp"}, isImage: true},
		{name: "avif file", attType: "file", payload: map[string]any{"name": "banner.avif"}, isImage: true},
		{name: "jfif file", attType: "file", payload: map[string]any{"name": "camera.jfif"}, isImage: true},
		{name: "heif document", attType: "document", payload: map[string]any{"name": "shot.heif"}, isImage: true},
		{name: "svg document", attType: "document", payload: map[string]any{"name": "icon.svg"}, isImage: true},
		{name: "dng raw document", attType: "document", payload: map[string]any{"name": "raw.dng"}, isImage: true},
		{name: "sticker type", attType: "sticker", payload: map[string]any{"url": "https://example.com/sticker"}, isImage: true},
		{name: "document with mime image/webp", attType: "document", payload: map[string]any{"mime_type": "image/webp"}, isImage: true},
		{name: "url with query and webp", attType: "file", payload: map[string]any{"url": "https://example.com/img.webp?token=xyz"}, isImage: true},
		{name: "text file is not image", attType: "file", payload: map[string]any{"name": "readme.txt"}, isImage: false},
	}

	for _, tc := range testFormats {
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
			t.Errorf("[%s] expected ImageAttachment=%v, got %v", tc.name, tc.isImage, detected)
		}
	}
}
