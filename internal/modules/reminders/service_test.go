package reminders

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/store"
)

func TestCalculateNextRecurrence(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	// Пятница 2026-11-27 10:00:00
	friday := time.Date(2026, 11, 27, 10, 0, 0, 0, loc)

	// 1. None
	nextNone := CalculateNextRecurrence("none", friday, loc)
	if !nextNone.IsZero() {
		t.Fatalf("expected zero time for 'none', got %v", nextNone)
	}

	// 2. Daily -> Суббота 2026-11-28 10:00:00
	nextDaily := CalculateNextRecurrence("daily", friday, loc)
	expectedDaily := time.Date(2026, 11, 28, 10, 0, 0, 0, loc)
	if !nextDaily.Equal(expectedDaily) {
		t.Fatalf("daily expected %v, got %v", expectedDaily, nextDaily)
	}

	// 3. Weekdays с пятницы -> Понедельник 2026-11-30 10:00:00 (+3 дня)
	nextWeekdayFromFri := CalculateNextRecurrence("weekdays", friday, loc)
	expectedWeekdayFromFri := time.Date(2026, 11, 30, 10, 0, 0, 0, loc)
	if !nextWeekdayFromFri.Equal(expectedWeekdayFromFri) {
		t.Fatalf("weekdays from Friday expected %v, got %v", expectedWeekdayFromFri, nextWeekdayFromFri)
	}

	// Weekdays с четверга -> Пятница (+1 день)
	thursday := time.Date(2026, 11, 26, 10, 0, 0, 0, loc)
	nextWeekdayFromThu := CalculateNextRecurrence("weekdays", thursday, loc)
	expectedWeekdayFromThu := time.Date(2026, 11, 27, 10, 0, 0, 0, loc)
	if !nextWeekdayFromThu.Equal(expectedWeekdayFromThu) {
		t.Fatalf("weekdays from Thursday expected %v, got %v", expectedWeekdayFromThu, nextWeekdayFromThu)
	}

	// 4. Weekly -> Пятница 2026-12-04 10:00:00 (+7 дней)
	nextWeekly := CalculateNextRecurrence("weekly", friday, loc)
	expectedWeekly := time.Date(2026, 12, 4, 10, 0, 0, 0, loc)
	if !nextWeekly.Equal(expectedWeekly) {
		t.Fatalf("weekly expected %v, got %v", expectedWeekly, nextWeekly)
	}
}

func TestParseTimeInput(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	baseTime := time.Date(2026, 11, 25, 12, 0, 0, 0, loc)

	// Относительные смещения
	p15m, err := ParseTimeInput("+15m", baseTime, loc)
	if err != nil || !p15m.Equal(baseTime.Add(15*time.Minute)) {
		t.Fatalf("ParseTimeInput +15m failed: %v, got %v", err, p15m)
	}

	p1h, err := ParseTimeInput("+1h", baseTime, loc)
	if err != nil || !p1h.Equal(baseTime.Add(1*time.Hour)) {
		t.Fatalf("ParseTimeInput +1h failed: %v, got %v", err, p1h)
	}

	pText, err := ParseTimeInput("через 30 минут", baseTime, loc)
	if err != nil || !pText.Equal(baseTime.Add(30*time.Minute)) {
		t.Fatalf("ParseTimeInput 'через 30 минут' failed: %v, got %v", err, pText)
	}

	// Время сегодня позже baseTime (15:30)
	pToday, err := ParseTimeInput("15:30", baseTime, loc)
	if err != nil {
		t.Fatalf("ParseTimeInput '15:30' failed: %v", err)
	}
	expectedToday := time.Date(2026, 11, 25, 15, 30, 0, 0, loc)
	if !pToday.Equal(expectedToday) {
		t.Fatalf("expected %v, got %v", expectedToday, pToday)
	}

	// Время сегодня раньше baseTime (10:00) -> должно стать завтра 10:00
	pTomorrow, err := ParseTimeInput("10:00", baseTime, loc)
	if err != nil {
		t.Fatalf("ParseTimeInput '10:00' failed: %v", err)
	}
	expectedTomorrow := time.Date(2026, 11, 26, 10, 0, 0, 0, loc)
	if !pTomorrow.Equal(expectedTomorrow) {
		t.Fatalf("expected tomorrow %v, got %v", expectedTomorrow, pTomorrow)
	}

	// Явное "завтра 14:00"
	pExplicitTomorrow, err := ParseTimeInput("завтра 14:00", baseTime, loc)
	if err != nil {
		t.Fatalf("ParseTimeInput 'завтра 14:00' failed: %v", err)
	}
	expectedExplicitTomorrow := time.Date(2026, 11, 26, 14, 0, 0, 0, loc)
	if !pExplicitTomorrow.Equal(expectedExplicitTomorrow) {
		t.Fatalf("expected %v, got %v", expectedExplicitTomorrow, pExplicitTomorrow)
	}
}

func TestService_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "service_test.db")

	s, err := store.NewFromEnv("file:"+dbPath, "")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	sqlStore := s.SQL()
	ctx := context.Background()
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "user_service_1", "srv_user", "ServiceUser", "+79998887766")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}

	loc, _ := time.LoadLocation("Europe/Moscow")
	service := NewService(sqlStore)

	// 1. Create personal reminder
	remTime := time.Now().In(loc).Add(2 * time.Hour)
	rem, err := service.CreatePersonalReminder(ctx, user.ID, "Подготовить отчет", "Сводка за ноябрь", remTime, "daily")
	if err != nil {
		t.Fatalf("CreatePersonalReminder failed: %v", err)
	}
	if rem.ID == 0 || rem.Status != "pending" {
		t.Fatalf("invalid created reminder: %+v", rem)
	}

	// 2. Snooze reminder
	snoozed, err := service.SnoozeReminder(ctx, rem.ID, 15)
	if err != nil {
		t.Fatalf("SnoozeReminder failed: %v", err)
	}
	if snoozed.Status != "pending" {
		t.Fatalf("expected status pending, got %s", snoozed.Status)
	}

	// 3. Complete reminder (should schedule next daily occurrence)
	completed, next, err := service.CompleteReminder(ctx, rem.ID, loc)
	if err != nil {
		t.Fatalf("CompleteReminder failed: %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("expected completed status, got %s", completed.Status)
	}
	if next == nil || next.Status != "pending" || next.ID == rem.ID {
		t.Fatalf("expected new pending recurring reminder, got %+v", next)
	}

	// 4. Schedule and cancel booking reminder
	slotTime := time.Now().In(loc).Add(24 * time.Hour)
	bRem, err := service.ScheduleBookingReminder(ctx, ScheduleBookingParams{
		UserID:       user.ID,
		BookingID:    "book_123",
		ServiceTitle: "Стрижка",
		SlotTime:     slotTime,
		RemindAt:     slotTime.Add(-2 * time.Hour),
		TemplateID:   "booking_reminder",
	})
	if err != nil {
		t.Fatalf("ScheduleBookingReminder failed: %v", err)
	}
	if bRem.SourceType != "reception" || bRem.SourceRefID != "book_123" {
		t.Fatalf("unexpected booking reminder: %+v", bRem)
	}

	// Cancel booking
	if err := service.CancelBookingReminder(ctx, "book_123"); err != nil {
		t.Fatalf("CancelBookingReminder failed: %v", err)
	}
	cancelled, err := service.GetReminder(ctx, bRem.ID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled reminder, got %+v", cancelled)
	}
}
