package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
)

// RegisterWebappScenario installs Mini App demonstration routes, WebBridge handler, and technical cards.
func RegisterWebappScenario(b *bot.Bot, hood *HoodEngine, httpMux *http.ServeMux, token string, port string) {
	// 1. Technical cards
	hood.Register("webapp_open_app", TechCard{
		Title:       "Кнопка запуска open_app vs Link",
		Endpoint:    "/messages",
		Method:      "POST",
		Description: "В MAX Bot API кнопка type: open_app принимает зарегистрированный slug/username бота (например, 'catalog_bot'), а не произвольный HTTP URL. Передача HTTP URL в open_app возвращает 404 LinkPK. Для внешних страниц используется type: link.",
		DocsLink:    "https://dev.max.ru/docs/buttons",
	})

	hood.Register("webapp_bridge", TechCard{
		Title:       "Двусторонний WebBridge протокол",
		Endpoint:    "/api/webapp/action",
		Method:      "POST",
		Description: "Mini App отправляет события и заказы на бэкенд бота через WebBridge. Бот валидирует initData, распаковывает payload и мгновенно отправляет подтверждение прямо в чат клиента.",
		DocsLink:    "https://dev.max.ru/docs/webapps",
	})

	hood.Register("webapp_security", TechCard{
		Title:       "Криптографическая верификация HMAC-SHA256",
		Endpoint:    "ValidateInitData()",
		Method:      "CRYPTO",
		Description: "Параметры initData сортируются по алфавиту и конкатенируются. Секретный ключ вычисляется как HMAC-SHA256('WebAppData', botToken). Полученный дайджест сверяется с полем hash через constant-time сравнение.",
		DocsLink:    "https://dev.max.ru/docs/webapps#security",
	})

	// 2. Setup WebBridge & HTTP Static Server
	bridge := webapp.NewBridge(token)
	bridge.OnAction("submit_cart", func(ctx context.Context, data *webapp.InitData, payload json.RawMessage) (any, error) {
		return map[string]any{
			"status":    "accepted",
			"timestamp": time.Now().Unix(),
		}, nil
	})

	httpMux.HandleFunc("/api/webapp/action", bridge.HTTPHandler())
	httpMux.HandleFunc("/webapp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(miniAppHTML))
	})

	// 3. Scenario Launch Route (scen:webapp)
	b.OnCallbackPayload("scen:webapp", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		appURL := fmt.Sprintf("http://localhost:%s/webapp", port)
		appBtn, _ := webapp.OpenAppButton("📱 Запустить в MAX (open_app)", "showcase_bot")

		kb := maxbot.NewInlineKeyboard().
			Row(appBtn).
			Row(maxbot.NewLinkButton("🌐 Открыть WebApp в браузере", appURL)).
			Row(maxbot.NewIntentCallbackButton("🧪 Симулировать заказ из Mini App", "webapp:simulate_order", maxbot.IntentPositive)).
			Row(hood.Button("webapp_open_app")).
			Row(hood.Button("webapp_bridge")).
			Row(hood.Button("webapp_security")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		userID := int64(0)
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
		}

		text := "📱 **Экосистема MAX Mini Apps (WebApps)**\n\n" +
			"Этот сценарий демонстрирует интеграцию встроенных HTML5-приложений в MAX Messenger:\n\n" +
			"• **Запуск через кнопку:** нативная шторка WebView внутри мессенджера (`open_app`).\n" +
			"• **Двусторонний WebBridge:** отправка корзины и действий из JS на сервер бота.\n" +
			"• **Криптозащита:** валидация подлинности пользователя по HMAC-SHA256 подписи.\n\n" +
			"💡 *Откройте веб-приложение в браузере или нажмите «Симулировать заказ» для мгновенной проверки WebBridge:* "

		formatted := hood.Format(userID, text, "webapp_open_app")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// Simulate order callback
	b.OnCallbackPayload("webapp:simulate_order", func(c *bot.Context) error {
		_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
			Notification: "🧪 WebBridge: заказ из Mini App получен!",
		})

		orderID := fmt.Sprintf("WEB-%d", time.Now().Unix()%100000)

		receipt := fmt.Sprintf("🚀 **Заказ из Mini App успешно принят!**\n\n"+
			"• **Номер заказа:** `%s`\n"+
			"• **Состав:** Кофемашина Barista (1 шт.) + Наушники Pro ANC (1 шт.)\n"+
			"• **Сумма:** `7 480 ₽`\n"+
			"• **Канал передачи:** WebBridge HTTP POST `/api/webapp/action`\n"+
			"• **Подпись initData:** `HMAC-SHA256 Verified ✅`\n\n"+
			"Данные были переданы из клиентского WebView на сервер бота с проверкой криптографической подписи токеном бота.",
			orderID)

		kb := maxbot.NewInlineKeyboard().
			Row(hood.Button("webapp_security")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		userID := int64(0)
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
		}

		formatted := hood.Format(userID, receipt, "webapp_security")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})
}
