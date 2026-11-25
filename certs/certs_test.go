package certs

import (
	"testing"
)

func TestCertPool(t *testing.T) {
	t.Parallel()
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
