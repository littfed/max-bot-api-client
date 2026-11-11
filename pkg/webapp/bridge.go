package webapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ActionHandler processes an authenticated Mini App action.
type ActionHandler func(ctx context.Context, data *InitData, payload json.RawMessage) (any, error)

// Bridge handles HTTP communication between the client-side Mini App WebView and the bot server.
type Bridge struct {
	botToken string
	maxAge   time.Duration
	handlers map[string]ActionHandler
	mu       sync.RWMutex
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
				Message: fmt.Sprintf("invalid json body: %v", err),
			})
			return
		}

		// Authenticate and verify initData
		initData, err := ValidateInitData(req.InitData, b.botToken, b.maxAge)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(ActionResponse{
				Success: false,
				Message: fmt.Sprintf("unauthorized: %v", err),
			})
			return
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
