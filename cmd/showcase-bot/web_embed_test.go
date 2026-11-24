package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterWebSPA(t *testing.T) {
	mux := http.NewServeMux()
	if err := RegisterWebSPA(mux); err != nil {
		t.Fatalf("RegisterWebSPA returned unexpected error: %v", err)
	}

	// 1. Root route "/" should serve index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Bot Hub") && !strings.Contains(body, "id=\"root\"") {
		t.Errorf("expected Bot Hub or id=root in index.html, got: %s", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got: %s", ct)
	}

	// 2. SPA client-side route "/shop" should fallback to index.html with 200 OK
	reqShop := httptest.NewRequest(http.MethodGet, "/shop", nil)
	recShop := httptest.NewRecorder()
	mux.ServeHTTP(recShop, reqShop)

	if recShop.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SPA route /shop, got %d", recShop.Code)
	}
	if !strings.Contains(recShop.Body.String(), "id=\"root\"") {
		t.Errorf("expected index.html fallback for /shop, got: %s", recShop.Body.String())
	}

	// 3. SPA client-side route "/admin" should fallback to index.html with 200 OK
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin", nil)
	recAdmin := httptest.NewRecorder()
	mux.ServeHTTP(recAdmin, reqAdmin)

	if recAdmin.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SPA route /admin, got %d", recAdmin.Code)
	}

	// 4. Missing assets under /assets/ should return 404
	reqMissingAsset := httptest.NewRequest(http.MethodGet, "/assets/nonexistent_file_123.js", nil)
	recMissingAsset := httptest.NewRecorder()
	mux.ServeHTTP(recMissingAsset, reqMissingAsset)

	if recMissingAsset.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing asset, got %d", recMissingAsset.Code)
	}

	// 5. API, webhook, and metrics routes should not be intercepted with index.html
	for _, apiPath := range []string{"/api/v1/bootstrap", "/webhook", "/metrics"} {
		reqAPI := httptest.NewRequest(http.MethodGet, apiPath, nil)
		recAPI := httptest.NewRecorder()
		mux.ServeHTTP(recAPI, reqAPI)

		if recAPI.Code != http.StatusNotFound {
			t.Errorf("expected 404 from SPA handler for %s, got %d", apiPath, recAPI.Code)
		}
	}

	// 6. Directory requests (e.g. /assets, /assets/) should not serve directory listings
	for _, dirPath := range []string{"/assets", "/assets/"} {
		reqDir := httptest.NewRequest(http.MethodGet, dirPath, nil)
		recDir := httptest.NewRecorder()
		mux.ServeHTTP(recDir, reqDir)

		if recDir.Code != http.StatusNotFound {
			t.Errorf("expected 404 for directory request %s, got %d", dirPath, recDir.Code)
		}
	}
}
