package images

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

var (
	_ modules.BotModule = (*Module)(nil)
	_ core.BotModule    = (*Module)(nil)
)

// FSM step constants for the Image Studio chat flow.
const (
	StepIdle          = "idle"
	StepAwaitingInput = "awaiting_input"
	StepReady         = "ready"
)

type fsmPayload struct {
	Mode            string `json:"mode,omitempty"`
	PresetID        string `json:"preset_id,omitempty"`
	ActiveProjectID int64  `json:"active_project_id,omitempty"`
}

// Module implements modules.BotModule for the Image Studio ("Фотостудия").
type Module struct {
	service   *Service
	webAppURL string
}

// New creates a new Image Studio BotModule.
func New(service *Service, webAppURL string) *Module {
	return &Module{
		service:   service,
		webAppURL: strings.TrimSpace(webAppURL),
	}
}

// ID returns the unique module identifier.
func (m *Module) ID() string {
	return "images"
}

// Title returns the human-readable module title.
func (m *Module) Title() string {
	return "Фотостудия"
}

// Service returns the underlying Image Studio service.
func (m *Module) Service() *Service {
	return m.service
}

// HandleMessage processes incoming text messages according to the current FSM step.
func (m *Module) HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	if u == nil || msg == nil {
		return &core.ModuleResult{Handled: false}, nil
	}
	if m.service == nil {
		return nil, errors.New("images: service is nil")
	}

	if msg.Command == "start" || msg.Command == "menu" {
		return m.renderHome(state), nil
	}

	step := StepIdle
	if state != nil && state.CurrentStep != "" {
		step = state.CurrentStep
	}
	payload := parseFSMPayload(state)
	text := strings.TrimSpace(msg.Text)

	switch step {
	case StepAwaitingInput:
		if text == "" {
			return &core.ModuleResult{
				Text:       "Опишите желаемое изображение или правку одним сообщением.",
				Buttons:    [][]core.ResponseButton{{{Text: "⬅️ В меню", Data: "img:home"}}},
				NewStep:    StepAwaitingInput,
				NewPayload: mustMarshalPayload(payload),
				Handled:    true,
			}, nil
		}

		mode := payload.Mode
		if mode == "" {
			mode = ModeGenerate
		}
		proj, err := m.service.CreateAndRun(ctx, CreateProjectInput{
			UserID:           u.ID,
			Mode:             mode,
			PresetID:         payload.PresetID,
			Prompt:           text,
			UseExampleSource: mode == ModeEdit,
		})
		if err != nil {
			return &core.ModuleResult{
				Text:       fmt.Sprintf("❌ Не удалось выполнить генерацию: %v\nПопробуйте изменить запрос.", err),
				Buttons:    [][]core.ResponseButton{{{Text: "🏠 В меню", Data: "img:home"}}},
				NewStep:    StepAwaitingInput,
				NewPayload: mustMarshalPayload(payload),
				Handled:    true,
			}, nil
		}

		nextPayload := fsmPayload{
			Mode:            proj.Mode,
			PresetID:        proj.PresetID,
			ActiveProjectID: proj.ID,
		}
		return m.renderProjectResult(proj, mustMarshalPayload(nextPayload)), nil

	case StepReady:
		if payload.ActiveProjectID > 0 && text != "" {
			proj, err := m.service.Refine(ctx, u.ID, payload.ActiveProjectID, text, "")
			if err != nil {
				ver := 1
				if cur, gErr := m.service.GetProject(ctx, u.ID, payload.ActiveProjectID); gErr == nil && cur.CurrentVersion > 0 {
					ver = cur.CurrentVersion
				}
				return &core.ModuleResult{
					Text:       fmt.Sprintf("❌ Не удалось применить правку: %v\nПредыдущая версия сохранена.", err),
					Buttons:    m.projectActionButtons(payload.ActiveProjectID, ver),
					NewStep:    StepReady,
					NewPayload: mustMarshalPayload(payload),
					Handled:    true,
				}, nil
			}
			return m.renderProjectResult(proj, mustMarshalPayload(payload)), nil
		}
		return m.renderHome(state), nil

	default:
		return m.renderHome(state), nil
	}
}

// HandleCallback processes inline button callbacks prefixed with "img:".
func (m *Module) HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	if u == nil || cb == nil {
		return &core.ModuleResult{Handled: false}, nil
	}
	if !strings.HasPrefix(cb.Data, "img:") {
		return &core.ModuleResult{Handled: false}, nil
	}
	if m.service == nil {
		return nil, errors.New("images: service is nil")
	}

	data := cb.Data

	switch {
	case data == "img:home":
		return m.renderHome(state), nil

	case data == "img:presets":
		return m.renderPresetCategories(), nil

	case data == "img:cat:edit":
		return m.renderPresetList(ModeEdit), nil

	case data == "img:cat:generate":
		return m.renderPresetList(ModeGenerate), nil

	case strings.HasPrefix(data, "img:preset:"):
		presetID := strings.TrimPrefix(data, "img:preset:")
		return m.renderPresetDetail(presetID), nil

	case strings.HasPrefix(data, "img:try:"):
		presetID := strings.TrimPrefix(data, "img:try:")
		preset, ok := GetPreset(presetID)
		if !ok {
			return m.renderPresetCategories(), nil
		}
		proj, err := m.service.CreateAndRun(ctx, CreateProjectInput{
			UserID:           u.ID,
			Mode:             preset.Mode,
			PresetID:         preset.ID,
			Prompt:           preset.ExamplePrompt,
			AspectRatio:      preset.DefaultAspectRatio,
			UseExampleSource: preset.Mode == ModeEdit,
		})
		if err != nil {
			return &core.ModuleResult{
				Text:    fmt.Sprintf("❌ Не удалось запустить пример: %v", err),
				Buttons: [][]core.ResponseButton{{{Text: "⬅️ Назад", Data: "img:preset:" + preset.ID}}},
				NewStep: StepIdle,
				Handled: true,
			}, nil
		}
		nextPayload := fsmPayload{
			Mode:            proj.Mode,
			PresetID:        proj.PresetID,
			ActiveProjectID: proj.ID,
		}
		return m.renderProjectResult(proj, mustMarshalPayload(nextPayload)), nil

	case strings.HasPrefix(data, "img:use:"):
		presetID := strings.TrimPrefix(data, "img:use:")
		preset, ok := GetPreset(presetID)
		if !ok {
			return m.renderPresetCategories(), nil
		}
		nextPayload := fsmPayload{
			Mode:     preset.Mode,
			PresetID: preset.ID,
		}
		return &core.ModuleResult{
			Text: fmt.Sprintf(
				"✨ Сценарий «%s»\n\nПришлите текстовый запрос (например: «%s»).",
				preset.Title,
				preset.ExamplePrompt,
			),
			Buttons: [][]core.ResponseButton{
				{{Text: "⬅️ Назад", Data: "img:preset:" + preset.ID}},
			},
			NewStep:    StepAwaitingInput,
			NewPayload: mustMarshalPayload(nextPayload),
			Handled:    true,
		}, nil

	case data == "img:mode:generate":
		nextPayload := fsmPayload{Mode: ModeGenerate}
		return &core.ModuleResult{
			Text:       "🎨 Опишите, какое изображение нужно сгенерировать.",
			Buttons:    [][]core.ResponseButton{{{Text: "⬅️ В меню", Data: "img:home"}}},
			NewStep:    StepAwaitingInput,
			NewPayload: mustMarshalPayload(nextPayload),
			Handled:    true,
		}, nil

	case data == "img:mode:edit":
		nextPayload := fsmPayload{Mode: ModeEdit}
		return &core.ModuleResult{
			Text:       "🪄 Опишите, как отредактировать фото (для загрузки своего фото откройте мини-приложение Фотостудии).",
			Buttons:    m.editModePromptButtons(),
			NewStep:    StepAwaitingInput,
			NewPayload: mustMarshalPayload(nextPayload),
			Handled:    true,
		}, nil

	case data == "img:history":
		projects, err := m.service.ListProjects(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		return m.renderHistory(projects), nil

	case strings.HasPrefix(data, "img:project:"):
		idStr := strings.TrimPrefix(data, "img:project:")
		projectID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || projectID <= 0 {
			return m.renderHome(state), nil
		}
		proj, err := m.service.GetProject(ctx, u.ID, projectID)
		if err != nil {
			return m.renderHome(state), nil
		}
		if proj.CurrentVersion == 0 || proj.CurrentImageURL == "" {
			return &core.ModuleResult{
				Text:    "❌ Эта работа ещё не готова или завершилась с ошибкой.",
				Buttons: [][]core.ResponseButton{{{Text: "🗂 Мои работы", Data: "img:history"}}, {{Text: "🏠 В меню", Data: "img:home"}}},
				NewStep: StepIdle,
				Handled: true,
			}, nil
		}
		nextPayload := fsmPayload{
			Mode:            proj.Mode,
			PresetID:        proj.PresetID,
			ActiveProjectID: proj.ID,
		}
		return m.renderProjectResult(proj, mustMarshalPayload(nextPayload)), nil

	case strings.HasPrefix(data, "img:undo:"):
		idStr := strings.TrimPrefix(data, "img:undo:")
		projectID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || projectID <= 0 {
			return m.renderHome(state), nil
		}
		proj, err := m.service.GetProject(ctx, u.ID, projectID)
		if err != nil {
			return m.renderHome(state), nil
		}
		if proj.CurrentVersion > 1 {
			proj, err = m.service.Restore(ctx, u.ID, projectID, proj.CurrentVersion-1)
			if err != nil {
				return nil, err
			}
		}
		nextPayload := fsmPayload{
			Mode:            proj.Mode,
			PresetID:        proj.PresetID,
			ActiveProjectID: proj.ID,
		}
		return m.renderProjectResult(proj, mustMarshalPayload(nextPayload)), nil

	default:
		return &core.ModuleResult{Handled: false}, nil
	}
}

func (m *Module) renderHome(state *core.State) *core.ModuleResult {
	buttons := [][]core.ResponseButton{
		{{Text: "📚 Сценарии с примерами", Data: "img:presets"}},
		{
			{Text: "🎨 Свободная генерация", Data: "img:mode:generate"},
			{Text: "🪄 Исправить фото", Data: "img:mode:edit"},
		},
		{{Text: "🗂 Мои работы", Data: "img:history"}},
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
		Text:       "🖼 Фотостудия\n\nСоздавайте изображения с нуля, улучшайте фото по готовым сценариям и итерируйте результат прямо в диалоге.",
		Buttons:    buttons,
		NewStep:    StepIdle,
		NewPayload: payload,
		Handled:    true,
	}
}

func (m *Module) renderPresetCategories() *core.ModuleResult {
	return &core.ModuleResult{
		Text: "📚 Выберите категорию сценариев:",
		Buttons: [][]core.ResponseButton{
			{{Text: "✏️ Редактирование фото", Data: "img:cat:edit"}},
			{{Text: "✨ Генерация с нуля", Data: "img:cat:generate"}},
			{{Text: "⬅️ В меню", Data: "img:home"}},
		},
		NewStep: StepIdle,
		Handled: true,
	}
}

func (m *Module) renderPresetList(mode string) *core.ModuleResult {
	presets := m.service.ListPresets()
	var buttons [][]core.ResponseButton
	for _, p := range presets {
		if p.Mode != mode {
			continue
		}
		buttons = append(buttons, []core.ResponseButton{
			{Text: p.Title, Data: "img:preset:" + p.ID},
		})
	}
	buttons = append(buttons, []core.ResponseButton{
		{Text: "⬅️ Назад", Data: "img:presets"},
	})

	title := "✨ Сценарии генерации:"
	if mode == ModeEdit {
		title = "✏️ Сценарии редактирования фото:"
	}

	return &core.ModuleResult{
		Text:    title,
		Buttons: buttons,
		NewStep: StepIdle,
		Handled: true,
	}
}

func (m *Module) renderPresetDetail(presetID string) *core.ModuleResult {
	preset, ok := GetPreset(presetID)
	if !ok {
		return m.renderPresetCategories()
	}

	backTarget := "img:cat:generate"
	if preset.Mode == ModeEdit {
		backTarget = "img:cat:edit"
	}

	text := fmt.Sprintf(
		"📌 %s\n\n%s\n\nПример запроса:\n«%s»",
		preset.Title,
		preset.Description,
		preset.ExamplePrompt,
	)

	return &core.ModuleResult{
		Text: text,
		Buttons: [][]core.ResponseButton{
			{{Text: "🧪 Попробовать с примером", Data: "img:try:" + preset.ID}},
			{{Text: "✨ Использовать сценарий", Data: "img:use:" + preset.ID}},
			{{Text: "⬅️ Назад", Data: backTarget}},
		},
		NewStep: StepIdle,
		Handled: true,
	}
}

func (m *Module) renderHistory(projects []Project) *core.ModuleResult {
	var ready []Project
	for _, p := range projects {
		if p.CurrentVersion == 0 || p.CurrentImageURL == "" {
			continue
		}
		ready = append(ready, p)
	}
	if len(ready) == 0 {
		return &core.ModuleResult{
			Text:    "🗂 У вас пока нет сохранённых работ.",
			Buttons: [][]core.ResponseButton{{{Text: "🏠 В меню", Data: "img:home"}}},
			NewStep: StepIdle,
			Handled: true,
		}
	}

	limit := len(ready)
	if limit > 8 {
		limit = 8
	}
	buttons := make([][]core.ResponseButton, 0, limit+1)
	for i := 0; i < limit; i++ {
		p := ready[i]
		label := fmt.Sprintf("%s (v%d)", p.Title, p.CurrentVersion)
		buttons = append(buttons, []core.ResponseButton{
			{Text: label, Data: fmt.Sprintf("img:project:%d", p.ID)},
		})
	}
	buttons = append(buttons, []core.ResponseButton{
		{Text: "🏠 В меню", Data: "img:home"},
	})

	return &core.ModuleResult{
		Text:    "🗂 Ваши недавние работы:",
		Buttons: buttons,
		NewStep: StepIdle,
		Handled: true,
	}
}

func (m *Module) renderProjectResult(proj *Project, payload json.RawMessage) *core.ModuleResult {
	text := fmt.Sprintf(
		"✅ %s (v%d)\nМодель: %s | Формат: %s\n\n%s\n\nОтправьте текстовую правку, чтобы создать следующую версию.",
		proj.Title,
		proj.CurrentVersion,
		proj.ImageModel,
		proj.AspectRatio,
		proj.CurrentImageURL,
	)

	return &core.ModuleResult{
		Text:       text,
		Buttons:    m.projectActionButtons(proj.ID, proj.CurrentVersion),
		NewStep:    StepReady,
		NewPayload: payload,
		Handled:    true,
	}
}

func (m *Module) projectActionButtons(projectID int64, version int) [][]core.ResponseButton {
	var buttons [][]core.ResponseButton
	if version > 1 {
		buttons = append(buttons, []core.ResponseButton{
			{Text: "↩️ Отменить последнюю правку", Data: fmt.Sprintf("img:undo:%d", projectID)},
		})
	}
	buttons = append(buttons, []core.ResponseButton{
		{Text: "🗂 Мои работы", Data: "img:history"},
		{Text: "🏠 В меню", Data: "img:home"},
	})
	if m.webAppURL != "" {
		buttons = append(buttons, []core.ResponseButton{
			{Text: "🚀 Открыть в студии", WebApp: m.webAppURL},
		})
	}
	return buttons
}

func (m *Module) editModePromptButtons() [][]core.ResponseButton {
	buttons := [][]core.ResponseButton{
		{{Text: "⬅️ В меню", Data: "img:home"}},
	}
	if m.webAppURL != "" {
		buttons = append([][]core.ResponseButton{
			{{Text: "🚀 Загрузить фото в студии", WebApp: m.webAppURL}},
		}, buttons...)
	}
	return buttons
}

func parseFSMPayload(state *core.State) fsmPayload {
	if state == nil || len(state.Payload) == 0 {
		return fsmPayload{}
	}
	var p fsmPayload
	_ = json.Unmarshal(state.Payload, &p)
	return p
}

func mustMarshalPayload(p fsmPayload) json.RawMessage {
	raw, err := json.Marshal(p)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
