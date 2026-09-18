package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	baseconfig "github.com/liaisonio/liaison/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestWebDomainRequiresMatchingLiveWildcardCertificate(t *testing.T) {
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, e)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "*.apps.example"}, DNSNames: []string{"*.apps.example"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, leaf, leaf, &key.PublicKey, key)
	require.NoError(t, e)
	private, e := x509.MarshalECPrivateKey(key)
	require.NoError(t, e)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "web.crt"), filepath.Join(dir, "web.key")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: private}), 0600))
	m := Manager{WebDomain: "apps.example", Listen: baseconfig.Listen{TLS: baseconfig.TLS{Enable: true, Certs: []baseconfig.CertKey{{Cert: certFile, Key: keyFile}}}}}
	require.True(t, m.WebDomainReady())
	for _, domain := range []string{"", "other.example", "*.apps.example", "apps.example:443", "apps.example/route"} {
		m.WebDomain = domain
		require.False(t, m.WebDomainReady(), domain)
	}
	m.WebDomain = "apps.example"
	m.Listen.TLS.Enable = false
	require.False(t, m.WebDomainReady())
	m.Listen.TLS.Enable = true
	leaf.NotAfter = time.Now().Add(-time.Minute)
	der, e = x509.CreateCertificate(rand.Reader, leaf, leaf, &key.PublicKey, key)
	require.NoError(t, e)
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.False(t, m.WebDomainReady())
}
