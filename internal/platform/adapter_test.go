package platform_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/internal/platform/max"
	"github.com/littfed/max-bot-api-client/internal/platform/telegram"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// mockMaxBotClient implements max.BotClient for testing.
type mockMaxBotClient struct {
	sentMessages     []*maxbot.SendMessageRequest
	sentRecipients   []maxbot.Recipient
	editedMessages   []*maxbot.EditMessageRequest
	editedIDs        []string
	answeredCallbacks map[string]string // callbackID -> text
	chatMembers      map[int64][]maxbot.ChatMember
	sendErr          error
	editErr          error
	answerErr        error
	getMembersErr    error
}

func (m *mockMaxBotClient) SendMessage(ctx context.Context, recipient maxbot.Recipient, msg *maxbot.SendMessageRequest) (*maxbot.Message, error) {
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	m.sentRecipients = append(m.sentRecipients, recipient)
	m.sentMessages = append(m.sentMessages, msg)
	return &maxbot.Message{MessageID: "msg_123"}, nil
}

func (m *mockMaxBotClient) EditMessage(ctx context.Context, messageID string, msg *maxbot.EditMessageRequest) (*maxbot.Message, error) {
	if m.editErr != nil {
		return nil, m.editErr
	}
	m.editedIDs = append(m.editedIDs, messageID)
	m.editedMessages = append(m.editedMessages, msg)
	return &maxbot.Message{MessageID: messageID}, nil
}

func (m *mockMaxBotClient) AnswerCallback(ctx context.Context, callbackID string, answer *maxbot.AnswerCallbackRequest) error {
	if m.answerErr != nil {
		return m.answerErr
	}
	if m.answeredCallbacks == nil {
		m.answeredCallbacks = make(map[string]string)
	}
	text := ""
	if answer != nil {
		text = answer.Notification
	}
	m.answeredCallbacks[callbackID] = text
	return nil
}

func (m *mockMaxBotClient) GetChatMembers(ctx context.Context, chatID int64, marker ...int64) (*maxbot.ChatMembersResponse, error) {
	if m.getMembersErr != nil {
		return nil, m.getMembersErr
	}
	members := m.chatMembers[chatID]
	return &maxbot.ChatMembersResponse{Members: members}, nil
}

func (m *mockMaxBotClient) SendChatAction(ctx context.Context, chatID int64, action string) error {
	return nil
}

type mockPaginationStuckClient struct {
	calls int
}

func (m *mockPaginationStuckClient) SendMessage(ctx context.Context, recipient maxbot.Recipient, msg *maxbot.SendMessageRequest) (*maxbot.Message, error) {
	return nil, nil
}
func (m *mockPaginationStuckClient) EditMessage(ctx context.Context, messageID string, msg *maxbot.EditMessageRequest) (*maxbot.Message, error) {
	return nil, nil
}
func (m *mockPaginationStuckClient) AnswerCallback(ctx context.Context, callbackID string, answer *maxbot.AnswerCallbackRequest) error {
	return nil
}
func (m *mockPaginationStuckClient) SendChatAction(ctx context.Context, chatID int64, action string) error {
	return nil
}
func (m *mockPaginationStuckClient) GetChatMembers(ctx context.Context, chatID int64, marker ...int64) (*maxbot.ChatMembersResponse, error) {
	m.calls++
	if m.calls > 5 {
		panic("infinite loop detected in GetChatMembers")
	}
	return &maxbot.ChatMembersResponse{
		Members: []maxbot.ChatMember{{UserID: 1}},
		Marker:  5,
	}, nil
}

func TestMAXAdapter_SendMessage(t *testing.T) {
	mockClient := &mockMaxBotClient{}
	adapter := max.NewAdapter(mockClient)
	if adapter.Platform() != "max" {
		t.Errorf("Expected platform 'max', got %s", adapter.Platform())
	}

	msg := &platform.OutMessage{
		Text: "Hello Bot Hub!",
		Buttons: [][]platform.Button{
			{{Text: "Сменить режим", Action: "change_mode"}},
		},
	}
	sentMID, err := adapter.SendMessage(context.Background(), "user_42", msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if sentMID != "msg_123" {
		t.Errorf("Expected sentMID 'msg_123', got %s", sentMID)
	}
	if len(mockClient.sentMessages) != 1 {
		t.Errorf("Expected 1 sent message, got %d", len(mockClient.sentMessages))
	}
	if len(mockClient.sentRecipients) != 1 {
		t.Fatalf("Expected 1 recipient, got %d", len(mockClient.sentRecipients))
	}
	if !mockClient.sentRecipients[0].IsUser() || mockClient.sentRecipients[0].ID() != 42 {
		t.Errorf("Expected recipient user:42, got %s", mockClient.sentRecipients[0].String())
	}
}

func TestMAXAdapter_RecipientTypes(t *testing.T) {
	mockClient := &mockMaxBotClient{}
	adapter := max.NewAdapter(mockClient)

	testCases := []struct {
		recipientID string
		isUser      bool
		expectedID  int64
	}{
		{"user_100", true, 100},
		{"user:200", true, 200},
		{"300", true, 300},
		{"chat_400", false, 400},
		{"chat:500", false, 500},
		{"-600", false, -600},
	}

	for _, tc := range testCases {
		_, err := adapter.SendMessage(context.Background(), tc.recipientID, &platform.OutMessage{Text: "test"})
		if err != nil {
			t.Fatalf("SendMessage(%s) failed: %v", tc.recipientID, err)
		}
		lastRec := mockClient.sentRecipients[len(mockClient.sentRecipients)-1]
		if lastRec.IsUser() != tc.isUser {
			t.Errorf("[%s] Expected isUser=%v, got %v", tc.recipientID, tc.isUser, lastRec.IsUser())
		}
		if lastRec.ID() != tc.expectedID {
			t.Errorf("[%s] Expected ID=%d, got %d", tc.recipientID, tc.expectedID, lastRec.ID())
		}
	}
}

func TestMAXAdapter_ButtonTypes(t *testing.T) {
	mockClient := &mockMaxBotClient{}
	adapter := max.NewAdapter(mockClient)

	msg := &platform.OutMessage{
		Text: "Options",
		Buttons: [][]platform.Button{
			{
				{Text: "Callback", Action: "do_action"},
				{Text: "Link", URL: "https://example.com"},
				{Text: "MiniApp", WebApp: "https://app.example.com"},
			},
		},
	}

	_, err := adapter.SendMessage(context.Background(), "user_1", msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	sentMsg := mockClient.sentMessages[0]
	if len(sentMsg.Attachments) != 1 {
		t.Fatalf("Expected 1 attachment (inline keyboard), got %d", len(sentMsg.Attachments))
	}
	att := sentMsg.Attachments[0]
	if att.Type != "inline_keyboard" {
		t.Fatalf("Expected attachment type 'inline_keyboard', got %s", att.Type)
	}
	kbPayload, err := maxbot.ParseKeyboardPayload(att)
	if err != nil {
		t.Fatalf("Failed to parse keyboard payload: %v", err)
	}
	if len(kbPayload.Buttons) != 1 || len(kbPayload.Buttons[0]) != 3 {
		t.Fatalf("Expected 1 row with 3 buttons, got %v", kbPayload.Buttons)
	}

	btn0 := kbPayload.Buttons[0][0]
	if btn0.Type != maxbot.ButtonTypeCallback || btn0.Payload != "do_action" {
		t.Errorf("Unexpected btn0: %+v", btn0)
	}
	btn1 := kbPayload.Buttons[0][1]
	if btn1.Type != maxbot.ButtonTypeLink || btn1.URL != "https://example.com" {
		t.Errorf("Unexpected btn1: %+v", btn1)
	}
	btn2 := kbPayload.Buttons[0][2]
	if btn2.Type != maxbot.ButtonTypeOpenApp || btn2.WebApp != "https://app.example.com" {
		t.Errorf("Unexpected btn2: %+v", btn2)
	}
}

func TestMAXAdapter_EditMessage(t *testing.T) {
	mockClient := &mockMaxBotClient{}
	adapter := max.NewAdapter(mockClient)

	msg := &platform.OutMessage{
		Text: "Updated Text",
		Buttons: [][]platform.Button{
			{{Text: "Next", Action: "next_step"}},
		},
	}

	err := adapter.EditMessage(context.Background(), "mid_999", msg)
	if err != nil {
		t.Fatalf("EditMessage failed: %v", err)
	}
	if len(mockClient.editedIDs) != 1 || mockClient.editedIDs[0] != "mid_999" {
		t.Errorf("Expected edited message mid_999, got %v", mockClient.editedIDs)
	}
	if mockClient.editedMessages[0].Text != "Updated Text" {
		t.Errorf("Expected text 'Updated Text', got %s", mockClient.editedMessages[0].Text)
	}
}

func TestMAXAdapter_AnswerCallback(t *testing.T) {
	mockClient := &mockMaxBotClient{}
	adapter := max.NewAdapter(mockClient)

	err := adapter.AnswerCallback(context.Background(), "cb_777", "Done!")
	if err != nil {
		t.Fatalf("AnswerCallback failed: %v", err)
	}
	if mockClient.answeredCallbacks["cb_777"] != "Done!" {
		t.Errorf("Expected answer 'Done!', got %s", mockClient.answeredCallbacks["cb_777"])
	}
}

func TestMAXAdapter_CheckMembership(t *testing.T) {
	mockClient := &mockMaxBotClient{
		chatMembers: map[int64][]maxbot.ChatMember{
			1001: {
				{UserID: 42, FirstName: "Alex"},
				{UserID: 99, FirstName: "Bob"},
			},
		},
	}
	adapter := max.NewAdapter(mockClient)

	// User 42 is in chat 1001
	isMember, err := adapter.CheckMembership(context.Background(), "chat_1001", "user_42")
	if err != nil || !isMember {
		t.Errorf("Expected user 42 to be member: isMember=%v, err=%v", isMember, err)
	}

	// User 50 is not in chat 1001
	isMember, err = adapter.CheckMembership(context.Background(), "1001", "50")
	if err != nil || isMember {
		t.Errorf("Expected user 50 not to be member: isMember=%v, err=%v", isMember, err)
	}

	// Pagination loop termination guard (resp.Marker == marker)
	stuckClient := &mockPaginationStuckClient{}
	stuckAdapter := max.NewAdapter(stuckClient)
	isMember, err = stuckAdapter.CheckMembership(context.Background(), "1001", "999")
	if err != nil || isMember {
		t.Errorf("Expected stuck pagination to terminate with false, got isMember=%v, err=%v", isMember, err)
	}
}

// Telegram Adapter Tests

func TestTelegramAdapter_SendMessage(t *testing.T) {
	var requestedPath string
	var requestedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&requestedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"result":{"message_id":123}}`)
	}))
	defer srv.Close()

	adapter := telegram.NewAdapter("test-token",
		telegram.WithBaseURL(srv.URL),
		telegram.WithHTTPClient(srv.Client()),
	)

	if adapter.Platform() != "telegram" {
		t.Errorf("Expected platform 'telegram', got %s", adapter.Platform())
	}

	msg := &platform.OutMessage{
		Text: "Hello Telegram!",
		Buttons: [][]platform.Button{
			{
				{Text: "Tap Me", Action: "tg_tap"},
				{Text: "Site", URL: "https://tg.example.com"},
				{Text: "App", WebApp: "https://tgapp.example.com"},
			},
		},
	}

	_, err := adapter.SendMessage(context.Background(), "chat_9999", msg)
	if err != nil {
		t.Fatalf("Telegram SendMessage failed: %v", err)
	}

	if requestedPath != "/bottest-token/sendMessage" {
		t.Errorf("Expected endpoint /bottest-token/sendMessage, got %s", requestedPath)
	}
	if requestedBody["chat_id"] != "9999" {
		t.Errorf("Expected chat_id '9999', got %v", requestedBody["chat_id"])
	}
	if requestedBody["text"] != "Hello Telegram!" {
		t.Errorf("Expected text 'Hello Telegram!', got %v", requestedBody["text"])
	}

	replyMarkup, ok := requestedBody["reply_markup"].(map[string]any)
	if !ok {
		t.Fatalf("Expected reply_markup in request body: %+v", requestedBody)
	}
	inlineKb, ok := replyMarkup["inline_keyboard"].([]any)
	if !ok || len(inlineKb) != 1 {
		t.Fatalf("Expected 1 row of inline keyboard: %+v", replyMarkup)
	}
	row := inlineKb[0].([]any)
	if len(row) != 3 {
		t.Fatalf("Expected 3 buttons in row: %+v", row)
	}

	btn0 := row[0].(map[string]any)
	if btn0["text"] != "Tap Me" || btn0["callback_data"] != "tg_tap" {
		t.Errorf("Unexpected btn0: %+v", btn0)
	}

	btn1 := row[1].(map[string]any)
	if btn1["text"] != "Site" || btn1["url"] != "https://tg.example.com" {
		t.Errorf("Unexpected btn1: %+v", btn1)
	}

	btn2 := row[2].(map[string]any)
	webAppObj, ok := btn2["web_app"].(map[string]any)
	if !ok || webAppObj["url"] != "https://tgapp.example.com" {
		t.Errorf("Unexpected btn2 web_app: %+v", btn2)
	}
}

func TestTelegramAdapter_EditMessage(t *testing.T) {
	var requestedPath string
	var requestedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&requestedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"result":{"message_id":456}}`)
	}))
	defer srv.Close()

	adapter := telegram.NewAdapter("test-token",
		telegram.WithBaseURL(srv.URL),
		telegram.WithHTTPClient(srv.Client()),
	)

	// Composite ID "chatID:messageID"
	msg := &platform.OutMessage{
		Text: "Edited Telegram Text",
	}
	err := adapter.EditMessage(context.Background(), "9999:456", msg)
	if err != nil {
		t.Fatalf("EditMessage failed: %v", err)
	}

	if requestedPath != "/bottest-token/editMessageText" {
		t.Errorf("Expected endpoint /bottest-token/editMessageText, got %s", requestedPath)
	}
	if requestedBody["chat_id"] != "9999" {
		t.Errorf("Expected chat_id '9999', got %v", requestedBody["chat_id"])
	}
	if fmt.Sprintf("%v", requestedBody["message_id"]) != "456" {
		t.Errorf("Expected message_id 456, got %v", requestedBody["message_id"])
	}
}

func TestTelegramAdapter_AnswerCallback(t *testing.T) {
	var requestedPath string
	var requestedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&requestedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"result":true}`)
	}))
	defer srv.Close()

	adapter := telegram.NewAdapter("test-token",
		telegram.WithBaseURL(srv.URL),
		telegram.WithHTTPClient(srv.Client()),
	)

	err := adapter.AnswerCallback(context.Background(), "cq_123", "Acknowledged")
	if err != nil {
		t.Fatalf("AnswerCallback failed: %v", err)
	}

	if requestedPath != "/bottest-token/answerCallbackQuery" {
		t.Errorf("Expected endpoint /bottest-token/answerCallbackQuery, got %s", requestedPath)
	}
	if requestedBody["callback_query_id"] != "cq_123" {
		t.Errorf("Expected callback_query_id 'cq_123', got %v", requestedBody["callback_query_id"])
	}
	if requestedBody["text"] != "Acknowledged" {
		t.Errorf("Expected text 'Acknowledged', got %v", requestedBody["text"])
	}
}

func TestTelegramAdapter_CheckMembership(t *testing.T) {
	var requestedPath string

	statusToReturn := "member"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"result":{"status":%q}}`+"\n", statusToReturn)
	}))
	defer srv.Close()

	adapter := telegram.NewAdapter("test-token",
		telegram.WithBaseURL(srv.URL),
		telegram.WithHTTPClient(srv.Client()),
	)

	// Case 1: Active member
	isMember, err := adapter.CheckMembership(context.Background(), "-10012345", "42")
	if err != nil || !isMember {
		t.Errorf("Expected isMember=true, got %v (err: %v)", isMember, err)
	}
	if requestedPath != "/bottest-token/getChatMember" {
		t.Errorf("Expected /bottest-token/getChatMember, got %s", requestedPath)
	}

	// Case 2: Left
	statusToReturn = "left"
	isMember, err = adapter.CheckMembership(context.Background(), "-10012345", "42")
	if err != nil || isMember {
		t.Errorf("Expected isMember=false, got %v (err: %v)", isMember, err)
	}

	// Case 3: Administrator
	statusToReturn = "administrator"
	isMember, err = adapter.CheckMembership(context.Background(), "-10012345", "42")
	if err != nil || !isMember {
		t.Errorf("Expected isMember=true for administrator, got %v (err: %v)", isMember, err)
	}

	// Case 4: Non-member Telegram API errors ("user not found", "PARTICIPANT_ID_INVALID", "chat not found")
	for _, notFoundErr := range []string{
		"Bad Request: user not found",
		"Bad Request: PARTICIPANT_ID_INVALID",
		"Bad Request: chat not found",
	} {
		errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"ok":false,"error_code":400,"description":%q}`+"\n", notFoundErr)
		}))
		errAdapter := telegram.NewAdapter("test-token",
			telegram.WithBaseURL(errSrv.URL),
			telegram.WithHTTPClient(errSrv.Client()),
		)
		isMember, err = errAdapter.CheckMembership(context.Background(), "-10012345", "42")
		errSrv.Close()
		if err != nil {
			t.Errorf("[%s] Expected nil error, got %v", notFoundErr, err)
		}
		if isMember {
			t.Errorf("[%s] Expected isMember=false, got true", notFoundErr)
		}
	}
}

func TestButton_EffectiveType_And_CallbackData(t *testing.T) {
	b1 := platform.Button{Text: "Btn", Action: "act_1"}
	if b1.EffectiveType() != platform.ButtonTypeCallback {
		t.Errorf("Expected callback type, got %s", b1.EffectiveType())
	}
	if b1.CallbackData() != "act_1" {
		t.Errorf("Expected act_1, got %s", b1.CallbackData())
	}

	b2 := platform.Button{Text: "Link", URL: "https://example.com"}
	if b2.EffectiveType() != platform.ButtonTypeURL {
		t.Errorf("Expected url type, got %s", b2.EffectiveType())
	}

	b3 := platform.Button{Text: "App", WebApp: "https://app.example.com"}
	if b3.EffectiveType() != platform.ButtonTypeWebApp {
		t.Errorf("Expected webapp type, got %s", b3.EffectiveType())
	}

	b4 := platform.Button{Text: "Explicit", Type: platform.ButtonTypeURL, Action: "ignore"}
	if b4.EffectiveType() != platform.ButtonTypeURL {
		t.Errorf("Expected explicit url type, got %s", b4.EffectiveType())
	}

	b5 := platform.Button{Text: "DataOnly", Data: "data_value"}
	if b5.CallbackData() != "data_value" {
		t.Errorf("Expected data_value, got %s", b5.CallbackData())
	}
}

func TestMAXAdapter_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// 1. Nil client
	nilAdapter := max.NewAdapter(nil)
	if _, err := nilAdapter.SendMessage(ctx, "42", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected error with nil client")
	}
	if err := nilAdapter.EditMessage(ctx, "mid", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected error with nil client in EditMessage")
	}
	if err := nilAdapter.AnswerCallback(ctx, "cb", "txt"); err == nil {
		t.Error("Expected error with nil client in AnswerCallback")
	}
	if _, err := nilAdapter.CheckMembership(ctx, "100", "42"); err == nil {
		t.Error("Expected error with nil client in CheckMembership")
	}

	// 2. Client error propagation
	mockClient := &mockMaxBotClient{
		sendErr:   fmt.Errorf("send error"),
		editErr:   fmt.Errorf("edit error"),
		answerErr: fmt.Errorf("answer error"),
	}
	adapter := max.NewAdapter(mockClient)

	if _, err := adapter.SendMessage(ctx, "42", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected propagated sendErr")
	}
	if err := adapter.EditMessage(ctx, "mid", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected propagated editErr")
	}
	if err := adapter.AnswerCallback(ctx, "cb", "txt"); err == nil {
		t.Error("Expected propagated answerErr")
	}

	// 3. Validation errors
	if _, err := adapter.SendMessage(ctx, "42", nil); err == nil {
		t.Error("Expected error with nil message body")
	}
	if _, err := adapter.SendMessage(ctx, "", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected error with empty recipient")
	}
	if _, err := adapter.SendMessage(ctx, "not_a_number", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected error with invalid recipient syntax")
	}
	if err := adapter.EditMessage(ctx, "", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected error with empty messageID")
	}
	if err := adapter.AnswerCallback(ctx, "", "txt"); err == nil {
		t.Error("Expected error with empty callbackID")
	}
	if _, err := adapter.CheckMembership(ctx, "abc", "42"); err == nil {
		t.Error("Expected error with non-numeric chatID")
	}
}

func TestTelegramAdapter_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// 1. API error response from Telegram
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`)
	}))
	defer srv.Close()

	adapter := telegram.NewAdapter("test-token",
		telegram.WithBaseURL(srv.URL),
		telegram.WithHTTPClient(srv.Client()),
	)

	_, err := adapter.SendMessage(ctx, "123", &platform.OutMessage{Text: "hi"})
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("Expected Telegram API error, got %v", err)
	}

	// 2. Validation errors
	if _, err := adapter.SendMessage(ctx, "123", nil); err == nil {
		t.Error("Expected error for nil body")
	}
	if _, err := adapter.SendMessage(ctx, "", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected error for empty recipientID")
	}
	if err := adapter.EditMessage(ctx, "", &platform.OutMessage{Text: "hi"}); err == nil {
		t.Error("Expected error for empty messageID")
	}
	if err := adapter.AnswerCallback(ctx, "", "txt"); err == nil {
		t.Error("Expected error for empty callbackID")
	}
	if _, err := adapter.CheckMembership(ctx, "", "42"); err == nil {
		t.Error("Expected error for empty chatID")
	}
	if _, err := adapter.CheckMembership(ctx, "123", "non-num"); err == nil {
		t.Error("Expected error for invalid userID")
	}

	// 3. EditMessage bare numeric messageID validation error
	if err := adapter.EditMessage(ctx, "12345", &platform.OutMessage{Text: "hi"}); err == nil || !strings.Contains(err.Error(), "chat message edit requires composite 'chatID:messageID'") {
		t.Errorf("Expected bare numeric edit error, got %v", err)
	}
}

