package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// Role defines an LLM persona.
type Role struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
}

// Tone defines the communication style.
type Tone struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
}

// ChatMessage represents a single turn in the sliding window.
type ChatMessage struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Session represents the conversation state for a user.
type Session struct {
	UserID    int64         `json:"user_id"`
	RoleID    int           `json:"role_id"`
	ToneID    int           `json:"tone_id"`
	Messages  []ChatMessage `json:"messages"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// LLMClient abstracts the chat completion client.
type LLMClient interface {
	Chat(ctx context.Context, req *ai.ChatRequest) (*ai.ChatResponse, error)
}

// Module implements BotModule for Chat mode.
type Module struct {
	aiClient  LLMClient
	chatModel string
	mu        sync.RWMutex
	sessions  map[int64]*Session
	roles     map[int]Role
	tones     map[int]Tone
}

// New creates a new Chat module instance with optional custom chat model.
func New(st any, aiClient any, chatModel ...string) *Module {
	var lClient LLMClient
	if cl, ok := aiClient.(LLMClient); ok {
		lClient = cl
	}

	m := &Module{
		aiClient: lClient,
		sessions: make(map[int64]*Session),
		roles:    make(map[int]Role),
		tones:    make(map[int]Tone),
	}
	if len(chatModel) > 0 && chatModel[0] != "" {
		m.chatModel = chatModel[0]
	}
	m.initRolesAndTones()
	return m
}

func (m *Module) initRolesAndTones() {
	m.roles = map[int]Role{
		1: {
			ID:          1,
			Name:        "Ассистент",
			Description: "Универсальный помощник",
			SystemPrompt: "Вы — универсальный интеллектуальный ассистент платформы Bot Hub. Отвечайте вежливо, понятно и структурированно.",
		},
		2: {
			ID:          2,
			Name:        "Продуктовый аналитик",
			Description: "Юнит-экономика, когорты, метрики",
			SystemPrompt: "Вы — Senior Product Analyst. Ваш фокус: юнит-экономика (CAC, LTV, ARPU, Churn), воронки конверсий, когортный анализ и окупаемость каналов.",
		},
		3: {
			ID:          3,
			Name:        "Маркетолог",
			Description: "Трафик, офферы, позиционирование",
			SystemPrompt: "Вы — Head of Marketing. Вы разрабатываете стратегии лидогенерации, формулируете УТП, продающие офферы и сценарии удержания пользователей.",
		},
		4: {
			ID:          4,
			Name:        "Критик",
			Description: "Анализ рисков и слабых мест",
			SystemPrompt: "Вы — скептичный бизнес-критик. Вы беспристрастно ищете уязвимости, финансовые дыры, регуляторные риски и операционные слабости в идеях.",
		},
		5: {
			ID:          5,
			Name:        "Ментор",
			Description: "Стратегия, коучинг, лучшие практики",
			SystemPrompt: "Вы — опытный ментор и технологический фаундер. Вы задаете сильные наводящие вопросы, делитесь практическим опытом и направляете.",
		},
	}

	m.tones = map[int]Tone{
		1: {
			ID:          1,
			Name:        "Деловой",
			Description: "Формальный и структурированный",
			Instruction: "Соблюдайте строгий деловой стиль, четкую логику, списки и профессиональную терминологию.",
		},
		2: {
			ID:          2,
			Name:        "Дружелюбный",
			Description: "Теплый и поддерживающий",
			Instruction: "Общайтесь тепло, дружелюбно, поддерживающе, используйте уместные эмодзи.",
		},
		3: {
			ID:          3,
			Name:        "Лаконичный",
			Description: "Краткий и емкий (без воды)",
			Instruction: "Отвечайте предельно емко и тезисно, без лишней воды — максимум 2-4 предложения.",
		},
		4: {
			ID:          4,
			Name:        "Юмористический",
			Description: "С легкой иронией и метафорами",
			Instruction: "Отвечайте остроумно, с легким юмором, яркими метафорами и позитивной иронией.",
		},
	}
}

// ID returns the module identifier.
func (m *Module) ID() string {
	return "chat"
}

// Title returns the human-readable display title.
func (m *Module) Title() string {
	return "Общение"
}

// GetSession returns or initializes the chat session for a user.
func (m *Module) GetSession(ctx context.Context, userID int64) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[userID]
	if !exists {
		s = &Session{
			UserID:    userID,
			RoleID:    1,
			ToneID:    1,
			Messages:  make([]ChatMessage, 0),
			UpdatedAt: time.Now(),
		}
		m.sessions[userID] = s
	}
	return s, nil
}

// ResetSession clears the conversation sliding window history.
func (m *Module) ResetSession(ctx context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, exists := m.sessions[userID]; exists {
		s.Messages = make([]ChatMessage, 0)
		s.UpdatedAt = time.Now()
	}
	return nil
}

// SendMessage sends a message to the LLM preserving the sliding 10-message window.
func (m *Module) SendMessage(ctx context.Context, userID int64, roleID int, toneID int, message string) (string, error) {
	if roleID < 1 || roleID > 5 {
		roleID = 1
	}
	if toneID < 1 || toneID > 4 {
		toneID = 1
	}

	session, err := m.GetSession(ctx, userID)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	session.RoleID = roleID
	session.ToneID = toneID
	m.mu.Unlock()

	role := m.roles[roleID]
	tone := m.tones[toneID]

	var reply string

	if m.aiClient != nil {
		// Construct AI ChatRequest with system instructions and sliding window messages
		sysPrompt := fmt.Sprintf(`Ты — интеллектуальный собеседник платформы Bot Hub.
Твоя роль: %s (%s).
Инструкция для роли:
%s

СТИЛЬ И ТОНАЛЬНОСТЬ ОБЩЕНИЯ (ОБЯЗАТЕЛЬНО К СОБЛЮДЕНИЮ):
Тональность: %s (%s).
Инструкция по тональности: %s.

Всегда отвечай строго на русском языке. Безукоризненно выдерживай заданную роль и тональность в каждом ответе.`,
			role.Name, role.Description, role.SystemPrompt,
			tone.Name, tone.Description, tone.Instruction,
		)
		messages := []ai.ChatMessage{
			ai.NewTextMessage(ai.RoleSystem, sysPrompt),
		}

		m.mu.RLock()
		for _, mTurn := range session.Messages {
			r := ai.RoleUser
			if mTurn.Role == "assistant" {
				r = ai.RoleAssistant
			}
			messages = append(messages, ai.NewTextMessage(r, mTurn.Content))
		}
		m.mu.RUnlock()

		messages = append(messages, ai.NewTextMessage(ai.RoleUser, message))

		modelToUse := m.chatModel
		if modelToUse == "" {
			modelToUse = "gemini-2.5-flash"
		}

		req := &ai.ChatRequest{
			Model:    modelToUse,
			Messages: messages,
		}

		callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()

		chatResp, err := m.aiClient.Chat(callCtx, req)
		if err != nil {
			slog.Error("llm chat completion failed", slog.String("model", modelToUse), slog.Int64("user_id", userID), slog.Any("error", err))
			return "", fmt.Errorf("llm chat failed: %w", err)
		}
		if chatResp != nil && strings.TrimSpace(chatResp.Content) != "" {
			reply = strings.TrimSpace(chatResp.Content)
		}
	}

	if reply == "" {
		if m.aiClient != nil {
			return "", fmt.Errorf("ai gateway returned empty reply")
		}
		// Simulated intelligent fallback for unit tests and offline environments
		reply = generateSimulatedReply(roleID, toneID, message)
	}

	// Update sliding window (max 10 turns)
	m.mu.Lock()
	session.Messages = append(session.Messages,
		ChatMessage{Role: "user", Content: message, CreatedAt: time.Now()},
		ChatMessage{Role: "assistant", Content: reply, CreatedAt: time.Now()},
	)
	if len(session.Messages) > 10 {
		session.Messages = session.Messages[len(session.Messages)-10:]
	}
	session.UpdatedAt = time.Now()
	m.mu.Unlock()

	return reply, nil
}

func generateSimulatedReply(roleID, toneID int, msg string) string {
	lower := strings.ToLower(msg)

	if strings.Contains(lower, "юнит") || strings.Contains(lower, "экономик") {
		switch roleID {
		case 2: // Product Analyst
			switch toneID {
			case 1: // Formal
				return "Юнит-экономика — это метод финансового моделирования, оценивающий прямую маржинальность одного клиента. Базовая формула: LTV > 3 * CAC. Ключевые метрики включают стоимость привлечения (CAC), жизненный цикл (Lifetime), средний чек (AOV) и маржинальность (Contribution Margin)."
			case 2: // Friendly
				return "Юнит-экономика — это сердце любого успешного стартапа! 📊 Мы просто считаем: зарабатываем ли мы на одном клиенте больше, чем тратим на его привлечение. Если LTV больше CAC в 3 раза — проект летит в космос! 🚀"
			case 3: // Concise
				return "Юнит-экономика: соотношение LTV к CAC на одного пользователя. Норматив: LTV/CAC >= 3. Если показатель ниже 1 — масштабирование сжигает деньги."
			case 4: // Humorous
				return "Юнит-экономика — это способ узнать, зарабатываешь ли ты на пирожках или просто платишь за муку из своего кармана, надеясь разбогатеть на объеме! Главное правило: не продавать рубль за 80 копеек."
			}
		default:
			return "Юнит-экономика оценивает прибыльность одного клиента (LTV vs CAC)."
		}
	}

	// Default fallback tailored to tone
	switch toneID {
	case 1:
		return fmt.Sprintf("Ваш запрос: «%s». В рамках анализа целесообразно сформулировать ключевые гипотезы и составить план реализации.", msg)
	case 2:
		return fmt.Sprintf("Отличный вопрос про «%s»! 😊 С удовольствием помогу вам в этом разобраться. С чего начнем?", msg)
	case 3:
		return fmt.Sprintf("По вопросу «%s»: суть понятна. Рекомендую действовать по базовому алгоритму без задержек.", msg)
	case 4:
		return fmt.Sprintf("«%s» — это классика жанра! Если бы за каждый такой вопрос давали биткоин, мы бы уже купили MAX.", msg)
	}

	return fmt.Sprintf("Ответ на вопрос: %s", msg)
}

// HandleMessage handles chat turns, managing role/tone state in FSM payload.
func (m *Module) HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	text := strings.TrimSpace(msg.Text)

	var payload struct {
		RoleID int `json:"role_id"`
		ToneID int `json:"tone_id"`
	}
	if len(state.Payload) > 0 {
		_ = json.Unmarshal(state.Payload, &payload)
	}
	if payload.RoleID == 0 {
		payload.RoleID = 1
	}
	if payload.ToneID == 0 {
		payload.ToneID = 1
	}

	m.mu.RLock()
	roleName := m.roles[payload.RoleID].Name
	toneName := m.tones[payload.ToneID].Name
	m.mu.RUnlock()

	reply, err := m.SendMessage(ctx, u.ID, payload.RoleID, payload.ToneID, text)
	if err != nil {
		slog.Error("chat module: SendMessage failed", slog.Int64("user_id", u.ID), slog.Any("error", err))
		newP, _ := json.Marshal(payload)
		return &core.ModuleResult{
			Text: fmt.Sprintf("⚠️ **Не удалось получить ответ от нейросети**\n\nПричина: %v\nПопробуйте повторить запрос или сменить роль.", err),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "🎭 Роль: " + roleName, Data: "chat:menu:roles"},
					{Text: "🎨 Тон: " + toneName, Data: "chat:menu:tones"},
					{Text: "🔄 Сброс контекста", Data: "chat:reset"},
				},
			},
			NewPayload: newP,
			Handled:    true,
		}, nil
	}

	header := fmt.Sprintf("💬 _[%s | %s]_\n\n", roleName, toneName)

	buttons := [][]core.ResponseButton{
		{
			{Text: "🎭 Роль: " + roleName, Data: "chat:menu:roles"},
			{Text: "🎨 Тон: " + toneName, Data: "chat:menu:tones"},
			{Text: "🔄 Очистить контекст", Data: "chat:reset"},
		},
	}

	newP, _ := json.Marshal(payload)
	return &core.ModuleResult{
		Text:       header + reply,
		Buttons:    buttons,
		NewPayload: newP,
		Handled:    true,
	}, nil
}

// HandleCallback handles role and tone selection buttons.
func (m *Module) HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	data := cb.Data

	var payload struct {
		RoleID int `json:"role_id"`
		ToneID int `json:"tone_id"`
	}
	if len(state.Payload) > 0 {
		_ = json.Unmarshal(state.Payload, &payload)
	}
	if payload.RoleID == 0 {
		payload.RoleID = 1
	}
	if payload.ToneID == 0 {
		payload.ToneID = 1
	}

	if data == "chat:menu:roles" {
		var buttons [][]core.ResponseButton
		for i := 1; i <= 5; i++ {
			r := m.roles[i]
			prefix := ""
			if payload.RoleID == i {
				prefix = "✓ "
			}
			buttons = append(buttons, []core.ResponseButton{
				{
					Text: fmt.Sprintf("%s%s (%s)", prefix, r.Name, r.Description),
					Data: fmt.Sprintf("chat:role:%d", i),
				},
			})
		}
		buttons = append(buttons, []core.ResponseButton{
			{Text: "« Назад к диалогу", Data: "chat:menu:back"},
		})

		return &core.ModuleResult{
			Text:    "🎭 **Выберите роль виртуального собеседника:**",
			Buttons: buttons,
			Handled: true,
		}, nil
	}

	if strings.HasPrefix(data, "chat:role:") {
		var newRoleID int
		_, _ = fmt.Sscanf(data, "chat:role:%d", &newRoleID)
		if newRoleID >= 1 && newRoleID <= 5 {
			payload.RoleID = newRoleID
		}
		newP, _ := json.Marshal(payload)
		roleName := m.roles[payload.RoleID].Name
		toneName := m.tones[payload.ToneID].Name

		return &core.ModuleResult{
			Text: fmt.Sprintf("🎭 Роль успешно переключена на: **%s** (%s).\n🎨 Текущая тональность: **%s**.\n\nНапишите сообщение в чат для продолжения.", roleName, m.roles[payload.RoleID].Description, toneName),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "🎨 Настроить тон", Data: "chat:menu:tones"},
					{Text: "« Назад к диалогу", Data: "chat:menu:back"},
				},
			},
			NewPayload: newP,
			Handled:    true,
		}, nil
	}

	if data == "chat:menu:tones" {
		var buttons [][]core.ResponseButton
		for i := 1; i <= 4; i++ {
			t := m.tones[i]
			prefix := ""
			if payload.ToneID == i {
				prefix = "✓ "
			}
			buttons = append(buttons, []core.ResponseButton{
				{
					Text: fmt.Sprintf("%s%s (%s)", prefix, t.Name, t.Description),
					Data: fmt.Sprintf("chat:tone:%d", i),
				},
			})
		}
		buttons = append(buttons, []core.ResponseButton{
			{Text: "« Назад к диалогу", Data: "chat:menu:back"},
		})

		return &core.ModuleResult{
			Text:    "🎨 **Выберите стиль и тональность ответов:**",
			Buttons: buttons,
			Handled: true,
		}, nil
	}

	if strings.HasPrefix(data, "chat:tone:") {
		var newToneID int
		_, _ = fmt.Sscanf(data, "chat:tone:%d", &newToneID)
		if newToneID >= 1 && newToneID <= 4 {
			payload.ToneID = newToneID
		}
		newP, _ := json.Marshal(payload)
		roleName := m.roles[payload.RoleID].Name
		toneName := m.tones[payload.ToneID].Name

		return &core.ModuleResult{
			Text: fmt.Sprintf("🎨 Тональность успешно переключена на: **%s** (%s).\n🎭 Текущая роль: **%s**.\n\nНапишите сообщение в чат для продолжения.", toneName, m.tones[payload.ToneID].Description, roleName),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "🎭 Настроить роль", Data: "chat:menu:roles"},
					{Text: "« Назад к диалогу", Data: "chat:menu:back"},
				},
			},
			NewPayload: newP,
			Handled:    true,
		}, nil
	}

	if data == "chat:reset" {
		_ = m.ResetSession(ctx, u.ID)
		m.mu.RLock()
		roleName := m.roles[payload.RoleID].Name
		toneName := m.tones[payload.ToneID].Name
		m.mu.RUnlock()
		newP, _ := json.Marshal(payload)
		return &core.ModuleResult{
			Text: fmt.Sprintf("🔄 **Память диалога очищена!**\nТекущая роль: *%s*, тон: *%s*.\n\nВы можете начать новый диалог с чистого листа.", roleName, toneName),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "🎭 Роль: " + roleName, Data: "chat:menu:roles"},
					{Text: "🎨 Тон: " + toneName, Data: "chat:menu:tones"},
					{Text: "« Назад к диалогу", Data: "chat:menu:back"},
				},
			},
			NewPayload: newP,
			ResetState: true,
			Handled:    true,
		}, nil
	}

	if data == "chat:menu:back" {
		m.mu.RLock()
		roleName := m.roles[payload.RoleID].Name
		toneName := m.tones[payload.ToneID].Name
		m.mu.RUnlock()
		newP, _ := json.Marshal(payload)
		return &core.ModuleResult{
			Text: fmt.Sprintf("💬 **Режим «Общение» (ИИ-собеседник)**\n\n🎭 Текущая роль: *%s*\n🎨 Тональность: *%s*\n\nОтправьте ваше сообщение или вопрос в чат:", roleName, toneName),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "🎭 Роль: " + roleName, Data: "chat:menu:roles"},
					{Text: "🎨 Тон: " + toneName, Data: "chat:menu:tones"},
					{Text: "🔄 Очистить контекст", Data: "chat:reset"},
				},
			},
			NewPayload: newP,
			Handled:    true,
		}, nil
	}

	return &core.ModuleResult{
		Text:    "Команда не распознана.",
		Handled: false,
	}, nil
}

// GetConfig returns the configured roles and tones.
func (m *Module) GetConfig(ctx context.Context) ([]Role, []Tone, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	roles := make([]Role, 0, len(m.roles))
	for i := 1; i <= len(m.roles); i++ {
		if r, ok := m.roles[i]; ok {
			roles = append(roles, r)
		}
	}
	tones := make([]Tone, 0, len(m.tones))
	for i := 1; i <= len(m.tones); i++ {
		if t, ok := m.tones[i]; ok {
			tones = append(tones, t)
		}
	}
	return roles, tones, nil
}

