package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// TechCard defines technical disclosure metadata for an action or scenario step.
type TechCard struct {
	Title       string
	Endpoint    string
	Method      string
	FSMState    string
	Payload     string
	Description string
	DocsLink    string
}

// HoodEngine manages "Under-The-Hood" technical cards and developer mode toggling.
type HoodEngine struct {
	cards   map[string]TechCard
	devMode map[int64]bool
	mu      sync.RWMutex
}

// NewHoodEngine creates a new Under-The-Hood explanation manager.
func NewHoodEngine() *HoodEngine {
	return &HoodEngine{
		cards:   make(map[string]TechCard),
		devMode: make(map[int64]bool),
	}
}

// Register adds or updates a technical card.
func (h *HoodEngine) Register(key string, card TechCard) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cards[key] = card
}

// IsDevMode returns true if developer mode is enabled for the given user.
func (h *HoodEngine) IsDevMode(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.devMode[userID]
}

// ToggleDevMode flips developer mode for the given user and returns the new state.
func (h *HoodEngine) ToggleDevMode(userID int64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	newState := !h.devMode[userID]
	h.devMode[userID] = newState
	return newState
}

// Format formats a message body, appending a developer badge if developer mode is active.
func (h *HoodEngine) Format(userID int64, text string, cardKey ...string) string {
	if !h.IsDevMode(userID) || len(cardKey) == 0 {
		return text
	}

	h.mu.RLock()
	card, ok := h.cards[cardKey[0]]
	h.mu.RUnlock()

	if !ok {
		return text
	}

	var sb strings.Builder
	sb.WriteString(text)
	sb.WriteString("\n\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("🛠 **[Режим разработчика: Под капотом]**\n")
	if card.Endpoint != "" {
		sb.WriteString(fmt.Sprintf("• **API Method:** `%s %s`\n", card.Method, card.Endpoint))
	}
	if card.FSMState != "" {
		sb.WriteString(fmt.Sprintf("• **FSM State:** `%s`\n", card.FSMState))
	}
	if card.Description != "" {
		sb.WriteString(fmt.Sprintf("• **Механика:** %s\n", card.Description))
	}
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return sb.String()
}

// Button returns a standard "🔍 Как это устроено?" inline callback button for a given card key.
func (h *HoodEngine) Button(cardKey string) maxbot.Button {
	return maxbot.NewCallbackButton("🔍 Как это устроено?", "hood:"+cardKey)
}

// RegisterRoutes hooks the callback handler into a bot router.
func (h *HoodEngine) RegisterRoutes(router *bot.Router) {
	router.OnCallbackPrefix("hood:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}

		key := strings.TrimPrefix(cb.Payload, "hood:")
		h.mu.RLock()
		card, ok := h.cards[key]
		h.mu.RUnlock()

		if !ok {
			return c.AnswerNotification("Информация для этого шага пока не зарегистрирована.")
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("⚙️ **ПОД КАПОТОМ: %s**\n\n", card.Title))
		if card.Endpoint != "" {
			sb.WriteString(fmt.Sprintf("🌐 **MAX API:** `%s %s`\n", card.Method, card.Endpoint))
		}
		if card.FSMState != "" {
			sb.WriteString(fmt.Sprintf("🔄 **FSM State:** `%s`\n", card.FSMState))
		}
		if card.Description != "" {
			sb.WriteString(fmt.Sprintf("\n💡 **Как это работает:**\n%s\n", card.Description))
		}
		if card.Payload != "" {
			sb.WriteString(fmt.Sprintf("\n📦 **Payload в коде:**\n```json\n%s\n```\n", card.Payload))
		}
		if card.DocsLink != "" {
			sb.WriteString(fmt.Sprintf("\n📖 **Документация:** %s\n", card.DocsLink))
		}

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("◀️ Скрыть карточку", "hood:dismiss"))

		_, err := c.ReplyKeyboard(sb.String(), kb)
		_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
			Notification: "Открыта техническая карточка",
		})
		return err
	})

	router.OnCallbackPayload("hood:dismiss", func(c *bot.Context) error {
		if cb := c.Callback(); cb != nil && cb.Message != nil {
			if c.Client() != nil {
				_ = c.Client().DeleteMessage(c.Ctx(), cb.Message.ID())
			}
		}
		return c.AnswerNotification("Карточка закрыта")
	})
}
