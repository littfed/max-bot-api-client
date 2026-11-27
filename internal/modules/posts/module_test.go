package posts

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

func collectButtons(rows [][]core.ResponseButton) []core.ResponseButton {
	var all []core.ResponseButton
	for _, row := range rows {
		all = append(all, row...)
	}
	return all
}

func hasButtonData(rows [][]core.ResponseButton, data string) bool {
	for _, btn := range collectButtons(rows) {
		if btn.Data == data {
			return true
		}
	}
	return false
}

func hasButtonPrefix(rows [][]core.ResponseButton, prefix string) bool {
	for _, btn := range collectButtons(rows) {
		if strings.HasPrefix(btn.Data, prefix) {
			return true
		}
	}
	return false
}

func hasWebAppButton(rows [][]core.ResponseButton, webAppURL string) bool {
	for _, btn := range collectButtons(rows) {
		if btn.WebApp != "" && (webAppURL == "" || strings.HasPrefix(btn.WebApp, webAppURL)) {
			return true
		}
	}
	return false
}

func TestModule_IdleAndNewPostFlow(t *testing.T) {
	ctx := context.Background()
	memStore := NewMemoryStore()
	gw := &mockAIGateway{}
	svc := NewService(memStore, NewPipeline(gw))
	mod := New(svc, "https://app.example.com/posts")

	if mod.ID() != "posts" {
		t.Fatalf("ID() = %q, want %q", mod.ID(), "posts")
	}
	if mod.Title() != "Контент-студия" {
		t.Fatalf("Title() = %q, want %q", mod.Title(), "Контент-студия")
	}
	if mod.Service() != svc {
		t.Fatal("Service() did not return configured service")
	}

	user := &store.User{ID: 501}
	state := &core.State{
		UserID:      user.ID,
		ModeID:      "posts",
		CurrentStep: StepIdle,
		Payload:     json.RawMessage(`{}`),
	}

	// 1. Callback pst:new on StepIdle -> StepIdeaInput
	cbRes, err := mod.HandleCallback(ctx, user, state, &core.IncomingCallback{
		ID:   "cb1",
		Data: "pst:new",
	})
	if err != nil {
		t.Fatalf("HandleCallback(pst:new) error = %v", err)
	}
	if !cbRes.Handled {
		t.Fatal("expected Handled=true on pst:new")
	}
	if cbRes.NewStep != StepIdeaInput {
		t.Fatalf("NewStep = %q, want %q", cbRes.NewStep, StepIdeaInput)
	}

	// 2. Message "Запуск новой AI-платформы" on StepIdeaInput -> StartDraft + StepBriefSetup
	state.CurrentStep = cbRes.NewStep
	if len(cbRes.NewPayload) > 0 {
		state.Payload = cbRes.NewPayload
	}

	msgRes, err := mod.HandleMessage(ctx, user, state, &core.IncomingMessage{
		ID:   "m1",
		Text: "Запуск новой AI-платформы",
	})
	if err != nil {
		t.Fatalf("HandleMessage(idea) error = %v", err)
	}
	if !msgRes.Handled {
		t.Fatal("expected Handled=true on idea message")
	}
	if msgRes.NewStep != StepBriefSetup {
		t.Fatalf("NewStep = %q, want %q", msgRes.NewStep, StepBriefSetup)
	}

	var payload FSMStatePayload
	if err := json.Unmarshal(msgRes.NewPayload, &payload); err != nil {
		t.Fatalf("failed to unmarshal NewPayload: %v", err)
	}
	if payload.ActiveDraftID == 0 {
		t.Fatal("expected non-zero active_draft_id in NewPayload")
	}

	draft, err := memStore.GetDraft(ctx, user.ID, payload.ActiveDraftID)
	if err != nil {
		t.Fatalf("GetDraft() error = %v", err)
	}
	if draft.RawIdea != "Запуск новой AI-платформы" {
		t.Fatalf("RawIdea = %q, want %q", draft.RawIdea, "Запуск новой AI-платформы")
	}

	if !hasButtonPrefix(msgRes.Buttons, "pst:goal:") {
		t.Fatal("expected goal buttons (pst:goal:...) in StepBriefSetup")
	}
	if !hasButtonPrefix(msgRes.Buttons, "pst:plat:") {
		t.Fatal("expected platform buttons (pst:plat:...) in StepBriefSetup")
	}
	if !hasButtonData(msgRes.Buttons, "pst:hooks") {
		t.Fatal("expected pst:hooks button in StepBriefSetup")
	}
}

func TestModule_HooksAndGenerateFlow(t *testing.T) {
	ctx := context.Background()
	memStore := NewMemoryStore()
	gw := &mockAIGateway{
		chatResponses: []string{
			// 1. GenerateHooks -> 3 hooks
			`{"hooks":[
				{"id":"h1","title":"Боль","text":"Почему команды теряют 10 часов на посты?","angle":"problem"},
				{"id":"h2","title":"Кейс","text":"Как мы запустили AI-платформу за 2 недели","angle":"case"},
				{"id":"h3","title":"Инсайт","text":"Главный тренд контент-маркетинга в этом году","angle":"insight"}
			]}`,
			// 2. GeneratePackage
			`{"variants":[
				{"platform":"max","text":"Готовый пост для MAX о запуске AI-платформы","hashtags":["#ai","#запуск"],"cta":"Попробуйте"},
				{"platform":"telegram","text":"Готовый пост для Telegram о запуске AI-платформы","hashtags":["#ai"],"cta":"Подписывайтесь"}
			],"image_prompt":"Futuristic AI platform launch dashboard, clean editorial 3d"}`,
		},
		imageResp: "https://cdn.example.com/generated-cover.png",
	}
	svc := NewService(memStore, NewPipeline(gw))
	const webAppURL = "https://app.example.com/posts"
	mod := New(svc, webAppURL)

	user := &store.User{ID: 502}
	draft, err := svc.StartDraft(ctx, user.ID, "Запуск новой AI-платформы", "")
	if err != nil {
		t.Fatalf("StartDraft() error = %v", err)
	}

	rawPayload, _ := json.Marshal(FSMStatePayload{ActiveDraftID: draft.ID})
	state := &core.State{
		UserID:      user.ID,
		ModeID:      "posts",
		CurrentStep: StepBriefSetup,
		Payload:     rawPayload,
	}

	// 1. Callback pst:hooks -> StepHooksSelect with 3 pst:hook:<id> buttons
	hooksRes, err := mod.HandleCallback(ctx, user, state, &core.IncomingCallback{
		ID:   "cb_hooks",
		Data: "pst:hooks",
	})
	if err != nil {
		t.Fatalf("HandleCallback(pst:hooks) error = %v", err)
	}
	if !hooksRes.Handled {
		t.Fatal("expected Handled=true on pst:hooks")
	}
	if hooksRes.NewStep != StepHooksSelect {
		t.Fatalf("NewStep = %q, want %q", hooksRes.NewStep, StepHooksSelect)
	}
	for _, hookID := range []string{"h1", "h2", "h3"} {
		if !hasButtonData(hooksRes.Buttons, "pst:hook:"+hookID) {
			t.Fatalf("expected button pst:hook:%s", hookID)
		}
	}

	// 2. Callback pst:hook:h1 -> StepReady with refine buttons and WebApp button
	state.CurrentStep = hooksRes.NewStep
	if len(hooksRes.NewPayload) > 0 {
		state.Payload = hooksRes.NewPayload
	}

	genRes, err := mod.HandleCallback(ctx, user, state, &core.IncomingCallback{
		ID:   "cb_hook_h1",
		Data: "pst:hook:h1",
	})
	if err != nil {
		t.Fatalf("HandleCallback(pst:hook:h1) error = %v", err)
	}
	if !genRes.Handled {
		t.Fatal("expected Handled=true on pst:hook:h1")
	}
	if genRes.NewStep != StepReady {
		t.Fatalf("NewStep = %q, want %q", genRes.NewStep, StepReady)
	}
	for _, expectedBtn := range []string{"pst:refine:shorter", "pst:refine:bolder", "pst:img"} {
		if !hasButtonData(genRes.Buttons, expectedBtn) {
			t.Fatalf("expected button %q in StepReady", expectedBtn)
		}
	}
	if !hasWebAppButton(genRes.Buttons, webAppURL) {
		t.Fatal("expected WebApp button when webAppURL is non-empty")
	}
}

func TestModule_ReadyRefineAndImage(t *testing.T) {
	ctx := context.Background()
	memStore := NewMemoryStore()
	gw := &mockAIGateway{
		chatResponses: []string{
			// 1. Refine shorter
			`{"variants":[
				{"platform":"max","text":"Короткий пост для MAX","hashtags":["#ai"],"cta":"Жмите"},
				{"platform":"telegram","text":"Короткий пост для Telegram","hashtags":["#ai"],"cta":"Жмите"}
			]}`,
		},
		imageResp: "https://cdn.example.com/regenerated.png",
	}
	svc := NewService(memStore, NewPipeline(gw))
	mod := New(svc, "")

	user := &store.User{ID: 503}
	draft := &PostDraft{
		UserID:          user.ID,
		Status:          DraftStatusReady,
		RawIdea:         "Запуск AI",
		TargetPlatforms: []string{"max", "telegram"},
		ImagePrompt:     "Clean AI cover",
		Variants: []PostVariant{
			{Platform: "max", Text: "Длинный исходный пост для MAX"},
			{Platform: "telegram", Text: "Длинный исходный пост для Telegram"},
		},
	}
	if err := memStore.CreateDraft(ctx, draft); err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}

	rawPayload, _ := json.Marshal(FSMStatePayload{ActiveDraftID: draft.ID})
	state := &core.State{
		UserID:      user.ID,
		ModeID:      "posts",
		CurrentStep: StepReady,
		Payload:     rawPayload,
	}

	res, err := mod.HandleCallback(ctx, user, state, &core.IncomingCallback{
		ID:   "cb_refine",
		Data: "pst:refine:shorter",
	})
	if err != nil {
		t.Fatalf("HandleCallback(pst:refine:shorter) error = %v", err)
	}
	if !res.Handled {
		t.Fatal("expected Handled=true on pst:refine:shorter")
	}
	if res.NewStep != StepReady {
		t.Fatalf("NewStep = %q, want %q", res.NewStep, StepReady)
	}

	updated, err := memStore.GetDraft(ctx, user.ID, draft.ID)
	if err != nil {
		t.Fatalf("GetDraft() error = %v", err)
	}
	if len(updated.Variants) == 0 || updated.Variants[0].Text != "Короткий пост для MAX" {
		t.Fatalf("expected refined variant text, got %+v", updated.Variants)
	}
	if hasWebAppButton(res.Buttons, "") {
		t.Fatal("did not expect WebApp button when webAppURL is empty")
	}

	// Also verify pst:img keeps StepReady and updates GeneratedImageURL
	imgRes, err := mod.HandleCallback(ctx, user, state, &core.IncomingCallback{
		ID:   "cb_img",
		Data: "pst:img",
	})
	if err != nil {
		t.Fatalf("HandleCallback(pst:img) error = %v", err)
	}
	if imgRes.NewStep != StepReady {
		t.Fatalf("NewStep after pst:img = %q, want %q", imgRes.NewStep, StepReady)
	}
	updated, err = memStore.GetDraft(ctx, user.ID, draft.ID)
	if err != nil {
		t.Fatalf("GetDraft() after pst:img error = %v", err)
	}
	if updated.GeneratedImageURL != "https://cdn.example.com/regenerated.png" {
		t.Fatalf("GeneratedImageURL = %q, want regenerated URL", updated.GeneratedImageURL)
	}
}
