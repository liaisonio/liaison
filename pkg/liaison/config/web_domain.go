package config

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"time"
)

// WebDomainReady deliberately fails closed. Merely setting a domain does not
// enable host routing without a matching certificate on the HTTPS listener.
func (m *Manager) WebDomainReady() bool {
	domain := strings.ToLower(strings.TrimSpace(m.WebDomain))
	if !m.Listen.TLS.Enable || domain == "" || len(domain) > 240 || strings.ContainsAny(domain, ":/*@ \\?#") {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	for _, pair := range m.Listen.TLS.Certs {
		cert, err := tls.LoadX509KeyPair(pair.Cert, pair.Key)
		if err != nil || len(cert.Certificate) == 0 {
			continue
		}
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil || time.Now().Before(leaf.NotBefore) || time.Now().After(leaf.NotAfter) {
			continue
		}
		for _, name := range leaf.DNSNames {
			if strings.EqualFold(name, "*."+domain) {
				return true
			}
		}
	}
	return false
}
