package reminders

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/store"
)

var (
	relMinRegex   = regexp.MustCompile(`^(?:\+)?(\d+)\s*(?:m|min|мин|минут|минуты|минута)$`)
	relHourRegex  = regexp.MustCompile(`^(?:\+)?(\d+)\s*(?:h|час|часа|часов|ч)$`)
	cherezRegex   = regexp.MustCompile(`^через\s+(\d+)\s*(минут|минуты|минуту|мин|м|час|часа|часов|ч)$`)
	timeOnlyRegex = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
	zavtraRegex   = regexp.MustCompile(`^(?:завтра|сегодня)\s+(\d{1,2}):(\d{2})$`)
	dateRegex1    = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\s+(\d{1,2}):(\d{2})$`)
	dateRegex2    = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4})\s+(\d{1,2}):(\d{2})$`)
)

// CalculateNextRecurrence computes the next occurrence timestamp according to the rule and location.
func CalculateNextRecurrence(rule string, from time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	curr := from.In(loc)

	switch strings.ToLower(strings.TrimSpace(rule)) {
	case "daily":
		return curr.AddDate(0, 0, 1)
	case "weekdays":
		// Mon..Thu -> +1 day
		// Fri -> +3 days (Mon)
		// Sat -> +2 days (Mon)
		switch curr.Weekday() {
		case time.Friday:
			return curr.AddDate(0, 0, 3)
		case time.Saturday:
			return curr.AddDate(0, 0, 2)
		default:
			return curr.AddDate(0, 0, 1)
		}
	case "weekly":
		return curr.AddDate(0, 0, 7)
	default:
		return time.Time{}
	}
}

// ParseTimeInput parses natural or formatted time strings taking user location and base reference into account.
func ParseTimeInput(input string, now time.Time, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	trimmed := strings.ToLower(strings.TrimSpace(input))
	base := now.In(loc)

	// 1. "+15m", "30мин", "1h"
	if m := relMinRegex.FindStringSubmatch(trimmed); len(m) == 2 {
		val, _ := strconv.Atoi(m[1])
		return base.Add(time.Duration(val) * time.Minute), nil
	}
	if m := relHourRegex.FindStringSubmatch(trimmed); len(m) == 2 {
		val, _ := strconv.Atoi(m[1])
		return base.Add(time.Duration(val) * time.Hour), nil
	}

	// 2. "через X минут / часов"
	if m := cherezRegex.FindStringSubmatch(trimmed); len(m) == 3 {
		val, _ := strconv.Atoi(m[1])
		unit := m[2]
		if strings.HasPrefix(unit, "мин") || unit == "м" {
			return base.Add(time.Duration(val) * time.Minute), nil
		}
		return base.Add(time.Duration(val) * time.Hour), nil
	}

	// 3. "завтра 14:00" или "сегодня 19:30"
	if m := zavtraRegex.FindStringSubmatch(trimmed); len(m) == 3 {
		hour, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		target := time.Date(base.Year(), base.Month(), base.Day(), hour, min, 0, 0, loc)
		if strings.HasPrefix(trimmed, "завтра") {
			target = target.AddDate(0, 0, 1)
		}
		return target, nil
	}

	// 4. "15:30" (time only)
	if m := timeOnlyRegex.FindStringSubmatch(trimmed); len(m) == 3 {
		hour, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		target := time.Date(base.Year(), base.Month(), base.Day(), hour, min, 0, 0, loc)
		if !target.After(base) {
			// If already passed today, assume tomorrow
			target = target.AddDate(0, 0, 1)
		}
		return target, nil
	}

	// 5. "26.11 10:00"
	if m := dateRegex1.FindStringSubmatch(trimmed); len(m) == 5 {
		day, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		hour, _ := strconv.Atoi(m[3])
		min, _ := strconv.Atoi(m[4])
		target := time.Date(base.Year(), time.Month(month), day, hour, min, 0, 0, loc)
		if !target.After(base) {
			target = target.AddDate(1, 0, 0)
		}
		return target, nil
	}

	// 6. "26.11.2026 10:00"
	if m := dateRegex2.FindStringSubmatch(trimmed); len(m) == 6 {
		day, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		year, _ := strconv.Atoi(m[3])
		hour, _ := strconv.Atoi(m[4])
		min, _ := strconv.Atoi(m[5])
		return time.Date(year, time.Month(month), day, hour, min, 0, 0, loc), nil
	}

	// 7. RFC3339 fallback
	if t, err := time.Parse(time.RFC3339, input); err == nil {
		return t.In(loc), nil
	}

	return time.Time{}, fmt.Errorf("не удалось распознать формат времени: %q", input)
}

// ScheduleBookingParams carries parameters to register an automated booking reminder.
type ScheduleBookingParams struct {
	UserID       int64
	BookingID    string
	ServiceTitle string
	SlotTime     time.Time
	RemindAt     time.Time
	TemplateID   string
}

// Service provides high-level business operations for managing reminders.
type Service struct {
	sqlStore *store.SQLStore
}

// NewService constructs a Reminders Service instance.
func NewService(sqlStore *store.SQLStore) *Service {
	return &Service{
		sqlStore: sqlStore,
	}
}

// CreatePersonalReminder creates a new user task or reminder.
func (s *Service) CreatePersonalReminder(ctx context.Context, userID int64, title, desc string, remindAt time.Time, recurrence string) (*store.Reminder, error) {
	if s.sqlStore == nil {
		return nil, errors.New("sqlStore is nil")
	}
	if strings.TrimSpace(title) == "" {
		return nil, errors.New("title cannot be empty")
	}
	if recurrence == "" {
		recurrence = "none"
	}

	rem := &store.Reminder{
		UserID:         userID,
		Title:          strings.TrimSpace(title),
		Description:    strings.TrimSpace(desc),
		RemindAt:       remindAt.UTC(),
		RecurrenceRule: recurrence,
		Status:         "pending",
		SourceType:     "user",
		ABVariant:      "A",
	}

	id, err := s.sqlStore.CreateReminder(ctx, rem)
	if err != nil {
		return nil, fmt.Errorf("failed to create reminder: %w", err)
	}
	rem.ID = id
	return rem, nil
}

// GetReminders fetches reminders for a user with optional status filter.
func (s *Service) GetReminders(ctx context.Context, userID int64, status string) ([]store.Reminder, error) {
	if s.sqlStore == nil {
		return nil, errors.New("sqlStore is nil")
	}
	return s.sqlStore.GetRemindersByUser(ctx, userID, status)
}

// GetReminder fetches a single reminder by ID.
func (s *Service) GetReminder(ctx context.Context, id int64) (*store.Reminder, error) {
	if s.sqlStore == nil {
		return nil, errors.New("sqlStore is nil")
	}
	return s.sqlStore.GetReminderByID(ctx, id)
}

// CompleteReminder marks a reminder as completed and spawns the next occurrence if recurring.
func (s *Service) CompleteReminder(ctx context.Context, id int64, loc *time.Location) (*store.Reminder, *store.Reminder, error) {
	if s.sqlStore == nil {
		return nil, nil, errors.New("sqlStore is nil")
	}
	rem, err := s.sqlStore.GetReminderByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find reminder: %w", err)
	}

	if err := s.sqlStore.UpdateReminderStatus(ctx, id, "completed"); err != nil {
		return nil, nil, fmt.Errorf("failed to complete reminder: %w", err)
	}
	rem.Status = "completed"

	// Log completion event
	_ = s.sqlStore.RecordNotificationEvent(ctx, &store.NotificationEvent{
		ReminderID: &id,
		TemplateID: rem.TemplateID,
		UserID:     rem.UserID,
		Variant:    rem.ABVariant,
		EventType:  "completed",
	})

	var nextRem *store.Reminder
	if rem.RecurrenceRule != "" && rem.RecurrenceRule != "none" {
		nextTime := CalculateNextRecurrence(rem.RecurrenceRule, rem.RemindAt, loc)
		if !nextTime.IsZero() {
			newRem := &store.Reminder{
				UserID:         rem.UserID,
				Title:          rem.Title,
				Description:    rem.Description,
				RemindAt:       nextTime.UTC(),
				RecurrenceRule: rem.RecurrenceRule,
				Status:         "pending",
				SourceType:     rem.SourceType,
				SourceRefID:    rem.SourceRefID,
				TemplateID:     rem.TemplateID,
				ABVariant:      rem.ABVariant,
			}
			newID, err := s.sqlStore.CreateReminder(ctx, newRem)
			if err == nil {
				newRem.ID = newID
				nextRem = newRem
			}
		}
	}

	return rem, nextRem, nil
}

// SnoozeReminder defers a reminder by specified minutes and resets status to 'pending'.
func (s *Service) SnoozeReminder(ctx context.Context, id int64, minutes int) (*store.Reminder, error) {
	if s.sqlStore == nil {
		return nil, errors.New("sqlStore is nil")
	}
	if minutes <= 0 {
		minutes = 15
	}
	newRemindAt := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	if err := s.sqlStore.SnoozeReminderInStore(ctx, id, newRemindAt); err != nil {
		return nil, fmt.Errorf("failed to snooze reminder: %w", err)
	}

	rem, err := s.sqlStore.GetReminderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Record snooze telemetry
	_ = s.sqlStore.RecordNotificationEvent(ctx, &store.NotificationEvent{
		ReminderID: &id,
		TemplateID: rem.TemplateID,
		UserID:     rem.UserID,
		Variant:    rem.ABVariant,
		EventType:  "snoozed",
	})

	return rem, nil
}

// CancelReminder marks a reminder as cancelled.
func (s *Service) CancelReminder(ctx context.Context, id int64) error {
	if s.sqlStore == nil {
		return errors.New("sqlStore is nil")
	}
	rem, err := s.sqlStore.GetReminderByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.sqlStore.UpdateReminderStatus(ctx, id, "cancelled"); err != nil {
		return err
	}

	_ = s.sqlStore.RecordNotificationEvent(ctx, &store.NotificationEvent{
		ReminderID: &id,
		TemplateID: rem.TemplateID,
		UserID:     rem.UserID,
		Variant:    rem.ABVariant,
		EventType:  "cancelled",
	})
	return nil
}

// ScheduleBookingReminder schedules a reminder for Reception mode booking.
func (s *Service) ScheduleBookingReminder(ctx context.Context, params ScheduleBookingParams) (*store.Reminder, error) {
	if s.sqlStore == nil {
		return nil, errors.New("sqlStore is nil")
	}

	abVariant := "A"
	if rand.Intn(100) >= 50 {
		abVariant = "B"
	}

	rem := &store.Reminder{
		UserID:         params.UserID,
		Title:          fmt.Sprintf("Визит: %s", params.ServiceTitle),
		Description:    fmt.Sprintf("Бронь на %s", params.SlotTime.Format("02.01.2006 15:04")),
		RemindAt:       params.RemindAt.UTC(),
		RecurrenceRule: "none",
		Status:         "pending",
		SourceType:     "reception",
		SourceRefID:    params.BookingID,
		TemplateID:     params.TemplateID,
		ABVariant:      abVariant,
	}

	id, err := s.sqlStore.CreateReminder(ctx, rem)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule booking reminder: %w", err)
	}
	rem.ID = id
	return rem, nil
}

// CancelBookingReminder cancels any active reminder linked to the booking ID.
func (s *Service) CancelBookingReminder(ctx context.Context, bookingID string) error {
	if s.sqlStore == nil {
		return errors.New("sqlStore is nil")
	}
	return s.sqlStore.CancelReminderBySourceRef(ctx, "reception", bookingID)
}

// ScheduleDemoSimulation creates a fast 10-second demo reminder for client demonstrations.
func (s *Service) ScheduleDemoSimulation(ctx context.Context, userID int64) (*store.Reminder, error) {
	if s.sqlStore == nil {
		return nil, errors.New("sqlStore is nil")
	}
	abVariant := "A"
	if rand.Intn(100) >= 50 {
		abVariant = "B"
	}

	rem := &store.Reminder{
		UserID:         userID,
		Title:          "Консультация архитектора",
		Description:    "Демонстрация триггерного уведомления Bot Hub",
		RemindAt:       time.Now().UTC().Add(10 * time.Second),
		RecurrenceRule: "none",
		Status:         "pending",
		SourceType:     "demo_simulation",
		SourceRefID:    "demo_slot_1",
		TemplateID:     "booking_reminder",
		ABVariant:      abVariant,
	}

	id, err := s.sqlStore.CreateReminder(ctx, rem)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule demo simulation: %w", err)
	}
	rem.ID = id
	return rem, nil
}
