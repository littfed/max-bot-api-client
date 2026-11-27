package posts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

var _ core.BotModule = (*Module)(nil)

var goalOptions = []struct {
	ID    string
	Label string
}{
	{ID: "engagement", Label: "Вовлечение"},
	{ID: "leads", Label: "Заявки"},
	{ID: "announce", Label: "Анонс"},
	{ID: "expert", Label: "Экспертность"},
}

var refinePresets = map[string]string{
	"shorter": "Сделай текст короче и плотнее, убери воду и сохрани главную мысль.",
	"bolder":  "Сделай подачу смелее, ярче и с более сильным эмоциональным акцентом.",
}

// Module implements core.BotModule for the Content Studio chat wizard.
type Module struct {
	service   *Service
	webAppURL string
}

// New creates a new Content Studio BotModule.
func New(service *Service, webAppURL string) *Module {
	return &Module{
		service:   service,
		webAppURL: strings.TrimSpace(webAppURL),
	}
}

// ID returns the unique module identifier.
func (m *Module) ID() string {
	return "posts"
}

// Title returns the human-readable module title.
func (m *Module) Title() string {
	return "Контент-студия"
}

// Service returns the underlying Content Studio service.
func (m *Module) Service() *Service {
	return m.service
}

// HandleMessage processes incoming text messages according to the current FSM step.
func (m *Module) HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	if u == nil || msg == nil {
		return &core.ModuleResult{Handled: false}, nil
	}
	if m.service == nil {
		return nil, errors.New("posts: service is nil")
	}

	step := StepIdle
	if state != nil && state.CurrentStep != "" {
		step = state.CurrentStep
	}

	if msg.Command == "start" || msg.Command == "menu" {
		return m.renderIdleMenu(state), nil
	}

	text := strings.TrimSpace(msg.Text)

	switch step {
	case StepIdeaInput:
		if text == "" {
			return &core.ModuleResult{
				Text:    "Пришлите идею, тезис или тему будущего поста одним сообщением.",
				Buttons: [][]core.ResponseButton{{{Text: "Отмена", Data: "pst:menu"}}},
				NewStep: StepIdeaInput,
				Handled: true,
			}, nil
		}

		draft, err := m.service.StartDraft(ctx, u.ID, text, "")
		if err != nil {
			return nil, err
		}

		payload, err := marshalPayload(draft.ID)
		if err != nil {
			return nil, err
		}
		return m.renderBriefSetup(draft, payload), nil

	case StepReady:
		draftID := extractActiveDraftID(state)
		if draftID > 0 && text != "" {
			draft, err := m.service.Refine(ctx, u.ID, draftID, "", text)
			if err != nil {
				return nil, err
			}
			payload, _ := marshalPayload(draft.ID)
			return m.renderReadyDraft(draft, payload), nil
		}
		return m.renderIdleMenu(state), nil

	default:
		return m.renderIdleMenu(state), nil
	}
}

// HandleCallback processes inline button callbacks prefixed with "pst:".
func (m *Module) HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	if u == nil || cb == nil {
		return &core.ModuleResult{Handled: false}, nil
	}
	if !strings.HasPrefix(cb.Data, "pst:") {
		return &core.ModuleResult{Handled: false}, nil
	}
	if m.service == nil {
		return nil, errors.New("posts: service is nil")
	}

	data := cb.Data

	switch {
	case data == "pst:menu":
		return m.renderIdleMenu(state), nil

	case data == "pst:new":
		return &core.ModuleResult{
			Text:    "✍️ Опишите идею поста: о чём пишем, какой повод или ключевая мысль?",
			Buttons: [][]core.ResponseButton{{{Text: "Назад", Data: "pst:menu"}}},
			NewStep: StepIdeaInput,
			Handled: true,
		}, nil

	case data == "pst:drafts":
		st, err := m.service.GetState(ctx, u.ID, 0)
		if err != nil {
			return nil, err
		}
		return m.renderDraftsList(st.Drafts), nil

	case strings.HasPrefix(data, "pst:draft:"):
		idStr := strings.TrimPrefix(data, "pst:draft:")
		draftID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || draftID <= 0 {
			return &core.ModuleResult{
				Text:    "Некорректный идентификатор черновика.",
				Buttons: [][]core.ResponseButton{{{Text: "В меню", Data: "pst:menu"}}},
				NewStep: StepIdle,
				Handled: true,
			}, nil
		}
		st, err := m.service.GetState(ctx, u.ID, draftID)
		if err != nil {
			return nil, err
		}
		if st.ActiveDraft == nil {
			return m.renderIdleMenu(state), nil
		}
		payload, _ := marshalPayload(st.ActiveDraft.ID)
		return m.renderDraftForCurrentStatus(st.ActiveDraft, payload), nil

	case data == "pst:profile":
		st, err := m.service.GetState(ctx, u.ID, 0)
		if err != nil {
			return nil, err
		}
		return m.renderProfile(st.Profiles), nil

	case strings.HasPrefix(data, "pst:goal:"):
		draftID := extractActiveDraftID(state)
		if draftID == 0 {
			return m.renderIdleMenu(state), nil
		}
		goal := strings.TrimPrefix(data, "pst:goal:")
		st, err := m.service.GetState(ctx, u.ID, draftID)
		if err != nil {
			return nil, err
		}
		if st.ActiveDraft == nil {
			return m.renderIdleMenu(state), nil
		}
		updated, err := m.service.UpdateBrief(ctx, u.ID, draftID, goal, st.ActiveDraft.Tone, st.ActiveDraft.TargetPlatforms)
		if err != nil {
			return nil, err
		}
		payload, _ := marshalPayload(updated.ID)
		return m.renderBriefSetup(updated, payload), nil

	case strings.HasPrefix(data, "pst:plat:"):
		draftID := extractActiveDraftID(state)
		if draftID == 0 {
			return m.renderIdleMenu(state), nil
		}
		plat := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(data, "pst:plat:")))
		st, err := m.service.GetState(ctx, u.ID, draftID)
		if err != nil {
			return nil, err
		}
		if st.ActiveDraft == nil {
			return m.renderIdleMenu(state), nil
		}
		nextPlatforms := togglePlatform(st.ActiveDraft.TargetPlatforms, plat)
		updated, err := m.service.UpdateBrief(ctx, u.ID, draftID, st.ActiveDraft.Goal, st.ActiveDraft.Tone, nextPlatforms)
		if err != nil {
			return nil, err
		}
		payload, _ := marshalPayload(updated.ID)
		return m.renderBriefSetup(updated, payload), nil

	case data == "pst:hooks":
		draftID := extractActiveDraftID(state)
		if draftID == 0 {
			return m.renderIdleMenu(state), nil
		}
		draft, err := m.service.GenerateHooks(ctx, u.ID, draftID)
		if err != nil {
			return nil, err
		}
		payload, _ := marshalPayload(draft.ID)
		return m.renderHooksSelect(draft, payload), nil

	case strings.HasPrefix(data, "pst:hook:"):
		draftID := extractActiveDraftID(state)
		if draftID == 0 {
			return m.renderIdleMenu(state), nil
		}
		hookID := strings.TrimPrefix(data, "pst:hook:")
		draft, err := m.service.GeneratePackage(ctx, u.ID, draftID, hookID)
		if err != nil {
			return nil, err
		}
		payload, _ := marshalPayload(draft.ID)
		return m.renderReadyDraft(draft, payload), nil

	case strings.HasPrefix(data, "pst:refine:"):
		draftID := extractActiveDraftID(state)
		if draftID == 0 {
			return m.renderIdleMenu(state), nil
		}
		preset := strings.TrimPrefix(data, "pst:refine:")
		instruction, ok := refinePresets[preset]
		if !ok {
			instruction = preset
		}
		draft, err := m.service.Refine(ctx, u.ID, draftID, "", instruction)
		if err != nil {
			return nil, err
		}
		payload, _ := marshalPayload(draft.ID)
		return m.renderReadyDraft(draft, payload), nil

	case data == "pst:img":
		draftID := extractActiveDraftID(state)
		if draftID == 0 {
			return m.renderIdleMenu(state), nil
		}
		draft, err := m.service.RegenerateImage(ctx, u.ID, draftID, "")
		if err != nil {
			return nil, err
		}
		payload, _ := marshalPayload(draft.ID)
		return m.renderReadyDraft(draft, payload), nil

	default:
		return &core.ModuleResult{Handled: false}, nil
	}
}

func (m *Module) renderIdleMenu(state *core.State) *core.ModuleResult {
	buttons := [][]core.ResponseButton{
		{{Text: "✨ Новый пост", Data: "pst:new"}},
		{
			{Text: "🗂 Черновики", Data: "pst:drafts"},
			{Text: "🎨 Бренд-профиль", Data: "pst:profile"},
		},
	}
	if m.webAppURL != "" {
		buttons = append(buttons, []core.ResponseButton{
			{Text: "🚀 Открыть студию", WebApp: m.webAppURL},
		})
	}

	var payload json.RawMessage
	if state != nil && len(state.Payload) > 0 {
		payload = state.Payload
	}

	return &core.ModuleResult{
		Text:       "📝 Контент-студия\n\nСоздавайте посты с AI-хуками, адаптацией под площадки и обложками.",
		Buttons:    buttons,
		NewStep:    StepIdle,
		NewPayload: payload,
		Handled:    true,
	}
}

func (m *Module) renderBriefSetup(draft *PostDraft, payload json.RawMessage) *core.ModuleResult {
	var goalRow []core.ResponseButton
	for _, g := range goalOptions {
		label := g.Label
		if draft.Goal == g.ID {
			label = "✓ " + label
		}
		goalRow = append(goalRow, core.ResponseButton{
			Text: label,
			Data: "pst:goal:" + g.ID,
		})
	}

	selected := make(map[string]bool, len(draft.TargetPlatforms))
	for _, p := range draft.TargetPlatforms {
		selected[p] = true
	}

	var platRow []core.ResponseButton
	for _, spec := range AllPlatformSpecs() {
		label := spec.DisplayName
		if selected[spec.ID] {
			label = "✓ " + label
		}
		platRow = append(platRow, core.ResponseButton{
			Text: label,
			Data: "pst:plat:" + spec.ID,
		})
	}

	goalText := draft.Goal
	if goalText == "" {
		goalText = "не выбрана"
	}

	text := fmt.Sprintf(
		"📌 Настройка брифа\n\nИдея: %s\nЦель: %s\nПлощадки: %s\n\nВыберите цель и площадки, затем запустите генерацию хуков.",
		draft.RawIdea,
		goalText,
		strings.Join(draft.TargetPlatforms, ", "),
	)

	buttons := [][]core.ResponseButton{
		goalRow,
		platRow,
		{{Text: "💡 Сгенерировать хуки", Data: "pst:hooks"}},
		{{Text: "В меню", Data: "pst:menu"}},
	}

	return &core.ModuleResult{
		Text:       text,
		Buttons:    buttons,
		NewStep:    StepBriefSetup,
		NewPayload: payload,
		Handled:    true,
	}
}

func (m *Module) renderHooksSelect(draft *PostDraft, payload json.RawMessage) *core.ModuleResult {
	var sb strings.Builder
	sb.WriteString("🎯 Выберите хук для поста:\n\n")

	var hookButtons [][]core.ResponseButton
	for i, h := range draft.Hooks {
		sb.WriteString(fmt.Sprintf("%d. %s — %s\n", i+1, h.Title, h.Text))
		btnText := h.Title
		if btnText == "" {
			btnText = fmt.Sprintf("Хук #%d", i+1)
		}
		hookButtons = append(hookButtons, []core.ResponseButton{
			{Text: btnText, Data: "pst:hook:" + h.ID},
		})
	}
	hookButtons = append(hookButtons, []core.ResponseButton{
		{Text: "🔄 Другие хуки", Data: "pst:hooks"},
		{Text: "В меню", Data: "pst:menu"},
	})

	return &core.ModuleResult{
		Text:       sb.String(),
		Buttons:    hookButtons,
		NewStep:    StepHooksSelect,
		NewPayload: payload,
		Handled:    true,
	}
}

func (m *Module) renderReadyDraft(draft *PostDraft, payload json.RawMessage) *core.ModuleResult {
	var sb strings.Builder
	sb.WriteString("✅ Готовый пакет постов\n")
	for _, v := range draft.Variants {
		sb.WriteString(fmt.Sprintf("\n[%s]\n%s\n", strings.ToUpper(v.Platform), v.Text))
		if len(v.Hashtags) > 0 {
			sb.WriteString(strings.Join(v.Hashtags, " ") + "\n")
		}
	}
	if draft.GeneratedImageURL != "" {
		sb.WriteString("\n🖼 Обложка: " + draft.GeneratedImageURL + "\n")
	}

	buttons := [][]core.ResponseButton{
		{
			{Text: "✂️ Короче", Data: "pst:refine:shorter"},
			{Text: "🔥 Смелее", Data: "pst:refine:bolder"},
		},
		{
			{Text: "🖼 Новая обложка", Data: "pst:img"},
			{Text: "В меню", Data: "pst:menu"},
		},
	}
	if m.webAppURL != "" {
		buttons = append(buttons, []core.ResponseButton{
			{Text: "🚀 Редактировать в студии", WebApp: m.webAppURL},
		})
	}

	return &core.ModuleResult{
		Text:       sb.String(),
		Buttons:    buttons,
		NewStep:    StepReady,
		NewPayload: payload,
		Handled:    true,
	}
}

func (m *Module) renderDraftsList(drafts []PostDraft) *core.ModuleResult {
	if len(drafts) == 0 {
		return &core.ModuleResult{
			Text: "🗂 У вас пока нет черновиков.",
			Buttons: [][]core.ResponseButton{
				{{Text: "✨ Новый пост", Data: "pst:new"}},
				{{Text: "В меню", Data: "pst:menu"}},
			},
			NewStep: StepIdle,
			Handled: true,
		}
	}

	var buttons [][]core.ResponseButton
	limit := len(drafts)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		d := drafts[i]
		title := strings.TrimSpace(d.RawIdea)
		if len([]rune(title)) > 32 {
			title = string([]rune(title)[:32]) + "…"
		}
		if title == "" {
			title = fmt.Sprintf("Черновик #%d", d.ID)
		}
		buttons = append(buttons, []core.ResponseButton{
			{Text: title, Data: fmt.Sprintf("pst:draft:%d", d.ID)},
		})
	}
	buttons = append(buttons, []core.ResponseButton{{Text: "В меню", Data: "pst:menu"}})

	return &core.ModuleResult{
		Text:    "🗂 Выберите черновик:",
		Buttons: buttons,
		NewStep: StepIdle,
		Handled: true,
	}
}

func (m *Module) renderProfile(profiles []BrandProfile) *core.ModuleResult {
	text := "🎨 Бренд-профиль не настроен. По умолчанию используются площадки MAX и Telegram."
	if len(profiles) > 0 {
		p := profiles[0]
		text = fmt.Sprintf(
			"🎨 Бренд-профиль: %s\nНиша: %s\nТон: %s\nПлощадки: %s",
			p.Name, p.Niche, p.ToneOfVoice, strings.Join(p.DefaultPlatforms, ", "),
		)
	}

	buttons := [][]core.ResponseButton{
		{{Text: "✨ Новый пост", Data: "pst:new"}, {Text: "В меню", Data: "pst:menu"}},
	}
	if m.webAppURL != "" {
		buttons = append(buttons, []core.ResponseButton{
			{Text: "⚙️ Настроить в студии", WebApp: m.webAppURL},
		})
	}

	return &core.ModuleResult{
		Text:    text,
		Buttons: buttons,
		NewStep: StepIdle,
		Handled: true,
	}
}

func (m *Module) renderDraftForCurrentStatus(draft *PostDraft, payload json.RawMessage) *core.ModuleResult {
	switch draft.Status {
	case DraftStatusReady:
		return m.renderReadyDraft(draft, payload)
	case DraftStatusHooks:
		if len(draft.Hooks) > 0 {
			return m.renderHooksSelect(draft, payload)
		}
	}
	return m.renderBriefSetup(draft, payload)
}

func extractActiveDraftID(state *core.State) int64 {
	if state == nil || len(state.Payload) == 0 {
		return 0
	}
	var p FSMStatePayload
	if err := json.Unmarshal(state.Payload, &p); err != nil {
		return 0
	}
	return p.ActiveDraftID
}

func marshalPayload(draftID int64) (json.RawMessage, error) {
	raw, err := json.Marshal(FSMStatePayload{ActiveDraftID: draftID})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func togglePlatform(current []string, target string) []string {
	if _, ok := GetPlatformSpec(target); !ok {
		return NormalizePlatforms(current)
	}
	normalized := NormalizePlatforms(current)
	out := make([]string, 0, len(normalized)+1)
	found := false
	for _, p := range normalized {
		if p == target {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		out = append(out, target)
		return NormalizePlatforms(out)
	}
	if len(out) == 0 {
		return normalized
	}
	return out
}
