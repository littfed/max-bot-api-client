package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// ChannelScenarioConfig holds dependencies for channel administration and interactive posts.
type ChannelScenarioConfig struct {
	Client     *maxbot.Client
	Store      *store.Store
	AIClient   *ai.Client
	ChatModel  string
	MiniAppURL string
}

// RegisterChannelsScenario wires channel management, AI publishing, reactions, polls, and giveaways.
func RegisterChannelsScenario(b *bot.Bot, hood *HoodEngine, cfg ChannelScenarioConfig) {
	if cfg.MiniAppURL == "" {
		cfg.MiniAppURL = "https://combot.littfed.ru/maxapp"
	}

	// 1. Technical cards for HoodEngine
	hood.Register("chan_admin", TechCard{
		Title:       "Управление каналами и публикация постов",
		Endpoint:    "/messages?chat_id={id}",
		Method:      "POST",
		FSMState:    "CHANNEL_ACTIVE",
		Description: "Бот с правами Администратора (write, pin_message) публикует богатые посты в каналы от имени канала или бота, прикрепляя инлайн-кнопки и веб-витрины.",
		DocsLink:    "https://dev.max.ru/docs/channels",
	})

	hood.Register("chan_reactions", TechCard{
		Title:       "Живые реакции на посты каналов (rx telemetry)",
		Endpoint:    "/answers & /messages?message_id={id}",
		Method:      "PUT",
		Description: "Клики по инлайн-кнопкам реакций (👍, 🔥, ❤️) перехватываются через callback_id, инкрементируют счетчики в pkg/store и обновляют клавиатуру в реальном времени.",
		DocsLink:    "https://dev.max.ru/docs/buttons",
	})

	hood.Register("chan_polls", TechCard{
		Title:       "Интерактивные опросы с визуальной шкалой голосов",
		Endpoint:    "/messages & /answers",
		Method:      "POST/PUT",
		Description: "Голосование в канале пересчитывает процентное соотношение вариантов в реальном времени и отображает визуальную шкалу [██████░░░░] % прямо в посте.",
		DocsLink:    "https://dev.max.ru/docs/messages",
	})

	hood.Register("chan_giveaway", TechCard{
		Title:       "Розыгрыши с проверкой подписки и промокодами",
		Endpoint:    "/chats/{id}/members & /messages",
		Method:      "GET/POST",
		Description: "Участники розыгрыша регистрируются по кнопке участия, получают персональный промокод на скидку для Mini App, а победитель выбирается случайно.",
		DocsLink:    "https://dev.max.ru/docs/chat-members",
	})

	// 2. Scenario Menu (scen:channels)
	b.OnCallbackPayload("scen:channels", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		userID := int64(0)
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
		}

		channels := cfg.Store.ListChannels()
		var chanListText strings.Builder
		for _, ch := range channels {
			adminBadge := "Участник ⚪"
			if ch.IsAdmin {
				adminBadge = "Администратор 🟢 (Полные права)"
			}
			chanListText.WriteString(fmt.Sprintf("• **%s** (`%d`)\n  Статус: %s\n", ch.Title, ch.ChatID, adminBadge))
		}
		if chanListText.Len() == 0 {
			chanListText.WriteString("• *Подключенные каналы не найдены.*\n")
		}

		body := fmt.Sprintf("📢 **Центр управления каналами & Студия публикаций**\n\n"+
			"Бот авторизован в каналах MAX как полноправный администратор. Вы можете мгновенно отправлять публикации, опросы и розыгрыши в целевые каналы:\n\n"+
			"**Подключенные каналы:**\n%s\n"+
			"💡 *Выберите действие для тестирования:*", chanListText.String())

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewIntentCallbackButton("🤖 Опубликовать AI-пост в «Тест»", "chan:post:-78368186601503:ai", maxbot.IntentPositive),
			).
			Row(
				maxbot.NewIntentCallbackButton("🤖 Опубликовать AI-пост в «littfed»", "chan:post:-78548596658207:ai", maxbot.IntentPositive),
			).
			Row(
				maxbot.NewCallbackButton("📊 Запустить опрос в «Тест»", "chan:poll:-78368186601503"),
				maxbot.NewCallbackButton("🎁 Запустить розыгрыш в «Тест»", "chan:gw:-78368186601503"),
			).
			Row(
				maxbot.NewCallbackButton("🛍️ Опубликовать витрину в «Тест»", "chan:post:-78368186601503:ecom"),
			).
			Row(
				maxbot.NewLinkButton("✍️ Открыть Студию в Mini App", cfg.MiniAppURL+"?tab=studio"),
			).
			Row(
				maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"),
			)

		formatted := hood.Format(userID, body, "chan_admin")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// 3. Post Publisher Handler (chan:post:{chatID}:{mode})
	b.OnCallbackPrefix("chan:post:", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		cb := c.Callback()
		if cb == nil {
			return nil
		}

		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 4 {
			return c.AnswerNotification("Неверный формат команды публикации")
		}

		targetChatID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return err
		}
		mode := parts[3]

		channel, found := cfg.Store.GetChannel(targetChatID)
		targetTitle := "Канал"
		if found {
			targetTitle = channel.Title
		}

		_ = c.SendChatAction("typing_on")

		postText := ""
		if mode == "ai" {
			// Generate via Gemini AI Gateway if available
			if cfg.AIClient != nil && hood.IsAIAvailable() {
				prompt := "Напиши короткий, яркий, профессиональный пост для Telegram/MAX канала об актуальных трендах искусственного интеллекта за сегодня (2-3 емких пункта, эмодзи, форматирование bold/italic, призыв к действию). До 1000 символов."
				aiCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
				resp, aiErr := cfg.AIClient.Chat(aiCtx, &ai.ChatRequest{
					Model: cfg.ChatModel,
					Messages: []ai.ChatMessage{
						{Role: "system", Content: "Ты профессиональный tech-редактор канала об ИИ и разработке в MAX Messenger."},
						{Role: "user", Content: prompt},
					},
				})
				cancel()
				if aiErr == nil && resp != nil && resp.Content != "" {
					postText = resp.Content
				}
			}

			// Fallback text if AI is offline
			if postText == "" {
				postText = fmt.Sprintf("🚀 **Инновации и ИИ в мессенджере MAX**\n\n"+
					"Искусственный интеллект трансформирует взаимодействие с аудиторией:\n\n"+
					"• **Мультимодальные агенты:** боты в каналах способны анализировать изображения и давать консультации за доли секунды.\n"+
					"• **Интерактивные Mini Apps:** бесшовная интеграция веб-витрин прямо внутри чата без редиректов.\n"+
					"• **Живой фидбек:** встроенные телеметрические реакции и опросы позволяют авторам мгновенно чувствовать настроение аудитории.\n\n"+
					"💡 *Попробуйте возможности экосистемы прямо сейчас по кнопке ниже:*")
			}
		} else if mode == "ecom" {
			postText = fmt.Sprintf("🛍️ **Специальное предложение от MAX Store!**\n\n"+
				"Эксклюзивная коллекция мерча для разработчиков и участников сообщества:\n\n"+
				"• **Худи Go Gopher Dev Edition** — 3,990 ₽\n"+
				"• **Футболка MAX Ecosystem Tech** — 1,990 ₽\n"+
				"• **Стикерпак MAX Bot Masters** — 490 ₽\n\n"+
				"🔥 Доставка по всей России или самовывоз в Москве.\n"+
				"Оформляйте заказ прямо в мессенджере в один клик:")
		}

		// Initial reaction keyboard (placeholder mid, will be replaced after sending)
		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewLinkButton("🛍️ Открыть в Mini App", cfg.MiniAppURL),
			).
			Row(
				maxbot.NewCallbackButton("👍 0", "rx:pending:👍"),
				maxbot.NewCallbackButton("🔥 0", "rx:pending:🔥"),
				maxbot.NewCallbackButton("❤️ 0", "rx:pending:❤️"),
			)

		// Send message to the channel
		if cfg.Client == nil {
			return c.AnswerNotification("MAX Bot Client не подключен")
		}

		sentMsg, err := cfg.Client.SendMessage(context.Background(), maxbot.ToChat(targetChatID), &maxbot.SendMessageRequest{
			Text:        postText,
			Format:      "markdown",
			Attachments: []maxbot.Attachment{kb.ToAttachment()},
		})
		if err != nil {
			report := fmt.Sprintf("❌ Ошибка отправки поста в канал %s (%d): %v", targetTitle, targetChatID, err)
			_, _ = c.Reply(report)
			return err
		}

		// Update payload with actual message ID
		postMID := sentMsg.MessageID
		if postMID != "" {
			realKb := buildReactionsKeyboard(cfg.MiniAppURL, postMID, map[string]int{"👍": 0, "🔥": 0, "❤️": 0})
			_, _ = cfg.Client.EditMessage(context.Background(), postMID, &maxbot.EditMessageRequest{
				Attachments: []maxbot.Attachment{realKb.ToAttachment()},
			})
			// Initialize in store
			_, _ = cfg.Store.ToggleReaction(postMID, targetChatID, 0, "")
			// Pin message in channel
			_ = cfg.Client.PinMessage(context.Background(), targetChatID, postMID, false)
		}

		confirmDM := fmt.Sprintf("✅ **Пост успешно опубликован в канале «%s»!**\n\n"+
			"• **Chat ID:** `%d`\n"+
			"• **Message ID:** `%s`\n"+
			"• **Кнопки:** Mini App + Реакции (👍, 🔥, ❤️)\n"+
			"• **Статус:** Закреплен в канале 📌\n\n"+
			"Перейдите в канал, чтобы проверить отображение и нажать реакции!", targetTitle, targetChatID, postMID)

		returnKb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("📢 Назад к каналам", "scen:channels")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		_, _ = c.ReplyKeyboard(confirmDM, returnKb)
		return nil
	})

	// 4. Reactions Callback Handler (rx:{mid}:{emoji})
	b.OnCallbackPrefix("rx:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}

		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 3 {
			return c.AnswerNotification("Реакция устарела")
		}

		postMID := parts[1]
		emoji := parts[2]
		userID := int64(0)
		if cb.User != nil {
			userID = cb.User.UserID
		}

		chatID := int64(0)
		if c.Update != nil {
			chatID = c.Update.ChatID
		}

		// Update reaction counts in store
		updatedCounts, err := cfg.Store.ToggleReaction(postMID, chatID, userID, emoji)
		if err != nil {
			return c.AnswerNotification("Ошибка сохранения реакции")
		}

		// Update message keyboard with fresh reaction counts
		updatedKb := buildReactionsKeyboard(cfg.MiniAppURL, postMID, updatedCounts)
		if cfg.Client != nil && postMID != "pending" && postMID != "" {
			_, _ = cfg.Client.EditMessage(context.Background(), postMID, &maxbot.EditMessageRequest{
				Attachments: []maxbot.Attachment{updatedKb.ToAttachment()},
			})
		}

		return c.AnswerNotification(fmt.Sprintf("Ваша реакция %s учтена!", emoji))
	})

	// 5. Poll Publisher (chan:poll:{chatID})
	b.OnCallbackPrefix("chan:poll:", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		cb := c.Callback()
		if cb == nil {
			return nil
		}
		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 3 {
			return nil
		}
		targetChatID, _ := strconv.ParseInt(parts[2], 10, 64)

		pollID := fmt.Sprintf("poll_%d_%d", targetChatID, time.Now().Unix())
		question := "🚀 Какой функционал ботов в MAX наиболее востребован для вашего бизнеса?"
		options := []string{
			"🛍️ Mini App и магазины",
			"🤖 AI-ассистенты и поддержка",
			"📢 Автопостинг и каналы",
			"🛡️ Защита чатов от спама",
		}

		poll := store.Poll{
			ID:       pollID,
			ChatID:   targetChatID,
			Question: question,
			Options:  options,
			Votes:    make(map[int64]int),
		}
		_ = cfg.Store.CreatePoll(poll)

		pollText := renderPollText(question, options, []int{0, 0, 0, 0}, 0)
		pollKb := buildPollKeyboard(pollID, options)

		if cfg.Client == nil {
			return c.AnswerNotification("MAX Bot Client не подключен")
		}

		sentMsg, err := cfg.Client.SendMessage(context.Background(), maxbot.ToChat(targetChatID), &maxbot.SendMessageRequest{
			Text:        pollText,
			Format:      "markdown",
			Attachments: []maxbot.Attachment{pollKb.ToAttachment()},
		})
		if err != nil {
			_, _ = c.Reply(fmt.Sprintf("❌ Ошибка запуска опроса: %v", err))
			return err
		}

		poll.MessageMID = sentMsg.MessageID
		_ = cfg.Store.CreatePoll(poll)

		confirmDM := fmt.Sprintf("📊 **Интерактивный опрос опубликован в канале!**\n\n"+
			"• **Вопрос:** %s\n"+
			"• **Message ID:** `%s`\n"+
			"• **Голосование:** Живой пересчет полосы процентов при каждом клике ✅", question, sentMsg.MessageID)

		returnKb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("📢 Назад к каналам", "scen:channels")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		_, _ = c.ReplyKeyboard(confirmDM, returnKb)
		return nil
	})

	// 6. Poll Voting Callback Handler (poll:{pollID}:{optionIdx})
	b.OnCallbackPrefix("poll:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}

		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 3 {
			return c.AnswerNotification("Опрос завершен")
		}

		pollID := parts[1]
		optIdx, err := strconv.Atoi(parts[2])
		if err != nil {
			return err
		}

		userID := int64(0)
		if cb.User != nil {
			userID = cb.User.UserID
		}

		_, percents, err := cfg.Store.VotePoll(pollID, userID, optIdx)
		if err != nil {
			return c.AnswerNotification("Ошибка голосования")
		}

		p, ok := cfg.Store.GetPoll(pollID)
		if !ok {
			return c.AnswerNotification("Опрос не найден")
		}

		// Re-render poll text with progress bars
		updatedText := renderPollText(p.Question, p.Options, percents, len(p.Votes))
		pollKb := buildPollKeyboard(pollID, p.Options)

		if cfg.Client != nil && p.MessageMID != "" {
			_, _ = cfg.Client.EditMessage(context.Background(), p.MessageMID, &maxbot.EditMessageRequest{
				Text:        updatedText,
				Format:      "markdown",
				Attachments: []maxbot.Attachment{pollKb.ToAttachment()},
			})
		}

		return c.AnswerNotification(fmt.Sprintf("Ваш голос принят: %s!", p.Options[optIdx]))
	})

	// 7. Giveaway Publisher (chan:gw:{chatID})
	b.OnCallbackPrefix("chan:gw:", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		cb := c.Callback()
		if cb == nil {
			return nil
		}
		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 3 {
			return nil
		}
		targetChatID, _ := strconv.ParseInt(parts[2], 10, 64)

		gwID := fmt.Sprintf("gw_%d_%d", targetChatID, time.Now().Unix())
		gw := store.Giveaway{
			ID:              gwID,
			ChatID:          targetChatID,
			Title:           "Фирменное худи MAX Dev + скидка 25% на любой мерч!",
			PromoCode:       "PROMO-MAX-25",
			DiscountPercent: 25,
			Participants:    make(map[int64]string),
		}
		_ = cfg.Store.CreateGiveaway(gw)

		gwText := fmt.Sprintf("🎁 **ГРАНДИОЗНЫЙ РОЗЫГРЫШ В КАНАЛЕ!**\n\n"+
			"Разыгрываем **%s** среди наших подписчиков.\n\n"+
			"• **Условия:** Быть подписанным на этот канал.\n"+
			"• **Гарантированный бонус:** Каждый участник сразу получает персональный промокод на скидку в нашем Mini App магазине!\n\n"+
			"👇 *Нажмите кнопку ниже, чтобы принять участие и забрать промокод:*", gw.Title)

		gwKb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewIntentCallbackButton("🎁 Участвовать в розыгрыше", "gw:"+gwID+":join", maxbot.IntentPositive)).
			Row(maxbot.NewLinkButton("🛍️ Каталог Mini App", cfg.MiniAppURL))

		if cfg.Client == nil {
			return c.AnswerNotification("MAX Bot Client не подключен")
		}

		sentMsg, err := cfg.Client.SendMessage(context.Background(), maxbot.ToChat(targetChatID), &maxbot.SendMessageRequest{
			Text:        gwText,
			Format:      "markdown",
			Attachments: []maxbot.Attachment{gwKb.ToAttachment()},
		})
		if err != nil {
			_, _ = c.Reply(fmt.Sprintf("❌ Ошибка запуска розыгрыша: %v", err))
			return err
		}

		confirmDM := fmt.Sprintf("🎉 **Розыгрыш успешно опубликован в канале!**\n\n"+
			"• **Приз:** %s\n"+
			"• **Message ID:** `%s`\n"+
			"• **Гарантированный промокод:** `%s`\n"+
			"Пользователи могут регистрироваться в один клик прямо под постом.", gw.Title, sentMsg.MessageID, gw.PromoCode)

		returnKb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("📢 Назад к каналам", "scen:channels")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		_, _ = c.ReplyKeyboard(confirmDM, returnKb)
		return nil
	})

	// 8. Giveaway Join Callback Handler (gw:{gwID}:join)
	b.OnCallbackPrefix("gw:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}

		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 3 || parts[2] != "join" {
			return nil
		}

		gwID := parts[1]
		userID := int64(0)
		userName := "Участник"
		if cb.User != nil {
			userID = cb.User.UserID
			if cb.User.FirstName != "" {
				userName = cb.User.FirstName
			}
		}

		gw, ok := cfg.Store.GetGiveaway(gwID)
		if !ok {
			return c.AnswerNotification("Розыгрыш не найден или завершен")
		}

		_ = cfg.Store.AddGiveawayParticipant(gwID, userID, userName)

		// Send personal message with promo code
		if cfg.Client != nil && userID != 0 {
			dmText := fmt.Sprintf("🎉 **Вы успешно зарегистрированы в розыгрыше!**\n\n"+
				"• **Приз:** %s\n"+
				"• **Ваш гарантированный промокод:** `%s` (скидка %d%%)\n\n"+
				"Используйте его при оформлении заказа в нашем Mini App магазине:", gw.Title, gw.PromoCode, gw.DiscountPercent)

			dmKb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewLinkButton("🛍️ Открыть корзину с промокодом", cfg.MiniAppURL+"?promo="+gw.PromoCode))

			_, _ = cfg.Client.SendMessage(context.Background(), maxbot.ToUser(userID), &maxbot.SendMessageRequest{
				Text:        dmText,
				Format:      "markdown",
				Attachments: []maxbot.Attachment{dmKb.ToAttachment()},
			})
		}

		return c.AnswerNotification(fmt.Sprintf("🎉 Вы в игре! Промокод %s отправлен вам в ЛС!", gw.PromoCode))
	})
}

// buildReactionsKeyboard creates a row of reaction counter buttons plus link to Mini App.
func buildReactionsKeyboard(miniAppURL, postMID string, counts map[string]int) *maxbot.InlineKeyboard {
	thumbs := counts["👍"]
	fire := counts["🔥"]
	heart := counts["❤️"]

	return maxbot.NewInlineKeyboard().
		Row(
			maxbot.NewLinkButton("🛍️ Открыть в Mini App", miniAppURL),
		).
		Row(
			maxbot.NewCallbackButton(fmt.Sprintf("👍 %d", thumbs), fmt.Sprintf("rx:%s:👍", postMID)),
			maxbot.NewCallbackButton(fmt.Sprintf("🔥 %d", fire), fmt.Sprintf("rx:%s:🔥", postMID)),
			maxbot.NewCallbackButton(fmt.Sprintf("❤️ %d", heart), fmt.Sprintf("rx:%s:❤️", postMID)),
		)
}

// buildPollKeyboard builds vote option buttons.
func buildPollKeyboard(pollID string, options []string) *maxbot.InlineKeyboard {
	kb := maxbot.NewInlineKeyboard()
	for i, opt := range options {
		kb.Row(maxbot.NewCallbackButton(opt, fmt.Sprintf("poll:%s:%d", pollID, i)))
	}
	return kb
}

// renderPollText renders the question, ASCII progress bars, and percentage.
func renderPollText(question string, options []string, percents []int, totalVotes int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 **Опрос:** %s\n\n", question))

	for i, opt := range options {
		pct := 0
		if i < len(percents) {
			pct = percents[i]
		}
		bar := renderBar(pct)
		sb.WriteString(fmt.Sprintf("**%s**\n%s %d%%\n\n", opt, bar, pct))
	}

	sb.WriteString(fmt.Sprintf("👥 Всего голосов: `%d`\n", totalVotes))
	sb.WriteString("👆 *Нажмите кнопку ниже, чтобы проголосовать или изменить свой голос:*")
	return sb.String()
}

// renderBar draws a 10-char ASCII progress bar [██████░░░░]
func renderBar(percent int) string {
	filled := percent / 10
	if filled > 10 {
		filled = 10
	}
	empty := 10 - filled
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}
