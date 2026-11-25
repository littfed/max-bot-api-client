package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestRouterDispatchAndMiddlewares(t *testing.T) {
	t.Parallel()
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

func TestBotLongPollingCycle(t *testing.T) {
	t.Parallel()
	var pollCount atomic.Int32

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
			count := pollCount.Add(1)
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

	pollingDone := make(chan struct{})
	go func() {
		defer close(pollingDone)
		_ = b.StartLongPolling(ctx)
	}()

	select {
	case <-pingReplied:
		// Ping command was dispatched and replied!
		cancel()
		<-pollingDone
	case <-time.After(3 * time.Second):
		cancel()
		<-pollingDone
		t.Fatal("timed out waiting for ping command to be processed")
	}
}

func TestBot_NewAndOptions(t *testing.T) {
	t.Parallel()

	customLogger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	b, err := New("test-token",
		WithLogger(customLogger),
		WithPollingTimeout(45),
		WithMaxConcurrency(16),
		WithPollingInterval(200*time.Millisecond),
		WithInitialMarker(99),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if b.Client() == nil || b.Router() == nil {
		t.Fatal("expected non-nil Client and Router")
	}
	if b.logger != customLogger {
		t.Error("WithLogger option did not apply")
	}
	if b.pollingTimeout != 45 || b.maxConcurrency != 16 || b.pollingInterval != 200*time.Millisecond || b.marker != 99 {
		t.Errorf("options mismatch: timeout=%d conc=%d interval=%v marker=%d",
			b.pollingTimeout, b.maxConcurrency, b.pollingInterval, b.marker)
	}
}

func TestBot_WebhookHandler(t *testing.T) {
	t.Parallel()

	b := NewWithClient(nil)
	var gotText string
	b.OnCommand("hello", func(c *Context) error {
		gotText = c.Text()
		return nil
	})

	tests := []struct {
		name       string
		secret     string
		hdrSecret  string
		body       string
		wantStatus int
		wantText   string
	}{
		{
			name:       "empty_configured_secret_unauthorized",
			secret:     "",
			hdrSecret:  "any",
			body:       `{}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong_header_secret_unauthorized",
			secret:     "my-secret",
			hdrSecret:  "wrong",
			body:       `{}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid_json_bad_request",
			secret:     "my-secret",
			hdrSecret:  "my-secret",
			body:       `{bad-json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid_webhook_dispatched",
			secret:     "my-secret",
			hdrSecret:  "my-secret",
			body:       `{"update_type":"message_created","message":{"body":{"text":"/hello world"}}}`,
			wantStatus: http.StatusOK,
			wantText:   "/hello world",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotText = ""
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(tc.body))
			if tc.hdrSecret != "" {
				req.Header.Set("X-Max-Bot-Api-Secret", tc.hdrSecret)
			}
			w := httptest.NewRecorder()
			b.WebhookHandler(tc.secret)(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if tc.wantText != "" && gotText != tc.wantText {
				t.Errorf("gotText = %q, want %q", gotText, tc.wantText)
			}
		})
	}
}

func TestBot_RecordActiveChatAndProcessUpdate(t *testing.T) {
	t.Parallel()

	b := NewWithClient(nil)
	b.RecordActiveChat(100, 200)
	b.RecordActiveChat(0, 200) // ignored

	b.recentChatsMu.RLock()
	uid, ok := b.recentChats[100]
	b.recentChatsMu.RUnlock()
	if !ok || uid != 200 {
		t.Fatalf("expected recentChats[100]=200, got %d (ok=%v)", uid, ok)
	}

	var handled bool
	b.OnCommand("ping", func(c *Context) error {
		handled = true
		return nil
	})
	err := b.ProcessUpdate(context.Background(), &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender:    &maxbot.User{UserID: 300},
			Recipient: &maxbot.MessageRecipient{ChatID: 400},
			Body:      maxbot.MessageBody{Text: "/ping"},
		},
	})
	if err != nil {
		t.Fatalf("ProcessUpdate failed: %v", err)
	}
	if !handled {
		t.Fatal("expected handler to be called")
	}
}
