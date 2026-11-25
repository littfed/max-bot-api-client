package reception

import (
	"context"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type stubReminderSched struct {
	scheduled int
	cancelled int
}

func (s *stubReminderSched) ScheduleBookingReminder(ctx context.Context, userID int64, bookingID, serviceTitle string, slotTime time.Time) error {
	s.scheduled++
	return nil
}

func (s *stubReminderSched) CancelBookingReminder(ctx context.Context, bookingID string) error {
	s.cancelled++
	return nil
}

func TestReception_BookingAndCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := New(nil, nil)
	sched := &stubReminderSched{}
	m.SetReminderScheduler(sched)

	if m.ID() != "reception" || m.Title() == "" {
		t.Fatalf("unexpected module metadata: id=%q title=%q", m.ID(), m.Title())
	}

	services, err := m.GetServices(ctx)
	if err != nil || len(services) < 2 {
		t.Fatalf("GetServices failed: len=%d err=%v", len(services), err)
	}

	slot := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	if err := m.BookSlot(ctx, 101, "srv_consult", slot); err != nil {
		t.Fatalf("BookSlot failed: %v", err)
	}
	if sched.scheduled != 1 {
		t.Errorf("expected 1 scheduled reminder, got %d", sched.scheduled)
	}

	// Double booking the same slot must fail
	if err := m.BookSlot(ctx, 102, "srv_consult", slot); err == nil {
		t.Fatal("expected error for double-booked slot, got nil")
	}

	bookings, err := m.GetUserBookings(ctx, 101)
	if err != nil || len(bookings) != 1 {
		t.Fatalf("GetUserBookings failed: len=%d err=%v", len(bookings), err)
	}

	if err := m.CancelBooking(ctx, bookings[0].ID); err != nil {
		t.Fatalf("CancelBooking failed: %v", err)
	}
	if sched.cancelled != 1 {
		t.Errorf("expected 1 cancelled reminder, got %d", sched.cancelled)
	}
}

func TestReception_EventsAndOnboarding(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := New(nil, nil)

	events, err := m.GetEvents(ctx)
	if err != nil || len(events) < 2 {
		t.Fatalf("GetEvents failed: len=%d err=%v", len(events), err)
	}

	reg, err := m.RegisterEvent(ctx, 202, "evt_launch")
	if err != nil || reg.TicketQR == "" {
		t.Fatalf("RegisterEvent failed: reg=%+v err=%v", reg, err)
	}

	if _, err := m.RegisterEvent(ctx, 202, "evt_missing"); err == nil {
		t.Fatal("expected error for unknown event, got nil")
	}

	regs, err := m.GetUserRegistrations(ctx, 202)
	if err != nil || len(regs) != 1 {
		t.Fatalf("GetUserRegistrations failed: len=%d err=%v", len(regs), err)
	}

	if err := m.SubmitOnboarding(ctx, 202, map[string]string{"name": "Иван", "contact": "@ivan"}); err != nil {
		t.Fatalf("SubmitOnboarding failed: %v", err)
	}
	sub, err := m.GetUserOnboarding(ctx, 202)
	if err != nil || sub == nil {
		t.Fatalf("GetUserOnboarding failed: sub=%+v err=%v", sub, err)
	}
}

func TestReception_ValidateContactAndHandlers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"telegram_handle", "@ivan_dev", true},
		{"phone_ru", "+79991234567", true},
		{"email_rejected", "test@example.com", false},
		{"too_short", "ab", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := validateContact(tc.input); got != tc.want {
				t.Errorf("validateContact(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}

	m := New(nil, nil)
	user := &store.User{ID: 1}
	res, err := m.HandleCallback(context.Background(), user, &core.State{}, &core.IncomingCallback{Data: "reception:menu"})
	if err != nil || res == nil || !res.Handled || res.Text == "" {
		t.Fatalf("HandleCallback(reception:menu) failed: res=%+v err=%v", res, err)
	}
}
