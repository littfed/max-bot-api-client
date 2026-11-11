package maxbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRecipientAddressing(t *testing.T) {
	userRec := ToUser(12345)
	if !userRec.IsUser() {
		t.Errorf("expected userRec.IsUser() to be true")
	}
	if userRec.ID() != 12345 {
		t.Errorf("expected ID 12345, got %d", userRec.ID())
	}
	qUser := make(url.Values)
	userRec.ApplyRecipient(qUser)
	if qUser.Get("user_id") != "12345" || qUser.Get("chat_id") != "" {
		t.Errorf("unexpected query values for user: %v", qUser)
	}

	chatRec := ToChat(67890)
	if chatRec.IsUser() {
		t.Errorf("expected chatRec.IsUser() to be false")
	}
	if chatRec.ID() != 67890 {
		t.Errorf("expected ID 67890, got %d", chatRec.ID())
	}
	qChat := make(url.Values)
	chatRec.ApplyRecipient(qChat)
	if qChat.Get("chat_id") != "67890" || qChat.Get("user_id") != "" {
		t.Errorf("unexpected query values for chat: %v", qChat)
	}
}

func TestKeyboardBuilder(t *testing.T) {
	kb := NewInlineKeyboard().
		Row(
			NewIntentCallbackButton("Positive", "payload_pos", IntentPositive),
			NewCallbackButton("Button 1", "payload_1").WithIntent(IntentNegative),
			NewLinkButton("Website", "https://example.com"),
		).
		Row(
			NewOpenAppButton("Open App", "https://app.example.com"),
			NewClipboardButton("Copy", "data123"),
		).
		Row(
			NewContactButton("Phone"),
			NewLocationButton("GPS", true),
		)

	if len(kb.Rows()) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.Rows()))
	}
	if len(kb.Rows()[0]) != 3 {
		t.Errorf("expected 3 buttons in row 0, got %d", len(kb.Rows()[0]))
	}
	if kb.Rows()[0][0].Intent != IntentPositive {
		t.Errorf("expected intent %s, got %s", IntentPositive, kb.Rows()[0][0].Intent)
	}

	att := kb.ToAttachment()
	if att.Type != "inline_keyboard" {
		t.Errorf("expected attachment type 'inline_keyboard', got '%s'", att.Type)
	}

	parsed, err := ParseKeyboardPayload(att)
	if err != nil {
		t.Fatalf("failed to parse keyboard payload: %v", err)
	}
	if len(parsed.Buttons) != 3 {
		t.Errorf("expected 3 parsed button rows, got %d", len(parsed.Buttons))
	}
}

func TestClientMockServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify raw token in Authorization header
		if r.Header.Get("Authorization") != "test-token-123" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(APIError{
				Code:    "verify.token",
				Message: "Invalid token",
			})
			return
		}

		switch r.URL.Path {
		case "/me":
			_ = json.NewEncoder(w).Encode(User{
				UserID:    1001,
				Name:      "Test Bot",
				Username:  "test_bot",
				IsBot:     true,
			})

		case "/messages":
			switch r.Method {
			case http.MethodPost:
				var req SendMessageRequest
				_ = json.NewDecoder(r.Body).Decode(&req)

				recipientQuery := r.URL.Query()
				var recip *MessageRecipient
				if u := recipientQuery.Get("user_id"); u != "" {
					recip = &MessageRecipient{UserID: 12345, ChatType: "dialog"}
				} else if c := recipientQuery.Get("chat_id"); c != "" {
					recip = &MessageRecipient{ChatID: 67890, ChatType: "chat"}
				}

				_ = json.NewEncoder(w).Encode(Message{
					MessageID: "msg-999",
					Timestamp: 1600000000,
					Body: MessageBody{
						Text:        req.Text,
						Format:      req.Format,
						Attachments: req.Attachments,
					},
					Recipient: recip,
				})

			case http.MethodPut:
				_ = json.NewEncoder(w).Encode(Message{
					MessageID: r.URL.Query().Get("message_id"),
					Timestamp: 1600000001,
					Body:      MessageBody{Text: "edited"},
				})

			case http.MethodDelete:
				w.WriteHeader(http.StatusOK)
			}

		case "/answers":
			if r.URL.Query().Get("callback_id") != "cb-42" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)

		case "/chats/67890/actions":
			w.WriteHeader(http.StatusOK)

		case "/chats/67890/pin":
			w.WriteHeader(http.StatusOK)

		case "/chats/67890":
			_ = json.NewEncoder(w).Encode(Chat{
				ChatID: 67890,
				Type:   "chat",
				Title:  "Test Group",
			})

		case "/updates":
			_ = json.NewEncoder(w).Encode(UpdatesResponse{
				Marker: 105,
				Updates: []Update{
					{
						UpdateType: UpdateTypeMessageCreated,
						Message: &Message{
							MessageID: "msg-1",
							Body:      MessageBody{Text: "hello"},
						},
					},
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client, err := NewClient("test-token-123",
		WithBaseURL(ts.URL),
		WithRussianCerts(false), // Local httptest server does not need Russian root certs
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// 1. Test GetMe
	me, err := client.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe failed: %v", err)
	}
	if me.Username != "test_bot" || me.UserID != 1001 {
		t.Errorf("unexpected user: %+v", me)
	}

	// 2. Test SendTextMessage
	msg, err := client.SendTextMessage(ctx, ToUser(12345), "Hello User")
	if err != nil {
		t.Fatalf("SendTextMessage failed: %v", err)
	}
	if msg.MessageID != "msg-999" || msg.Body.Text != "Hello User" {
		t.Errorf("unexpected message: %+v", msg)
	}

	// 3. Test EditMessage
	edited, err := client.EditMessage(ctx, "msg-999", &EditMessageRequest{Text: "edited"})
	if err != nil {
		t.Fatalf("EditMessage failed: %v", err)
	}
	if edited.Body.Text != "edited" {
		t.Errorf("unexpected edit result: %+v", edited)
	}

	// 4. Test DeleteMessage
	if err := client.DeleteMessage(ctx, "msg-999"); err != nil {
		t.Fatalf("DeleteMessage failed: %v", err)
	}

	// 5. Test AnswerCallback
	err = client.AnswerCallback(ctx, "cb-42", &AnswerCallbackRequest{
		Notification: "Action completed",
	})
	if err != nil {
		t.Fatalf("AnswerCallback failed: %v", err)
	}

	// 6. Test GetUpdates
	updates, err := client.GetUpdates(ctx, &GetUpdatesOptions{Marker: 100})
	if err != nil {
		t.Fatalf("GetUpdates failed: %v", err)
	}
	if len(updates.Updates) != 1 || updates.Marker != 105 {
		t.Errorf("unexpected updates: %+v", updates)
	}

	// 7. Test SendChatAction
	if err := client.SendChatAction(ctx, 67890, ActionTypingOn); err != nil {
		t.Fatalf("SendChatAction failed: %v", err)
	}

	// 8. Test PinMessage & UnpinMessage
	if err := client.PinMessage(ctx, 67890, "msg-999", true); err != nil {
		t.Fatalf("PinMessage failed: %v", err)
	}
	if err := client.UnpinMessage(ctx, 67890); err != nil {
		t.Fatalf("UnpinMessage failed: %v", err)
	}

	// 9. Test GetChat
	chat, err := client.GetChat(ctx, 67890)
	if err != nil {
		t.Fatalf("GetChat failed: %v", err)
	}
	if chat.ChatID != 67890 || chat.Title != "Test Group" {
		t.Errorf("unexpected chat: %+v", chat)
	}
}

func TestClientAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(APIError{
			Code:    "verify.token",
			Message: "No access token",
		})
	}))
	defer ts.Close()

	client, err := NewClient("bad-token", WithBaseURL(ts.URL), WithRussianCerts(false))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !IsUnauthorized(err) {
		t.Errorf("expected IsUnauthorized(err) to be true, got %v", err)
	}
}
