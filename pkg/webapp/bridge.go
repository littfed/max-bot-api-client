package webapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// ActionHandler processes an authenticated Mini App action.
type ActionHandler func(ctx context.Context, data *InitData, payload json.RawMessage) (any, error)

// Bridge handles HTTP communication between the client-side Mini App WebView and the bot server.
// Bridge is safe for concurrent use by multiple goroutines.
type Bridge struct {
	botToken             string
	maxAge               time.Duration
	allowUnauthenticated bool
	handlers             map[string]ActionHandler
	mu                   sync.RWMutex
}

// NewBridge creates a new Mini App WebBridge.
func NewBridge(botToken string, maxAge ...time.Duration) *Bridge {
	age := 24 * time.Hour
	if len(maxAge) > 0 && maxAge[0] > 0 {
		age = maxAge[0]
	}

	return &Bridge{
		botToken: botToken,
		maxAge:   age,
		handlers: make(map[string]ActionHandler),
	}
}

// SetAllowUnauthenticated permits requests without valid HMAC signature (e.g. for demo showcase or direct browser testing).
func (b *Bridge) SetAllowUnauthenticated(allow bool) *Bridge {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.allowUnauthenticated = allow
	return b
}

// OnAction registers a handler for a specific Mini App action (e.g. "order_checkout", "select_item").
func (b *Bridge) OnAction(action string, handler ActionHandler) *Bridge {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[action] = handler
	return b
}

// HTTPHandler returns an http.HandlerFunc handling POST requests sent from the webview bridge.
func (b *Bridge) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(ActionResponse{
				Success: false,
				Message: "method not allowed",
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(ActionResponse{
				Success: false,
				Message: "failed to read request body",
			})
			return
		}

		var req ActionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(ActionResponse{
				Success: false,
				Message: "invalid json body",
			})
			return
		}

		// Authenticate and verify initData
		initData, err := ValidateInitData(req.InitData, b.botToken, b.maxAge)
		if err != nil {
			b.mu.RLock()
			allowUnauth := b.allowUnauthenticated
			b.mu.RUnlock()

			if !allowUnauth {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(ActionResponse{
					Success: false,
					Message: fmt.Sprintf("unauthorized: %v", err),
				})
				return
			}

			// In unauthenticated demo mode, provide fallback InitData
			initData = &InitData{
				Raw: req.InitData,
			}
			if vals, parseErr := url.ParseQuery(req.InitData); parseErr == nil {
				if userRaw := vals.Get("user"); userRaw != "" {
					var user InitDataUser
					if json.Unmarshal([]byte(userRaw), &user) == nil {
						initData.User = &user
					}
				}
				if uid, parseErr := strconv.ParseInt(vals.Get("user_id"), 10, 64); parseErr == nil && uid != 0 {
					if initData.User == nil {
						initData.User = &InitDataUser{ID: uid}
					}
				}
			}
		}

		b.mu.RLock()
		handler, ok := b.handlers[req.Action]
		b.mu.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(ActionResponse{
				Success: false,
				Message: fmt.Sprintf("action %q not recognized", req.Action),
			})
			return
		}

		result, err := handler(r.Context(), initData, req.Payload)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(ActionResponse{
				Success: false,
				Message: err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ActionResponse{
			Success: true,
			Data:    result,
		})
	}
}
