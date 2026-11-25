package reminders

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

// Worker periodically checks and delivers due reminders to users across messaging platforms.
type Worker struct {
	service  *Service
	sqlStore *store.SQLStore
	adapters map[string]platform.PlatformAdapter
	limiter  *rate.Limiter
	interval time.Duration
}

// NewWorker constructs a background delivery worker.
func NewWorker(service *Service, sqlStore *store.SQLStore, adapters map[string]platform.PlatformAdapter, interval time.Duration, rps float64) *Worker {
	if rps <= 0 {
		rps = 25
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if adapters == nil {
		adapters = make(map[string]platform.PlatformAdapter)
	}

	return &Worker{
		service:  service,
		sqlStore: sqlStore,
		adapters: adapters,
		limiter:  rate.NewLimiter(rate.Limit(rps), 1),
		interval: interval,
	}
}

// RegisterAdapter adds or replaces a platform adapter in the worker.
func (w *Worker) RegisterAdapter(plat string, adapter platform.PlatformAdapter) {
	if w.adapters == nil {
		w.adapters = make(map[string]platform.PlatformAdapter)
	}
	w.adapters[plat] = adapter
}

// Start runs the periodic background delivery loop until the context is cancelled.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("[RemindersWorker] Started background worker with interval %v", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[RemindersWorker] Background worker stopping...")
			return
		case <-ticker.C:
			if _, err := w.ProcessPendingOnce(ctx); err != nil {
				log.Printf("[RemindersWorker] ProcessPendingOnce error: %v", err)
			}
		}
	}
}

// ProcessPendingOnce runs a single delivery batch pass.
func (w *Worker) ProcessPendingOnce(ctx context.Context) (int, error) {
	if w.sqlStore == nil {
		return 0, fmt.Errorf("sqlStore is nil")
	}

	reminders, err := w.sqlStore.GetPendingRemindersForUpdate(ctx, 50)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pending reminders: %w", err)
	}
	if len(reminders) == 0 {
		return 0, nil
	}

	processed := 0
	for _, rem := range reminders {
		if err := w.deliverReminder(ctx, &rem); err != nil {
			log.Printf("[RemindersWorker] Failed to deliver reminder %d (user %d): %v", rem.ID, rem.UserID, err)
			continue
		}
		processed++
	}

	return processed, nil
}

func (w *Worker) deliverReminder(ctx context.Context, rem *store.Reminder) error {
	// 1. Rate limiter wait
	if err := w.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	// 2. Resolve platform and user identity
	targetPlatform := "max"
	adapter, ok := w.adapters[targetPlatform]
	if !ok {
		// Fallback to any registered adapter
		for p, a := range w.adapters {
			targetPlatform = p
			adapter = a
			break
		}
	}
	if adapter == nil {
		return fmt.Errorf("no platform adapter available for reminder delivery")
	}

	ident, err := w.sqlStore.GetUserIdentity(ctx, rem.UserID, targetPlatform)
	if err != nil || ident == nil || ident.PlatformUserID == "" {
		// Try fallback to telegram identity
		ident, err = w.sqlStore.GetUserIdentity(ctx, rem.UserID, "telegram")
		if err == nil && ident != nil && ident.PlatformUserID != "" {
			targetPlatform = "telegram"
			adapter = w.adapters["telegram"]
		}
	}
	if ident == nil || ident.PlatformUserID == "" || adapter == nil {
		return fmt.Errorf("user identity not found for user %d on platform %s", rem.UserID, targetPlatform)
	}

	// 3. Render message text
	text := w.renderMessageText(ctx, rem)

	// 4. Assemble action buttons
	buttons := [][]platform.Button{
		{
			{
				Text:   "✅ Выполнено",
				Type:   platform.ButtonTypeCallback,
				Action: fmt.Sprintf("rmd:done:%d", rem.ID),
			},
			{
				Text:   "⏰ +15 мин",
				Type:   platform.ButtonTypeCallback,
				Action: fmt.Sprintf("rmd:snz:15:%d", rem.ID),
			},
		},
		{
			{
				Text:   "⏰ +1 час",
				Type:   platform.ButtonTypeCallback,
				Action: fmt.Sprintf("rmd:snz:60:%d", rem.ID),
			},
			{
				Text:   "❌ Удалить",
				Type:   platform.ButtonTypeCallback,
				Action: fmt.Sprintf("rmd:del:%d", rem.ID),
			},
		},
	}

	outMsg := &platform.OutMessage{
		Text:      text,
		Buttons:   buttons,
		ParseMode: "markdown",
	}

	// 5. Send message
	if _, err := adapter.SendMessage(ctx, ident.PlatformUserID, outMsg); err != nil {
		return fmt.Errorf("adapter.SendMessage error: %w", err)
	}

	// 6. Record sent event for telemetry & A/B tracking
	_ = w.sqlStore.RecordNotificationEvent(ctx, &store.NotificationEvent{
		ReminderID: &rem.ID,
		TemplateID: rem.TemplateID,
		UserID:     rem.UserID,
		Variant:    rem.ABVariant,
		EventType:  "sent",
	})

	// 7. Update reminder status to 'sent'
	if err := w.sqlStore.UpdateReminderStatus(ctx, rem.ID, "sent"); err != nil {
		return fmt.Errorf("failed to update reminder status to sent: %w", err)
	}

	return nil
}

func (w *Worker) renderMessageText(ctx context.Context, rem *store.Reminder) string {
	if rem.TemplateID != "" {
		tmpl, err := w.sqlStore.GetNotificationTemplate(ctx, rem.TemplateID)
		if err == nil && tmpl != nil {
			var baseText string
			if rem.ABVariant == "B" && tmpl.VariantBText != "" {
				baseText = tmpl.VariantBText
			} else {
				baseText = tmpl.VariantAText
			}

			// Variable replacements
			serviceTitle := rem.Title
			if strings.HasPrefix(serviceTitle, "Визит: ") {
				serviceTitle = strings.TrimPrefix(serviceTitle, "Визит: ")
			}
			baseText = strings.ReplaceAll(baseText, "{service_title}", serviceTitle)
			baseText = strings.ReplaceAll(baseText, "{title}", rem.Title)
			baseText = strings.ReplaceAll(baseText, "{slot_time}", rem.Description)

			return "🔔 " + baseText
		}
	}

	// Personal reminder fallback
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⏰ **Напоминание:** %s\n", rem.Title))
	if rem.Description != "" {
		sb.WriteString(fmt.Sprintf("📝 %s\n", rem.Description))
	}
	if rem.RecurrenceRule != "" && rem.RecurrenceRule != "none" {
		ruleName := rem.RecurrenceRule
		switch ruleName {
		case "daily":
			ruleName = "каждый день"
		case "weekdays":
			ruleName = "по будням"
		case "weekly":
			ruleName = "раз в неделю"
		}
		sb.WriteString(fmt.Sprintf("🔁 *Повтор:* %s\n", ruleName))
	}
	return sb.String()
}
