package modules_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules"
	"github.com/littfed/max-bot-api-client/internal/modules/chat"
	"github.com/littfed/max-bot-api-client/internal/modules/reception"
	"github.com/littfed/max-bot-api-client/internal/modules/reminders"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/internal/modules/support"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

func setupTestShopModule(t *testing.T) *shop.Module {
	return shop.New(nil)
}

func setupTestReceptionModule(t *testing.T) *reception.Module {
	lockManager := core.NewLockManager(nil)
	return reception.New(nil, lockManager)
}

func setupTestSupportModule(t *testing.T) *support.Module {
	return support.New(nil)
}

func setupTestChatModule(t *testing.T) *chat.Module {
	return chat.New(nil, nil)
}

func TestShopModule_DemoSimulation(t *testing.T) {
	shopMod := setupTestShopModule(t)
	resPaid, err := shopMod.SimulatePayment(context.Background(), "ord_demo_1", "paid")
	if err != nil || resPaid.Status != "paid" {
		t.Errorf("SimulatePayment paid failed: %v, res: %+v", err, resPaid)
	}

	resAbandoned, err := shopMod.SimulatePayment(context.Background(), "ord_demo_2", "abandoned")
	if err != nil || resAbandoned.Status != "abandoned" {
		t.Errorf("SimulatePayment abandoned failed: %v, res: %+v", err, resAbandoned)
	}
}

func TestReceptionModule_AntiOverbooking(t *testing.T) {
	receptionMod := setupTestReceptionModule(t)
	ctx := context.Background()
	slotTime := time.Now().Add(24 * time.Hour).Truncate(time.Hour)

	// User 1 books
	err1 := receptionMod.BookSlot(ctx, 101, "srv_consult", slotTime)
	if err1 != nil {
		t.Fatalf("First booking should succeed: %v", err1)
	}

	// User 2 attempts same slot -> must fail
	err2 := receptionMod.BookSlot(ctx, 102, "srv_consult", slotTime)
	if err2 == nil {
		t.Errorf("Expected overbooking error for user 2, got nil!")
	}
}

func TestSupportModule_ScopedRAG(t *testing.T) {
	supportMod := setupTestSupportModule(t)
	ctx := context.Background()
	ans, score, err := supportMod.Ask(ctx, "Как оформить подписку?", "shop")
	if err != nil {
		t.Fatalf("Support Ask failed: %v", err)
	}
	if score < 0.5 || ans == "" {
		t.Errorf("Unexpected RAG answer: %s, score: %f", ans, score)
	}
}

func TestChatModule_RoleAndTone(t *testing.T) {
	chatMod := setupTestChatModule(t)
	ctx := context.Background()
	resp, err := chatMod.SendMessage(ctx, 42, 2, 1, "Что такое юнит-экономика?")
	if err != nil {
		t.Fatalf("Chat SendMessage failed: %v", err)
	}
	if resp == "" {
		t.Errorf("Expected non-empty LLM response")
	}
}

func TestModules_BotModuleInterfaces(t *testing.T) {
	// Verify that all 4 modules strictly implement modules.BotModule & core.BotModule
	shopMod := setupTestShopModule(t)
	receptionMod := setupTestReceptionModule(t)
	supportMod := setupTestSupportModule(t)
	chatMod := setupTestChatModule(t)

	var _ modules.BotModule = shopMod
	var _ core.BotModule = shopMod

	var _ modules.BotModule = receptionMod
	var _ core.BotModule = receptionMod

	var _ modules.BotModule = supportMod
	var _ core.BotModule = supportMod

	var _ modules.BotModule = chatMod
	var _ core.BotModule = chatMod

	if shopMod.ID() != "shop" || shopMod.Title() == "" {
		t.Errorf("Invalid shop module ID/Title: %s, %s", shopMod.ID(), shopMod.Title())
	}
	if receptionMod.ID() != "reception" || receptionMod.Title() == "" {
		t.Errorf("Invalid reception module ID/Title: %s, %s", receptionMod.ID(), receptionMod.Title())
	}
	if supportMod.ID() != "support" || supportMod.Title() == "" {
		t.Errorf("Invalid support module ID/Title: %s, %s", supportMod.ID(), supportMod.Title())
	}
	if chatMod.ID() != "chat" || chatMod.Title() == "" {
		t.Errorf("Invalid chat module ID/Title: %s, %s", chatMod.ID(), chatMod.Title())
	}
}

func TestShopModule_WorkflowAndCatalog(t *testing.T) {
	shopMod := setupTestShopModule(t)
	ctx := context.Background()
	user := &store.User{ID: 10, ActiveModeID: "shop"}
	state := &core.State{UserID: 10, ModeID: "shop"}

	// HandleMessage returns catalog
	msgRes, err := shopMod.HandleMessage(ctx, user, state, &core.IncomingMessage{Text: "Привет"})
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if msgRes == nil || len(msgRes.Buttons) == 0 {
		t.Fatalf("Expected catalog buttons in message response")
	}

	// HandleCallback for buying product
	cbRes, err := shopMod.HandleCallback(ctx, user, state, &core.IncomingCallback{
		Data: "shop:buy:prod_guide",
	})
	if err != nil {
		t.Fatalf("HandleCallback buy failed: %v", err)
	}
	if cbRes == nil || !strings.Contains(strings.ToLower(cbRes.Text), "заказ") {
		t.Errorf("Expected order details in buy response: %+v", cbRes)
	}

	// Paid simulation delivering content
	simRes, err := shopMod.SimulatePayment(ctx, "ord_demo_10", "paid")
	if err != nil || simRes.Status != "paid" || simRes.DeliveryPayload == "" {
		t.Errorf("Expected paid status and non-empty delivery payload: %+v", simRes)
	}

	// Abandoned simulation scheduling reminder
	simAb, err := shopMod.SimulatePayment(ctx, "ord_demo_11", "abandoned")
	if err != nil || simAb.Status != "abandoned" || !simAb.ReminderScheduled {
		t.Errorf("Expected abandoned status and reminder scheduled: %+v", simAb)
	}
}

func TestReceptionModule_ConcurrentOverbooking(t *testing.T) {
	receptionMod := setupTestReceptionModule(t)
	ctx := context.Background()
	slotTime := time.Now().Add(48 * time.Hour).Truncate(time.Hour)

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	for i := int64(1); i <= 10; i++ {
		wg.Add(1)
		go func(uid int64) {
			defer wg.Done()
			err := receptionMod.BookSlot(ctx, uid, "srv_consult", slotTime)
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if successCount != 1 || failCount != 9 {
		t.Errorf("Anti-overbooking concurrent test failed: got %d success, %d fails (expected 1 success, 9 fails)",
			successCount, failCount)
	}
}

func TestReceptionModule_EventAndOnboarding(t *testing.T) {
	receptionMod := setupTestReceptionModule(t)
	ctx := context.Background()

	// Event registration
	reg, err := receptionMod.RegisterEvent(ctx, 201, "evt_launch")
	if err != nil {
		t.Fatalf("RegisterEvent failed: %v", err)
	}
	if reg == nil || reg.TicketQR == "" {
		t.Fatalf("Expected non-empty ticket QR")
	}

	// 4-step onboarding questionnaire validation
	user := &store.User{ID: 201, ActiveModeID: "reception"}
	state := &core.State{UserID: 201, ModeID: "reception", CurrentStep: "onboarding_step_1"}

	// Step 1: Name
	res1, err := receptionMod.HandleMessage(ctx, user, state, &core.IncomingMessage{Text: "Иван Иванов"})
	if err != nil || res1.NewStep != "onboarding_step_2" {
		t.Errorf("Step 1 failed: res=%+v, err=%v", res1, err)
	}

	// Step 2: Business
	state.CurrentStep = "onboarding_step_2"
	state.Payload = res1.NewPayload
	res2, err := receptionMod.HandleMessage(ctx, user, state, &core.IncomingMessage{Text: "Интернет-магазин одежды"})
	if err != nil || res2.NewStep != "onboarding_step_3" {
		t.Errorf("Step 2 failed: res=%+v, err=%v", res2, err)
	}

	// Step 3: Tasks
	state.CurrentStep = "onboarding_step_3"
	state.Payload = res2.NewPayload
	res3, err := receptionMod.HandleMessage(ctx, user, state, &core.IncomingMessage{Text: "Автоматизация консультаций и прием оплат"})
	if err != nil || res3.NewStep != "onboarding_step_4" {
		t.Errorf("Step 3 failed: res=%+v, err=%v", res3, err)
	}

	// Step 4: Invalid contact should fail
	state.CurrentStep = "onboarding_step_4"
	state.Payload = res3.NewPayload
	resInvalid, err := receptionMod.HandleMessage(ctx, user, state, &core.IncomingMessage{Text: "invalid"})
	if err != nil || resInvalid.NewStep == "onboarding_done" {
		t.Errorf("Expected step 4 validation failure on invalid contact: %+v", resInvalid)
	}

	// Step 4: Valid contact
	res4, err := receptionMod.HandleMessage(ctx, user, state, &core.IncomingMessage{Text: "+79991234567"})
	if err != nil || (!res4.ResetState && res4.NewStep != "onboarding_done") {
		t.Errorf("Step 4 valid failed: res=%+v, err=%v", res4, err)
	}
}

func TestSupportModule_ConfidenceThresholdAndEscalation(t *testing.T) {
	supportMod := setupTestSupportModule(t)
	ctx := context.Background()

	// High confidence match in shop scope
	ans, conf, err := supportMod.Ask(ctx, "Как оформить подписку?", "shop")
	if err != nil || conf < 0.72 || ans == "" {
		t.Errorf("Expected high confidence answer for shop question: ans=%s, conf=%f, err=%v", ans, conf, err)
	}

	// Low confidence for unknown question -> below 0.72
	unknownAns, lowConf, err := supportMod.Ask(ctx, "Какой курс биткоина на Марсе?", "shop")
	if err != nil || lowConf >= 0.72 {
		t.Errorf("Expected low confidence (<0.72) for unknown question: ans=%s, conf=%f, err=%v", unknownAns, lowConf, err)
	}

	// Escalation: Ticket creation
	ticket, err := supportMod.CreateTicket(ctx, 301, "max", "shop", "Вопрос по оплате", "Не проходит платеж по карте")
	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}
	if ticket == nil || ticket.ID == 0 || ticket.Status != "open" || ticket.Message != "Не проходит платеж по карте" {
		t.Errorf("Unexpected ticket created: %+v", ticket)
	}
}

func TestChatModule_SlidingWindowAndRoles(t *testing.T) {
	chatMod := setupTestChatModule(t)
	ctx := context.Background()
	userID := int64(555)

	// Send 12 messages to test sliding window (max 10 turns)
	for i := 1; i <= 12; i++ {
		resp, err := chatMod.SendMessage(ctx, userID, 1, 1, "Тестовое сообщение")
		if err != nil || resp == "" {
			t.Fatalf("SendMessage %d failed: resp=%s, err=%v", i, resp, err)
		}
	}

	session, err := chatMod.GetSession(ctx, userID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if len(session.Messages) > 10 {
		t.Errorf("Expected sliding window capped at 10 messages, got %d", len(session.Messages))
	}

	// Reset session clears history
	err = chatMod.ResetSession(ctx, userID)
	if err != nil {
		t.Fatalf("ResetSession failed: %v", err)
	}

	sessionAfter, err := chatMod.GetSession(ctx, userID)
	if err != nil {
		t.Fatalf("GetSession after reset failed: %v", err)
	}
	if len(sessionAfter.Messages) != 0 {
		t.Errorf("Expected empty session after reset, got %d messages", len(sessionAfter.Messages))
	}
}

type testReminderRecorder struct {
	scheduled []string
	cancelled []string
}

func (r *testReminderRecorder) ScheduleBookingReminder(ctx context.Context, userID int64, bookingID, serviceTitle string, slotTime time.Time) error {
	r.scheduled = append(r.scheduled, bookingID)
	return nil
}

func (r *testReminderRecorder) CancelBookingReminder(ctx context.Context, bookingID string) error {
	r.cancelled = append(r.cancelled, bookingID)
	return nil
}

func TestReceptionModule_LinkedReminder(t *testing.T) {
	receptionMod := setupTestReceptionModule(t)
	rec := &testReminderRecorder{}
	receptionMod.SetReminderScheduler(rec)

	ctx := context.Background()
	slotTime := time.Now().Add(24 * time.Hour).Truncate(time.Hour)

	if err := receptionMod.BookSlot(ctx, 101, "srv_consultation", slotTime); err != nil {
		t.Fatalf("BookSlot failed: %v", err)
	}

	if len(rec.scheduled) != 1 {
		t.Fatalf("expected 1 scheduled reminder, got %d", len(rec.scheduled))
	}

	bookingID := rec.scheduled[0]
	if err := receptionMod.CancelBooking(ctx, bookingID); err != nil {
		t.Fatalf("CancelBooking failed: %v", err)
	}

	if len(rec.cancelled) != 1 || rec.cancelled[0] != bookingID {
		t.Fatalf("expected cancelled reminder for booking %s, got %+v", bookingID, rec.cancelled)
	}
}

type testReceptionScheduler struct {
	svc *reminders.Service
}

func (a *testReceptionScheduler) ScheduleBookingReminder(ctx context.Context, userID int64, bookingID, serviceTitle string, slotTime time.Time) error {
	return a.svc.ScheduleBookingReminderSimple(ctx, userID, bookingID, serviceTitle, slotTime)
}

func (a *testReceptionScheduler) CancelBookingReminder(ctx context.Context, bookingID string) error {
	return a.svc.CancelBookingReminder(ctx, bookingID)
}

func TestReceptionModule_LinkedReminder_Integration(t *testing.T) {
	s, err := store.NewFromEnv("file:"+t.TempDir()+"/reception_rem.db", "")
	if err != nil {
		t.Fatalf("store.NewFromEnv failed: %v", err)
	}
	defer s.Close()

	sqlStore := s.SQL()
	ctx := context.Background()
	user, err := sqlStore.ResolveUserIdentity(ctx, "max", "rec_int_1", "rec_user", "RecUser", "+79991112233")
	if err != nil {
		t.Fatalf("ResolveUserIdentity failed: %v", err)
	}

	remSvc := reminders.NewService(sqlStore)
	receptionMod := reception.New(sqlStore, core.NewLockManager(nil))
	receptionMod.SetReminderScheduler(&testReceptionScheduler{svc: remSvc})

	slotTime := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	if err := receptionMod.BookSlot(ctx, user.ID, "srv_consultation", slotTime); err != nil {
		t.Fatalf("BookSlot failed: %v", err)
	}

	list, err := remSvc.GetReminders(ctx, user.ID, "pending")
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 pending reminder in DB, got %d (err=%v)", len(list), err)
	}
	if list[0].SourceType != "reception" || list[0].SourceRefID == "" {
		t.Fatalf("unexpected reminder metadata: %+v", list[0])
	}

	if err := receptionMod.CancelBooking(ctx, list[0].SourceRefID); err != nil {
		t.Fatalf("CancelBooking failed: %v", err)
	}
	updated, err := remSvc.GetReminder(ctx, list[0].ID)
	if err != nil || updated.Status != "cancelled" {
		t.Fatalf("expected cancelled reminder in DB, got %+v (err=%v)", updated, err)
	}
}
