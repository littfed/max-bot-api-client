package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/littfed/max-bot-api-client/internal/modules/reminders"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type remindersHandler struct {
	cfg RouterConfig
}

func newRemindersHandler(cfg RouterConfig) *remindersHandler {
	return &remindersHandler{cfg: cfg}
}

type createReminderRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	RemindAt       string `json:"remind_at"`
	RecurrenceRule string `json:"recurrence_rule,omitempty"`
}

type reminderActionRequest struct {
	Action  string `json:"action"` // "complete", "snooze", "cancel"
	Minutes int    `json:"minutes,omitempty"`
}

func (h *remindersHandler) getService() *reminders.Service {
	if h.cfg.RemindersModule != nil {
		return h.cfg.RemindersModule.Service()
	}
	return nil
}

func (h *remindersHandler) handleListReminders(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	svc := h.getService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "reminders module not configured")
		return
	}

	statusFilter := r.URL.Query().Get("status")
	list, err := svc.GetReminders(r.Context(), user.ID, statusFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list reminders: %v", err))
		return
	}
	if list == nil {
		list = []store.Reminder{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"reminders": list,
	})
}

func (h *remindersHandler) handleCreateReminder(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	svc := h.getService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "reminders module not configured")
		return
	}

	var req createReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	remindAt, err := time.Parse(time.RFC3339, req.RemindAt)
	if err != nil {
		// Try parsing without timezone using local time
		loc, _ := time.LoadLocation("Europe/Moscow")
		parsed, parseErr := reminders.ParseTimeInput(req.RemindAt, time.Now(), loc)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid remind_at format: %v", err))
			return
		}
		remindAt = parsed
	}

	rem, err := svc.CreatePersonalReminder(r.Context(), user.ID, req.Title, req.Description, remindAt, req.RecurrenceRule)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create reminder: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"reminder": rem,
	})
}

func (h *remindersHandler) handleReminderAction(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	svc := h.getService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "reminders module not configured")
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid reminder id")
		return
	}

	var req reminderActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	loc, _ := time.LoadLocation("Europe/Moscow")
	if user.Timezone != "" {
		if l, err := time.LoadLocation(user.Timezone); err == nil {
			loc = l
		}
	}

	switch req.Action {
	case "complete":
		completed, next, err := svc.CompleteReminderForUser(r.Context(), id, user.ID, loc)
		if err != nil {
			writeReminderServiceError(w, "failed to complete reminder", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success":   true,
			"completed": completed,
			"next":      next,
		})

	case "snooze":
		mins := req.Minutes
		if mins <= 0 {
			mins = 15
		}
		snoozed, err := svc.SnoozeReminderForUser(r.Context(), id, user.ID, mins)
		if err != nil {
			writeReminderServiceError(w, "failed to snooze reminder", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"snoozed": snoozed,
		})

	case "cancel":
		if err := svc.CancelReminderForUser(r.Context(), id, user.ID); err != nil {
			writeReminderServiceError(w, "failed to cancel reminder", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
		})

	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported action: %s", req.Action))
	}
}

func (h *remindersHandler) handleDeleteReminder(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	svc := h.getService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "reminders module not configured")
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid reminder id")
		return
	}

	if err := svc.CancelReminderForUser(r.Context(), id, user.ID); err != nil {
		writeReminderServiceError(w, "failed to delete reminder", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func writeReminderServiceError(w http.ResponseWriter, prefix string, err error) {
	switch {
	case errors.Is(err, reminders.ErrReminderForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, reminders.ErrReminderNotFound):
		writeError(w, http.StatusNotFound, "reminder not found")
	case errors.Is(err, reminders.ErrReminderNotActionable):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("%s: %v", prefix, err))
	}
}

func (h *remindersHandler) handleSimulateReminder(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	svc := h.getService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "reminders module not configured")
		return
	}

	rem, err := svc.ScheduleDemoSimulation(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to schedule demo simulation: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"reminder": rem,
	})
}

func (h *remindersHandler) handleAdminABStats(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "database store not available")
		return
	}

	stats, err := h.cfg.Store.GetNotificationABStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to query AB stats: %v", err))
		return
	}
	if stats == nil {
		stats = []store.NotificationABStat{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"stats": stats,
	})
}
