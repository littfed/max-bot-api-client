package certs

import (
	"net/http"
	"testing"
	"time"
)

func TestCertPool(t *testing.T) {
	if len(MincifryBundlePEM) == 0 {
		t.Fatal("embedded MincifryBundlePEM is empty")
	}

	pool, err := CertPool()
	if err != nil {
		t.Fatalf("CertPool() returned error: %v", err)
	}
	if pool == nil {
		t.Fatal("CertPool() returned nil pool")
	}

	cfg, err := NewTLSConfig()
	if err != nil {
		t.Fatalf("NewTLSConfig() returned error: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("NewTLSConfig() RootCAs is nil")
	}
}

// TestLiveTLSHandshake performs a live TLS handshake to platform-api2.max.ru
// to ensure the embedded certificate bundle works in practice.
func TestLiveTLSHandshake(t *testing.T) {
	tlsConfig, err := NewTLSConfig()
	if err != nil {
		t.Fatalf("failed to create TLS config: %v", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// We expect HTTP 401 Unauthorized because we don't pass an access token,
	// but the TLS handshake MUST succeed without x509 verification errors!
	resp, err := client.Get("https://platform-api2.max.ru/me")
	if err != nil {
		t.Fatalf("TLS connection to platform-api2.max.ru failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Logf("unexpected status code: %d (expected 401 Unauthorized)", resp.StatusCode)
	}
}
