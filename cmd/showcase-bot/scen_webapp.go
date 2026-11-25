package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

func authenticateWebappRequest(w http.ResponseWriter, r *http.Request, botToken string) (*webapp.InitData, bool) {
	initDataStr := r.Header.Get("X-Init-Data")
	if initDataStr == "" {
		initDataStr = r.URL.Query().Get("init_data")
	}
	initData, err := webapp.ValidateInitData(initDataStr, botToken, 24*time.Hour)
	if err != nil || initData == nil || initData.User == nil || initData.User.ID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "unauthorized",
		})
		return nil, false
	}
	return initData, true
}

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
	skipRoot ...bool,
) {
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

	hood.Register("orders_history", TechCard{
		Title:       "История заказов и бронирований",
		Endpoint:    "/api/webapp/orders",
		Method:      "GET",
		Description: "Выборка сохраненных заказов и бронирований пользователя из постоянного хранилища (PostgreSQL/SQLite) через sql_store.",
	})

	// 2. Setup WebBridge & HTTP Static Server
	bridge := webapp.NewBridge(token)
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

		if targetUserID == 0 {
			return nil, errors.New("authenticated user required")
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

		sentToChat := false

		if targetUserID != 0 && b.Client() != nil {
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
				itemsStr = "• Корзина из каталога (1 шт.)"
			}

			text := fmt.Sprintf("🛍️ **Ваш заказ успешно оформлен!**\n\n"+
				"• **Номер заказа:** `%s`\n"+
				"• **Сумма к оплате:** `%d ₽`\n\n"+
				"**Состав заказа:**\n%s\n\n"+
				"Спасибо за покупку! Наш менеджер уже готовит заказ к отправке. Мы пришлем уведомление, когда статус изменится.",
				orderID, cartData.TotalPrice, itemsStr)

			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

			formatted := hood.Format(targetUserID, text, "webapp_action")

			msgReq := &maxbot.SendMessageRequest{
				Text:        formatted,
				Format:      "markdown",
				Attachments: []maxbot.Attachment{kb.ToAttachment()},
			}
			_, sendErr := b.Client().SendMessage(ctx, maxbot.ToUser(targetUserID), msgReq)
			if sendErr != nil {
				slog.Error("failed to send order confirmation to chat", slog.Int64("user_id", targetUserID), slog.Any("err", sendErr))
			} else {
				sentToChat = true
				slog.Info("order confirmation sent to user chat", slog.Int64("user_id", targetUserID), slog.String("order_id", orderID))
			}
		}

		return map[string]any{
			"status":       "accepted",
			"order_id":     orderID,
			"sent_to_chat": sentToChat,
			"timestamp":    time.Now().Unix(),
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

		if targetUserID == 0 {
			return nil, errors.New("authenticated user required")
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

		sentToChat := false

		if targetUserID != 0 && b.Client() != nil {
			priceStr := "Бесплатно"
			if req.Price > 0 {
				priceStr = fmt.Sprintf("%d ₽", req.Price)
			}

			text := fmt.Sprintf("📅 **Запись на услугу успешно оформлена!**\n\n"+
				"• **Номер бронирования:** `%s`\n"+
				"• **Услуга:** %s\n"+
				"• **Специалист:** %s\n"+
				"• **Дата и время:** %s\n"+
				"• **Стоимость:** %s\n\n"+
				"Ждем вас! За 2 часа до начала мы пришлем напоминание в этот чат.",
				bookingID, req.ServiceName, req.Specialist, req.Slot, priceStr)

			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

			formatted := hood.Format(targetUserID, text, "webapp_action")

			msgReq := &maxbot.SendMessageRequest{
				Text:        formatted,
				Format:      "markdown",
				Attachments: []maxbot.Attachment{kb.ToAttachment()},
			}
			_, sendErr := b.Client().SendMessage(ctx, maxbot.ToUser(targetUserID), msgReq)
			if sendErr != nil {
				slog.Error("failed to send booking confirmation to chat", slog.Int64("user_id", targetUserID), slog.Any("err", sendErr))
			} else {
				sentToChat = true
				slog.Info("booking confirmation sent to user chat", slog.Int64("user_id", targetUserID), slog.String("booking_id", bookingID))
			}
		}

		return map[string]any{
			"status":       "accepted",
			"booking_id":   bookingID,
			"sent_to_chat": sentToChat,
			"timestamp":    time.Now().Unix(),
		}, nil
	})

	httpMux.HandleFunc("/api/webapp/action", bridge.HTTPHandler())

	// Endpoint: GET /api/webapp/orders
	httpMux.HandleFunc("/api/webapp/orders", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		initData, ok := authenticateWebappRequest(w, r, token)
		if !ok {
			return
		}
		uid := initData.User.ID

		orders := make([]store.Order, 0)
		if appStore != nil {
			orders = appStore.ListOrdersByUser(uid)
		}
		if orders == nil {
			orders = make([]store.Order, 0)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"orders":  orders,
		})
	})

	// Endpoint: GET /api/webapp/history
	httpMux.HandleFunc("/api/webapp/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		initData, ok := authenticateWebappRequest(w, r, token)
		if !ok {
			return
		}
		uid := initData.User.ID

		artifactType := strings.TrimSpace(r.URL.Query().Get("type"))
		limitStr := r.URL.Query().Get("limit")
		limit, _ := strconv.Atoi(limitStr)
		if limit <= 0 || limit > 100 {
			limit = 30
		}

		var items []store.MediaArtifact
		if appStore != nil {
			var err error
			items, err = appStore.ListMediaArtifacts(uid, artifactType, limit)
			if err != nil {
				slog.Error("failed to list media artifacts", slog.Int64("user_id", uid), slog.Any("error", err))
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   "failed to load history",
				})
				return
			}
		}
		if items == nil {
			items = []store.MediaArtifact{}
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"user_id": uid,
			"type":    artifactType,
			"items":   items,
		})
	})

	// Endpoint: POST /api/webapp/spin
	httpMux.HandleFunc("/api/webapp/spin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, ok := authenticateWebappRequest(w, r, token); !ok {
			return
		}
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
		if _, ok := authenticateWebappRequest(w, r, token); !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

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
			aiCtx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
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
		initData, ok := authenticateWebappRequest(w, r, token)
		if !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req struct {
			Prompt     string `json:"prompt"`
			Style      string `json:"style"`
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

		if aiClient != nil {
			aiCtx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
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
					if _, urlErr := validatePublicHTTPURL(imgResp.URL); urlErr == nil {
						fetchReq, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, imgResp.URL, nil)
						fetchResp, fetchErr := newSafeHTTPClient(30 * time.Second).Do(fetchReq)
						if fetchErr == nil && fetchResp.StatusCode == http.StatusOK {
							imgBytes, _ = io.ReadAll(io.LimitReader(fetchResp.Body, 10<<20))
							_ = fetchResp.Body.Close()
						}
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
		var sendErrMsg string
		targetUID := initData.User.ID
		if req.SendToChat && b.Client() != nil && len(imgBytes) > 0 && targetUID != 0 {
			uploadResp, uploadErr := b.Client().UploadMedia(r.Context(), maxbot.UploadTypeImage, "ai_artwork.png", bytes.NewReader(imgBytes))
			if uploadErr == nil && uploadResp != nil && uploadResp.Token != "" {
				att := maxbot.Attachment{
					Type: maxbot.UploadTypeImage,
					Payload: map[string]any{
						"token": uploadResp.Token,
					},
				}
				caption := fmt.Sprintf("🎨 **Ваше сгенерированное изображение:**\n\n«%s»", fullPrompt)
				kb := maxbot.NewInlineKeyboard().
					Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))
				_, sendErr := b.Client().SendMessage(r.Context(), maxbot.ToUser(targetUID), &maxbot.SendMessageRequest{
					Text:        caption,
					Format:      "markdown",
					Attachments: []maxbot.Attachment{att, kb.ToAttachment()},
				})
				if sendErr == nil {
					sentToChat = true
					slog.Info("ai image sent to user chat", slog.Int64("user_id", targetUID))
				} else {
					sendErrMsg = "failed to send image"
					slog.Error("failed to send ai image to chat", slog.Int64("user_id", targetUID), slog.Any("err", sendErr))
				}
			} else {
				sendErrMsg = "failed to upload image"
				slog.Error("failed to upload ai image media", slog.Any("err", uploadErr))
			}
		}

		// Save generated image to media history
		if appStore != nil && len(imgBytes) > 0 && targetUID != 0 {
			title := "Арт: " + fullPrompt
			if len([]rune(title)) > 40 {
				title = string([]rune(title)[:40]) + "…"
			}
			_, _ = appStore.SaveMediaArtifact(store.MediaArtifact{
				UserID:       targetUID,
				ArtifactType: "image",
				Title:        title,
				Prompt:       fullPrompt,
				MediaURL:     dataURL,
				Metadata: map[string]any{
					"style":        req.Style,
					"size":         len(imgBytes),
					"sent_to_chat": sentToChat,
				},
				CreatedAt: time.Now(),
			})
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":      true,
			"image_url":    dataURL,
			"prompt":       fullPrompt,
			"sent_to_chat": sentToChat,
			"message":      sendErrMsg,
		})
	})

	// Endpoint: POST /api/webapp/ai_chat
	httpMux.HandleFunc("/api/webapp/ai_chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, ok := authenticateWebappRequest(w, r, token); !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

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
			aiCtx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
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
			replyText = fmt.Sprintf("🤖 **Ответ ассистента:**\n\nВы спросили: *«%s»*\n\nК сожалению, AI-модуль сейчас временно недоступен. Пожалуйста, попробуйте повторить запрос чуть позже.", userPrompt)
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
		initData, ok := authenticateWebappRequest(w, r, token)
		if !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req struct {
			ChannelID int64  `json:"channel_id"`
			Text      string `json:"text"`
			Pin       bool   `json:"pin"`
			Reactions bool   `json:"reactions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Security Gating: Only verified admin can publish to live channels
		if !store.IsAdminPlatformUser(strconv.FormatInt(initData.User.ID, 10)) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Доступ запрещен: публикация в каналы разрешена только администратору бота",
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

		sentMsg, err := client.SendMessage(r.Context(), maxbot.ToChat(req.ChannelID), &maxbot.SendMessageRequest{
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

		postCtx := context.WithoutCancel(r.Context())
		if sentMsg.MessageID != "" {
			if req.Reactions {
				realKb := buildReactionsKeyboard(publicURL, sentMsg.MessageID, map[string]int{"👍": 0, "🔥": 0, "❤️": 0})
				_, _ = client.EditMessage(postCtx, sentMsg.MessageID, &maxbot.EditMessageRequest{
					Attachments: []maxbot.Attachment{realKb.ToAttachment()},
				})
				if appStore != nil {
					_, _ = appStore.ToggleReaction(sentMsg.MessageID, req.ChannelID, 0, "")
				}
			}
			if req.Pin {
				_ = client.PinMessage(postCtx, req.ChannelID, sentMsg.MessageID, false)
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"message_id": sentMsg.MessageID,
		})
	})

	if len(skipRoot) == 0 || !skipRoot[0] {
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
	}

	// 3. Scenario Launch Route (scen:webapp and /app commands)
	openMiniAppHandler := func(c *bot.Context) error {
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
		if c.Sender() != nil {
			userID = c.Sender().UserID
			firstName = c.Sender().FirstName
			username = c.Sender().Username
		} else if cb := c.Callback(); cb != nil && cb.User != nil {
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
			Row(maxbot.NewLinkButton("🌐 Открыть витрину (веб-ссылка)", appURLWithAuth)).
			Row(maxbot.NewIntentCallbackButton("🛍️ Быстрый тестовый заказ", "webapp:simulate_order", maxbot.IntentPositive)).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		text := "📱 **Онлайн-витрина и сервисы**\n\n" +
			"Добро пожаловать в наше интерактивное приложение прямо в мессенджере:\n\n" +
			"• 🛍️ **Магазин товаров:** каталог, фильтры, удобная корзина и быстрое оформление.\n" +
			"• 📅 **Онлайн-запись:** выбор услуг, специалистов и удобного времени.\n" +
			"• ✨ **AI Студия:** создание иллюстраций по описанию и быстрый умный ассистент.\n" +
			"• 🎁 **Бонусы и история:** колесо скидок, квиз и список ваших заказов.\n\n" +
			"💡 *Нажмите кнопку ниже, чтобы запустить приложение:*"

		formatted := hood.Format(userID, text, "webapp_open_app")
		_, err := c.EditOrReplyKeyboard(formatted, kb)
		return err
	}

	b.OnCallbackPayload("scen:webapp", openMiniAppHandler)
	b.OnCommand("app", openMiniAppHandler)
	b.OnCommand("webapp", openMiniAppHandler)
	b.OnCommand("shop", openMiniAppHandler)
	b.OnCommand("store", openMiniAppHandler)
	b.OnCommand("miniapp", openMiniAppHandler)

	// Simulate order callback
	b.OnCallbackPayload("webapp:simulate_order", func(c *bot.Context) error {
		// Anti-Ghost-Click: clear interactive button on previous message
		_, _ = c.ClearKeyboard()

		_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
			Notification: "🛍️ Заказ успешно принят!",
		})

		orderID := fmt.Sprintf("WEB-%d", time.Now().Unix()%100000)

		userID := int64(0)
		userName := "Федор В"
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
			if cb.User.FirstName != "" {
				userName = cb.User.FirstName
			}
		}
		if userID == 0 {
			return c.AnswerNotification("Не удалось определить пользователя")
		}

		if appStore != nil {
			_ = appStore.CreateOrder(store.Order{
				ID:       orderID,
				UserID:   userID,
				UserName: userName,
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

		receipt := fmt.Sprintf("🛍️ **Ваш заказ успешно оформлен!**\n\n"+
			"• **Номер заказа:** `%s`\n"+
			"• **Сумма к оплате:** `7 480 ₽`\n\n"+
			"**Состав заказа:**\n"+
			"• Кофемашина Barista: 1 шт.\n"+
			"• Наушники Pro ANC: 1 шт.\n\n"+
			"Спасибо за покупку! Наш менеджер уже готовит заказ к отправке.",
			orderID)

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		formatted := hood.Format(userID, receipt, "webapp_security")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// Handler for viewing orders & bookings in chat
	showOrdersHandler := func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		userID := int64(0)
		userName := "Пользователь"
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
			if sender.FirstName != "" {
				userName = sender.FirstName
			}
		} else if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
			if cb.User.FirstName != "" {
				userName = cb.User.FirstName
			}
		}

		var orders []store.Order
		if appStore != nil && userID != 0 {
			orders = appStore.ListOrdersByUser(userID)
		}

		if len(orders) == 0 {
			text := fmt.Sprintf("📦 **Мои заказы и записи**\n\n"+
				"%s, у вас пока нет оформленных заказов или бронирований.\n\n"+
				"Вы можете выбрать товары в магазине или записаться на консультацию:", userName)

			kb := maxbot.NewInlineKeyboard().
				Row(
					maxbot.NewCallbackButton("🛍️ В магазин", "scen:ecom").WithIntent(maxbot.IntentPrimary),
					maxbot.NewCallbackButton("📅 На запись", "scen:booking").WithIntent(maxbot.IntentPrimary),
				).
				Row(
					maxbot.NewCallbackButton("📱 Открыть витрину Mini App", "scen:webapp").WithIntent(maxbot.IntentPositive),
				).
				Row(
					maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"),
				)

			formatted := hood.Format(userID, text, "orders_history")
			_, err := c.EditOrReplyKeyboard(formatted, kb)
			return err
		}

		var sb strings.Builder
		sb.WriteString("📦 **Мои заказы и бронирования**\n\n")
		sb.WriteString(fmt.Sprintf("Всего записей: **%d**\n\n", len(orders)))

		limit := 5
		if len(orders) < limit {
			limit = len(orders)
		}

		for i := 0; i < limit; i++ {
			o := orders[i]
			isBooking := o.DeliveryType == "booking"
			dateStr := o.CreatedAt.Format("02.01.2006 15:04")

			statusLabel := "Принят 🟢"
			switch o.Status {
			case "paid", "completed":
				statusLabel = "Оплачено ✅"
			case "processing":
				statusLabel = "В обработке ⏳"
			case "cancelled":
				statusLabel = "Отменен ❌"
			}

			if isBooking {
				srvName := "Консультация"
				if len(o.Items) > 0 && o.Items[0].Title != "" {
					srvName = o.Items[0].Title
				}
				priceStr := "Бесплатно"
				if o.Total > 0 {
					priceStr = fmt.Sprintf("%d ₽", o.Total)
				}
				sb.WriteString(fmt.Sprintf("📅 **Бронь #%s** [%s]\n", o.ID, statusLabel))
				sb.WriteString(fmt.Sprintf("• **Услуга:** %s\n", srvName))
				if o.Address != "" {
					sb.WriteString(fmt.Sprintf("• **Детали:** %s\n", o.Address))
				}
				sb.WriteString(fmt.Sprintf("• **Стоимость:** %s\n", priceStr))
				sb.WriteString(fmt.Sprintf("• **Дата:** %s\n\n", dateStr))
			} else {
				totalStr := fmt.Sprintf("%d ₽", o.Total)
				sb.WriteString(fmt.Sprintf("🛍️ **Заказ #%s** [%s]\n", o.ID, statusLabel))
				if len(o.Items) > 0 {
					itemNames := make([]string, 0, len(o.Items))
					for _, it := range o.Items {
						if it.Quantity > 1 {
							itemNames = append(itemNames, fmt.Sprintf("%s (%d шт.)", it.Title, it.Quantity))
						} else {
							itemNames = append(itemNames, it.Title)
						}
					}
					sb.WriteString(fmt.Sprintf("• **Товары:** %s\n", strings.Join(itemNames, ", ")))
				}
				sb.WriteString(fmt.Sprintf("• **Сумма:** %s\n", totalStr))
				sb.WriteString(fmt.Sprintf("• **Дата:** %s\n\n", dateStr))
			}
		}

		if len(orders) > limit {
			sb.WriteString(fmt.Sprintf("💡 *Показаны последние %d из %d записей. Полную историю можно просмотреть в Mini App.*\n", limit, len(orders)))
		}

		kb := maxbot.NewInlineKeyboard().
			Row(
				maxbot.NewCallbackButton("🔄 Обновить", "nav:orders"),
				maxbot.NewCallbackButton("📱 Открыть в Mini App", "scen:webapp").WithIntent(maxbot.IntentPositive),
			).
			Row(
				maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"),
			)

		formatted := hood.Format(userID, sb.String(), "orders_history")
		_, err := c.EditOrReplyKeyboard(formatted, kb)
		return err
	}

	b.OnCallbackPayload("nav:orders", showOrdersHandler)
	b.OnCommand("orders", showOrdersHandler)
	b.OnCommand("myorders", showOrdersHandler)
}
