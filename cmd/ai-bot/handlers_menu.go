package main

import (
	"fmt"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/gma"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// Available model presets for switching.
var availableChatModels = []string{
	"@scout",
	"garda-core",
	"archipelago-core-70b",
	"glm-4.6-plus",
	"codestral-2",
}

var availableVisionModels = []string{
	"@scout-vision",
	"glm-4.6-plus",
	"archipelago-core-70b",
}

var availableImageModels = []string{
	"flux",
	"sdxl",
	"turbo",
}

// BuildQuickActionsKeyboard returns a compact inline keyboard for everyday actions.
func BuildQuickActionsKeyboard() *maxbot.InlineKeyboard {
	return maxbot.NewInlineKeyboard().
		Row(
			maxbot.NewCallbackButton("💬 Новый диалог", "action:clear"),
			maxbot.NewCallbackButton("⚙️ Выбор моделей", "nav:models"),
		).
		Row(
			maxbot.NewCallbackButton("🎨 Сгенерировать арт", "nav:prompt_image"),
			maxbot.NewCallbackButton("❓ Помощь", "nav:help"),
		)
}

// RegisterMenuHandlers registers model selection and navigation callbacks.
func RegisterMenuHandlers(b *bot.Bot, sm *gma.SessionManager) {
	// Command: /model
	b.OnCommand("model", func(c *bot.Context) error {
		userID := getUserID(c)
		chatModel, visionModel, imageModel := sm.GetModels(userID)

		text := fmt.Sprintf("⚙️ **Управление AI-моделями**\n\n"+
			"Текущие настройки для вашего аккаунта:\n"+
			"• 💬 Текстовая модель (Чат): `%s`\n"+
			"• 👁️ Мультимодальная модель (Vision): `%s`\n"+
			"• 🎨 Модель генерации картинок: `%s`\n\n"+
			"Выберите категорию для изменения:", chatModel, visionModel, imageModel)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("💬 Сменить модель Чата", "menu:models:chat")).
			Row(maxbot.NewCallbackButton("👁️ Сменить модель Vision", "menu:models:vision")).
			Row(maxbot.NewCallbackButton("🎨 Сменить модель Картинок", "menu:models:image")).
			Row(maxbot.NewCallbackButton("◀️ Назад в меню", "nav:menu"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Callback: nav:models
	b.OnCallbackPayload("nav:models", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := getUserID(c)
		chatModel, visionModel, imageModel := sm.GetModels(userID)

		text := fmt.Sprintf("⚙️ **Управление AI-моделями**\n\n"+
			"Текущие настройки:\n"+
			"• 💬 Текст: `%s`\n"+
			"• 👁️ Vision: `%s`\n"+
			"• 🎨 Картинки: `%s`\n\n"+
			"Выберите параметр для настройки:", chatModel, visionModel, imageModel)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("💬 Сменить модель Чата", "menu:models:chat")).
			Row(maxbot.NewCallbackButton("👁️ Сменить модель Vision", "menu:models:vision")).
			Row(maxbot.NewCallbackButton("🎨 Сменить модель Картинок", "menu:models:image")).
			Row(maxbot.NewCallbackButton("◀️ Назад в меню", "nav:menu"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Callback: nav:menu
	b.OnCallbackPayload("nav:menu", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := getUserID(c)
		chatModel, visionModel, imageModel := sm.GetModels(userID)

		text := fmt.Sprintf("🤖 **Главное меню AI Ассистента**\n\n"+
			"• Чат: `%s` | Vision: `%s` | Картинки: `%s`", chatModel, visionModel, imageModel)

		kb := BuildQuickActionsKeyboard()
		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Callback: nav:prompt_image
	b.OnCallbackPayload("nav:prompt_image", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		text := "🎨 **Как сгенерировать изображение:**\n\n" +
			"Отправьте команду с описанием того, что вы хотите увидеть:\n\n" +
			"`/image футуристический город в стиле киберпанк на закате`\n\n" +
			"или\n\n" +
			"`/draw милый рыжий кот в очках читает книгу в библиотеке`"
		_, err := c.Reply(text)
		return err
	})

	// Callback: menu:models:chat
	b.OnCallbackPayload("menu:models:chat", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := getUserID(c)
		current, _, _ := sm.GetModels(userID)

		text := fmt.Sprintf("💬 **Выберите языковую модель для чата**\nТекущая: `%s`", current)
		kb := maxbot.NewInlineKeyboard()
		for _, m := range availableChatModels {
			mark := ""
			if m == current {
				mark = " ✅"
			}
			kb.Row(maxbot.NewCallbackButton(m+mark, "set:chat:"+m))
		}
		kb.Row(maxbot.NewCallbackButton("◀️ К списку моделей", "nav:models"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Callback: menu:models:vision
	b.OnCallbackPayload("menu:models:vision", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := getUserID(c)
		_, current, _ := sm.GetModels(userID)

		text := fmt.Sprintf("👁️ **Выберите модель для анализа изображений (Vision)**\nТекущая: `%s`", current)
		kb := maxbot.NewInlineKeyboard()
		for _, m := range availableVisionModels {
			mark := ""
			if m == current {
				mark = " ✅"
			}
			kb.Row(maxbot.NewCallbackButton(m+mark, "set:vision:"+m))
		}
		kb.Row(maxbot.NewCallbackButton("◀️ К списку моделей", "nav:models"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Callback: menu:models:image
	b.OnCallbackPayload("menu:models:image", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := getUserID(c)
		_, _, current := sm.GetModels(userID)

		text := fmt.Sprintf("🎨 **Выберите модель генерации изображений**\nТекущая: `%s`", current)
		kb := maxbot.NewInlineKeyboard()
		for _, m := range availableImageModels {
			mark := ""
			if m == current {
				mark = " ✅"
			}
			kb.Row(maxbot.NewCallbackButton(m+mark, "set:image:"+m))
		}
		kb.Row(maxbot.NewCallbackButton("◀️ К списку моделей", "nav:models"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Callback: set:chat:<model>
	b.OnCallbackPrefix("set:chat:", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		cb := c.Callback()
		model := strings.TrimPrefix(cb.Payload, "set:chat:")
		userID := getUserID(c)
		sm.SetChatModel(userID, model)

		_, err := c.Reply(fmt.Sprintf("✅ **Текстовая модель изменена на:** `%s`", model))
		return err
	})

	// Callback: set:vision:<model>
	b.OnCallbackPrefix("set:vision:", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		cb := c.Callback()
		model := strings.TrimPrefix(cb.Payload, "set:vision:")
		userID := getUserID(c)
		sm.SetVisionModel(userID, model)

		_, err := c.Reply(fmt.Sprintf("✅ **Модель распознавания фото изменена на:** `%s`", model))
		return err
	})

	// Callback: set:image:<model>
	b.OnCallbackPrefix("set:image:", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		cb := c.Callback()
		model := strings.TrimPrefix(cb.Payload, "set:image:")
		userID := getUserID(c)
		sm.SetImageModel(userID, model)

		_, err := c.Reply(fmt.Sprintf("✅ **Модель генерации картинок изменена на:** `%s`", model))
		return err
	})
}
