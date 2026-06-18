package certmgr

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"

	"golang.org/x/crypto/acme"
)

type ACMEClientWrapper struct {
	client    *acme.Client
	account   *acme.Account
	secretKey []byte
	store     *Store
}

// directoryURL maps issuer names to ACME directory URLs.
func directoryURL(issuer string) string {
	switch issuer {
	case IssuerLetsEncrypt:
		return DirectoryLetsEncrypt
	case IssuerLetsEncryptStaging:
		return DirectoryLetsEncryptStaging
	default:
		if issuer != "" {
			return issuer
		}
		return DirectoryLetsEncryptStaging
	}
}

// NewACMEClientWrapper creates or loads an ACME client for the given account.
func NewACMEClientWrapper(ctx context.Context, acc ACMEAccount, store *Store, secretKey []byte) (*ACMEClientWrapper, error) {
	var privKey *ecdsa.PrivateKey
	var keyLoadErr error

	// Try to load existing key
	encrypted, err := store.LoadPrivateKey(acc.PrivateKeyRef)
	if err == nil && len(encrypted) > 0 {
		keyPEM := decrypt(secretKey, encrypted)
		privKey, keyLoadErr = parseECPrivateKey(keyPEM)
	}

	if privKey == nil {
		// Generate new key
		privKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generating ACME key: %w", err)
		}
		keyPEM, err := ecPrivKeyToPEM(privKey)
		if err != nil {
			return nil, err
		}
		encryptedKey := encrypt(secretKey, keyPEM)
		_ = store.StorePrivateKey(acc.PrivateKeyRef, encryptedKey)
	}

	c := &ACMEClientWrapper{
		client: &acme.Client{
			Key:          privKey,
			DirectoryURL: acc.DirectoryURL,
		},
		secretKey: secretKey,
		store:     store,
	}

	// Register or fetch account
	if keyLoadErr != nil || err != nil {
		// New key: register with ACME CA
		a, regErr := c.client.Register(ctx, &acme.Account{
			Contact: []string{"mailto:" + acc.Email},
		}, acme.AcceptTOS)
		if regErr != nil {
			return nil, fmt.Errorf("ACME registration: %w", regErr)
		}
		c.account = a
	}

	return c, nil
}

// OrderCertificate requests a certificate for the given domains using the provided solver.
func (c *ACMEClientWrapper) OrderCertificate(ctx context.Context, domains []string, solver interface {
	Present(domain, token, keyAuth string) error
	CleanUp(domain, token, keyAuth string) error
}) (*CertResult, error) {
	// Build auth IDs
	authzIDs := acme.DomainIDs(domains...)

	// Create order
	order, err := c.client.AuthorizeOrder(ctx, authzIDs)
	if err != nil {
		return nil, fmt.Errorf("ACME AuthorizeOrder: %w", err)
	}

	// Complete each authorization
	for _, authzURL := range order.AuthzURLs {
		authz, err := c.client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return nil, fmt.Errorf("GetAuthorization: %w", err)
		}
		if authz.Status == acme.StatusValid {
			continue
		}

		// Find http-01 challenge first, fall back to dns-01
		var challenge *acme.Challenge
		for _, ch := range authz.Challenges {
			if ch.Type == "http-01" {
				challenge = ch
				break
			}
		}
		if challenge == nil {
			for _, ch := range authz.Challenges {
				if ch.Type == "dns-01" {
					challenge = ch
					break
				}
			}
		}
		if challenge == nil {
			return nil, fmt.Errorf("no supported challenge type for %s", authz.Identifier.Value)
		}

		keyAuth, err := c.client.HTTP01ChallengeResponse(challenge.Token)
		if err != nil {
			return nil, fmt.Errorf("computing key auth: %w", err)
		}

		if err := solver.Present(authz.Identifier.Value, challenge.Token, keyAuth); err != nil {
			return nil, fmt.Errorf("presenting challenge: %w", err)
		}

		if _, err := c.client.Accept(ctx, challenge); err != nil {
			_ = solver.CleanUp(authz.Identifier.Value, challenge.Token, keyAuth)
			return nil, fmt.Errorf("accepting challenge: %w", err)
		}

		if _, err := c.client.WaitAuthorization(ctx, authzURL); err != nil {
			_ = solver.CleanUp(authz.Identifier.Value, challenge.Token, keyAuth)
			return nil, fmt.Errorf("authorization failed: %w", err)
		}

		_ = solver.CleanUp(authz.Identifier.Value, challenge.Token, keyAuth)
	}

	// Wait for order to be ready
	order, err = c.client.WaitOrder(ctx, order.URI)
	if err != nil {
		return nil, fmt.Errorf("waiting for order: %w", err)
	}

	// Generate new key pair for certificate
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating cert key: %w", err)
	}

	// Build CSR
	template := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domains[0]},
		DNSNames: domains,
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, certKey)
	if err != nil {
		return nil, fmt.Errorf("creating CSR: %w", err)
	}

	// Finalize order
	chain, _, err := c.client.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	if err != nil {
		return nil, fmt.Errorf("finalizing order: %w", err)
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("empty certificate chain")
	}

	// Parse certificate for metadata
	cert, err := x509.ParseCertificate(chain[0])
	if err != nil {
		return nil, fmt.Errorf("parsing certificate: %w", err)
	}

	// Encode PEMs
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: chain[0]})
	var chainPEMBuf []byte
	fullChainPEMBuf := append([]byte{}, certPEM...)
	for i, der := range chain {
		block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		if i > 0 {
			chainPEMBuf = append(chainPEMBuf, block...)
			fullChainPEMBuf = append(fullChainPEMBuf, block...)
		}
	}

	keyPEM, err := ecPrivKeyToPEM(certKey)
	if err != nil {
		return nil, err
	}

	return &CertResult{
		CertPEM:      string(certPEM),
		ChainPEM:     string(chainPEMBuf),
		FullChainPEM: string(fullChainPEMBuf),
		KeyPEM:       string(keyPEM),
		NotBefore:    cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:     cert.NotAfter.UTC().Format(time.RFC3339),
		Fingerprint:  fingerprintSHA256(chain[0]),
		SerialNumber: cert.SerialNumber.String(),
	}, nil
}

func (c *ACMEClientWrapper) RevokeCertificate(ctx context.Context, certDER []byte) error {
	return c.client.RevokeCert(ctx, c.client.Key, certDER, acme.CRLReasonUnspecified)
}

func parseECPrivateKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func ecPrivKeyToPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

// encrypt/decrypt use XOR with key. If secretKey is nil or empty, store plaintext.
func encrypt(key []byte, data []byte) []byte {
	if len(key) == 0 {
		return data
	}
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ key[i%len(key)]
	}
	return result
}

func decrypt(key []byte, data []byte) []byte {
	return encrypt(key, data) // XOR is its own inverse
}
