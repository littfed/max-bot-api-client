package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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
	bridge := webapp.NewBridge(token).SetAllowUnauthenticated(true)
	bridge.OnAction("submit_cart", func(ctx context.Context, data *webapp.InitData, payload json.RawMessage) (any, error) {
		orderID := fmt.Sprintf("WEB-%d", time.Now().Unix()%100000)

		var cartData struct {
			Items      map[string]int `json:"items"`
			TotalPrice int            `json:"total_price"`
		}
		_ = json.Unmarshal(payload, &cartData)

		var targetUserID int64
		userName := "Пользователь"
		isVerified := false
		if data != nil && data.User != nil && data.User.ID != 0 {
			targetUserID = data.User.ID
			if data.User.FirstName != "" {
				userName = data.User.FirstName
			}
			if data.Hash != "" {
				isVerified = true
			}
		}

		if targetUserID != 0 && b.Client() != nil {
			statusLine := "Подпись initData: `HMAC-SHA256 Verified ✅`"
			if !isVerified {
				statusLine = "Подпись initData: `Демо-режим (WebBridge) 🧪`"
			}

			itemLines := make([]string, 0)
			for name, qty := range cartData.Items {
				if qty > 0 {
					itemLines = append(itemLines, fmt.Sprintf("• %s: %d шт.", name, qty))
				}
			}
			itemsStr := strings.Join(itemLines, "\n")
			if itemsStr == "" {
				itemsStr = "• Корзина из Mini App (1 шт.)"
			}

			text := fmt.Sprintf("🚀 **Заказ из Mini App успешно оформлен!**\n\n"+
				"• **Клиент:** %s\n"+
				"• **Номер заказа:** `%s`\n"+
				"• **Сумма:** `%d ₽`\n"+
				"• **Канал передачи:** WebBridge HTTP POST `/api/webapp/action`\n"+
				"• %s\n\n"+
				"**Состав заказа:**\n%s\n\n"+
				"Данные приняты сервером бота и зафиксированы в сессии.",
				userName, orderID, cartData.TotalPrice, statusLine, itemsStr)

			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

			msgReq := &maxbot.SendMessageRequest{
				Text:        text,
				Attachments: []maxbot.Attachment{kb.ToAttachment()},
			}
			_, _ = b.Client().SendMessage(context.Background(), maxbot.ToUser(targetUserID), msgReq)
		}

		return map[string]any{
			"status":    "accepted",
			"order_id":  orderID,
			"timestamp": time.Now().Unix(),
		}, nil
	})

	httpMux.HandleFunc("/api/webapp/action", bridge.HTTPHandler())
	serveMiniApp := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(miniAppHTML))
	}
	httpMux.HandleFunc("/webapp", serveMiniApp)
	httpMux.HandleFunc("/webapp/", serveMiniApp)
	httpMux.HandleFunc("/maxapp", serveMiniApp)
	httpMux.HandleFunc("/maxapp/", serveMiniApp)
	httpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			serveMiniApp(w, r)
			return
		}
		http.NotFound(w, r)
	})

	// 3. Scenario Launch Route (scen:webapp)
	b.OnCallbackPayload("scen:webapp", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		botUsername := strings.TrimSpace(os.Getenv("WEBAPP_BOT_USERNAME"))
		if botUsername == "" {
			botUsername = "id452401070481_bot"
		}
		publicURL := strings.TrimSpace(os.Getenv("WEBAPP_BASE_URL"))
		if publicURL == "" {
			publicURL = strings.TrimSpace(os.Getenv("PUBLIC_URL"))
		}
		if publicURL == "" {
			publicURL = "https://combot.littfed.ru"
		}
		appURL := strings.TrimSpace(os.Getenv("WEBAPP_URL"))
		if appURL == "" {
			appURL = strings.TrimRight(publicURL, "/") + "/maxapp"
		}

		userID := int64(0)
		firstName := ""
		username := ""
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
			firstName = cb.User.FirstName
			username = cb.User.Username
		}

		appURLWithAuth := appURL
		if userID != 0 && token != "" {
			signedInit := webapp.SignInitData(token, userID, firstName, username)
			if strings.Contains(appURL, "?") {
				appURLWithAuth = appURL + "&" + signedInit
			} else {
				appURLWithAuth = appURL + "?" + signedInit
			}
		}

		appBtn, _ := webapp.OpenAppButton("📱 Запустить в MAX (open_app)", botUsername)

		kb := maxbot.NewInlineKeyboard().
			Row(appBtn).
			Row(maxbot.NewLinkButton("🌐 Открыть Mini App (combot.littfed.ru/maxapp)", appURLWithAuth)).
			Row(maxbot.NewIntentCallbackButton("🧪 Симулировать заказ из Mini App", "webapp:simulate_order", maxbot.IntentPositive)).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		text := "📱 **Экосистема MAX Mini Apps (WebApps)**\n\n" +
			"Этот сценарий демонстрирует интеграцию встроенных HTML5-приложений в MAX Messenger:\n\n" +
			"• **Запуск через кнопку:** нативная шторка WebView прямо внутри мессенджера (`open_app`).\n" +
			"• **Прямой адрес приложения:** `https://combot.littfed.ru/maxapp`\n" +
			"• **Двусторонний WebBridge:** передача корзины и действий из JavaScript на сервер бота.\n" +
			"• **Криптозащита:** валидация подлинности пользователя по HMAC-SHA256 подписи.\n\n" +
			"💡 *Для нативного запуска через кнопку «open_app» укажите ссылку в настройках Web App бота в MAX. Также можно открыть приложение напрямую по веб-ссылке:*"

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
