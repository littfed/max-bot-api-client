package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/platform"
)

func TestCleanRecipientID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"plain", "12345", "12345"},
		{"user_prefix", "user:12345", "12345"},
		{"chat_prefix", "chat:-100999", "-100999"},
		{"user_underscore", "user_555", "555"},
		{"whitespace", "  user_777  ", "777"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := cleanRecipientID(tc.raw); got != tc.want {
				t.Errorf("cleanRecipientID(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestTelegramAdapter_Methods(t *testing.T) {
	t.Parallel()

	var lastMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bottest-token/sendMessage":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":     true,
				"result": map[string]any{"message_id": 42},
			})
		case "/bottest-token/getChatMember":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":     true,
				"result": map[string]any{"status": "member"},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
		}
	}))
	defer srv.Close()

	adapter := NewAdapter("test-token", WithBaseURL(srv.URL))
	if adapter.Platform() != platform.PlatformTelegram {
		t.Fatalf("Platform() = %q, want %q", adapter.Platform(), platform.PlatformTelegram)
	}

	ctx := context.Background()
	mid, err := adapter.SendMessage(ctx, "user:100", &platform.OutMessage{
		Text: "Hello",
		Buttons: [][]platform.Button{
			{{Text: "Open", URL: "https://example.com"}, {Text: "Click", Action: "do:1"}},
		},
	})
	if err != nil || mid != "100:0" {
		t.Fatalf("SendMessage failed: mid=%q err=%v", mid, err)
	}

	if err := adapter.EditMessage(ctx, "100:42", &platform.OutMessage{Text: "Edited"}); err != nil {
		t.Fatalf("EditMessage failed: %v", err)
	}
	if err := adapter.AnswerCallback(ctx, "cb_1", "OK"); err != nil {
		t.Fatalf("AnswerCallback failed: %v", err)
	}
	isMember, err := adapter.CheckMembership(ctx, "-1001", "100")
	if err != nil || !isMember {
		t.Fatalf("CheckMembership failed: isMember=%v err=%v", isMember, err)
	}
	if err := adapter.SendChatAction(ctx, "100", "typing_on"); err != nil {
		t.Fatalf("SendChatAction failed: %v (lastMethod=%s)", err, lastMethod)
	}

	// Error cases
	if _, err := adapter.SendMessage(ctx, "100", nil); err == nil {
		t.Error("expected error for nil body")
	}
	if _, err := adapter.SendMessage(ctx, "", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("expected error for empty recipientID")
	}
}
