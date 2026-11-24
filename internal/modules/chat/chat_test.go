package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type mockLLM struct {
	lastReq *ai.ChatRequest
	reply   string
	err     error
}

func (m *mockLLM) Chat(ctx context.Context, req *ai.ChatRequest) (*ai.ChatResponse, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &ai.ChatResponse{
		Content: m.reply,
	}, nil
}

func TestChatModule_GetConfig(t *testing.T) {
	mod := New(nil, nil)
	roles, tones, err := mod.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 5 {
		t.Errorf("expected 5 roles, got %d", len(roles))
	}
	if len(tones) != 4 {
		t.Errorf("expected 4 tones, got %d", len(tones))
	}
}

func TestChatModule_LLMInvocationWithRoleAndTone(t *testing.T) {
	mock := &mockLLM{
		reply: "Привет! Как продуктовый аналитик, вижу CAC=500 и LTV=2000. Взлетаем!",
	}
	mod := New(nil, mock, "gemini-2.5-flash")

	ctx := context.Background()
	reply, err := mod.SendMessage(ctx, 12345, 2, 4, "Проанализируй наш стартап")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if reply != mock.reply {
		t.Errorf("expected reply %q, got %q", mock.reply, reply)
	}

	if mock.lastReq == nil {
		t.Fatal("expected mock to record request")
	}

	if mock.lastReq.Model != "gemini-2.5-flash" {
		t.Errorf("expected model gemini-2.5-flash, got %s", mock.lastReq.Model)
	}

	if len(mock.lastReq.Messages) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(mock.lastReq.Messages))
	}

	sysMsg, _ := mock.lastReq.Messages[0].Content.(string)
	if !strings.Contains(sysMsg, "Продуктовый аналитик") {
		t.Errorf("expected system message to contain role, got: %s", sysMsg)
	}
	if !strings.Contains(sysMsg, "Юмористический") {
		t.Errorf("expected system message to contain tone, got: %s", sysMsg)
	}
}

func TestChatModule_HandleCallbacks(t *testing.T) {
	mock := &mockLLM{reply: "Ответ ИИ"}
	mod := New(nil, mock)
	ctx := context.Background()
	user := &store.User{ID: 100}
	state := &core.State{}

	// 1. Open Roles Menu
	res, err := mod.HandleCallback(ctx, user, state, &core.IncomingCallback{Data: "chat:menu:roles"})
	if err != nil || !res.Handled {
		t.Fatalf("failed chat:menu:roles: %v", err)
	}
	if !strings.Contains(res.Text, "Выберите роль") {
		t.Errorf("unexpected text: %s", res.Text)
	}

	// 2. Select Role 3 (Маркетолог)
	res, err = mod.HandleCallback(ctx, user, state, &core.IncomingCallback{Data: "chat:role:3"})
	if err != nil || !res.Handled {
		t.Fatalf("failed chat:role:3: %v", err)
	}
	if !strings.Contains(res.Text, "Маркетолог") {
		t.Errorf("unexpected text: %s", res.Text)
	}

	// Update state with new payload
	state.Payload = res.NewPayload

	// 3. Open Tones Menu
	res, err = mod.HandleCallback(ctx, user, state, &core.IncomingCallback{Data: "chat:menu:tones"})
	if err != nil || !res.Handled {
		t.Fatalf("failed chat:menu:tones: %v", err)
	}

	// 4. Select Tone 2 (Дружелюбный)
	res, err = mod.HandleCallback(ctx, user, state, &core.IncomingCallback{Data: "chat:tone:2"})
	if err != nil || !res.Handled {
		t.Fatalf("failed chat:tone:2: %v", err)
	}
	if !strings.Contains(res.Text, "Дружелюбный") {
		t.Errorf("unexpected text: %s", res.Text)
	}

	// 5. Back to Dialog
	state.Payload = res.NewPayload
	res, err = mod.HandleCallback(ctx, user, state, &core.IncomingCallback{Data: "chat:menu:back"})
	if err != nil || !res.Handled {
		t.Fatalf("failed chat:menu:back: %v", err)
	}
	if !strings.Contains(res.Text, "Маркетолог") || !strings.Contains(res.Text, "Дружелюбный") {
		t.Errorf("expected prompt to show role and tone, got: %s", res.Text)
	}
}

func TestChatModule_HandleMessage(t *testing.T) {
	mock := &mockLLM{reply: "Отличный вопрос!"}
	mod := New(nil, mock, "gemini-2.5-flash")
	ctx := context.Background()
	user := &store.User{ID: 200}
	state := &core.State{
		Payload: []byte(`{"role_id": 3, "tone_id": 2}`),
	}

	res, err := mod.HandleMessage(ctx, user, state, &core.IncomingMessage{Text: "Как продвигать сервис?"})
	if err != nil || !res.Handled {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if !strings.Contains(res.Text, "Маркетолог") {
		t.Errorf("expected header to contain role, got: %s", res.Text)
	}
	if !strings.Contains(res.Text, "Отличный вопрос!") {
		t.Errorf("expected reply in text, got: %s", res.Text)
	}
}
