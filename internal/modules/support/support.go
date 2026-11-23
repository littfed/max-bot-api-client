package support

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// Ticket represents an escalated support ticket.
type Ticket struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Platform  string    `json:"platform"`
	ModeScope string    `json:"mode_scope"`
	Subject   string    `json:"subject"`
	Status    string    `json:"status"` // "open", "answered", "closed"
	CreatedAt time.Time `json:"created_at"`
}

type kbChunk struct {
	id        int64
	modeScope string
	title     string
	content   string
	tokens    map[string]struct{}
}

// Module implements the BotModule interface for Support mode with Scoped RAG.
type Module struct {
	mu       sync.RWMutex
	chunks   []kbChunk
	ticketMu sync.Mutex
	tickets  []Ticket
}

// New creates a new Support module instance.
func New(st any) *Module {
	m := &Module{
		chunks:  make([]kbChunk, 0),
		tickets: make([]Ticket, 0),
	}
	m.seedKnowledgeBase()
	return m
}

func tokenize(text string) map[string]struct{} {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make(map[string]struct{}, len(words))
	for _, w := range words {
		if len(w) > 2 { // filter out 1-2 char words
			tokens[w] = struct{}{}
		}
	}
	return tokens
}

func (m *Module) seedKnowledgeBase() {
	rawDocs := []struct {
		scope   string
		title   string
		content string
	}{
		{
			scope:   "shop",
			title:   "Как оформить подписку",
			content: "Для оформления подписки выберите тариф в каталоге Магазина (MAX Pro или VIP Клуб), нажмите «Купить» и проведите оплату по QR-коду. Доступ к закрытым каналам и сервисам будет предоставлен мгновенно.",
		},
		{
			scope:   "shop",
			title:   "Возврат средств",
			content: "Возврат средств за цифровые покупки осуществляется в течение 14 дней при обращении в техническую поддержку.",
		},
		{
			scope:   "reception",
			title:   "Запись на консультацию",
			content: "В разделе Ресепшен выберите доступную услугу (Консультация архитектора или Экспресс-аудит), подберите удобный временной слот и подтвердите бронирование.",
		},
		{
			scope:   "reception",
			title:   "QR-билет на мероприятие",
			content: "После успешной регистрации на мероприятие в разделе Ресепшен бот автоматически генерирует персональный QR-билет для входа.",
		},
		{
			scope:   "chat",
			title:   "Смена роли и тональности",
			content: "В режиме «Общение» нажмите кнопку «Роль» (Ассистент, Аналитик, Маркетолог, Критик, Ментор) или «Тон» (Деловой, Дружелюбный, Лаконичный, Юмористический) для настройки LLM.",
		},
		{
			scope:   "chat",
			title:   "Очистка контекста диалога",
			content: "Нажмите кнопку «Очистить диалог» в меню режима Общение, чтобы сбросить скользящее окно памяти из 10 сообщений.",
		},
		{
			scope:   "global",
			title:   "О платформе Bot Hub",
			content: "Bot Hub — это мультирежимная демо-платформа «8 виртуальных ботов в одном», демонстрирующая возможности коммерческих ботов и Mini App.",
		},
		{
			scope:   "global",
			title:   "Заказ коммерческого бота",
			content: "Чтобы заказать разработку собственного бота или Mini App, нажмите закрепленную кнопку «Хочу такого бота» или отправьте команду /lead.",
		},
	}

	for i, doc := range rawDocs {
		fullText := doc.title + " " + doc.content
		m.chunks = append(m.chunks, kbChunk{
			id:        int64(i + 1),
			modeScope: doc.scope,
			title:     doc.title,
			content:   doc.content,
			tokens:    tokenize(fullText),
		})
	}
}

// ID returns the module identifier.
func (m *Module) ID() string {
	return "support"
}

// Title returns the human-readable display title.
func (m *Module) Title() string {
	return "Поддержка"
}

// Ask queries the hybrid RAG knowledge base filtered by mode_scope.
// Returns the best matching chunk answer, confidence score (0.0 to 1.0), and error.
func (m *Module) Ask(ctx context.Context, question string, modeScope string) (string, float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	qTokens := tokenize(question)
	if len(qTokens) == 0 {
		return "", 0.0, nil
	}

	var bestContent string
	var bestScore float64

	for _, chunk := range m.chunks {
		// Filter by mode scope: matches specific scope or global
		if chunk.modeScope != modeScope && chunk.modeScope != "global" && modeScope != "" && modeScope != "all" {
			continue
		}

		matchCount := 0
		for qt := range qTokens {
			// Exact match or substring prefix match
			for ct := range chunk.tokens {
				if qt == ct || strings.HasPrefix(ct, qt) || strings.HasPrefix(qt, ct) {
					matchCount++
					break
				}
			}
		}

		if matchCount == 0 {
			continue
		}

		// Calculate similarity score (Jaccard + query coverage)
		coverage := float64(matchCount) / float64(len(qTokens))
		jaccard := float64(matchCount) / float64(len(qTokens)+len(chunk.tokens)-matchCount)
		// Combined confidence score
		score := (coverage * 0.7) + (jaccard * 0.3)
		if coverage >= 0.6 {
			// Boost confidence when most query keywords matched
			score = math.Min(1.0, score+0.35)
		}

		if score > bestScore {
			bestScore = score
			bestContent = chunk.content
		}
	}

	return bestContent, bestScore, nil
}

// CreateTicket creates a support ticket escalated to an operator.
func (m *Module) CreateTicket(ctx context.Context, userID int64, platform string, modeScope string, subject string, message string) (*Ticket, error) {
	m.ticketMu.Lock()
	defer m.ticketMu.Unlock()

	ticketID := int64(len(m.tickets) + 1)
	ticket := Ticket{
		ID:        ticketID,
		UserID:    userID,
		Platform:  platform,
		ModeScope: modeScope,
		Subject:   subject,
		Status:    "open",
		CreatedAt: time.Now(),
	}

	m.tickets = append(m.tickets, ticket)
	return &ticket, nil
}

// HandleMessage processes incoming questions with RAG or ticket creation.
func (m *Module) HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	text := strings.TrimSpace(msg.Text)

	if state.CurrentStep == "awaiting_ticket_desc" {
		ticket, err := m.CreateTicket(ctx, u.ID, msg.Platform, u.ActiveModeID, "Вопрос пользователя", text)
		if err != nil {
			return nil, err
		}

		return &core.ModuleResult{
			Text: fmt.Sprintf(
				"✅ **Тикет #%d успешно создан!**\n\n"+
					"Тема: _%s_\n"+
					"Статус: _В очереди к живому оператору_\n\n"+
					"Оператор техподдержки свяжется с вами прямо в этом диалоге.",
				ticket.ID, text,
			),
			Buttons: [][]core.ResponseButton{
				{{Text: "« Задать другой вопрос", Data: "support:faq"}},
			},
			ResetState: true,
			Handled:    true,
		}, nil
	}

	activeScope := u.ActiveModeID
	if activeScope == "" {
		activeScope = "global"
	}

	ans, score, err := m.Ask(ctx, text, activeScope)
	if err != nil {
		return nil, err
	}

	// Confidence threshold is 0.72
	const confidenceThreshold = 0.72

	if score >= confidenceThreshold && ans != "" {
		reply := fmt.Sprintf(
			"🤖 **Ответ базы знаний (уверенность: %.0f%%):**\n\n%s",
			score*100, ans,
		)
		buttons := [][]core.ResponseButton{
			{
				{Text: "👍 Ответ помог", Data: "support:feedback:ok"},
				{Text: "👨‍💻 Связаться с оператором", Data: "support:ticket:open"},
			},
		}
		return &core.ModuleResult{
			Text:    reply,
			Buttons: buttons,
			Handled: true,
		}, nil
	}

	// Below threshold: offer operator escalation
	reply := fmt.Sprintf(
		"🤖 В базе знаний нет точного ответа на ваш вопрос (уверенность: %.0f%%).\n\n"+
			"Хотите передать обращение живому специалисту службы поддержки?",
		score*100,
	)
	buttons := [][]core.ResponseButton{
		{
			{Text: "👨‍💻 Создать тикет оператору", Data: "support:ticket:open"},
		},
		{
			{Text: "« Частые вопросы (FAQ)", Data: "support:faq"},
		},
	}

	return &core.ModuleResult{
		Text:    reply,
		Buttons: buttons,
		Handled: true,
	}, nil
}

// HandleCallback handles buttons in Support mode.
func (m *Module) HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	data := cb.Data

	if data == "support:ticket:open" {
		return &core.ModuleResult{
			Text: "👨‍💻 **Обращение в службу поддержки**\n\n" +
				"Пожалуйста, подробно опишите ваш вопрос или проблему одним сообщением, и мы передадим ее оператору:",
			NewStep: "awaiting_ticket_desc",
			Handled: true,
		}, nil
	}

	if data == "support:feedback:ok" {
		return &core.ModuleResult{
			Text: "🎉 Рады, что смогли помочь! Если у вас появятся новые вопросы — просто напишите их в чат.",
			Buttons: [][]core.ResponseButton{
				{{Text: "« Задать еще вопрос", Data: "support:faq"}},
			},
			ResetState: true,
			Handled:    true,
		}, nil
	}

	if data == "support:faq" {
		return &core.ModuleResult{
			Text: "🎧 **Служба поддержки Bot Hub**\n\n" +
				"Напишите любой вопрос обычным текстом, например:\n" +
				"• _«Как оформить подписку?»_\n" +
				"• _«Как записаться на консультацию?»_\n" +
				"• _«Как сменить роль или тон?»_\n" +
				"• _«Что такое Bot Hub?»_",
			Handled: true,
		}, nil
	}

	return &core.ModuleResult{
		Text:    "Команда не распознана.",
		Handled: false,
	}, nil
}
