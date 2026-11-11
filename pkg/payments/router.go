package payments

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// PaymentHandler handles a validated payment lifecycle event.
type PaymentHandler func(ctx context.Context, payment *Payment) error

// Router dispatches incoming payment webhooks to registered gateway processors and fulfillment callbacks.
type Router struct {
	gateways   map[string]Gateway
	onSuccess  []PaymentHandler
	onCanceled []PaymentHandler
	onRefunded []PaymentHandler
	mu         sync.RWMutex
}

// NewRouter creates a new payment webhook router.
func NewRouter() *Router {
	return &Router{
		gateways:   make(map[string]Gateway),
		onSuccess:  make([]PaymentHandler, 0),
		onCanceled: make([]PaymentHandler, 0),
		onRefunded: make([]PaymentHandler, 0),
	}
}

// RegisterGateway adds a payment gateway parser to the router.
func (r *Router) RegisterGateway(g Gateway) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gateways[g.Name()] = g
	return r
}

// OnSuccess registers a callback executed when a payment is confirmed/succeeded.
func (r *Router) OnSuccess(handler PaymentHandler) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onSuccess = append(r.onSuccess, handler)
	return r
}

// OnCanceled registers a callback executed when a payment is rejected or canceled.
func (r *Router) OnCanceled(handler PaymentHandler) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onCanceled = append(r.onCanceled, handler)
	return r
}

// OnRefunded registers a callback executed when a payment is refunded.
func (r *Router) OnRefunded(handler PaymentHandler) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onRefunded = append(r.onRefunded, handler)
	return r
}

// HTTPHandler returns an http.HandlerFunc that accepts webhooks for the specified gateway name.
func (r *Router) HTTPHandler(gatewayName string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.mu.RLock()
		gw, ok := r.gateways[gatewayName]
		r.mu.RUnlock()

		if !ok {
			http.Error(w, fmt.Sprintf("gateway %q not configured", gatewayName), http.StatusNotFound)
			return
		}

		payment, err := gw.ParseAndVerify(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("verification failed: %v", err), http.StatusBadRequest)
			return
		}

		ctx := req.Context()

		r.mu.RLock()
		var handlers []PaymentHandler
		switch payment.Status {
		case StatusSucceeded:
			handlers = append([]PaymentHandler(nil), r.onSuccess...)
		case StatusCanceled:
			handlers = append([]PaymentHandler(nil), r.onCanceled...)
		case StatusRefunded:
			handlers = append([]PaymentHandler(nil), r.onRefunded...)
		}
		r.mu.RUnlock()

		for _, h := range handlers {
			if err := h(ctx, payment); err != nil {
				http.Error(w, fmt.Sprintf("handler error: %v", err), http.StatusInternalServerError)
				return
			}
		}

		// Most payment gateways (including T-Bank) expect "OK" as the HTTP 200 response body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}
