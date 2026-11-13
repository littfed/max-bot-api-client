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
