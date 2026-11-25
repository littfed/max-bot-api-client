package reminders

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type reminderDraft struct {
	Title    string `json:"title,omitempty"`
	RemindAt string `json:"remind_at,omitempty"`
}

func (m *Module) getUserLocation(u *store.User) *time.Location {
	tz := "Europe/Moscow"
	if u != nil && u.Timezone != "" {
		tz = u.Timezone
		if tz == "UTC+3" {
			tz = "Europe/Moscow"
		}
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("UTC+3", 3*3600)
	}
	return loc
}

func (m *Module) handleMessageFSM(ctx context.Context, u *store.User, state *core.State, msg *core.IncomingMessage) (*core.ModuleResult, error) {
	loc := m.getUserLocation(u)
	step := ""
	if state != nil {
		step = state.CurrentStep
	}

	switch step {
	case "wait_title":
		text := strings.TrimSpace(msg.Text)
		if len(text) < 2 {
			return &core.ModuleResult{
				Text: "⚠️ Текст напоминания слишком короткий. Напишите задачу подробнее (минимум 2 символа):",
				Buttons: [][]core.ResponseButton{
					{{Text: "❌ Отмена", Data: "rmd:cancel_creation"}},
				},
				Handled: true,
			}, nil
		}
		if len(text) > 255 {
			text = text[:255]
		}

		draft := reminderDraft{Title: text}
		payloadBytes, _ := json.Marshal(draft)

		return &core.ModuleResult{
			Text: fmt.Sprintf("📌 **Задача:** %s\n\n⏰ Выберите время напоминания или введите дату/время сообщением (например, `18:30`, `завтра 10:00`, `+30м`):", text),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "⏰ +15 мин", Data: "rmd:time:+15m"},
					{Text: "⏰ +1 час", Data: "rmd:time:+1h"},
					{Text: "⏰ +3 часа", Data: "rmd:time:+3h"},
				},
				{
					{Text: "🌙 Вечером (19:00)", Data: "rmd:time:19:00"},
					{Text: "☀️ Завтра в 10:00", Data: "rmd:time:tomorrow_10:00"},
				},
				{
					{Text: "❌ Отмена", Data: "rmd:cancel_creation"},
				},
			},
			NewStep:    "wait_time",
			NewPayload: payloadBytes,
			Handled:    true,
		}, nil

	case "wait_time":
		var draft reminderDraft
		if state != nil && len(state.Payload) > 0 {
			_ = json.Unmarshal(state.Payload, &draft)
		}
		if draft.Title == "" {
			draft.Title = "Напоминание"
		}

		parsedTime, err := ParseTimeInput(msg.Text, time.Now(), loc)
		if err != nil {
			return &core.ModuleResult{
				Text: fmt.Sprintf("⚠️ %v\n\nПожалуйста, введите время в понятном формате (например: `18:30`, `завтра 14:00`, `+15м`):", err),
				Buttons: [][]core.ResponseButton{
					{
						{Text: "⏰ +15 мин", Data: "rmd:time:+15m"},
						{Text: "⏰ +1 час", Data: "rmd:time:+1h"},
					},
					{
						{Text: "❌ Отмена", Data: "rmd:cancel_creation"},
					},
				},
				Handled: true,
			}, nil
		}

		draft.RemindAt = parsedTime.Format(time.RFC3339)
		payloadBytes, _ := json.Marshal(draft)

		return &core.ModuleResult{
			Text: fmt.Sprintf("📌 **Задача:** %s\n⏰ **Срок:** %s (МСК)\n\n🔁 Выберите правило повтора:", draft.Title, parsedTime.In(loc).Format("02.01 в 15:04")),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "1️⃣ Однократно", Data: "rmd:rec:none"},
					{Text: "🔁 Каждый день", Data: "rmd:rec:daily"},
				},
				{
					{Text: "💼 По будням (Пн-Пт)", Data: "rmd:rec:weekdays"},
					{Text: "📅 Раз в неделю", Data: "rmd:rec:weekly"},
				},
				{
					{Text: "❌ Отмена", Data: "rmd:cancel_creation"},
				},
			},
			NewStep:    "wait_recurrence",
			NewPayload: payloadBytes,
			Handled:    true,
		}, nil

	default:
		// Default main menu
		return m.renderMainMenu(ctx, u)
	}
}

func (m *Module) handleCallbackFSM(ctx context.Context, u *store.User, state *core.State, cb *core.IncomingCallback) (*core.ModuleResult, error) {
	loc := m.getUserLocation(u)
	data := cb.Data

	switch {
	case data == "rmd:menu":
		return m.renderMainMenu(ctx, u)

	case data == "rmd:create":
		return &core.ModuleResult{
			Text: "✍️ **Новое напоминание**\n\nО чём вам напомнить? Напишите текст задачи (например, «Позвонить клиенту», «Принять витамины»):",
			Buttons: [][]core.ResponseButton{
				{{Text: "❌ Отмена", Data: "rmd:cancel_creation"}},
			},
			NewStep: "wait_title",
			Handled: true,
		}, nil

	case data == "rmd:cancel_creation":
		res, err := m.renderMainMenu(ctx, u)
		if err != nil {
			return nil, err
		}
		res.ResetState = true
		res.Text = "❌ Создание напоминания отменено.\n\n" + res.Text
		return res, nil

	case strings.HasPrefix(data, "rmd:time:"):
		timePreset := strings.TrimPrefix(data, "rmd:time:")
		var draft reminderDraft
		if state != nil && len(state.Payload) > 0 {
			_ = json.Unmarshal(state.Payload, &draft)
		}
		if draft.Title == "" {
			draft.Title = "Напоминание"
		}

		var targetTime time.Time
		now := time.Now().In(loc)
		switch timePreset {
		case "+15m":
			targetTime = now.Add(15 * time.Minute)
		case "+1h":
			targetTime = now.Add(1 * time.Hour)
		case "+3h":
			targetTime = now.Add(3 * time.Hour)
		case "19:00":
			targetTime = time.Date(now.Year(), now.Month(), now.Day(), 19, 0, 0, 0, loc)
			if !targetTime.After(now) {
				targetTime = targetTime.AddDate(0, 0, 1)
			}
		case "tomorrow_10:00":
			targetTime = time.Date(now.Year(), now.Month(), now.Day()+1, 10, 0, 0, 0, loc)
		default:
			t, err := ParseTimeInput(timePreset, now, loc)
			if err != nil {
				targetTime = now.Add(1 * time.Hour)
			} else {
				targetTime = t
			}
		}

		draft.RemindAt = targetTime.Format(time.RFC3339)
		payloadBytes, _ := json.Marshal(draft)

		return &core.ModuleResult{
			Text: fmt.Sprintf("📌 **Задача:** %s\n⏰ **Срок:** %s (МСК)\n\n🔁 Выберите правило повтора:", draft.Title, targetTime.Format("02.01 в 15:04")),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "1️⃣ Однократно", Data: "rmd:rec:none"},
					{Text: "🔁 Каждый день", Data: "rmd:rec:daily"},
				},
				{
					{Text: "💼 По будням (Пн-Пт)", Data: "rmd:rec:weekdays"},
					{Text: "📅 Раз в неделю", Data: "rmd:rec:weekly"},
				},
				{
					{Text: "❌ Отмена", Data: "rmd:cancel_creation"},
				},
			},
			NewStep:    "wait_recurrence",
			NewPayload: payloadBytes,
			Handled:    true,
		}, nil

	case strings.HasPrefix(data, "rmd:rec:"):
		rule := strings.TrimPrefix(data, "rmd:rec:")
		var draft reminderDraft
		if state != nil && len(state.Payload) > 0 {
			_ = json.Unmarshal(state.Payload, &draft)
		}
		if draft.Title == "" {
			draft.Title = "Напоминание"
		}
		remindAt, err := time.Parse(time.RFC3339, draft.RemindAt)
		if err != nil || remindAt.IsZero() {
			remindAt = time.Now().In(loc).Add(1 * time.Hour)
		}

		rem, err := m.service.CreatePersonalReminder(ctx, u.ID, draft.Title, "", remindAt, rule)
		if err != nil {
			return &core.ModuleResult{
				Text:       fmt.Sprintf("❌ Ошибка при создании напоминания: %v", err),
				ResetState: true,
				Handled:    true,
			}, nil
		}

		ruleDisplay := "Однократно"
		switch rem.RecurrenceRule {
		case "daily":
			ruleDisplay = "Каждый день"
		case "weekdays":
			ruleDisplay = "По будням"
		case "weekly":
			ruleDisplay = "Раз в неделю"
		}

		return &core.ModuleResult{
			Text: fmt.Sprintf("✅ **Напоминание запланировано!**\n\n📌 **Задача:** %s\n⏰ **Срок:** %s (МСК)\n🔁 **Повтор:** %s",
				rem.Title, rem.RemindAt.In(loc).Format("02.01 в 15:04"), ruleDisplay),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "➕ Создать ещё", Data: "rmd:create"},
					{Text: "📋 Мои напоминания", Data: "rmd:list"},
				},
			},
			ResetState: true,
			Handled:    true,
		}, nil

	case data == "rmd:list":
		return m.renderList(ctx, u)

	case data == "rmd:demo_trigger":
		rem, err := m.service.ScheduleDemoSimulation(ctx, u.ID)
		if err != nil {
			return &core.ModuleResult{
				Text:    fmt.Sprintf("❌ Не удалось запустить демо: %v", err),
				Handled: true,
			}, nil
		}
		return &core.ModuleResult{
			Text: fmt.Sprintf("⚡ **Демо-триггер запущен!**\n\nТестовое напоминание `%s` запланировано на отправку через 10 секунд (A/B вариант `%s`). Ожидайте всплывающее пуш-сообщение!", rem.Title, rem.ABVariant),
			Buttons: [][]core.ResponseButton{
				{
					{Text: "📋 Мои напоминания", Data: "rmd:list"},
					{Text: "🏠 Главное меню", Data: "rmd:menu"},
				},
			},
			Handled: true,
		}, nil

	case strings.HasPrefix(data, "rmd:done:"):
		idStr := strings.TrimPrefix(data, "rmd:done:")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			return &core.ModuleResult{
				Text:    "❌ Некорректный идентификатор напоминания.",
				Handled: true,
			}, nil
		}
		_, nextRem, err := m.service.CompleteReminderForUser(ctx, id, u.ID, loc)
		if err != nil {
			return &core.ModuleResult{
				Text:    fmt.Sprintf("❌ Ошибка при выполнении задачи: %v", err),
				Handled: true,
			}, nil
		}

		var sb strings.Builder
		sb.WriteString("✅ **Отлично! Задача отмечена как выполненная.**")
		if nextRem != nil {
			sb.WriteString(fmt.Sprintf("\n\n🔁 Следующее напоминание автоматически запланировано на %s (МСК).", nextRem.RemindAt.In(loc).Format("02.01 в 15:04")))
		}
		return &core.ModuleResult{
			Text: sb.String(),
			Buttons: [][]core.ResponseButton{
				{{Text: "📋 Мои напоминания", Data: "rmd:list"}},
			},
			Handled: true,
		}, nil

	case strings.HasPrefix(data, "rmd:snz:"):
		// rmd:snz:<mins>:<id>
		parts := strings.Split(data, ":")
		if len(parts) >= 4 {
			mins, minsErr := strconv.Atoi(parts[2])
			id, idErr := strconv.ParseInt(parts[3], 10, 64)
			if minsErr != nil || idErr != nil || id <= 0 {
				return &core.ModuleResult{
					Text:    "❌ Некорректные параметры откладывания.",
					Handled: true,
				}, nil
			}
			rem, err := m.service.SnoozeReminderForUser(ctx, id, u.ID, mins)
			if err != nil {
				return &core.ModuleResult{
					Text:    fmt.Sprintf("❌ Ошибка: %v", err),
					Handled: true,
				}, nil
			}
			return &core.ModuleResult{
				Text: fmt.Sprintf("⏰ **Напоминание отложено на %d мин!**\nНовое время: %s (МСК)", mins, rem.RemindAt.In(loc).Format("15:04")),
				Buttons: [][]core.ResponseButton{
					{{Text: "📋 Мои напоминания", Data: "rmd:list"}},
				},
				Handled: true,
			}, nil
		}

	case strings.HasPrefix(data, "rmd:del:"):
		idStr := strings.TrimPrefix(data, "rmd:del:")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			return &core.ModuleResult{
				Text:    "❌ Некорректный идентификатор напоминания.",
				Handled: true,
			}, nil
		}
		if err := m.service.CancelReminderForUser(ctx, id, u.ID); err != nil {
			return &core.ModuleResult{
				Text:    fmt.Sprintf("❌ Ошибка при удалении: %v", err),
				Handled: true,
			}, nil
		}
		return &core.ModuleResult{
			Text: "🗑️ **Напоминание удалено.**",
			Buttons: [][]core.ResponseButton{
				{{Text: "📋 Мои напоминания", Data: "rmd:list"}},
			},
			Handled: true,
		}, nil
	}

	return m.renderMainMenu(ctx, u)
}

func (m *Module) renderMainMenu(ctx context.Context, u *store.User) (*core.ModuleResult, error) {
	return &core.ModuleResult{
		Text: "⏰ **Режим «Напоминания»** — ваш персональный тайм-менеджер и сервис уведомлений Bot Hub.\n\nСоздавайте быстрые задачи, настраивайте регулярные привычки или тестируйте триггеры!",
		Buttons: [][]core.ResponseButton{
			{
				{Text: "➕ Новое напоминание", Data: "rmd:create"},
				{Text: "📋 Мои напоминания", Data: "rmd:list"},
			},
			{
				{Text: "⚡ Тест триггера (демо 10 сек)", Data: "rmd:demo_trigger"},
			},
		},
		Handled: true,
	}, nil
}

func (m *Module) renderList(ctx context.Context, u *store.User) (*core.ModuleResult, error) {
	loc := m.getUserLocation(u)
	reminders, err := m.service.GetReminders(ctx, u.ID, "active")
	if err != nil {
		return nil, err
	}

	if len(reminders) == 0 {
		return &core.ModuleResult{
			Text: "📋 У вас пока нет активных напоминаний.\nНажмите «➕ Новое напоминание», чтобы запланировать задачу!",
			Buttons: [][]core.ResponseButton{
				{
					{Text: "➕ Новое напоминание", Data: "rmd:create"},
					{Text: "⚡ Демо 10 сек", Data: "rmd:demo_trigger"},
				},
			},
			Handled: true,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString("📋 **Ваши активные напоминания:**\n\n")

	var rows [][]core.ResponseButton
	for i, r := range reminders {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("_...и ещё %d задач_\n", len(reminders)-5))
			break
		}
		badge := "📌"
		if r.SourceType == "reception" {
			badge = "🛎️ [Ресепшен]"
		} else if r.SourceType == "demo_simulation" {
			badge = "⚡ [Демо]"
		}
		sb.WriteString(fmt.Sprintf("%s **%s**\n⏰ %s", badge, r.Title, r.RemindAt.In(loc).Format("02.01 в 15:04")))
		if r.RecurrenceRule != "" && r.RecurrenceRule != "none" {
			sb.WriteString(fmt.Sprintf(" (🔁 %s)", r.RecurrenceRule))
		}
		sb.WriteString("\n\n")

		rows = append(rows, []core.ResponseButton{
			{Text: fmt.Sprintf("✅ %s", r.Title), Data: fmt.Sprintf("rmd:done:%d", r.ID)},
			{Text: "🗑️", Data: fmt.Sprintf("rmd:del:%d", r.ID)},
		})
	}

	rows = append(rows, []core.ResponseButton{
		{Text: "➕ Добавить", Data: "rmd:create"},
		{Text: "🏠 Меню", Data: "rmd:menu"},
	})

	return &core.ModuleResult{
		Text:    sb.String(),
		Buttons: rows,
		Handled: true,
	}, nil
}
