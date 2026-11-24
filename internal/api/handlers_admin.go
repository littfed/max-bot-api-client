package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type adminHandler struct {
	cfg RouterConfig
}

func newAdminHandler(cfg RouterConfig) *adminHandler {
	return &adminHandler{cfg: cfg}
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	return r.RemoteAddr
}

func (h *adminHandler) handleAdminModes(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not available")
		return
	}

	configs, err := h.cfg.Store.GetModeConfigs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get mode configs: %v", err))
		return
	}

	type adminModeItem struct {
		ID                 string `json:"id"`
		ModeID             string `json:"mode_id"`
		Title              string `json:"title"`
		IsActive           bool   `json:"is_active"`
		MaintenanceMessage string `json:"maintenance_message"`
		DailyQuota         int    `json:"daily_quota"`
	}

	modes := make([]adminModeItem, len(configs))
	for i, c := range configs {
		modes[i] = adminModeItem{
			ID:                 c.ModeID,
			ModeID:             c.ModeID,
			Title:              c.Title,
			IsActive:           c.IsActive,
			MaintenanceMessage: c.MaintenanceMessage,
			DailyQuota:         c.DailyQuota,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"modes": modes,
	})
}

func (h *adminHandler) handleAdminToggleMode(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.AdminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	modeID := r.PathValue("id")
	if modeID == "" {
		writeError(w, http.StatusBadRequest, "mode id is required")
		return
	}

	var req struct {
		IsActive           bool   `json:"is_active"`
		MaintenanceMessage string `json:"maintenance_message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := extractIP(r)
	if err := h.cfg.AdminService.ToggleMode(r.Context(), user.ID, modeID, req.IsActive, req.MaintenanceMessage, ip); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to toggle mode: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (h *adminHandler) handleAdminLeads(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.AdminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	status := r.URL.Query().Get("status")
	leads, err := h.cfg.AdminService.ListLeads(r.Context(), user.ID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list leads: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"leads": leads,
	})
}

func (h *adminHandler) handleAdminLeadStatus(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.AdminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	idStr := r.PathValue("id")
	leadID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid lead id")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}

	ip := extractIP(r)
	if err := h.cfg.AdminService.UpdateLeadStatus(r.Context(), user.ID, leadID, req.Status, ip); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update lead status: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (h *adminHandler) handleAdminTickets(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.AdminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	status := r.URL.Query().Get("status")
	tickets, err := h.cfg.AdminService.ListTickets(r.Context(), user.ID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list tickets: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tickets": tickets,
	})
}

func (h *adminHandler) handleAdminTicketReply(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.AdminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	idStr := r.PathValue("id")
	ticketID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	ip := extractIP(r)
	if err := h.cfg.AdminService.ReplyTicket(r.Context(), user.ID, ticketID, req.Message, ip); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to reply to ticket: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (h *adminHandler) handleAdminAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.AdminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, err := h.cfg.AdminService.GetAuditLogs(r.Context(), user.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get audit logs: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"audit_logs": logs,
	})
}
