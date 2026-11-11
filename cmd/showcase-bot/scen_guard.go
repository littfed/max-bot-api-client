package main

import (
	"context"
	"fmt"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/guard"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// RegisterGuardScenario installs community moderation, interactive in-dialog simulation, and live in-group protections.
func RegisterGuardScenario(b *bot.Bot, hood *HoodEngine, client *maxbot.Client) {
	// 1. Technical cards
	hood.Register("guard_captcha", TechCard{
		Title:       "Интерактивная капча-гейт против спам-ботов",
		Endpoint:    "/chats/{id}/members",
		Method:      "DELETE",
		FSMState:    "GUARD_ACTIVE",
		Description: "При входе нового участника бот выдает инлайн-кнопку «Я человек» с уникальным payload guard:v:<chatID>:<userID>. Клики других участников отклоняются, а нерешенная капча приводит к автокику по таймауту.",
		DocsLink:    "https://dev.max.ru/docs/chat-members",
	})

	hood.Register("guard_kick", TechCard{
		Title:       "Удаление участников (DELETE /chats/{id}/members)",
		Endpoint:    "/chats/{id}/members?user_id={uid}",
		Method:      "DELETE",
		Description: "Метод Client.KickChatMember(ctx, chatID, userID) исключает спамера или неавторизованного пользователя из группового чата или супергруппы MAX.",
		DocsLink:    "https://dev.max.ru/docs/chat-members",
	})

	hood.Register("guard_live", TechCard{
		Title:       "События bot_added и user_added",
		Endpoint:    "/updates",
		Method:      "LONG-POLLING",
		Description: "Бот перехватывает событие bot_added при добавлении в группу, а также сообщения о присоединении новых участников для мгновенной выдачи верификационного челленджа.",
		DocsLink:    "https://dev.max.ru/docs/events",
	})

	// 2. Setup Captcha Gate
	captchaGate := guard.NewCaptchaGate(client,
		guard.WithTimeout(30*time.Second),
		guard.WithOnVerified(func(ctx context.Context, chatID, userID int64) {
			// Callback on live verification
		}),
		guard.WithOnKicked(func(ctx context.Context, chatID, userID int64) {
			// Callback on live timeout kick
		}),
	)
	captchaGate.Register(b.Router())

	// 3. Scenario Menu (scen:guard)
	b.OnCallbackPayload("scen:guard", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewIntentCallbackButton("🧪 Тест входа: симуляция капчи (30 сек)", "guard:sim_join", maxbot.IntentPositive)).
			Row(maxbot.NewIntentCallbackButton("⏱ Тест: автокик по таймауту", "guard:sim_timeout", maxbot.IntentNegative)).
			Row(hood.Button("guard_captcha")).
			Row(hood.Button("guard_kick")).
			Row(hood.Button("guard_live")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		userID := int64(0)
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
		}

		text := "🛡️ **Комьюнити & Модерация чатов в MAX**\n\n" +
			"Этот сценарий демонстрирует инструменты защиты сообществ, каналов и групп:\n\n" +
			"• **Капча-гейт:** мгновенная выдача проверки новым участникам («Я человек»).\n" +
			"• **Защита от кликов чужих:** клик другого пользователя отклоняется с алертом.\n" +
			"• **Автокик по таймауту:** удаление спам-ботов через `DELETE /chats/{id}/members`.\n" +
			"• **Боевой режим:** добавьте бота администратором в группу, и он начнет защищать чат в реальном времени!\n\n" +
			"💡 *Выберите действие для тестирования:* "

		formatted := hood.Format(userID, text, "guard_captcha")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// In-dialog simulation: New member join
	b.OnCallbackPayload("guard:sim_join", func(c *bot.Context) error {
		_ = c.AnswerNotification("Симуляция входа запущена")

		userID := int64(0)
		userName := "Новый участник"
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
			if cb.User.FirstName != "" {
				userName = cb.User.FirstName
			}
		}

		payloadVerify := fmt.Sprintf("guard:v:1000:%d", userID)
		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewIntentCallbackButton("✅ Я человек (нажмите)", payloadVerify, maxbot.IntentPositive)).
			Row(hood.Button("guard_captcha")).
			Row(maxbot.NewCallbackButton("◀️ Назад в меню", "nav:menu"))

		text := fmt.Sprintf("👋 **Симуляция входа: Добро пожаловать, %s!**\n\n"+
			"⏳ Внимание: включен антиспам-фильтр сообщества.\n"+
			"Пожалуйста, нажмите на кнопку «✅ Я человек» в течение **30 секунд**, чтобы подтвердить, что вы реальный пользователь.\n\n"+
			"_Если вы не нажмете кнопку вовремя, бот автоматически вызовет `DELETE /chats/{id}/members?user_id=%d` и исключит вас из чата._",
			userName, userID)

		formatted := hood.Format(userID, text, "guard_captcha")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// In-dialog simulation: Timeout kick
	b.OnCallbackPayload("guard:sim_timeout", func(c *bot.Context) error {
		_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
			Notification: "⏱ Таймаут истек: бот удален!",
		})

		userID := int64(0)
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
		}

		kickReport := fmt.Sprintf("🚫 **Таймаут истек: Участник автоматически исключен!**\n\n"+
			"• **Действие:** `DELETE /chats/1000/members?user_id=%d`\n"+
			"• **Причина:** Не пройдена проверка «Я человек» за отведенное время (30 сек)\n"+
			"• **Статус API:** `200 OK (Member removed)`\n"+
			"• **Очистка:** Сервисное сообщение с кнопкой капчи автоматически удалено из чата.\n\n"+
			"Так работает автоматическая защита групп и каналов в MAX Messenger от массового наплыва спам-ботов.",
			userID)

		kb := maxbot.NewInlineKeyboard().
			Row(hood.Button("guard_kick")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		formatted := hood.Format(userID, kickReport, "guard_kick")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// Live in-group bot_added event handler
	b.OnBotAdded(func(c *bot.Context) error {
		greeting := "🛡️ **Бот-модератор успешно добавлен в чат!**\n\n" +
			"Пожалуйста, назначьте меня **Администратором** группы с правом удаления участников, чтобы автоматический капча-гейт и антиспам-защита могли работать в полную силу."

		_, err := c.Reply(greeting)
		return err
	})
}
