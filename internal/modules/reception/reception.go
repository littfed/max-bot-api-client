package reception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// EventRegistrationResult represents the generated ticket registration.
type EventRegistrationResult struct {
	RegistrationID string `json:"registration_id"`
	EventID        string `json:"event_id"`
	UserID         int64  `json:"user_id"`
	TicketQR       string `json:"ticket_qr"`
}

// Module implements the BotModule interface for the Reception mode.
type Module struct {
	lockManager   *core.LockManager
	mu            sync.RWMutex
	services      map[string]store.Service
	bookedSlots   map[string]store.Booking
	events        map[string]*store.Event
	submissionsMu sync.Mutex
	submissions   []store.OnboardingSubmission
}

// New creates a new Reception module instance.
func New(st any, lockManager *core.LockManager) *Module {
	if lockManager == nil {
		lockManager = core.NewLockManager(nil)
	}

	m := &Module{
		lockManager: lockManager,
		services:    make(map[string]store.Service),
		bookedSlots: make(map[string]store.Booking),
		events:      make(map[string]*store.Event),
		submissions: make([]store.OnboardingSubmission, 0),
	}
	m.seedDefaults()
	return m
}

func (m *Module) seedDefaults() {
	m.services["srv_consult"] = store.Service{
		ID:              "srv_consult",
		Title:           "Консультация архитектора",
		DurationMinutes: 30,
		Price:           0,
		Description:     "Индивидуальный разбор архитектуры вашего бота или Mini App.",
	}
	m.services["srv_audit"] = store.Service{
		ID:              "srv_audit",
		Title:           "Экспресс-аудит проекта",
		DurationMinutes: 60,
		Price:           5000,
		Description:     "Комплексный аудит безопасности, отказоустойчивости и юзабилити.",
	}

	tomorrow := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	inThreeDays := time.Now().Add(72 * time.Hour).Truncate(time.Hour)

	m.events["evt_launch"] = &store.Event{
		ID:          "evt_launch",
		Title:       "Презентация Bot Hub v2.0",
		EventTime:   tomorrow,
		TotalSeats:  50,
		BookedSeats: 10,
	}
	m.events["evt_webinar"] = &store.Event{
		ID:          "evt_webinar",
		Title:       "Воркшоп по разработке Telegram & MAX Mini Apps",
		EventTime:   inThreeDays,
		TotalSeats:  100,
		BookedSeats: 45,
	}
}

// ID returns the module identifier.
func (m *Module) ID() string {
	return "reception"
}

// Title returns the human-readable display title.
func (m *Module) Title() string {
	return "Ресепшен"
}

// slotKey constructs the booking map key.
func slotKey(serviceID string, slotTime time.Time) string {
	return fmt.Sprintf("%s:%d", serviceID, slotTime.Unix())
}

// BookSlot reserves a time slot with distributed anti-overbooking check using LockManager.
func (m *Module) BookSlot(ctx context.Context, userID int64, serviceID string, slotTime time.Time) error {
	lockKey := fmt.Sprintf("lock:reception:slot:%s:%d", serviceID, slotTime.Unix())

	acquired, err := m.lockManager.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for slot: %w", err)
	}
	if !acquired {
		return errors.New("concurrent booking in progress: try again")
	}
	defer func() {
		_ = m.lockManager.Release(ctx, lockKey)
	}()

	m.mu.Lock()
	defer m.mu.Unlock()

	k := slotKey(serviceID, slotTime)
	if _, exists := m.bookedSlots[k]; exists {
		return errors.New("slot already booked: anti-overbooking triggered")
	}

	bookingID := fmt.Sprintf("bk_%s_%d_%d", serviceID, userID, time.Now().Unix())
	m.bookedSlots[k] = store.Booking{
		ID:        bookingID,
		UserID:    userID,
		ServiceID: serviceID,
		SlotTime:  slotTime,
		Status:    "confirmed",
		CreatedAt: time.Now(),
	}

	return nil
}

// RegisterEvent reserves a seat for an event and generates a QR ticket.
func (m *Module) RegisterEvent(ctx context.Context, userID int64, eventID string) (*EventRegistrationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ev, exists := m.events[eventID]
	if !exists {
		return nil, errors.New("event not found")
	}

	if ev.BookedSeats >= ev.TotalSeats {
		return nil, errors.New("event is fully booked")
	}

	ev.BookedSeats++
	regID := fmt.Sprintf("reg_%s_%d_%d", eventID, userID, time.Now().Unix())
	ticketQR := fmt.Sprintf("QR_TICKET:%s:USER_%d:%d", eventID, userID, time.Now().Unix())

	return &EventRegistrationResult{
		RegistrationID: regID,
		EventID:        eventID,
		UserID:         userID,
		TicketQR:       ticketQR,
	}, nil
}

// SubmitOnboarding stores completed questionnaire answers.
func (m *Module) SubmitOnboarding(ctx context.Context, userID int64, answers map[string]string) error {
	m.submissionsMu.Lock()
	defer m.submissionsMu.Unlock()

	b, _ := json.Marshal(answers)
	m.submissions = append(m.submissions, store.OnboardingSubmission{
		ID:        int64(len(m.submissions) + 1),
		UserID:    userID,
		Answers:   b,
		CreatedAt: time.Now(),
	})
	return nil
}

var (
	phoneRegex    = regexp.MustCompile(`^[+]?[0-9\s\-()]{7,20}$`)
	nonDigitRegex = regexp.MustCompile(`\D`)
)

// validateContact checks phone or telegram handle.
func validateContact(input string) bool {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "@") && len(trimmed) >= 3 {
		return true
	}
	digitsOnly := nonDigitRegex.ReplaceAllString(trimmed, "")
	if len(digitsOnly) >= 7 && phoneRegex.MatchString(trimmed) {
		return true
	}
	return false
}

// HandleMessage handles text messages, including the 4-step onboarding questionnaire.
func (m *Module) HandleMessage(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	text := strings.TrimSpace(msg.Text)
	step := state.CurrentStep

	var answers map[string]string
	if len(state.Payload) > 0 {
		_ = json.Unmarshal(state.Payload, &answers)
	}
	if answers == nil {
		answers = make(map[string]string)
	}

	switch step {
	case "onboarding_step_1":
		if len(text) < 2 {
			return &core.ModuleResult{
				Text:    "⚠️ Пожалуйста, укажите ваше имя корректно (не менее 2 символов):",
				NewStep: "onboarding_step_1",
				Handled: true,
			}, nil
		}
		answers["name"] = text
		p, _ := json.Marshal(answers)
		return &core.ModuleResult{
			Text: "📋 **Шаг 2 из 4:**\nКакая сфера вашего бизнеса или проект? (например: Ecommerce, EdTech, Услуги, HoReCa)",
			NewStep:    "onboarding_step_2",
			NewPayload: p,
			Handled:    true,
		}, nil

	case "onboarding_step_2":
		if len(text) < 2 {
			return &core.ModuleResult{
				Text:    "⚠️ Пожалуйста, укажите сферу деятельности (не менее 2 символов):",
				NewStep: "onboarding_step_2",
				Handled: true,
			}, nil
		}
		answers["business"] = text
		p, _ := json.Marshal(answers)
		return &core.ModuleResult{
			Text: "📋 **Шаг 3 из 4:**\nКакие задачи должен решать бот? (например: прием платежей, поддержка клиентов, запись на услуги)",
			NewStep:    "onboarding_step_3",
			NewPayload: p,
			Handled:    true,
		}, nil

	case "onboarding_step_3":
		if len(text) < 5 {
			return &core.ModuleResult{
				Text:    "⚠️ Пожалуйста, опишите задачи чуть подробнее (не менее 5 символов):",
				NewStep: "onboarding_step_3",
				Handled: true,
			}, nil
		}
		answers["tasks"] = text
		p, _ := json.Marshal(answers)
		return &core.ModuleResult{
			Text: "📋 **Шаг 4 из 4:**\nУкажите ваш контактный телефон (+7...) или ник в Telegram (@username) для обратной связи:",
			NewStep:    "onboarding_step_4",
			NewPayload: p,
			Handled:    true,
		}, nil

	case "onboarding_step_4":
		if !validateContact(text) {
			return &core.ModuleResult{
				Text: "⚠️ Некорректный формат контакта.\nПожалуйста, укажите номер телефона (например, +79991234567) или ник Telegram (@username):",
				NewStep:    "onboarding_step_4",
				NewPayload: state.Payload,
				Handled:    true,
			}, nil
		}
		answers["contact"] = text
		_ = m.SubmitOnboarding(ctx, u.ID, answers)

		return &core.ModuleResult{
			Text: "🎉 **Спасибо! Анкета успешно отправлена.**\n\nНаш ведущий архитектор свяжется с вами в течение 15 минут для обсуждения проекта.",
			Buttons: [][]core.ResponseButton{
				{{Text: "« Главное меню ресепшена", Data: "reception:menu"}},
			},
			ResetState: true,
			NewStep:    "onboarding_done",
			Handled:    true,
		}, nil
	}

	// Default main reception menu
	replyText := "🏨 **Стойка регистрации (Ресепшен) Bot Hub**\n\n" +
		"Выберите интересующий вас раздел:\n" +
		"• Запись на персональную консультацию или аудит\n" +
		"• Регистрация на мероприятия и воркшопы\n" +
		"• Экспресс-анкета для расчета стоимости разработки"

	buttons := [][]core.ResponseButton{
		{
			{Text: "📅 Запись на консультацию", Data: "reception:services"},
			{Text: "🎟 Мероприятия", Data: "reception:events"},
		},
		{
			{Text: "📋 Заполнить анкету онбординга", Data: "reception:onboarding:start"},
		},
	}

	return &core.ModuleResult{
		Text:    replyText,
		Buttons: buttons,
		Handled: true,
	}, nil
}

// HandleCallback handles inline button interactions in Reception mode.
func (m *Module) HandleCallback(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	data := cb.Data

	if data == "reception:menu" {
		return m.HandleMessage(ctx, u, state, &core.IncomingMessage{Text: "/menu"})
	}

	if data == "reception:services" {
		m.mu.RLock()
		defer m.mu.RUnlock()

		var sb strings.Builder
		sb.WriteString("📅 **Доступные услуги для записи:**\n\n")

		var buttons [][]core.ResponseButton
		for _, s := range m.services {
			sb.WriteString(fmt.Sprintf("• **%s** (%d мин) — %d ₽\n  _%s_\n\n", s.Title, s.DurationMinutes, s.Price, s.Description))
			buttons = append(buttons, []core.ResponseButton{
				{
					Text: fmt.Sprintf("Записаться: %s", s.Title),
					Data: fmt.Sprintf("reception:book:%s", s.ID),
				},
			})
		}
		buttons = append(buttons, []core.ResponseButton{
			{Text: "« Назад", Data: "reception:menu"},
		})

		return &core.ModuleResult{
			Text:    sb.String(),
			Buttons: buttons,
			Handled: true,
		}, nil
	}

	if strings.HasPrefix(data, "reception:book:") {
		serviceID := strings.TrimPrefix(data, "reception:book:")
		// Offer slots
		slot1 := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
		slot2 := time.Now().Add(26 * time.Hour).Truncate(time.Hour)

		text := "⏰ **Выберите удобный слот времени для записи:**"
		buttons := [][]core.ResponseButton{
			{
				{Text: slot1.Format("02.01 15:04"), Data: fmt.Sprintf("reception:slot:%s:%d", serviceID, slot1.Unix())},
				{Text: slot2.Format("02.01 15:04"), Data: fmt.Sprintf("reception:slot:%s:%d", serviceID, slot2.Unix())},
			},
			{
				{Text: "« Назад к услугам", Data: "reception:services"},
			},
		}
		return &core.ModuleResult{
			Text:    text,
			Buttons: buttons,
			Handled: true,
		}, nil
	}

	if strings.HasPrefix(data, "reception:slot:") {
		parts := strings.Split(data, ":")
		if len(parts) < 4 {
			return &core.ModuleResult{Text: "Некорректный слот.", Handled: true}, nil
		}
		serviceID := parts[2]
		var unixTime int64
		_, _ = fmt.Sscanf(parts[3], "%d", &unixTime)
		slotTime := time.Unix(unixTime, 0)

		err := m.BookSlot(ctx, u.ID, serviceID, slotTime)
		if err != nil {
			return &core.ModuleResult{
				Text: fmt.Sprintf("❌ Не удалось забронировать: %v", err),
				Buttons: [][]core.ResponseButton{
					{{Text: "« Выбрать другой слот", Data: fmt.Sprintf("reception:book:%s", serviceID)}},
				},
				Handled: true,
			}, nil
		}

		return &core.ModuleResult{
			Text: fmt.Sprintf("✅ **Вы успешно записаны!**\nВремя: %s\nМы пришлем вам напоминание за 1 час до начала.", slotTime.Format("02.01.2006 15:04")),
			Buttons: [][]core.ResponseButton{
				{{Text: "« Главное меню", Data: "reception:menu"}},
			},
			Handled: true,
		}, nil
	}

	if data == "reception:events" {
		m.mu.RLock()
		defer m.mu.RUnlock()

		var sb strings.Builder
		sb.WriteString("🎟 **Ближайшие мероприятия Bot Hub:**\n\n")

		var buttons [][]core.ResponseButton
		for _, ev := range m.events {
			freeSeats := ev.TotalSeats - ev.BookedSeats
			sb.WriteString(fmt.Sprintf("• **%s**\n  Дата: %s | Свободно мест: %d/%d\n\n",
				ev.Title, ev.EventTime.Format("02.01 15:04"), freeSeats, ev.TotalSeats))
			if freeSeats > 0 {
				buttons = append(buttons, []core.ResponseButton{
					{
						Text: fmt.Sprintf("Зарегистрироваться: %s", ev.Title),
						Data: fmt.Sprintf("reception:register:%s", ev.ID),
					},
				})
			}
		}
		buttons = append(buttons, []core.ResponseButton{
			{Text: "« Назад", Data: "reception:menu"},
		})

		return &core.ModuleResult{
			Text:    sb.String(),
			Buttons: buttons,
			Handled: true,
		}, nil
	}

	if strings.HasPrefix(data, "reception:register:") {
		eventID := strings.TrimPrefix(data, "reception:register:")
		reg, err := m.RegisterEvent(ctx, u.ID, eventID)
		if err != nil {
			return &core.ModuleResult{
				Text: fmt.Sprintf("❌ Ошибка регистрации: %v", err),
				Buttons: [][]core.ResponseButton{
					{{Text: "« К списку мероприятий", Data: "reception:events"}},
				},
				Handled: true,
			}, nil
		}

		text := fmt.Sprintf(
			"🎉 **Вы успешно зарегистрированы на мероприятие!**\n\n"+
				"Ваш персональный QR-билет:\n`%s`\n\n"+
				"Покажите этот код на стойке регистрации при входе.",
			reg.TicketQR,
		)
		return &core.ModuleResult{
			Text: text,
			Buttons: [][]core.ResponseButton{
				{{Text: "« В меню ресепшена", Data: "reception:menu"}},
			},
			Handled: true,
		}, nil
	}

	if data == "reception:onboarding:start" {
		return &core.ModuleResult{
			Text:    "📋 **Анкета онбординга (Шаг 1 из 4):**\nКак вас зовут? (Имя и Фамилия):",
			NewStep: "onboarding_step_1",
			Handled: true,
		}, nil
	}

	return &core.ModuleResult{
		Text:    "Команда не распознана.",
		Handled: false,
	}, nil
}

// GetServices returns the list of bookable services.
func (m *Module) GetServices(ctx context.Context) ([]store.Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]store.Service, 0, len(m.services))
	for _, s := range m.services {
		res = append(res, s)
	}
	return res, nil
}

// GetEvents returns the list of upcoming events.
func (m *Module) GetEvents(ctx context.Context) ([]store.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]store.Event, 0, len(m.events))
	for _, ev := range m.events {
		res = append(res, *ev)
	}
	return res, nil
}

// GetSlots returns available time slots for a service on a given date.
func (m *Module) GetSlots(ctx context.Context, serviceID string, targetDate time.Time) ([]store.TimeSlot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	baseDate := targetDate
	if baseDate.IsZero() {
		baseDate = time.Now().Add(24 * time.Hour)
	}
	baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 0, 0, 0, 0, baseDate.Location())

	hours := []int{10, 12, 14, 16, 18}
	slots := make([]store.TimeSlot, 0, len(hours))
	for _, h := range hours {
		st := baseDate.Add(time.Duration(h) * time.Hour)
		k := slotKey(serviceID, st)
		isBooked := false
		if _, exists := m.bookedSlots[k]; exists {
			isBooked = true
		}
		slots = append(slots, store.TimeSlot{
			ID:        fmt.Sprintf("slot_%s_%d", serviceID, st.Unix()),
			ServiceID: serviceID,
			SlotTime:  st,
			IsBooked:  isBooked,
		})
	}
	return slots, nil
}

