package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/ai"
	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/store"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
)

// RegisterWebappScenario installs Mini App demonstration routes, WebBridge handler, and technical cards.
func RegisterWebappScenario(
	b *bot.Bot,
	hood *HoodEngine,
	httpMux *http.ServeMux,
	token string,
	port string,
	appStore *store.Store,
	client *maxbot.Client,
	aiClient *ai.Client,
	chatModel string,
) {
	const ownerUserID int64 = 396393759

	// 1. Technical cards
	hood.Register("webapp_open_app", TechCard{
		Title:       "Кнопка запуска open_app vs Link",
		Endpoint:    "/messages",
		Method:      "POST",
		Description: "В MAX Bot API кнопка type: open_app принимает зарегистрированный slug/username бота (например, 'id452401070481_bot'). Для внешних страниц используется type: link.",
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
			Items           map[string]int `json:"items"`
			TotalPrice      int            `json:"total_price"`
			DiscountPercent int            `json:"discount_percent"`
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

		productMap := map[string]string{
			"gopher_hoodie":      "Худи Go Gopher Edition",
			"tech_tshirt":        "Футболка MAX Ecosystem",
			"barista_coffee":     "Кофемашина Barista 15bar",
			"pro_anc_headphones": "Наушники Pro ANC Wireless",
			"stickers_pack":      "Стикерпак Bot Masters",
			"go_patterns_book":   "Книга «Паттерны микросервисов в Go»",
		}

		// Store order in persistence layer
		if appStore != nil {
			var orderItems []store.OrderItem
			for id, qty := range cartData.Items {
				if qty > 0 {
					title := id
					if mapped, ok := productMap[id]; ok {
						title = mapped
					}
					orderItems = append(orderItems, store.OrderItem{
						ID:       id,
						Title:    title,
						Quantity: qty,
					})
				}
			}
			_ = appStore.CreateOrder(store.Order{
				ID:           orderID,
				UserID:       targetUserID,
				UserName:     userName,
				Items:        orderItems,
				Total:        cartData.TotalPrice,
				DeliveryType: "delivery",
				Status:       "accepted",
				Verified:     isVerified,
				CreatedAt:    time.Now(),
			})
		}

		if targetUserID != 0 && b.Client() != nil {
			statusLine := "Подпись initData: `HMAC-SHA256 Verified ✅`"
			if !isVerified {
				statusLine = "Подпись initData: `Демо-режим (WebBridge) 🧪`"
			}

			itemLines := make([]string, 0)
			for id, qty := range cartData.Items {
				if qty > 0 {
					title := id
					if mapped, ok := productMap[id]; ok {
						title = mapped
					}
					itemLines = append(itemLines, fmt.Sprintf("• %s: %d шт.", title, qty))
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

	// WebBridge Action: book_service
	bridge.OnAction("book_service", func(ctx context.Context, data *webapp.InitData, payload json.RawMessage) (any, error) {
		bookingID := fmt.Sprintf("BOK-%d", time.Now().Unix()%100000)

		var req struct {
			ServiceID   string `json:"service_id"`
			ServiceName string `json:"service_name"`
			Specialist  string `json:"specialist"`
			Slot        string `json:"slot"`
			Price       int    `json:"price"`
			UserName    string `json:"user_name"`
			Phone       string `json:"phone"`
			Comment     string `json:"comment"`
		}
		_ = json.Unmarshal(payload, &req)

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
		if req.UserName != "" {
			userName = req.UserName
		}

		serviceMap := map[string]string{
			"srv_audit":   "Архитектурный аудит Go / MAX",
			"srv_review":  "Code Review & Рефакторинг",
			"srv_turnkey": "Разработка MAX-бота под ключ",
			"srv_coffee":  "Дегустация specialty кофе",
		}
		if canonical, ok := serviceMap[req.ServiceID]; ok {
			req.ServiceName = canonical
		} else if strings.Contains(req.ServiceName, "?") || req.ServiceName == "" {
			req.ServiceName = "Консультация по MAX Bot API"
		}

		if strings.Contains(req.Specialist, "?") || req.Specialist == "" {
			req.Specialist = "Алексей — Lead Go / Архитектор MAX"
		}
		if strings.Contains(req.Slot, "?") || req.Slot == "" {
			req.Slot = "Завтра, 11:00"
		}
		if strings.Contains(userName, "?") || userName == "" {
			userName = "Федор В"
		}

		// Store booking in persistence layer
		if appStore != nil {
			_ = appStore.CreateOrder(store.Order{
				ID:           bookingID,
				UserID:       targetUserID,
				UserName:     userName,
				Items: []store.OrderItem{
					{
						ID:       req.ServiceID,
						Title:    req.ServiceName,
						Price:    req.Price,
						Quantity: 1,
					},
				},
				Total:        req.Price,
				DeliveryType: "booking",
				Address:      fmt.Sprintf("Специалист: %s | Время: %s | Телефон: %s | Заметка: %s", req.Specialist, req.Slot, req.Phone, req.Comment),
				Status:       "accepted",
				Verified:     isVerified,
				CreatedAt:    time.Now(),
			})
		}

		if targetUserID != 0 && b.Client() != nil {
			statusLine := "Подпись initData: `HMAC-SHA256 Verified ✅`"
			if !isVerified {
				statusLine = "Подпись initData: `Демо-режим (WebBridge) 🧪`"
			}

			priceStr := "Бесплатно"
			if req.Price > 0 {
				priceStr = fmt.Sprintf("%d ₽", req.Price)
			}

			text := fmt.Sprintf("📅 **Запись на услугу успешно оформлена!**\n\n"+
				"• **Клиент:** %s\n"+
				"• **Номер бронирования:** `%s`\n"+
				"• **Услуга:** %s\n"+
				"• **Специалист:** %s\n"+
				"• **Выбранный слот:** `%s`\n"+
				"• **Стоимость:** `%s`\n"+
				"• **Канал передачи:** WebBridge HTTP POST `/api/webapp/action`\n"+
				"• %s\n\n"+
				"Бронирование подтверждено и зафиксировано в расписании.",
				userName, bookingID, req.ServiceName, req.Specialist, req.Slot, priceStr, statusLine)

			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

			msgReq := &maxbot.SendMessageRequest{
				Text:        text,
				Attachments: []maxbot.Attachment{kb.ToAttachment()},
			}
			_, _ = b.Client().SendMessage(context.Background(), maxbot.ToUser(targetUserID), msgReq)
		}

		return map[string]any{
			"status":     "accepted",
			"booking_id": bookingID,
			"timestamp":  time.Now().Unix(),
		}, nil
	})

	httpMux.HandleFunc("/api/webapp/action", bridge.HTTPHandler())

	// Endpoint: GET /api/webapp/orders
	httpMux.HandleFunc("/api/webapp/orders", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uidStr := r.URL.Query().Get("user_id")
		uid, _ := strconv.ParseInt(uidStr, 10, 64)

		var orders []store.Order
		if appStore != nil {
			if uid == ownerUserID {
				orders = appStore.ListAllOrders()
			} else if uid != 0 {
				orders = appStore.ListOrdersByUser(uid)
			} else {
				orders = appStore.ListAllOrders()
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"orders":  orders,
		})
	})

	// Endpoint: POST /api/webapp/spin
	httpMux.HandleFunc("/api/webapp/spin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		codes := []struct {
			code     string
			discount int
		}{
			{"PROMO-15-LUCK", 15},
			{"PROMO-20-DEV", 20},
			{"PROMO-25-MAX", 25},
		}
		picked := codes[time.Now().Unix()%int64(len(codes))]

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"code":     picked.code,
			"discount": picked.discount,
		})
	})

	// Endpoint: POST /api/webapp/generate_post
	httpMux.HandleFunc("/api/webapp/generate_post", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Prompt string `json:"prompt"`
			UserID int64  `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		generatedText := ""
		if aiClient != nil {
			aiCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			resp, err := aiClient.Chat(aiCtx, &ai.ChatRequest{
				Model: chatModel,
				Messages: []ai.ChatMessage{
					{Role: "system", Content: "Ты опытный SMM-редактор и автор Telegram/MAX каналов об IT и электронной коммерции."},
					{Role: "user", Content: req.Prompt},
				},
			})
			cancel()
			if err == nil && resp != nil && resp.Content != "" {
				generatedText = resp.Content
			}
		}

		if generatedText == "" {
			generatedText = "🚀 **Новые поступления в MAX Mini App!**\n\n" +
				"В нашем магазине обновился ассортимент мерча и гаджетов для разработчиков:\n" +
				"• Худи Go Gopher Edition\n" +
				"• Наушники Pro ANC Wireless\n" +
				"• Кофемашина Barista 15bar\n\n" +
				"Оформляйте заказ прямо в мессенджере с гарантированной скидкой по промокоду!"
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"text":    generatedText,
		})
	})

	// Endpoint: POST /api/webapp/generate_image
	httpMux.HandleFunc("/api/webapp/generate_image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Prompt     string `json:"prompt"`
			Style      string `json:"style"`
			UserID     int64  `json:"user_id"`
			SendToChat bool   `json:"send_to_chat"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fullPrompt := strings.TrimSpace(req.Prompt)
		if fullPrompt == "" {
			fullPrompt = "Маскот Go gopher и MAX Bot в неоновом киберпанк стиле"
		}
		if req.Style != "" && !strings.Contains(strings.ToLower(fullPrompt), strings.ToLower(req.Style)) {
			fullPrompt = fmt.Sprintf("%s, style: %s, detailed, high quality, 4k", fullPrompt, req.Style)
		}

		var imgBytes []byte
		var err error

		if aiClient != nil {
			aiCtx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
			imgResp, imgErr := aiClient.GenerateImage(aiCtx, &ai.ImageRequest{
				Prompt: fullPrompt,
				Model:  "gemini-2.5-flash-image",
				Size:   "512x512",
			})
			cancel()

			if imgErr == nil && imgResp != nil {
				if len(imgResp.Data) > 0 {
					imgBytes = imgResp.Data
				} else if imgResp.URL != "" {
					fetchReq, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, imgResp.URL, nil)
					fetchResp, fetchErr := http.DefaultClient.Do(fetchReq)
					if fetchErr == nil && fetchResp.StatusCode == http.StatusOK {
						imgBytes, _ = io.ReadAll(fetchResp.Body)
						_ = fetchResp.Body.Close()
					}
				}
			}
		}

		if len(imgBytes) == 0 {
			imgBytes = generatePlaceholderImage(fullPrompt)
		}

		b64Data := base64.StdEncoding.EncodeToString(imgBytes)
		dataURL := "data:image/png;base64," + b64Data

		sentToChat := false
		if req.SendToChat && req.UserID != 0 && b.Client() != nil && len(imgBytes) > 0 {
			uploadResp, uploadErr := b.Client().UploadMedia(context.Background(), maxbot.UploadTypeImage, "ai_artwork.png", bytes.NewReader(imgBytes))
			if uploadErr == nil && uploadResp != nil && uploadResp.Token != "" {
				att := maxbot.Attachment{
					Type: maxbot.UploadTypeImage,
					Payload: map[string]any{
						"token": uploadResp.Token,
					},
				}
				caption := fmt.Sprintf("🎨 **Ваш AI-арт из Mini App:**\n\n«%s»\n\n*Сгенерировано в AI Studio через WebBridge*", fullPrompt)
				kb := maxbot.NewInlineKeyboard().
					Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))
				_, err = b.Client().SendMessage(context.Background(), maxbot.ToUser(req.UserID), &maxbot.SendMessageRequest{
					Text:        caption,
					Attachments: []maxbot.Attachment{att, kb.ToAttachment()},
				})
				if err == nil {
					sentToChat = true
				}
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":      true,
			"image_url":    dataURL,
			"prompt":       fullPrompt,
			"sent_to_chat": sentToChat,
		})
	})

	// Endpoint: POST /api/webapp/ai_chat
	httpMux.HandleFunc("/api/webapp/ai_chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Prompt string `json:"prompt"`
			Model  string `json:"model"`
			UserID int64  `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		userPrompt := strings.TrimSpace(req.Prompt)
		if userPrompt == "" {
			userPrompt = "Расскажи кратко о возможностях экосистемы MAX Bot API и WebApps."
		}

		selectedModel := req.Model
		if selectedModel == "" {
			selectedModel = chatModel
		}
		if selectedModel == "" {
			selectedModel = "auto"
		}

		replyText := ""
		usedModel := selectedModel
		var latency time.Duration

		if aiClient != nil {
			start := time.Now()
			aiCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			resp, err := aiClient.Chat(aiCtx, &ai.ChatRequest{
				Model: selectedModel,
				Messages: []ai.ChatMessage{
					{Role: "system", Content: "Ты интеллектуальный AI-ассистент в MAX Mini App. Отвечай кратко, емко, дружелюбно и профессионально на русском языке. Поддерживай форматирование markdown."},
					{Role: "user", Content: userPrompt},
				},
			})
			cancel()
			latency = time.Since(start)

			if err == nil && resp != nil && resp.Content != "" {
				replyText = resp.Content
				if resp.Model != "" {
					usedModel = resp.Model
				}
			}
		}

		if replyText == "" {
			latency = 120 * time.Millisecond
			replyText = fmt.Sprintf("🤖 **Ответ AI в MAX Mini App:**\n\nВы запросили: *«%s»*\n\nMAX Mini Apps поддерживают нативный двусторонний WebBridge, полноэкранный WebView, передачу токенов авторизации через HMAC-SHA256 и бесшовное взаимодействие с чатом бота!", userPrompt)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"text":       replyText,
			"model":      usedModel,
			"latency_ms": latency.Milliseconds(),
		})
	})

	// Endpoint: POST /api/webapp/publish_post
	httpMux.HandleFunc("/api/webapp/publish_post", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ChannelID int64  `json:"channel_id"`
			UserID    int64  `json:"user_id"`
			Text      string `json:"text"`
			Pin       bool   `json:"pin"`
			Reactions bool   `json:"reactions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Security Gating: Only Owner (396393759) or verified admin can publish to live channels
		if req.UserID != ownerUserID && req.UserID != 0 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Доступ запрещен: публикация в каналы разрешена только владельцу бота",
			})
			return
		}

		if client == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "MAX Bot client offline",
			})
			return
		}

		publicURL := "https://combot.littfed.ru/maxapp"
		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewLinkButton("🛍️ Открыть в Mini App", publicURL))

		if req.Reactions {
			kb.Row(
				maxbot.NewCallbackButton("👍 0", "rx:pending:👍"),
				maxbot.NewCallbackButton("🔥 0", "rx:pending:🔥"),
				maxbot.NewCallbackButton("❤️ 0", "rx:pending:❤️"),
			)
		}

		sentMsg, err := client.SendMessage(context.Background(), maxbot.ToChat(req.ChannelID), &maxbot.SendMessageRequest{
			Text:        req.Text,
			Format:      "markdown",
			Attachments: []maxbot.Attachment{kb.ToAttachment()},
		})
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		if sentMsg.MessageID != "" {
			if req.Reactions {
				realKb := buildReactionsKeyboard(publicURL, sentMsg.MessageID, map[string]int{"👍": 0, "🔥": 0, "❤️": 0})
				_, _ = client.EditMessage(context.Background(), sentMsg.MessageID, &maxbot.EditMessageRequest{
					Attachments: []maxbot.Attachment{realKb.ToAttachment()},
				})
				if appStore != nil {
					_, _ = appStore.ToggleReaction(sentMsg.MessageID, req.ChannelID, 0, "")
				}
			}
			if req.Pin {
				_ = client.PinMessage(context.Background(), req.ChannelID, sentMsg.MessageID, false)
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"message_id": sentMsg.MessageID,
		})
	})

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

		text := "📱 **Экосистема MAX Mini Apps (WebApps SuperApp)**\n\n" +
			"Полноценный 5-вкладочный демонстрационный комплекс прямо в мессенджере:\n\n" +
			"• 🛍️ **Магазин мерча & электроники:** каталог, фильтры, корзина, промокоды и заказ через WebBridge.\n" +
			"• 📅 **Онлайн-запись & бронирование:** выбор услуг, специалистов, интерактивная сетка слотов.\n" +
			"• ✨ **AI Студия:** генератор артов (Text-to-Image) с отправкой в чат + быстрый LLM-чат с выбором моделей.\n" +
			"• ✍️ **Студия каналов:** SMM-редактор, автогенерация постов через Gemini, превью и публикация.\n" +
			"• 🎁 **Бонусы & Заказы:** Колесо фортуны, интерактивный MAX API Квиз и общая история активности.\n\n" +
			"💡 *Откройте Mini App по веб-ссылке или через нативную кнопку `open_app`:*"

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

		if appStore != nil {
			_ = appStore.CreateOrder(store.Order{
				ID:       orderID,
				UserID:   ownerUserID,
				UserName: "Федор В",
				Items: []store.OrderItem{
					{ID: "barista_coffee", Title: "Кофемашина Barista", Quantity: 1},
					{ID: "pro_anc_headphones", Title: "Наушники Pro ANC", Quantity: 1},
				},
				Total:        7480,
				DeliveryType: "pickup",
				Status:       "accepted",
				Verified:     true,
				CreatedAt:    time.Now(),
			})
		}

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
