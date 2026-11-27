package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

type hubHandler struct {
	cfg RouterConfig
}

func newHubHandler(cfg RouterConfig) *hubHandler {
	return &hubHandler{cfg: cfg}
}

// ModeItem represents a Bot Hub mode and its availability status.
type ModeItem struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	IsActive           bool   `json:"is_active"`
	MaintenanceMessage string `json:"maintenance_message"`
	DailyQuota         int    `json:"daily_quota"`
}

var default8Modes = []ModeItem{
	{ID: "shop", Title: "Магазин", IsActive: true, DailyQuota: 100},
	{ID: "reception", Title: "Ресепшен", IsActive: true, DailyQuota: 100},
	{ID: "reminders", Title: "Напоминания", IsActive: false, MaintenanceMessage: "Модуль находится в разработке (Этап 2)", DailyQuota: 100},
	{ID: "channels", Title: "Каналы", IsActive: false, MaintenanceMessage: "Модуль находится в разработке (Этап 2)", DailyQuota: 100},
	{ID: "posts", Title: "Контент-студия", IsActive: true, DailyQuota: 50},
	{ID: "giveaways", Title: "Конкурсы", IsActive: false, MaintenanceMessage: "Модуль находится в разработке (Этап 2)", DailyQuota: 100},
	{ID: "support", Title: "Поддержка", IsActive: true, DailyQuota: 100},
	{ID: "docs", Title: "Документы", IsActive: false, MaintenanceMessage: "Модуль находится в разработке (Этап 2)", DailyQuota: 100},
	{ID: "chat", Title: "Общение", IsActive: true, DailyQuota: 100},
}

func (h *hubHandler) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := UserFromContext(ctx)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// 1. Fetch Mode Configs
	modeMap := make(map[string]ModeItem)
	for _, m := range default8Modes {
		modeMap[m.ID] = m
	}

	if h.cfg.Store != nil {
		if configs, err := h.cfg.Store.GetModeConfigs(ctx); err == nil && len(configs) > 0 {
			for _, c := range configs {
				modeMap[c.ModeID] = ModeItem{
					ID:                 c.ModeID,
					Title:              c.Title,
					IsActive:           c.IsActive,
					MaintenanceMessage: c.MaintenanceMessage,
					DailyQuota:         c.DailyQuota,
				}
			}
		}
	}

	// Overlay dynamic guard status
	modes := make([]ModeItem, 0, len(default8Modes))
	availableCount := 0
	for _, def := range default8Modes {
		item := modeMap[def.ID]
		if h.cfg.Guard != nil {
			active, msg, err := h.cfg.Guard.IsActive(ctx, item.ID)
			if err == nil {
				item.IsActive = active
				if msg != "" {
					item.MaintenanceMessage = msg
				}
			}
		}
		if item.IsActive {
			availableCount++
		}
		modes = append(modes, item)
	}

	// 2. Select Mascot Phrase
	phraseText := "Привет! Я бот-маскот MAX. Нажмите на любой режим, чтобы начать тестирование."
	priority := 4
	emotion := "friendly"

	if h.cfg.Store != nil {
		if phrases, err := h.cfg.Store.GetMascotPhrases(ctx); err == nil && len(phrases) > 0 {
			// Find phrase with lowest priority number (1 is highest priority)
			for _, p := range phrases {
				if p.IsActive && p.Priority < priority {
					phraseText = p.Text
					priority = p.Priority
					if priority == 1 {
						emotion = "alert"
					} else if priority == 2 {
						emotion = "excited"
					} else {
						emotion = "friendly"
					}
				}
			}
		}
	}

	// 3. Generate JWT Token
	platformName := PlatformFromContext(ctx)
	token, _ := GenerateJWT(JWTClaims{
		UserID:    user.ID,
		Role:      user.Role,
		Platform:  platformName,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, h.cfg.JWTSecret)

	activeMode := user.ActiveModeID
	if activeMode == "" {
		activeMode = "shop"
	}

	hasAdmin := user.Role == "admin" || user.Role == "superadmin"

	writeJSON(w, http.StatusOK, map[string]any{
		"user":        user,
		"token":       token,
		"active_mode": activeMode,
		"modes":       modes,
		"mascot": map[string]any{
			"phrase":   phraseText,
			"priority": priority,
			"emotion":  emotion,
		},
		"capabilities": map[string]any{
			"can_switch_mode":       true,
			"has_admin_access":      hasAdmin,
			"available_modes_count": availableCount,
			"total_modes_count":     len(modes),
		},
	})
}

func (h *hubHandler) handleUserMode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := UserFromContext(ctx)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ModeID string `json:"mode_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ModeID = strings.TrimSpace(req.ModeID)
	if req.ModeID == "" {
		writeError(w, http.StatusBadRequest, "mode_id is required")
		return
	}

	// Validate mode ID
	isValid := false
	for _, m := range default8Modes {
		if m.ID == req.ModeID {
			isValid = true
			break
		}
	}
	if !isValid {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid mode_id: %s", req.ModeID))
		return
	}

	// Check maintenance guard
	if h.cfg.Guard != nil {
		active, maintMsg, err := h.cfg.Guard.IsActive(ctx, req.ModeID)
		if err == nil && !active {
			if maintMsg == "" {
				maintMsg = "Режим временно недоступен (техническое обслуживание)"
			}
			writeError(w, http.StatusForbidden, maintMsg)
			return
		}
	}

	// Check if mode is actually changing
	modeChanged := user.ActiveModeID != req.ModeID

	// Update user active mode in DB if changed
	if modeChanged {
		if h.cfg.Store != nil {
			if err := h.cfg.Store.SetUserActiveMode(ctx, user.ID, req.ModeID); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update user mode: %v", err))
				return
			}
		}
		user.ActiveModeID = req.ModeID

		// Notification to user chat only on actual switch
		if h.cfg.PlatformAdapter != nil {
			recipientID := strconv.FormatInt(user.ID, 10)
			platformName := PlatformFromContext(ctx)
			if h.cfg.Store != nil {
				if ident, err := h.cfg.Store.GetUserIdentity(ctx, user.ID, platformName); err == nil && ident != nil && ident.PlatformUserID != "" {
					recipientID = ident.PlatformUserID
				}
			}

			modeSwitchMsgText := fmt.Sprintf("🔄 **Режим успешно переключен:** _%s_", req.ModeID)
			var lastSwitchMID string

			// Check if previous message was also a mode switch notification
			if h.cfg.FSM != nil {
				if hubState, err := h.cfg.FSM.Get(ctx, user.ID, "hub"); err == nil && hubState != nil && len(hubState.Payload) > 0 {
					var payload struct {
						LastSwitchMID string `json:"last_switch_mid"`
					}
					if err := json.Unmarshal(hubState.Payload, &payload); err == nil {
						lastSwitchMID = payload.LastSwitchMID
					}
				}
			}

			edited := false
			if lastSwitchMID != "" {
				err := h.cfg.PlatformAdapter.EditMessage(ctx, lastSwitchMID, &platform.OutMessage{
					Text: modeSwitchMsgText,
				})
				if err == nil {
					edited = true
				}
			}

			if !edited {
				sentMID, err := h.cfg.PlatformAdapter.SendMessage(ctx, recipientID, &platform.OutMessage{
					Text: modeSwitchMsgText,
				})
				if err == nil && sentMID != "" && h.cfg.FSM != nil {
					pBytes, _ := json.Marshal(map[string]any{
						"last_switch_mid": sentMID,
						"mode_id":         req.ModeID,
					})
					_ = h.cfg.FSM.Set(ctx, user.ID, "hub", "mode_switched", pBytes)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"active_mode": req.ModeID,
	})
}

func (h *hubHandler) handleMascotTap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var tapPhrases []string
	if h.cfg.Store != nil {
		if phrases, err := h.cfg.Store.GetMascotPhrases(ctx); err == nil {
			for _, p := range phrases {
				if p.IsActive && p.Priority == 4 {
					tapPhrases = append(tapPhrases, p.Text)
				}
			}
		}
	}

	phrase := "Привет! Я бот-маскот MAX. Нажмите на любой режим, чтобы начать тестирование."
	if len(tapPhrases) > 0 {
		phrase = tapPhrases[rand.Intn(len(tapPhrases))]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"phrase":   phrase,
		"priority": 4,
		"emotion":  "happy",
	})
}

func (h *hubHandler) handleLead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := UserFromContext(ctx)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Contact     string          `json:"contact"`
		Comment     string          `json:"comment"`
		SourceMode  string          `json:"source_mode"`
		TestedModes json.RawMessage `json:"tested_modes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Contact = strings.TrimSpace(req.Contact)
	if req.Contact == "" {
		writeError(w, http.StatusBadRequest, "contact is required")
		return
	}

	if len(req.TestedModes) == 0 {
		req.TestedModes = json.RawMessage("[]")
	}

	lead := &store.Lead{
		UserID:      user.ID,
		Platform:    PlatformFromContext(ctx),
		SourceMode:  req.SourceMode,
		TestedModes: req.TestedModes,
		Contact:     req.Contact,
		Comment:     req.Comment,
		Status:      "new",
		CreatedAt:   time.Now(),
	}

	var leadID int64
	if h.cfg.Store != nil {
		var err error
		leadID, err = h.cfg.Store.SaveLead(ctx, lead)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save lead: %v", err))
			return
		}
	} else {
		leadID = time.Now().Unix()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"lead_id": leadID,
	})
}
