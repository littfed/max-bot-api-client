package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/pkg/store"
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
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orderID := r.PathValue("id")
	if orderID == "" {
		writeError(w, http.StatusBadRequest, "order id is required")
		return
	}
	if h.cfg.ShopModule == nil {
		writeError(w, http.StatusServiceUnavailable, "shop module not available")
		return
	}

	ord, err := h.cfg.ShopModule.GetOrder(r.Context(), orderID)
	if err != nil || ord == nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if ord.UserID != user.ID && user.Role != "admin" && user.Role != "superadmin" {
		writeError(w, http.StatusForbidden, "forbidden")
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
		writeError(w, http.StatusBadRequest, "payment simulation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result": result,
	})
}

func (h *modulesHandler) handleShopUserOrders(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ShopModule == nil {
		writeError(w, http.StatusServiceUnavailable, "shop module not available")
		return
	}

	orders, err := h.cfg.ShopModule.GetUserOrders(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get user orders: %v", err))
		return
	}
	if orders == nil {
		orders = []*shop.Order{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"orders": orders,
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

func (h *modulesHandler) handleReceptionUserBookings(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ReceptionModule == nil {
		writeError(w, http.StatusServiceUnavailable, "reception module not available")
		return
	}

	bookings, err := h.cfg.ReceptionModule.GetUserBookings(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get user bookings: %v", err))
		return
	}
	if bookings == nil {
		bookings = []store.Booking{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bookings": bookings,
	})
}

func (h *modulesHandler) handleReceptionUserRegistrations(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ReceptionModule == nil {
		writeError(w, http.StatusServiceUnavailable, "reception module not available")
		return
	}

	regs, err := h.cfg.ReceptionModule.GetUserRegistrations(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get user registrations: %v", err))
		return
	}
	if regs == nil {
		regs = []*store.EventRegistration{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registrations": regs,
	})
}

func (h *modulesHandler) handleReceptionGetOnboarding(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ReceptionModule == nil {
		writeError(w, http.StatusServiceUnavailable, "reception module not available")
		return
	}

	sub, err := h.cfg.ReceptionModule.GetUserOnboarding(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get user onboarding: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"submission": sub,
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

	// Persist ticket into SQL store if available so AdminService reads the same tickets
	var ticketID int64
	if h.cfg.Store != nil {
		sqlTicket := &store.SupportTicket{
			UserID:    user.ID,
			Platform:  platformName,
			ModeScope: req.ModeScope,
			Subject:   req.Subject,
			Status:    "open",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		msg := &store.SupportMessage{
			SenderType:  "user",
			MessageText: req.Message,
			CreatedAt:   time.Now(),
		}
		var err error
		ticketID, err = h.cfg.Store.CreateSupportTicketWithMessage(r.Context(), sqlTicket, msg)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save support ticket: %v", err))
			return
		}
	}

	// Also register in support module for in-memory tracking
	ticket, err := h.cfg.SupportModule.CreateTicket(r.Context(), user.ID, platformName, req.ModeScope, req.Subject, req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create ticket: %v", err))
		return
	}
	if ticketID > 0 {
		ticket.ID = ticketID
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

	// 1. Fetch from SQL store if available
	if h.cfg.Store != nil {
		storeTickets, err := h.cfg.Store.GetSupportTickets(r.Context(), "")
		if err == nil {
			userTickets := make([]store.SupportTicket, 0)
			for _, t := range storeTickets {
				if t.UserID == user.ID {
					userTickets = append(userTickets, t)
				}
			}
			if len(userTickets) > 0 {
				writeJSON(w, http.StatusOK, map[string]any{
					"tickets": userTickets,
				})
				return
			}
		}
	}

	// 2. Fallback to in-memory support module
	if h.cfg.SupportModule != nil {
		tickets, err := h.cfg.SupportModule.GetTickets(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get tickets: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tickets": tickets,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tickets": []any{},
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

func (h *modulesHandler) handleChatSession(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.cfg.ChatModule == nil {
		writeError(w, http.StatusServiceUnavailable, "chat module not available")
		return
	}

	session, err := h.cfg.ChatModule.GetSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get chat session: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"role_id":  session.RoleID,
		"tone_id":  session.ToneID,
		"messages": session.Messages,
	})
}

func (h *modulesHandler) handleChatSettings(w http.ResponseWriter, r *http.Request) {
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
		RoleID int `json:"role_id"`
		ToneID int `json:"tone_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.cfg.ChatModule.SetSessionSettings(r.Context(), user.ID, req.RoleID, req.ToneID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update chat settings: %v", err))
		return
	}

	if h.cfg.FSM != nil {
		if payload, err := h.cfg.ChatModule.GetSessionPayload(r.Context(), user.ID); err == nil && len(payload) > 0 {
			_ = h.cfg.FSM.Set(r.Context(), user.ID, "chat", "", payload)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"role_id": req.RoleID,
		"tone_id": req.ToneID,
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

	// Trigger typing indicator in messenger (MAX / Telegram) while AI processes the prompt
	if h.cfg.PlatformAdapter != nil {
		recipientID := strconv.FormatInt(user.ID, 10)
		platformName := PlatformFromContext(r.Context())
		if h.cfg.Store != nil {
			if ident, err := h.cfg.Store.GetUserIdentity(r.Context(), user.ID, platformName); err == nil && ident != nil && ident.PlatformUserID != "" {
				recipientID = ident.PlatformUserID
			}
		}
		reqCtx := r.Context()
		_ = h.cfg.PlatformAdapter.SendChatAction(reqCtx, recipientID, "typing_on")
		stopTyping := make(chan struct{})
		defer close(stopTyping)
		go func(ctx context.Context, recID string) {
			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopTyping:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = h.cfg.PlatformAdapter.SendChatAction(ctx, recID, "typing_on")
				}
			}
		}(reqCtx, recipientID)
	}

	reply, err := h.cfg.ChatModule.SendMessage(r.Context(), user.ID, req.RoleID, req.ToneID, req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send chat message: %v", err))
		return
	}

	if h.cfg.FSM != nil {
		if payload, err := h.cfg.ChatModule.GetSessionPayload(r.Context(), user.ID); err == nil && len(payload) > 0 {
			_ = h.cfg.FSM.Set(r.Context(), user.ID, "chat", "", payload)
		}
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

	if h.cfg.FSM != nil {
		if payload, err := h.cfg.ChatModule.GetSessionPayload(r.Context(), user.ID); err == nil && len(payload) > 0 {
			_ = h.cfg.FSM.Set(r.Context(), user.ID, "chat", "", payload)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

