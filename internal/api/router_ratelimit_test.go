package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitMiddleware(t *testing.T) {
	t.Parallel()

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := rateLimitMiddleware(RouterConfig{
		RateLimitRPS:   1,
		RateLimitBurst: 2,
	}, okHandler)

	// First 2 requests from same IP succeed within burst=2.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bootstrap", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got status %d, want 200", i+1, w.Code)
		}
	}

	// 3rd immediate request from same IP is rate-limited.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bootstrap", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited request: got status %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}

	// Different IP still has its own bucket.
	reqOther := httptest.NewRequest(http.MethodGet, "/api/v1/bootstrap", nil)
	reqOther.RemoteAddr = "203.0.113.11:1234"
	wOther := httptest.NewRecorder()
	h.ServeHTTP(wOther, reqOther)
	if wOther.Code != http.StatusOK {
		t.Fatalf("other IP request: got status %d, want 200", wOther.Code)
	}
}
