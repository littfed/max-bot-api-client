package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

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
			maxbot.NewCallbackButton("🛍️ 1. Магазин & СБП Checkout", "scen:ecom").WithIntent(maxbot.IntentPrimary),
		).
		Row(
			maxbot.NewCallbackButton("📅 2. Запись на услуги & Контакт", "scen:booking").WithIntent(maxbot.IntentPrimary),
		).
		Row(
			maxbot.NewCallbackButton("🧠 3. Интерактивный Квиз & TTK", "scen:quiz"),
		).
		Row(
			maxbot.NewCallbackButton("📱 4. Mini App (HTML5 WebView)", "scen:webapp").WithIntent(maxbot.IntentPositive),
		).
		Row(
			maxbot.NewCallbackButton("📦 Мои заказы и записи", "nav:orders"),
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
			maxbot.NewCallbackButton("👨‍💼 Позвать человека", "nav:operator"),
			maxbot.NewCallbackButton("ℹ️ О витрине", "nav:info"),
		).
		Row(
			maxbot.NewCallbackButton("🔄 Сбросить сессию", "nav:reset").WithIntent(maxbot.IntentNegative),
		)

	return kb
}

// RegisterMenuRoutes installs main menu command and callback handlers.
func RegisterMenuRoutes(b *bot.Bot, hood *HoodEngine, aiClient ...*ai.Client) {
	hood.Register("operator_hitl", TechCard{
		Title:       "Human-in-the-Loop (HITL) Эскалация на оператора",
		Endpoint:    "/messages",
		Method:      "POST",
		FSMState:    "OPERATOR_PAUSED",
		Description: "Бот переводит сессию в состояние OPERATOR_PAUSED, замораживая автоматические AI-ответы. Формируется Lead Dossier и запускается 30-минутный watchdog таймер неактивности.",
		DocsLink:    "https://dev.max.ru/docs",
	})

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
		_, err := c.EditOrReplyKeyboard(formatted, kb)
		return err
	}

	sendStartGreeting := func(c *bot.Context) error {
		userID := int64(0)
		userName := "Гость"
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
			if sender.FirstName != "" {
				userName = sender.FirstName
			}
		}

		greeting := fmt.Sprintf("👋 Привет, *%s*!\n\n"+
			"Я умный мультимодальный AI-ассистент на платформе **MAX Messenger**.\n\n"+
			"✨ **Вы можете общаться со мной совершенно естественно:**\n"+
			"• 💬 **Диалог:** задавайте любые вопросы текстом в чат\n"+
			"• 🖼️ **Компьютерное зрение:** отправьте фото или картинку (опишу детали, сделаю OCR текста)\n"+
			"• 🎙️ **Голос:** запишите голосовое сообщение (расшифрую через Whisper и отвечу)\n"+
			"• 📁 **Документы и код:** присылайте PDF, TXT, MD или файлы кода (сделаю выжимку и отвечу на вопросы)\n"+
			"• 🎨 **Генерация картинок:** напишите *«Нарисуй кота»* или команду `/img <описание>`\n"+
			"• 🌐 **Ссылки:** отправьте URL статьи, и я подготовлю краткий пересказ\n\n"+
			"👇 *А если хотите посмотреть интерактивную витрину других возможностей MAX (магазин, бронь, квиз, Mini App) — нажмите кнопку ниже:*", userName)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("✨ Витрина возможностей MAX", "nav:menu").WithIntent(maxbot.IntentPrimary))

		formatted := hood.Format(userID, greeting, "main_menu")
		_, err := c.EditOrReplyKeyboard(formatted, kb)
		return err
	}

	// Commands
	b.OnStart(func(c *bot.Context) error {
		return sendStartGreeting(c)
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
			_ = fsmCtx.Set("ai_chat_history", "[]")
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}
		_, err := c.Reply("🔄 Контекст диалога очищен (0/10). Начните новый диалог — задайте вопрос или отправьте фото/файл!")
		return err
	})

	b.OnCommand("new", func(c *bot.Context) error {
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil {
			_ = fsmCtx.Set("ai_chat_history", "[]")
			_ = fsmCtx.SetState("AI_STEP_CHAT")
		}
		_, err := c.Reply("🔄 Начат новый диалог с чистого листа. Задайте любой вопрос или отправьте фото/файл!")
		return err
	})

	showModelSelector := func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		activeModel := "auto"
		if fsmCtx != nil {
			if m, ok := fsmCtx.Get("ai_active_chat_model"); ok {
				if mStr, isStr := m.(string); isStr && mStr != "" {
					activeModel = mStr
				}
			}
		}

		text := fmt.Sprintf("⚙️ **Выбор активной языковой модели (LLM)**\n\n"+
			"• Текущая модель: `%s`\n\n"+
			"Выберите модель для диалога:\n"+
			"• **auto** — сбалансированная модель для любых задач\n"+
			"• **best-coding** — специализированная модель для программирования и кода\n"+
			"• **reasoning** — глубокие рассуждения и логика\n"+
			"• **fast** — максимальная скорость генерации ответа", activeModel)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("⚡ auto (сбалансированная)", "ai:set_model:auto")).
			Row(maxbot.NewCallbackButton("💻 best-coding (код и разработка)", "ai:set_model:best-coding")).
			Row(maxbot.NewCallbackButton("🧠 reasoning (логика и анализ)", "ai:set_model:reasoning")).
			Row(maxbot.NewCallbackButton("🚀 fast (быстрая)", "ai:set_model:fast"))

		_, err := c.ReplyKeyboard(text, kb)
		return err
	}

	b.OnCommand("model", func(c *bot.Context) error {
		return showModelSelector(c)
	})

	b.OnCommand("models", func(c *bot.Context) error {
		return showModelSelector(c)
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
		_ = c.AnswerCallback(nil)
		infoText := "🤖 **Флагманский бот-витрина MAX Bot API**\n\n" +
			"• **Библиотека:** `github.com/littfed/max-bot-api-client`\n" +
			"• **TLS:** Встроенные сертификаты Минцифры РФ (Zero-Config)\n" +
			"• **FSM:** Постоянное хранилище сессий (SQLite / Memory)\n" +
			"• **Платежи:** СБП (НСПК) + эквайринг Т-Банка и ЮKassa\n" +
			"• **Mini Apps:** Двусторонний WebBridge + HMAC-SHA256\n" +
			"• **Модерация:** Капча-гейт с защитой от спам-ботов\n" +
			"• **Телеметрия:** OpenMetrics & Prometheus экспортер (/metrics)"

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("◀️ Назад в меню", "nav:menu").WithIntent(maxbot.IntentPrimary))

		userID := int64(0)
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
		}
		formatted := hood.Format(userID, infoText, "main_menu")
		_, err := c.EditOrReplyKeyboard(formatted, kb)
		return err
	})

	handleOperatorEscalation := func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := int64(0)
		userName := "Гость"
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
			if sender.FirstName != "" {
				userName = sender.FirstName
				if sender.LastName != "" {
					userName += " " + sender.LastName
				}
			}
		}

		fsmCtx := fsm.FromContext(c)
		dossier := BuildLeadDossier(c)

		// Log structured Lead Dossier
		slog.Info("📋 [LEAD DOSSIER] Запрос связи с оператором",
			slog.Int64("user_id", userID),
			slog.String("user_name", userName),
			slog.String("dossier", dossier),
		)

		if fsmCtx != nil {
			_ = fsmCtx.SetState("OPERATOR_PAUSED")
			_ = fsmCtx.Set("operator_paused_at", float64(time.Now().Unix()))
		}

		text := fmt.Sprintf("👨‍💼 **Переключение на оператора (Human-in-the-Loop)**\n\n"+
			"Бот переведен в режим ожидания. Ваше обращение и контекст диалога переданы менеджеру поддержки.\n"+
			"Специалист подключится к чату в ближайшее время.\n\n"+
			"📋 **Сформированное досье обращения:**\n%s\n\n"+
			"💡 *Пока оператор на связи, автоматические ответы бота отключены.*\n"+
			"Для возврата к боту нажмите кнопку ниже или введите команду `/resume`:", dossier)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🤖 Вернуться к боту", "nav:resume").WithIntent(maxbot.IntentPositive)).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		formatted := hood.Format(userID, text, "operator_hitl")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	}

	handleOperatorResume := func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)
		userID := int64(0)
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
		}

		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.Reset()
		}

		text := "🤖 **Автоматический режим возобновлен!**\n\n" +
			"Бот снова на связи и готов отвечать на ваши вопросы. Задайте любой вопрос текстом или выберите раздел витрины из меню ниже:"

		kb := BuildMainMenuKeyboard(hood, userID)
		formatted := hood.Format(userID, text, "main_menu")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	}

	b.OnCommand("operator", handleOperatorEscalation)
	b.OnCallbackPayload("nav:operator", handleOperatorEscalation)
	b.OnCallbackPayload("ai:operator", handleOperatorEscalation)

	b.OnCommand("resume", handleOperatorResume)
	b.OnCallbackPayload("nav:resume", handleOperatorResume)

	b.OnState("OPERATOR_PAUSED", func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		var pausedAt int64
		if fsmCtx != nil {
			if val, ok := fsmCtx.Get("operator_paused_at"); ok {
				if fVal, isFloat := val.(float64); isFloat {
					pausedAt = int64(fVal)
				} else if iVal, isInt64 := val.(int64); isInt64 {
					pausedAt = iVal
				} else if iVal, isInt := val.(int); isInt {
					pausedAt = int64(iVal)
				}
			}
		}

		// 30-minute inactivity watchdog check (1800 seconds)
		if pausedAt > 0 && time.Now().Unix()-pausedAt > 1800 {
			if fsmCtx != nil {
				_ = fsmCtx.Reset()
			}
			userID := int64(0)
			if sender := c.Sender(); sender != nil {
				userID = sender.UserID
			}
			timeoutMsg := "⌛ **Время ожидания оператора истекло (30 минут).**\n\n" +
				"Сессия автоматически переведена обратно в стандартный режим. Бот снова готов отвечать на ваши вопросы!"
			kb := BuildMainMenuKeyboard(hood, userID)
			_, err := c.ReplyKeyboard(timeoutMsg, kb)
			return err
		}

		// Automated bot responses are muted while in pause mode
		return nil
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

		// If AI is available and configured, answer as smart assistant with full conversational context!
		if (hood.IsAIAvailable() || globalAIDispatcher != nil) && aiCli != nil {
			if globalAIDispatcher != nil {
				return globalAIDispatcher(c)
			}
			fsmCtx := fsm.FromContext(c)
			activeModel := "auto"
			if fsmCtx != nil {
				if m, ok := fsmCtx.Get("ai_active_chat_model"); ok {
					if mStr, isStr := m.(string); isStr && mStr != "" {
						activeModel = mStr
					}
				}
			}

			history := loadChatHistory(fsmCtx)
			history = append(history, ai.ChatMessage{
				Role:    ai.RoleUser,
				Content: userText,
			})
			if len(history) > 10 {
				history = history[len(history)-10:]
			}

			messagesPayload := make([]ai.ChatMessage, 0, len(history)+1)
			messagesPayload = append(messagesPayload, ai.ChatMessage{
				Role:    ai.RoleSystem,
				Content: aiSystemPrompt,
			})
			messagesPayload = append(messagesPayload, history...)

			_ = c.Typing()
			stopTyping := make(chan struct{})
			go func() {
				ticker := time.NewTicker(4 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-stopTyping:
						return
					case <-ticker.C:
						_ = c.Typing()
					}
				}
			}()

			temp := 0.7
			chatReq := &ai.ChatRequest{
				Model:       activeModel,
				Messages:    messagesPayload,
				Temperature: &temp,
			}

			candidates := []string{activeModel}
			for _, m := range []string{"auto", "fast", "best-coding", "reasoning", "gemini-3.5-flash-lite"} {
				if m != activeModel {
					candidates = append(candidates, m)
				}
			}

			var chatResp *ai.ChatResponse
			var lastErr error
			var usedModel string

			for _, m := range candidates {
				chatReq.Model = m
				chatResp, lastErr = aiCli.Chat(c.Ctx(), chatReq)
				if lastErr == nil && chatResp != nil && strings.TrimSpace(chatResp.Content) != "" {
					usedModel = m
					break
				}
			}
			close(stopTyping)

			if lastErr == nil && chatResp != nil && strings.TrimSpace(chatResp.Content) != "" {
				history = append(history, ai.ChatMessage{
					Role:    ai.RoleAssistant,
					Content: chatResp.Content,
				})
				if len(history) > 10 {
					history = history[len(history)-10:]
				}
				_ = saveChatHistory(fsmCtx, history)
				if fsmCtx != nil {
					_ = fsmCtx.SetState("AI_STEP_CHAT")
					_ = fsmCtx.Set("last_ai_reply", chatResp.Content)
				}

				modelReport := usedModel
				if usedModel != activeModel && usedModel != "" {
					modelReport = fmt.Sprintf("%s (резерв вместо %s)", usedModel, activeModel)
				}
				hood.Register("ai_chat", TechCard{
					Title:       "AI-Диалог & Скользящее окно контекста (Sliding Window)",
					Endpoint:    "/v1/chat/completions",
					Method:      "POST",
					FSMState:    "AI_STEP_CHAT",
					Description: fmt.Sprintf("Модель: %s | Задержка: %s | Токенов: %d | История: %d/10 сообщений.", modelReport, chatResp.Latency.Round(time.Millisecond), chatResp.TotalTokens, len(history)),
					DocsLink:    "https://platform.openai.com/docs/api-reference/chat",
				})

				kb := maxbot.NewInlineKeyboard().
					Row(
						maxbot.NewCallbackButton("🔄 Новый диалог", "ai:clear_context"),
						maxbot.NewCallbackButton("🔊 Озвучить", "ai:tts:speak_last"),
					).
					Row(
						maxbot.NewCallbackButton("👨‍💼 Позвать человека", "nav:operator"),
						maxbot.NewCallbackButton("◀️ В меню", "nav:menu"),
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

// BuildLeadDossier compiles customer context, selected items, and recent dialogue for the operator.
func BuildLeadDossier(c *bot.Context) string {
	var sb strings.Builder
	userID := int64(0)
	userName := "Гость"
	userHandle := ""
	if sender := c.Sender(); sender != nil {
		userID = sender.UserID
		if sender.FirstName != "" {
			userName = sender.FirstName
			if sender.LastName != "" {
				userName += " " + sender.LastName
			}
		}
		userHandle = sender.Username
	}

	sb.WriteString(fmt.Sprintf("• **Клиент:** %s (`ID: %d`)", userName, userID))
	if userHandle != "" {
		sb.WriteString(fmt.Sprintf(" (@%s)", userHandle))
	}
	sb.WriteString("\n")

	fsmCtx := fsm.FromContext(c)
	if fsmCtx != nil {
		data, _ := fsmCtx.GetData()
		state := fsmCtx.State()
		if state != "" && state != "OPERATOR_PAUSED" {
			sb.WriteString(fmt.Sprintf("• **Текущий шаг:** `%s`\n", state))
		}

		// E-Commerce context
		if prodID, ok := data["prod_id"].(string); ok && prodID != "" {
			prodName := prodID
			if p := findProduct(prodID); p != nil {
				prodName = fmt.Sprintf("%s %s (%.0f ₽)", p.Icon, p.Name, p.Price)
			}
			sb.WriteString(fmt.Sprintf("• **Товар в корзине:** %s\n", prodName))
		}
		if orderID, ok := data["order_id"].(string); ok && orderID != "" {
			sb.WriteString(fmt.Sprintf("• **Номер заказа:** `#%s`\n", orderID))
		}
		if delivery, ok := data["delivery"].(string); ok && delivery != "" {
			sb.WriteString(fmt.Sprintf("• **Адрес доставки:** %s\n", delivery))
		}

		// Booking context
		if srvID, ok := data["service_id"].(string); ok && srvID != "" {
			srvName := srvID
			if s := findService(srvID); s != nil {
				srvName = fmt.Sprintf("%s %s", s.Icon, s.Name)
			}
			sb.WriteString(fmt.Sprintf("• **Выбранная услуга:** %s\n", srvName))
		}
		if slot, ok := data["slot"].(string); ok && slot != "" {
			sb.WriteString(fmt.Sprintf("• **Слот записи:** %s\n", slot))
		}
		if phone, ok := data["phone"].(string); ok && phone != "" {
			sb.WriteString(fmt.Sprintf("• **Телефон клиента:** `%s`\n", phone))
		}

		// Quiz context
		if score, ok := data["quiz_score"].(float64); ok {
			sb.WriteString(fmt.Sprintf("• **Баллы в квизе:** %.0f/3\n", score))
		}

		// Recent AI conversation context
		history := loadChatHistory(fsmCtx)
		if len(history) > 0 {
			lastMsg := history[len(history)-1]
			contentStr := ""
			switch v := lastMsg.Content.(type) {
			case string:
				contentStr = v
			default:
				contentStr = fmt.Sprintf("%v", v)
			}
			snippet := strings.TrimSpace(contentStr)
			if len(snippet) > 100 {
				snippet = snippet[:97] + "..."
			}
			if snippet != "" {
				sb.WriteString(fmt.Sprintf("• **Последнее сообщение:** *«%s»*\n", snippet))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("• **Время вызова:** %s", time.Now().Format("15:04:05 (02.01.2006)")))
	return sb.String()
}
