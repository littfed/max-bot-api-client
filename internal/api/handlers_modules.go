package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type modulesHandler struct {
	cfg RouterConfig
}

func newModulesHandler(cfg RouterConfig) *modulesHandler {
	return &modulesHandler{cfg: cfg}
}

// --- Shop Module Handlers ---

func (h *modulesHandler) handleShopCatalog(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ShopModule == nil {
		writeError(w, http.StatusServiceUnavailable, "shop module not available")
		return
	}

	products, err := h.cfg.ShopModule.GetCatalog(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get catalog: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"products": products,
	})
}

func (h *modulesHandler) handleShopCreateOrder(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ShopModule == nil {
		writeError(w, http.StatusServiceUnavailable, "shop module not available")
		return
	}

	var req struct {
		ProductID string `json:"product_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	order, err := h.cfg.ShopModule.CreateOrder(r.Context(), user.ID, req.ProductID)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to create order: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"order": order,
	})
}

func (h *modulesHandler) handleShopSimulate(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	if orderID == "" {
		writeError(w, http.StatusBadRequest, "order id is required")
		return
	}
	if h.cfg.ShopModule == nil {
		writeError(w, http.StatusServiceUnavailable, "shop module not available")
		return
	}

	var req struct {
		Action string `json:"action"` // "paid" or "abandoned"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.cfg.ShopModule.SimulatePayment(r.Context(), orderID, req.Action)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("payment simulation failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result": result,
	})
}

// --- Reception Module Handlers ---

func (h *modulesHandler) handleReceptionSlots(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ReceptionModule == nil {
		writeError(w, http.StatusServiceUnavailable, "reception module not available")
		return
	}

	serviceID := r.URL.Query().Get("service_id")
	if serviceID == "" {
		serviceID = "srv_consult"
	}

	dateStr := r.URL.Query().Get("date")
	var targetDate time.Time
	if dateStr != "" {
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			targetDate = t
		} else if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			targetDate = t
		}
	}
	if targetDate.IsZero() {
		targetDate = time.Now().Add(24 * time.Hour)
	}

	slots, err := h.cfg.ReceptionModule.GetSlots(r.Context(), serviceID, targetDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get slots: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service_id": serviceID,
		"slots":      slots,
	})
}

func (h *modulesHandler) handleReceptionBook(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ReceptionModule == nil {
		writeError(w, http.StatusServiceUnavailable, "reception module not available")
		return
	}

	var req struct {
		ServiceID string `json:"service_id"`
		SlotTime  string `json:"slot_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	slotTime, err := time.Parse(time.RFC3339, req.SlotTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid slot_time format, expected RFC3339")
		return
	}

	if err := h.cfg.ReceptionModule.BookSlot(r.Context(), user.ID, req.ServiceID, slotTime); err != nil {
		if strings.Contains(err.Error(), "already booked") || strings.Contains(err.Error(), "concurrent") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Бронирование подтверждено",
	})
}

func (h *modulesHandler) handleReceptionEvents(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ReceptionModule == nil {
		writeError(w, http.StatusServiceUnavailable, "reception module not available")
		return
	}

	events, err := h.cfg.ReceptionModule.GetEvents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get events: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
	})
}

func (h *modulesHandler) handleReceptionRegisterEvent(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ReceptionModule == nil {
		writeError(w, http.StatusServiceUnavailable, "reception module not available")
		return
	}

	eventID := r.PathValue("id")
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "event id is required")
		return
	}

	reg, err := h.cfg.ReceptionModule.RegisterEvent(r.Context(), user.ID, eventID)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("registration failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registration": reg,
	})
}

func (h *modulesHandler) handleReceptionOnboarding(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ReceptionModule == nil {
		writeError(w, http.StatusServiceUnavailable, "reception module not available")
		return
	}

	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.cfg.ReceptionModule.SubmitOnboarding(r.Context(), user.ID, req.Answers); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to submit onboarding: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

// --- Support Module Handlers ---

func (h *modulesHandler) handleSupportAsk(w http.ResponseWriter, r *http.Request) {
	if h.cfg.SupportModule == nil {
		writeError(w, http.StatusServiceUnavailable, "support module not available")
		return
	}

	var req struct {
		Question  string `json:"question"`
		ModeScope string `json:"mode_scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}
	if req.ModeScope == "" {
		req.ModeScope = "global"
	}

	answer, score, err := h.cfg.SupportModule.Ask(r.Context(), req.Question, req.ModeScope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("support ask failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"answer":       answer,
		"score":        score,
		"can_escalate": score < 0.72,
	})
}

func (h *modulesHandler) handleSupportCreateTicket(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.SupportModule == nil {
		writeError(w, http.StatusServiceUnavailable, "support module not available")
		return
	}

	var req struct {
		Subject   string `json:"subject"`
		Message   string `json:"message"`
		ModeScope string `json:"mode_scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Subject == "" {
		req.Subject = "Вопрос пользователя"
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	if req.ModeScope == "" {
		req.ModeScope = "global"
	}

	platformName := PlatformFromContext(r.Context())
	ticket, err := h.cfg.SupportModule.CreateTicket(r.Context(), user.ID, platformName, req.ModeScope, req.Subject, req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create ticket: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ticket": ticket,
	})
}

func (h *modulesHandler) handleSupportGetTickets(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.SupportModule == nil {
		writeError(w, http.StatusServiceUnavailable, "support module not available")
		return
	}

	tickets, err := h.cfg.SupportModule.GetTickets(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get tickets: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tickets": tickets,
	})
}

// --- Chat Module Handlers ---

func (h *modulesHandler) handleChatConfig(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ChatModule == nil {
		writeError(w, http.StatusServiceUnavailable, "chat module not available")
		return
	}

	roles, tones, err := h.cfg.ChatModule.GetConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get chat config: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"roles": roles,
		"tones": tones,
	})
}

func (h *modulesHandler) handleChatMessage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ChatModule == nil {
		writeError(w, http.StatusServiceUnavailable, "chat module not available")
		return
	}

	var req struct {
		RoleID  int    `json:"role_id"`
		ToneID  int    `json:"tone_id"`
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

	reply, err := h.cfg.ChatModule.SendMessage(r.Context(), user.ID, req.RoleID, req.ToneID, req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send chat message: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"reply": reply,
	})
}

func (h *modulesHandler) handleChatClear(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ChatModule == nil {
		writeError(w, http.StatusServiceUnavailable, "chat module not available")
		return
	}

	if err := h.cfg.ChatModule.ResetSession(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to clear chat session: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}
