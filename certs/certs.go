package certs

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
)

//go:embed mincifry_bundle.pem
var MincifryBundlePEM []byte

// AppendMincifryCerts appends the embedded Russian Trusted Root and Sub CA
// certificates issued by the Ministry of Digital Development (Минцифры РФ)
// into the provided x509.CertPool.
func AppendMincifryCerts(pool *x509.CertPool) error {
	if pool == nil {
		return fmt.Errorf("target cert pool is nil")
	}
	if !pool.AppendCertsFromPEM(MincifryBundlePEM) {
		return fmt.Errorf("failed to append Russian Trusted Root CA certificates from PEM bundle")
	}
	return nil
}

// CertPool returns a new x509.CertPool containing both the system certificates
// and the embedded Russian Trusted Root CA certificates.
func CertPool() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if err := AppendMincifryCerts(pool); err != nil {
		return nil, err
	}
	return pool, nil
}

// NewTLSConfig returns a tls.Config configured with the Russian Trusted Root CA certificates.
func NewTLSConfig() (*tls.Config, error) {
	pool, err := CertPool()
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		RootCAs: pool,
	}, nil
}
