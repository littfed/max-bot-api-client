package main

import (
	"fmt"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// BuildMainMenuKeyboard constructs the main navigation keyboard reflecting current dev mode state.
func BuildMainMenuKeyboard(hood *HoodEngine, userID int64) *maxbot.InlineKeyboard {
	devStatus := "ВЫКЛ ⚪"
	if hood.IsDevMode(userID) {
		devStatus = "ВКЛ 🟢"
	}

	kb := maxbot.NewInlineKeyboard().
		Row(
			maxbot.NewCallbackButton("🛍️ 1. Магазин & СБП Checkout", "scen:ecom"),
		).
		Row(
			maxbot.NewCallbackButton("📅 2. Запись на услуги & Контакт", "scen:booking"),
		).
		Row(
			maxbot.NewCallbackButton("🧠 3. Интерактивный Квиз & TTK", "scen:quiz"),
		).
		Row(
			maxbot.NewCallbackButton("📱 4. Mini App (HTML5 WebView)", "scen:webapp"),
		).
		Row(
			maxbot.NewCallbackButton("🛡️ 5. Модерация & Капча-гейт", "scen:guard"),
		)

	if hood.IsAIAvailable() {
		kb.Row(
			maxbot.NewCallbackButton("✨ 6. Искусственный Интеллект (LLM / Image / Voice)", "scen:ai"),
		)
	}

	kb.Row(
		maxbot.NewCallbackButton("📢 7. Мои каналы & Студия постов", "scen:channels"),
	)

	kb.Row(
		maxbot.NewCallbackButton(fmt.Sprintf("🛠 Режим разработчика: %s", devStatus), "nav:toggle_dev"),
		maxbot.NewCallbackButton("📊 Метрики", "nav:stats"),
	).
		Row(
			maxbot.NewCallbackButton("🔄 Сбросить сессию", "nav:reset"),
			maxbot.NewCallbackButton("ℹ️ О витрине", "nav:info"),
		)

	return kb
}

// RegisterMenuRoutes installs main menu command and callback handlers.
func RegisterMenuRoutes(b *bot.Bot, hood *HoodEngine, aiClient ...*ai.Client) {
	var aiCli *ai.Client
	if len(aiClient) > 0 {
		aiCli = aiClient[0]
	}

	sendMenu := func(c *bot.Context) error {
		userID := int64(0)
		userName := "Гость"
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
			if sender.FirstName != "" {
				userName = sender.FirstName
			}
		}

		greeting := fmt.Sprintf("👋 Привет, *%s*!\n\n"+
			"Добро пожаловать в **Флагманский бот-витрину MAX Bot API**.\n"+
			"Здесь собраны реальные интерактивные демонстрации всех возможностей платформы MAX Messenger.\n\n"+
			"💡 *Выберите интересующий сценарий из меню ниже:*", userName)

		kb := BuildMainMenuKeyboard(hood, userID)
		formatted := hood.Format(userID, greeting, "main_menu")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	}

	// Commands
	b.OnStart(func(c *bot.Context) error {
		return sendMenu(c)
	})

	b.OnCommand("menu", func(c *bot.Context) error {
		return sendMenu(c)
	})

	b.OnCommand("dev", func(c *bot.Context) error {
		userID := int64(0)
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
		}
		newStatus := hood.ToggleDevMode(userID)
		statusStr := "отключен ⚪"
		if newStatus {
			statusStr = "включен 🟢\nТеперь к ответам бота будет добавляться технический блок с API-методом и состоянием FSM."
		}
		_, err := c.Reply(fmt.Sprintf("🛠 Режим разработчика %s", statusStr))
		return err
	})

	b.OnCommand("reset", func(c *bot.Context) error {
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil {
			_ = fsmCtx.Reset()
		}
		_, err := c.Reply("🔄 Ваша сессия и состояние FSM сброшены в начало. Напишите /start или /menu.")
		return err
	})

	// Callbacks
	b.OnCallbackPayload("nav:menu", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		return sendMenu(c)
	})

	b.OnCallbackPayload("nav:toggle_dev", func(c *bot.Context) error {
		userID := int64(0)
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
		}
		newStatus := hood.ToggleDevMode(userID)
		statusToast := "Режим разработчика выключен"
		if newStatus {
			statusToast = "Режим разработчика включен! 🟢"
		}
		_ = c.AnswerNotification(statusToast)
		return sendMenu(c)
	})

	b.OnCallbackPayload("nav:reset", func(c *bot.Context) error {
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil {
			_ = fsmCtx.Reset()
		}
		_ = c.AnswerNotification("Сессия сброшена!")
		return sendMenu(c)
	})

	b.OnCallbackPayload("nav:info", func(c *bot.Context) error {
		infoText := "🤖 **Флагманский бот-витрина MAX Bot API**\n\n" +
			"• **Библиотека:** `github.com/littfed/max-bot-api-client`\n" +
			"• **TLS:** Встроенные сертификаты Минцифры РФ (Zero-Config)\n" +
			"• **FSM:** Постоянное хранилище сессий (SQLite / Memory)\n" +
			"• **Платежи:** СБП (НСПК) + эквайринг Т-Банка и ЮKassa\n" +
			"• **Mini Apps:** Двусторонний WebBridge + HMAC-SHA256\n" +
			"• **Модерация:** Капча-гейт с защитой от спам-ботов\n" +
			"• **Телеметрия:** OpenMetrics & Prometheus экспортер (/metrics)"

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("◀️ Назад в меню", "nav:menu"))

		_, err := c.ReplyKeyboard(infoText, kb)
		_ = c.AnswerCallback(nil)
		return err
	})

	// Fallback for user messages sent when not inside a command or active FSM wizard state
	b.OnIdleMessage(func(c *bot.Context) error {
		userText := strings.TrimSpace(c.Text())
		if userText == "" {
			return nil
		}

		userID := int64(0)
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
		}

		// Intercept invite links sent to the bot
		if strings.Contains(userText, "max.ru/join/") || strings.Contains(userText, "/join/") {
			botUsername := "id452401070481_bot"
			if c.Client() != nil {
				if me, _ := c.Client().GetMe(c.Ctx()); me != nil && me.Username != "" {
					botUsername = me.Username
				}
			}

			text := "👥 **Добавление бота в группу или канал**\n\n" +
				"Вы прислали ссылку-приглашение в чат:\n" +
				"`" + userText + "`\n\n" +
				"В платформе **MAX Messenger** (по соображениям безопасности) боты не имеют персональных профилей и **не могут автоматически переходить по ссылкам-приглашениям** (в MAX Bot API намеренно нет метода автоматического перехода по ссылкам).\n\n" +
				"### Как добавить бота в вашу группу или канал:\n" +
				"1️⃣ В приложении MAX откройте вашу группу или канал.\n" +
				"2️⃣ Нажмите на название чата вверху ➔ **«Участники»** ➔ **«Добавить участника»** (или в меню **«Администраторы»** ➔ **«Добавить»**).\n" +
				"3️⃣ В строке поиска введите логин бота: `@`" + botUsername + " и добавьте его в чат.\n" +
				"4️⃣ Выдайте боту права администратора (на удаление спама и блокировку ботов).\n\n" +
				"🛡️ *Как только вы добавите бота в чат, он автоматически поймает событие `bot_added`, поприветствует участников и активирует защиту чата!*"

			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewClipboardButton("📋 Скопировать логин бота", "@"+botUsername)).
				Row(maxbot.NewLinkButton("➕ Добавить в группу или канал", "https://max.ru/"+botUsername+"?startgroup=true")).
				Row(maxbot.NewCallbackButton("🛡️ Сценарий модерации (Капча)", "scen:guard")).
				Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

			_, err := c.ReplyKeyboard(text, kb)
			return err
		}

		// If AI is available and configured, answer as smart assistant!
		if hood.IsAIAvailable() && aiCli != nil {
			_ = c.Typing()
			chatReq := &ai.ChatRequest{
				Model: "gemini-2.5-flash",
				Messages: []ai.ChatMessage{
					{Role: ai.RoleSystem, Content: aiSystemPrompt},
					{Role: ai.RoleUser, Content: userText},
				},
			}
			chatResp, err := aiCli.Chat(c.Ctx(), chatReq)
			if err == nil && chatResp.Content != "" {
				kb := maxbot.NewInlineKeyboard().
					Row(
						maxbot.NewCallbackButton("💬 Начать диалог с памятью", "ai:chat:start"),
						maxbot.NewCallbackButton("🛍️ Главное меню", "nav:menu"),
					)
				formatted := hood.Format(userID, chatResp.Content, "ai_chat")
				_, rErr := c.ReplyKeyboard(formatted, kb)
				return rErr
			}
		}

		// Otherwise show main menu
		return sendMenu(c)
	})
}
