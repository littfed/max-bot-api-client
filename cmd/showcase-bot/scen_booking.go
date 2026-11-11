package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/fsm"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// Service represents an appointment service option.
type Service struct {
	ID       string
	Name     string
	Price    float64
	Duration string
	Icon     string
}

var demoServices = []Service{
	{ID: "srv_cut", Name: "Мужская стрижка & Стайлинг", Price: 1800, Duration: "45 мин", Icon: "💇‍♂️"},
	{ID: "srv_spa", Name: "Спа-уход & Массаж лица", Price: 2500, Duration: "60 мин", Icon: "💆"},
	{ID: "srv_coffee", Name: "Дегустация specialty кофе", Price: 0, Duration: "30 мин", Icon: "☕"},
}

var demoTimeSlots = []string{
	"Завтра, 11:00",
	"Завтра, 14:30",
	"Завтра, 18:00",
	"Послезавтра, 12:00",
	"Послезавтра, 16:30",
}

func findService(id string) *Service {
	for i := range demoServices {
		if demoServices[i].ID == id {
			return &demoServices[i]
		}
	}
	return nil
}

// RegisterBookingScenario registers the service booking flow, contact request handling, and technical cards.
func RegisterBookingScenario(b *bot.Bot, hood *HoodEngine) {
	// 1. Technical cards
	hood.Register("booking_slots", TechCard{
		Title:       "Сетка слотов и выбор времени",
		Endpoint:    "/messages",
		Method:      "POST",
		FSMState:    "BOOKING_STEP_SLOT",
		Description: "Инлайн-клавиатура с сеткой слотов формируется динамически на основе доступных дат. Callback payload содержит ID услуги и выбранный слот времени.",
		DocsLink:    "https://dev.max.ru/docs/buttons",
	})

	hood.Register("booking_contact", TechCard{
		Title:       "Системный запрос контакта (request_contact)",
		Endpoint:    "/messages",
		Method:      "POST",
		FSMState:    "BOOKING_STEP_PHONE",
		Description: "Кнопка с type: request_contact запрашивает у клиента подтвержденный номер телефона из профиля MAX. Ответ приходит в поле контактных данных сообщения.",
		DocsLink:    "https://dev.max.ru/docs/buttons",
	})

	hood.Register("booking_confirm", TechCard{
		Title:       "Подтверждение записи и Уведомления",
		Endpoint:    "/messages",
		Method:      "POST",
		FSMState:    "BOOKING_STEP_CONFIRMED",
		Description: "Данные записи сохраняются в сессию FSM. Клиент получает форматированную карточку бронирования с возможностью отмены или изменения даты.",
		DocsLink:    "https://dev.max.ru/docs/formatting",
	})

	// 2. FSM Wizard for Booking
	wizard := fsm.NewWizard("booking_wizard",
		fsm.WithTimeout(15*time.Minute),
		fsm.WithOnCancel(func(c *bot.Context) error {
			_ = c.AnswerNotification("Запись отменена")
			kb := maxbot.NewInlineKeyboard().
				Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))
			_, err := c.ReplyKeyboard("❌ Запись на услугу отменена.", kb)
			return err
		}),
	)

	// Step: Phone input
	wizard.Step("BOOKING_STEP_PHONE", func(c *bot.Context) error {
		phone := strings.TrimSpace(c.Text())
		if phone == "" {
			phone = "+7 (999) 000-11-22 (демо-контакт)"
		}

		fsmCtx := fsm.FromContext(c)
		_ = fsmCtx.Set("phone", phone)

		data, _ := fsmCtx.GetData()
		srvID, _ := data["service_id"].(string)
		slot, _ := data["slot"].(string)

		srv := findService(srvID)
		if srv == nil {
			srv = &demoServices[0]
		}

		bookingID := fmt.Sprintf("BOOK-%d", time.Now().Unix()%100000)

		// Reset state upon completion
		_ = fsmCtx.Reset()

		userID := int64(0)
		if sender := c.Sender(); sender != nil {
			userID = sender.UserID
		}

		priceStr := fmt.Sprintf("%.0f ₽", srv.Price)
		if srv.Price == 0 {
			priceStr = "Бесплатно"
		}

		summary := fmt.Sprintf("✅ **Запись успешно подтверждена!**\n\n"+
			"• **Номер брони:** `#%s`\n"+
			"• **Услуга:** %s %s\n"+
			"• **Длительность:** %s\n"+
			"• **Дата и время:** 🗓 %s\n"+
			"• **Телефон клиента:** `%s`\n"+
			"• **Стоимость:** %s\n\n"+
			"🔔 Мы отправим напоминание за 2 часа до визита. До встречи!",
			bookingID, srv.Icon, srv.Name, srv.Duration, slot, phone, priceStr)

		kb := maxbot.NewInlineKeyboard().
			Row(hood.Button("booking_confirm")).
			Row(maxbot.NewCallbackButton("◀️ В главное меню", "nav:menu"))

		formatted := hood.Format(userID, summary, "booking_confirm")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	wizard.Register(b.Router())

	// 3. Launch Booking (scen:booking)
	b.OnCallbackPayload("scen:booking", func(c *bot.Context) error {
		_ = c.AnswerCallback(nil)

		kb := maxbot.NewInlineKeyboard()
		for _, s := range demoServices {
			priceStr := fmt.Sprintf("%.0f ₽", s.Price)
			if s.Price == 0 {
				priceStr = "Бесплатно"
			}
			btnText := fmt.Sprintf("%s %s (%s)", s.Icon, s.Name, priceStr)
			kb.Row(maxbot.NewCallbackButton(btnText, "booking:srv:"+s.ID))
		}
		kb.Row(hood.Button("booking_slots"))
		kb.Row(maxbot.NewCallbackButton("◀️ Назад в меню", "nav:menu"))

		userID := int64(0)
		if cb := c.Callback(); cb != nil && cb.User != nil {
			userID = cb.User.UserID
		}

		text := "📅 **Онлайн-запись на услуги**\n\n" +
			"Этот сценарий демонстрирует интеграцию календаря слотов, работу с нативным запросом контакта (`request_contact`) и подтверждение бронирования.\n\n" +
			"Выберите желаемую услугу:"

		formatted := hood.Format(userID, text, "booking_slots")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// Service selected -> Show time slots
	b.OnCallbackPrefix("booking:srv:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}
		srvID := strings.TrimPrefix(cb.Payload, "booking:srv:")
		srv := findService(srvID)
		if srv == nil {
			srv = &demoServices[0]
		}

		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.Set("service_id", srv.ID)
		}

		_ = c.AnswerNotification(fmt.Sprintf("Выбрано: %s", srv.Name))

		kb := maxbot.NewInlineKeyboard()
		for _, slot := range demoTimeSlots {
			payload := fmt.Sprintf("booking:slot:%s:%s", srv.ID, slot)
			kb.Row(maxbot.NewCallbackButton("🕒 "+slot, payload))
		}
		kb.Row(
			maxbot.NewCallbackButton("↩️ К списку услуг", "scen:booking"),
			maxbot.NewCallbackButton("❌ Отмена", "wizard:cancel"),
		)

		text := fmt.Sprintf("Вы выбрали: **%s %s** (%s)\n\n"+
			"🗓 **Шаг 1 из 2:** Выберите удобный день и время визита:",
			srv.Icon, srv.Name, srv.Duration)

		_, err := c.ReplyKeyboard(text, kb)
		return err
	})

	// Slot selected -> Request phone contact
	b.OnCallbackPrefix("booking:slot:", func(c *bot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}
		parts := strings.Split(cb.Payload, ":")
		if len(parts) < 4 {
			return c.AnswerNotification("Ошибка слота")
		}

		srvID := parts[2]
		slot := parts[3]

		_ = wizard.Start(c)
		fsmCtx := fsm.FromContext(c)
		if fsmCtx != nil {
			_ = fsmCtx.Set("service_id", srvID)
			_ = fsmCtx.Set("slot", slot)
		}

		_ = c.AnswerNotification("Слот забронирован")

		kb := maxbot.NewInlineKeyboard().
			Row(maxbot.NewContactButton("📱 Поделиться номером телефона")).
			Row(maxbot.NewCallbackButton("⚡ Использовать +7 (999) 123-45-67", "booking:quick_phone")).
			Row(hood.Button("booking_contact")).
			Row(maxbot.NewCallbackButton("❌ Отмена", "wizard:cancel"))

		text := fmt.Sprintf("🗓 Выбран слот: **%s**\n\n"+
			"📱 **Шаг 2 из 2:** Пожалуйста, отправьте контактный номер телефона для подтверждения записи.\n"+
			"Вы можете нажать кнопку ниже или просто написать номер в чат:", slot)

		userID := int64(0)
		if cb.User != nil {
			userID = cb.User.UserID
		}

		formatted := hood.Format(userID, text, "booking_contact")
		_, err := c.ReplyKeyboard(formatted, kb)
		return err
	})

	// Quick demo phone shortcut
	b.OnCallbackPayload("booking:quick_phone", func(c *bot.Context) error {
		_ = c.AnswerNotification("Номер принят")
		c.Update.Message = &maxbot.Message{
			Body: maxbot.MessageBody{Text: "+7 (999) 123-45-67"},
		}
		return b.Router().Handle(c)
	})
}
