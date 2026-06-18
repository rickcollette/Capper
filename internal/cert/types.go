package cert

// CertStatus indicates whether a certificate is valid or has been revoked.
type CertStatus string

const (
	CertStatusValid   CertStatus = "valid"
	CertStatusRevoked CertStatus = "revoked"
)

// CertRecord stores certificate metadata and the PEM-encoded certificate.
// The private key is never persisted — callers hold it after issue.
type CertRecord struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Project    string     `json:"project"`
	CommonName string     `json:"commonName"`
	DNSNames   []string   `json:"dnsNames,omitempty"`
	Status     CertStatus `json:"status"`
	CertPEM    string     `json:"certPem"`
	IssuedAt   string     `json:"issuedAt"`
	ExpiresAt  string     `json:"expiresAt"`
	RevokedAt  string     `json:"revokedAt,omitempty"`
}

// IssueResult is returned by Manager.Issue and includes the private key PEM.
type IssueResult struct {
	CertRecord
	KeyPEM string `json:"keyPem"`
}
