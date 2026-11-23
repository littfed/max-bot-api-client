package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// PaymentSimulationResult represents the outcome of a demo payment simulation.
type PaymentSimulationResult struct {
	OrderID           string `json:"order_id"`
	Status            string `json:"status"` // "paid" or "abandoned"
	Message           string `json:"message"`
	DeliveryPayload   string `json:"delivery_payload,omitempty"`
	ReminderScheduled bool   `json:"reminder_scheduled,omitempty"`
}

// Order represents a shop order.
type Order struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	ProductID string    `json:"product_id"`
	Status    string    `json:"status"` // "pending", "paid", "abandoned"
	QRCode    string    `json:"qr_code"`
	CreatedAt time.Time `json:"created_at"`
}

// Module implements the BotModule interface for the Shop mode.
type Module struct {
	mu       sync.RWMutex
	products map[string]store.Product
	orders   map[string]*Order
}

// New creates a new Shop module instance.
func New(st any) *Module {
	m := &Module{
		products: make(map[string]store.Product),
		orders:   make(map[string]*Order),
	}
	m.seedDefaultProducts()
	return m
}

func (m *Module) seedDefaultProducts() {
	defaults := []store.Product{
		{
			ID:             "prod_guide",
			Title:          "📘 PDF Гайд по разработке ботов",
			Description:    "Полное практическое руководство по архитектуре и монетизации ботов в MAX и Telegram.",
			Price:          490,
			Category:       "digital",
			ItemType:       "digital",
			ContentPayload: "https://combot.littfed.ru/downloads/bot-development-guide.pdf",
			IsActive:       true,
		},
		{
			ID:             "prod_prompts",
			Title:          "⚡ Промпт-пак для ChatGPT и Claude",
			Description:    "100+ готовых системных промптов для маркетинга, продуктовой аналитики и автоматизации.",
			Price:          790,
			Category:       "digital",
			ItemType:       "digital",
			ContentPayload: "https://combot.littfed.ru/downloads/ai-prompt-pack.zip",
			IsActive:       true,
		},
		{
			ID:             "prod_sub_pro",
			Title:          "💎 Подписка MAX Pro (1 месяц)",
			Description:    "Безлимитный доступ ко всем режимам Bot Hub, увеличенные квоты и приоритетный AI.",
			Price:          1490,
			Category:       "subscription",
			ItemType:       "subscription",
			ContentPayload: "https://t.me/+bothub_pro_private_channel",
			IsActive:       true,
		},
		{
			ID:             "prod_sub_vip",
			Title:          "👑 VIP Клуб разработчиков (3 месяца)",
			Description:    "Закрытое сообщество: доступ к заказам, код-ревью архитектора и закрытые вебинары.",
			Price:          3990,
			Category:       "subscription",
			ItemType:       "subscription",
			ContentPayload: "https://t.me/+bothub_vip_club_invite",
			IsActive:       true,
		},
	}

	for _, p := range defaults {
		m.products[p.ID] = p
	}
}

// ID returns the unique identifier for the Shop mode.
func (m *Module) ID() string {
	return "shop"
}

// Title returns the human-readable display title.
func (m *Module) Title() string {
	return "Магазин"
}

// GetCatalog returns all active products and subscriptions.
func (m *Module) GetCatalog(ctx context.Context) ([]store.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]store.Product, 0, len(m.products))
	for _, p := range m.products {
		if p.IsActive {
			items = append(items, p)
		}
	}
	return items, nil
}

// GetProduct retrieves a product by its ID.
func (m *Module) GetProduct(ctx context.Context, id string) (*store.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.products[id]
	if !ok {
		return nil, errors.New("product not found")
	}
	return &p, nil
}

// CreateOrder registers a new order in pending status.
func (m *Module) CreateOrder(ctx context.Context, userID int64, productID string) (*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.products[productID]
	if !ok {
		return nil, errors.New("product not found")
	}

	orderID := fmt.Sprintf("ord_%s_%d", productID, time.Now().UnixNano())
	qrPayload := fmt.Sprintf("https://combot.littfed.ru/pay/demo?order=%s&amount=%d", orderID, p.Price)

	ord := &Order{
		ID:        orderID,
		UserID:    userID,
		ProductID: productID,
		Status:    "pending",
		QRCode:    qrPayload,
		CreatedAt: time.Now(),
	}

	m.orders[orderID] = ord
	return ord, nil
}

// GetOrder fetches an existing order by ID.
func (m *Module) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ord, ok := m.orders[orderID]
	if !ok {
		return nil, errors.New("order not found")
	}
	return ord, nil
}

// SimulatePayment executes the demo checkout workflow:
// - "paid": updates order to paid, delivers instant digital content or channel invite.
// - "abandoned": updates order to abandoned, schedules reminder event.
func (m *Module) SimulatePayment(ctx context.Context, orderID string, action string) (*PaymentSimulationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ord, exists := m.orders[orderID]
	if !exists {
		// Create on the fly if simulating an arbitrary test order
		ord = &Order{
			ID:        orderID,
			ProductID: "prod_guide",
			Status:    "pending",
			CreatedAt: time.Now(),
		}
		m.orders[orderID] = ord
	}

	prod, prodExists := m.products[ord.ProductID]
	payload := ""
	if prodExists {
		payload = prod.ContentPayload
	} else {
		payload = "https://combot.littfed.ru/downloads/demo-content.pdf"
	}

	switch strings.ToLower(action) {
	case "paid":
		ord.Status = "paid"
		return &PaymentSimulationResult{
			OrderID:           orderID,
			Status:            "paid",
			Message:           "🎉 Оплата успешно подтверждена! Спасибо за заказ.",
			DeliveryPayload:   payload,
			ReminderScheduled: false,
		}, nil

	case "abandoned":
		ord.Status = "abandoned"
		return &PaymentSimulationResult{
			OrderID:           orderID,
			Status:            "abandoned",
			Message:           "⏳ Заказ сохранен как неоплаченный. Мы напомним вам о нем позже!",
			DeliveryPayload:   "",
			ReminderScheduled: true,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported payment action: %s", action)
	}
}

// HandleMessage handles text messages sent while in Shop mode.
func (m *Module) HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	catalog, err := m.GetCatalog(ctx)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString("🛒 **Каталог цифровых товаров и подписок Bot Hub**\n\n")
	sb.WriteString("Выберите товар или услугу для мгновенного оформления:\n\n")

	var rows [][]core.ResponseButton
	for _, item := range catalog {
		sb.WriteString(fmt.Sprintf("• **%s** — %d ₽\n  _%s_\n\n", item.Title, item.Price, item.Description))
		btnText := fmt.Sprintf("Купить %s (%d ₽)", item.Title, item.Price)
		if len(btnText) > 35 {
			btnText = fmt.Sprintf("Купить за %d ₽", item.Price)
		}
		rows = append(rows, []core.ResponseButton{
			{
				Text: btnText,
				Data: fmt.Sprintf("shop:buy:%s", item.ID),
			},
		})
	}

	return &core.ModuleResult{
		Text:    sb.String(),
		Buttons: rows,
		Handled: true,
	}, nil
}

// HandleCallback processes inline keyboard button clicks in Shop mode.
func (m *Module) HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	data := cb.Data

	if data == "shop:catalog" {
		return m.HandleMessage(ctx, u, state, &core.IncomingMessage{Text: "/catalog"})
	}

	if strings.HasPrefix(data, "shop:buy:") {
		prodID := strings.TrimPrefix(data, "shop:buy:")
		p, err := m.GetProduct(ctx, prodID)
		if err != nil {
			return &core.ModuleResult{
				Text:    "❌ Товар не найден или недоступен.",
				Handled: true,
			}, nil
		}

		ord, err := m.CreateOrder(ctx, u.ID, prodID)
		if err != nil {
			return nil, err
		}

		text := fmt.Sprintf(
			"📦 **Оформление заказа #%s**\n\n"+
				"Товар: **%s**\n"+
				"Сумма к оплате: **%d ₽**\n\n"+
				"📱 **Демо-оплата:**\n"+
				"Отсканируйте QR-код или нажмите тестовую кнопку симуляции:\n`%s`\n\n"+
				"Нажмите кнопку подтверждения оплаты ниже:",
			ord.ID, p.Title, p.Price, ord.QRCode,
		)

		buttons := [][]core.ResponseButton{
			{
				{Text: "✅ Я оплатил", Data: fmt.Sprintf("shop:pay:%s:paid", ord.ID)},
				{Text: "❌ Я не оплатил", Data: fmt.Sprintf("shop:pay:%s:abandoned", ord.ID)},
			},
			{
				{Text: "« Назад в каталог", Data: "shop:catalog"},
			},
		}

		statePayload, _ := json.Marshal(map[string]any{"current_order_id": ord.ID})
		return &core.ModuleResult{
			Text:       text,
			Buttons:    buttons,
			NewStep:    "awaiting_payment",
			NewPayload: statePayload,
			Handled:    true,
		}, nil
	}

	if strings.HasPrefix(data, "shop:pay:") {
		parts := strings.Split(data, ":")
		if len(parts) < 4 {
			return &core.ModuleResult{Text: "Некорректные параметры оплаты.", Handled: true}, nil
		}
		orderID := parts[2]
		action := parts[3]

		res, err := m.SimulatePayment(ctx, orderID, action)
		if err != nil {
			return nil, err
		}

		var text string
		if res.Status == "paid" {
			text = fmt.Sprintf(
				"%s\n\nВаша ссылка для получения:\n🔗 %s",
				res.Message, res.DeliveryPayload,
			)
		} else {
			text = fmt.Sprintf("%s", res.Message)
		}

		buttons := [][]core.ResponseButton{
			{
				{Text: "« Вернуться в каталог", Data: "shop:catalog"},
			},
		}

		return &core.ModuleResult{
			Text:       text,
			Buttons:    buttons,
			ResetState: true,
			Handled:    true,
		}, nil
	}

	return &core.ModuleResult{
		Text:    "Неизвестная команда магазина.",
		Handled: false,
	}, nil
}
