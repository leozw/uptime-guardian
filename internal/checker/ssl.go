package checker

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/leozw/uptime-guardian/internal/core"
)

type SSLChecker struct {
	timeout time.Duration
}

func NewSSLChecker() *SSLChecker {
	return &SSLChecker{
		timeout: 10 * time.Second,
	}
}

func (s *SSLChecker) Check(domain string) (*core.SSLCheckDetails, error) {
	conn, err := tls.DialWithDialer(&net.Dialer{
		Timeout: s.timeout,
	}, "tcp", domain+":443", &tls.Config{
		ServerName: domain,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no certificates found")
	}

	cert := state.PeerCertificates[0]

	details := &core.SSLCheckDetails{
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		ValidFrom:    cert.NotBefore,
		ValidTo:      cert.NotAfter,
		DaysToExpiry: int(time.Until(cert.NotAfter).Hours() / 24),
		Protocol:     tlsVersionString(state.Version),
		CipherSuite:  tls.CipherSuiteName(state.CipherSuite),
	}

	// Certificate chain
	for _, cert := range state.PeerCertificates {
		details.CertificateChain = append(details.CertificateChain, cert.Subject.String())
	}

	// Calculate grade
	details.Grade = s.calculateGrade(state, cert)

	// Check if expired or expiring soon
	if time.Now().After(cert.NotAfter) {
		return details, fmt.Errorf("certificate expired")
	}

	if details.DaysToExpiry < 30 {
		return details, fmt.Errorf("certificate expiring soon (%d days)", details.DaysToExpiry)
	}

	return details, nil
}

func (s *SSLChecker) calculateGrade(state tls.ConnectionState, cert *tls.Certificate) string {
	score := 100

	// Protocol version
	switch state.Version {
	case tls.VersionTLS13:
		// Best
	case tls.VersionTLS12:
		score -= 10
	default:
		score -= 30
	}

	// Key strength
	if cert != nil {
		// Simplified - would need to check actual key size
		score -= 5
	}

	// Cipher suite strength
	switch state.CipherSuite {
	case tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256:
		// Good modern ciphers
	default:
		score -= 10
	}

	// Convert score to grade
	switch {
	case score >= 90:
		return "A+"
	case score >= 80:
		return "A"
	case score >= 70:
		return "B"
	case score >= 60:
		return "C"
	default:
		return "F"
	}
}

func tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "Unknown"
	}
}
