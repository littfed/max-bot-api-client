package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
	"github.com/littfed/max-bot-api-client/pkg/payments"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// Product represents a demo catalog item.
type Product struct {
	ID    string
	Name  string
	Price float64
	Icon  string
	Desc  string
}

var demoCatalog = []Product{
	{ID: "prod_coffee", Name: "Премиум Кофемашина MAX Barista", Price: 4990.00, Icon: "☕", Desc: "15 бар, встроенный капучинатор, быстрый нагрев"},
	{ID: "prod_headphones", Name: "Беспроводные наушники Pro ANC", Price: 2490.00, Icon: "🎧", Desc: "Активное шумоподавление, 36ч работы, Bluetooth 5.4"},
	{ID: "prod_powerbank", Name: "Зарядная станция 20000mAh", Price: 1290.00, Icon: "⚡", Desc: "Быстрая зарядка 65W, 3 порта Type-C/USB-A"},
}

func findProduct(id string) *Product {
	for i := range demoCatalog {
		if demoCatalog[i].ID == id {
			return &demoCatalog[i]
		}
	}
	return nil
}

// RegisterEcomScenario registers the complete E-Commerce & SBP checkout wizard and technical cards.
func RegisterEcomScenario(b *bot.Bot, hood *HoodEngine, paymentRouter *payments.Router, appStore ...*store.Store) {
	var st *store.Store
	if len(appStore) > 0 {
		st = appStore[0]
	}

	// 1. Register Under-The-Hood technical cards
	hood.Register("ecom_catalog", TechCard{
		Title:       "Каталог товаров и FSM Сессии",
		Endpoint:    "/messages",
		Method:      "POST",
		FSMState:    "ECOM_STEP_CATALOG",
		Description: "Каталог выводится в виде инлайн-кнопок. Сессия пользователя изолируется SessionKey{UserID, ChatID}. При клике FSM сохраняет ID выбранного товара.",
		DocsLink:    "https://dev.max.ru/docs/buttons",
	})

	hood.Register("ecom_sbp", TechCard{
		Title:       "СБП НСПК Динамические платежные ссылки",
		Endpoint:    "qr.nspk.ru",
		Method:      "DEEP-LINK",
		FSMState:    "ECOM_STEP_CHECKOUT",
		Description: "Генератор pkg/payments.GenerateSBPLink создает ссылку вида https://qr.nspk.ru/{qr_id}?type=02&sum={kopecks}&cur=RUB. Кнопка получает intent: positive (зеленый акцент).",
		DocsLink:    "https://sbp.nspk.ru/business/api/",
	})

	hood.Register("ecom_webhook", TechCard{
		Title:       "Валидация вебхука эквайринга (SHA-256)",
		Endpoint:    "/webhook/payments/tbank",
		Method:      "POST",
		FSMState:    "ECOM_STEP_PAID",
		Description: "При получении уведомления от шлюза парсер TBankGateway сортирует поля по алфавиту, конкатенирует с паролем мерчанта и сверяет SHA-256 хэш через subtle.ConstantTimeCompare.",
		DocsLink:    "https://www.tbank.ru/kassa/dev/payments/",
	})

	// 2. Setup E-Commerce FSM Wizard
	wizard := fsm.NewWizard("ecom_wizard",
		fsm.WithTimeout(15*time.Minute),
		fsm.WithOnCancel(func(c *bot.Context) error {
			_ = c.AnswerNotification("Заказ отменен")
			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu").WithIntent(maxbot.IntentPrimary))
			_, err := c.EditOrReplyKeyboard("❌ Оформление заказа отменено. Вы можете начать заново в любой момент.", kb)
			return err
		}),
		fsm.WithOnTimeout(func(c *bot.Context) error {
			_, err := c.Reply("⌛ Время оформления заказа истекло (таймаут 15 мин).")
			return err
		}),
	)

	handleDelivery := func(c *bot.Context, deliveryText string) error {
		if deliveryText == "" {
			deliveryText = "г. Москва, ул. Ленина, д. 1 (по умолчанию)"
		}

		fsmCtx := fsm.FromContext(c)
		_ = fsmCtx.Set("delivery", deliveryText)

		data, _ := fsmCtx.GetData()
		prodID, _ := data["prod_id"].(string)
		prod := findProduct(prodID)
		if prod == nil {
			prod = &demoCatalog[0]
		}

		orderID := fmt.Sprintf("ORD-%d", time.Now().Unix()%100000)
		_ = fsmCtx.Set("order_id", orderID)

		// Generate real SBP dynamic link
		sbpLink := payments.GenerateSBPLink(orderID, payments.SBPOptions{
			Type:   payments.SBPTypeDynamic,
			Amount: prod.Price,
		})

		userID := int64(0)
		chatID := int64(0)
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
		}
		if rec, err := c.Recipient(); err == nil && rec != nil {
			chatID = rec.ID()
		}

		// Sandbox simulator callback payload: ecom:sandbox:<orderID>:<price>:<chatID>:<userID>
		sandboxPayload := fmt.Sprintf("ecom:sandbox:%s:%.0f:%d:%d", orderID, prod.Price, chatID, userID)

		kb := maxbot.NewInlineKeyboard().
			Row(payments.PaymentButton(fmt.Sprintf("💳 Оплатить через СБП (%.0f ₽)", prod.Price), sbpLink)).
			Row(maxbot.NewIntentCallbackButton("🧪 Симулировать оплату (Sandbox)", sandboxPayload, maxbot.IntentPositive)).
			Row(
				maxbot.NewCallbackButton("↩️ Назад", "wizard:back"),
				maxbot.NewCallbackButton("❌ Отмена", "wizard:cancel").WithIntent(maxbot.IntentNegative),
			)

		summary := fmt.Sprintf("📦 **Подтверждение заказа #%s**\n\n"+
			"• **Товар:** %s %s\n"+
			"• **Стоимость:** %.0f ₽\n"+
			"• **Адрес доставки:** %s\n\n"+
			"Выберите способ оплаты ниже для завершения оформления заказа:",
			orderID, prod.Icon, prod.Name, prod.Price, deliveryText)

		formatted := hood.Format(userID, summary, "ecom_sbp")
		_, err := c.EditOrReplyKeyboard(formatted, kb)
		if err != nil {
			return err
		}
		return wizard.Next(c)
	}

	// Step 1: Delivery details input
	wizard.Step("ECOM_STEP_DELIVERY", func(c *bot.Context) error {
		return handleDelivery(c, strings.TrimSpace(c.Text()))
	})

	// Step 2: Checkout waiting state
	wizard.Step("ECOM_STEP_CHECKOUT", func(c *bot.Context) error {
		_, err := c.Reply("Ожидаем подтверждения оплаты. Нажмите кнопку «Оплатить через СБП» или «Симулировать оплату» выше.")
		return err
	})

	wizard.Register(b.Router())

	// 3. Launch Catalog Route (scen:ecom)
	b.OnCallbackPayload("scen:ecom", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		kb := maxbot.NewInlineKeyboard()
		for _, p := range demoCatalog {
			btnText := fmt.Sprintf("%s %s — %.0f ₽", p.Icon, p.Name, p.Price)
			kb.Row(maxbot.NewCallbackButton(btnText, "ecom:choose:"+p.ID))
		}
		kb.Row(maxbot.NewCallbackButton("◀️ Назад в меню", "nav:menu"))

		userID := int64(0)
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
		}

		text := "🛍️ **Каталог товаров MAX Shop**\n\n" +
			"Выберите интересующий товар для быстрого оформления заказа и доставки:"

		formatted := hood.Format(userID, text, "ecom_catalog")
		_, err := c.EditOrReplyKeyboard(formatted, kb)
		return err
	})

	// Product selection click
	b.OnCallbackPrefix("ecom:choose:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}
		prodID := strings.TrimPrefix(cb.Payload, "ecom:choose:")
		prod := findProduct(prodID)
		if prod == nil {
			prod = &demoCatalog[0]
		}

		_ = wizard.Start(c)
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.Set("prod_id", prod.ID)
		}

		_ = c.AnswerNotification(fmt.Sprintf("Выбрано: %s", prod.Name))

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("🏢 Заберу самовывозом", "ecom:pickup").WithIntent(maxbot.IntentPrimary)).
			Row(maxbot.NewCallbackButton("❌ Отмена", "wizard:cancel").WithIntent(maxbot.IntentNegative))

		msg := fmt.Sprintf("Вы выбрали: **%s %s** (%.0f ₽)\n_%s_\n\n"+
			"📍 **Шаг 1 из 2:** Введите адрес доставки ответным сообщением (или нажмите «Заберу самовывозом»):",
			prod.Icon, prod.Name, prod.Price, prod.Desc)

		_, err := c.EditOrReplyKeyboard(msg, kb)
		return err
	})

	// Pickup shortcut
	b.OnCallbackPayload("ecom:pickup", func(c *bot.Context) error {
		fsmCtx := fsm.FromContext(c)
		if fsmCtx == nil || fsmCtx.State() != "ECOM_STEP_DELIVERY" {
			return c.AnswerCallbackAlert("Это действие уже выполнено или не актуально")
		}
		_ = c.AnswerNotification("Самовывоз выбран")
		return handleDelivery(c, "Пункт выдачи MAX Express (Самовывоз)")
	})

	// Sandbox Payment Simulation Handler
	b.OnCallbackPrefix("ecom:sandbox:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}
		fsmCtx := fsm.FromContext(c)
		if fsmCtx == nil || fsmCtx.State() != "ECOM_STEP_CHECKOUT" {
			return c.AnswerCallbackAlert("Этот заказ уже оплачен или действие не актуально!")
		}
		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 6 {
			return c.AnswerNotification("Ошибка формата симулятора")
		}

		orderID := parts[2]
		amount, _ := strconv.ParseFloat(parts[3], 64)
		userID, _ := strconv.ParseInt(parts[5], 10, 64)

		userName := "Клиент"
		if cb := c.Callback(); cb != nil && cb.User != nil {
			if cb.User.FirstName != "" {
				userName = cb.User.FirstName
			}
		}

		// Store order in persistence layer if store is configured
		if st != nil {
			deliveryAddr := "г. Москва (по умолчанию)"
			if fsmCtx := fsm.FromContext(c); fsmCtx != nil {
				if d, _ := fsmCtx.GetData(); d != nil {
					if addr, ok := d["delivery"].(string); ok && addr != "" {
						deliveryAddr = addr
					}
				}
			}
			prodName := "Товар MAX Shop"
			prodID := ""
			for _, p := range demoCatalog {
				if p.Price == amount {
					prodName = p.Name
					prodID = p.ID
					break
				}
			}
			_ = st.CreateOrder(store.Order{
				ID:           orderID,
				UserID:       userID,
				UserName:     userName,
				Items:        []store.OrderItem{{ID: prodID, Title: prodName, Quantity: 1, Price: int(amount)}},
				Total:        int(amount),
				DeliveryType: "delivery",
				Address:      deliveryAddr,
				Status:       "paid",
				Verified:     true,
				CreatedAt:    time.Now(),
			})
		}

		// Anti-Ghost-Click: clear the payment keyboard on the previous message
		_, _ = c.ClearKeyboard()

		_ = c.AnswerCallback(&maxbot.AnswerCallbackRequest{
			Notification: "💳 Оплата успешно подтверждена!",
		})

		// Reset FSM session upon completion
		if fsmCtx := fsm.FromContext(c); fsmCtx != nil {
			_ = fsmCtx.Reset()
		}

		receiptText := fmt.Sprintf("🎉 **Оплата успешно подтверждена!**\n\n"+
			"• **Номер заказа:** `%s`\n"+
			"• **Сумма:** `%.2f ₽`\n"+
			"• **Статус:** Оплачено ✅\n"+
			"• **Дата и время:** %s\n\n"+
			"Товар передан в службу сборки и доставки! Спасибо за ваш заказ. Мы свяжемся с вами для уточнения деталей.",
			orderID, amount, time.Now().Format("02.01.2006 15:04:05"))

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu").WithIntent(maxbot.IntentPrimary))

		formatted := hood.Format(userID, receiptText, "ecom_webhook")
		_, err := c.ReplyKeyboard(formatted, kb)

		// Fire paymentRouter success hooks
		if paymentRouter != nil {
			_ = paymentRouter.OnSuccess(nil)
		}

		return err
	})
}
