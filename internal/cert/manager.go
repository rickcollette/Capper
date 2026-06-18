package cert

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// Manager wraps the CA and the cert store.
type Manager struct {
	ca    *CA
	store *Store
}

func NewManager(ca *CA, s *Store) *Manager {
	return &Manager{ca: ca, store: s}
}

// CACertPEM returns the PEM of the local root CA certificate.
func (m *Manager) CACertPEM() []byte { return m.ca.CACertPEM() }

// Issue signs a new leaf certificate and stores the record.
func (m *Manager) Issue(name, project, commonName string, dnsNames []string) (IssueResult, error) {
	if commonName == "" {
		commonName = name
	}
	certPEM, keyPEM, expiresAt, err := m.ca.Issue(commonName, dnsNames)
	if err != nil {
		return IssueResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rec := CertRecord{
		ID:         newID(),
		Name:       name,
		Project:    project,
		CommonName: commonName,
		DNSNames:   dnsNames,
		Status:     CertStatusValid,
		CertPEM:    certPEM,
		IssuedAt:   now,
		ExpiresAt:  expiresAt.UTC().Format(time.RFC3339),
	}
	if err := m.store.Insert(rec); err != nil {
		return IssueResult{}, fmt.Errorf("cert: store: %w", err)
	}
	return IssueResult{CertRecord: rec, KeyPEM: keyPEM}, nil
}

// IssueNodeCert issues an x509 certificate for a CSD or agent node, signed by
// the Capper internal CA. The TTL parameter is advisory — the CA may enforce
// its own maximum. Returns (certPEM, keyPEM) as byte slices.
func (m *Manager) IssueNodeCert(commonName string, dnsNames []string, _ time.Duration) (certPEM, keyPEM []byte, err error) {
	certStr, keyStr, _, issueErr := m.ca.Issue(commonName, dnsNames)
	if issueErr != nil {
		return nil, nil, fmt.Errorf("cert: IssueNodeCert %s: %w", commonName, issueErr)
	}
	return []byte(certStr), []byte(keyStr), nil
}

// GetExpiry returns the expiry time of the named certificate.
func (m *Manager) GetExpiry(nameOrID string) (time.Time, error) {
	rec, err := m.store.Get(nameOrID, "")
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, rec.ExpiresAt)
}

// CAPool returns an x509.CertPool containing the Capper internal CA certificate.
// Use this as RootCAs / ClientCAs in TLS configs to verify peer certificates.
func (m *Manager) CAPool() (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	block, _ := pem.Decode(m.ca.CACertPEM())
	if block == nil {
		return nil, fmt.Errorf("cert: failed to decode CA PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cert: parse CA cert: %w", err)
	}
	pool.AddCert(caCert)
	return pool, nil
}

// Get returns a certificate record by name or ID.
func (m *Manager) Get(nameOrID, project string) (CertRecord, error) {
	return m.store.Get(nameOrID, project)
}

// List returns all certificate records for the project.
func (m *Manager) List(project string) ([]CertRecord, error) {
	return m.store.List(project)
}

// Revoke marks a certificate as revoked.
func (m *Manager) Revoke(nameOrID, project string) error {
	return m.store.Revoke(nameOrID, project)
}
