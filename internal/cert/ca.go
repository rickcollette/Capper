package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	caKeyFile  = "ca.key"
	caCertFile = "ca.crt"
	caValidity = 10 * 365 * 24 * time.Hour
	certValid  = 365 * 24 * time.Hour
)

// CA holds the loaded or generated local certificate authority.
type CA struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
}

// LoadOrCreateCA reads the CA key+cert from dir, generating them if missing.
func LoadOrCreateCA(dir string) (*CA, error) {
	keyPath := filepath.Join(dir, caKeyFile)
	certPath := filepath.Join(dir, caCertFile)

	keyPEM, keyErr := os.ReadFile(keyPath)
	certPEM, certErr := os.ReadFile(certPath)

	if keyErr == nil && certErr == nil {
		// Parse existing CA.
		keyBlock, _ := pem.Decode(keyPEM)
		if keyBlock == nil {
			return nil, fmt.Errorf("cert: invalid CA key PEM")
		}
		key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("cert: parse CA key: %w", err)
		}
		certBlock, _ := pem.Decode(certPEM)
		if certBlock == nil {
			return nil, fmt.Errorf("cert: invalid CA cert PEM")
		}
		caCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("cert: parse CA cert: %w", err)
		}
		return &CA{cert: caCert, key: key, certPEM: certPEM}, nil
	}

	// Generate new CA.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("cert: generate CA key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("cert: serial: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Capper Local CA"},
		NotBefore:    now,
		NotAfter:     now.Add(caValidity),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("cert: sign CA: %w", err)
	}
	caCert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("cert: parse CA cert: %w", err)
	}
	// Persist.
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("cert: marshal CA key: %w", err)
	}
	rawKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	rawCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(keyPath, rawKeyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("cert: write CA key: %w", err)
	}
	if err := os.WriteFile(certPath, rawCertPEM, 0o644); err != nil {
		return nil, fmt.Errorf("cert: write CA cert: %w", err)
	}
	return &CA{cert: caCert, key: key, certPEM: rawCertPEM}, nil
}

// CACertPEM returns the PEM-encoded CA certificate.
func (ca *CA) CACertPEM() []byte { return ca.certPEM }

// Issue signs a leaf certificate for the given common name and optional DNS SANs.
// Returns (certPEM, keyPEM, expiresAt, error).
func (ca *CA) Issue(commonName string, dnsNames []string) (certPEM, keyPEM string, expiresAt time.Time, err error) {
	leafKey, kerr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if kerr != nil {
		return "", "", time.Time{}, fmt.Errorf("cert: generate leaf key: %w", kerr)
	}
	serial, serr := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if serr != nil {
		return "", "", time.Time{}, fmt.Errorf("cert: serial: %w", serr)
	}
	now := time.Now()
	expiresAt = now.Add(certValid)
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
		NotBefore:    now,
		NotAfter:     expiresAt,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, derr := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &leafKey.PublicKey, ca.key)
	if derr != nil {
		return "", "", time.Time{}, fmt.Errorf("cert: sign: %w", derr)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyDER, merr := x509.MarshalECPrivateKey(leafKey)
	if merr != nil {
		return "", "", time.Time{}, fmt.Errorf("cert: marshal key: %w", merr)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM, expiresAt, nil
}
