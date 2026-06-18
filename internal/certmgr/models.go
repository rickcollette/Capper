package certmgr

import "time"

const (
	CertStatusPending    = "pending"
	CertStatusValidating = "validating"
	CertStatusIssued     = "issued"
	CertStatusAttached   = "attached"
	CertStatusRenewing   = "renewing"
	CertStatusRenewed    = "renewed"
	CertStatusFailed     = "failed"
	CertStatusExpired    = "expired"
	CertStatusRevoked    = "revoked"
	CertStatusImported   = "imported"

	IssuerLetsEncrypt        = "letsencrypt"
	IssuerLetsEncryptStaging = "letsencrypt-staging"
	IssuerImported           = "imported"

	ValidationHTTP01 = "http-01"
	ValidationDNS01  = "dns-01"

	DirectoryLetsEncrypt        = "https://acme-v02.api.letsencrypt.org/directory"
	DirectoryLetsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

type Certificate struct {
	ID               string    `json:"id"`
	Project          string    `json:"project"`
	AccountID        string    `json:"accountId"`
	Name             string    `json:"name"`
	CommonName       string    `json:"commonName"`
	SANs             []string  `json:"sans"`
	Issuer           string    `json:"issuer"`
	Status           string    `json:"status"`
	ValidationMethod string    `json:"validationMethod"`
	ACMEAccountID    string    `json:"acmeAccountId"`
	ActiveVersionID  string    `json:"activeVersionId"`
	NotBefore        string    `json:"notBefore"`
	NotAfter         string    `json:"notAfter"`
	AutoRenew        bool      `json:"autoRenew"`
	RenewAfter       string    `json:"renewAfter"`
	FailureReason    string    `json:"failureReason,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type CertificateVersion struct {
	ID                string    `json:"id"`
	CertificateID     string    `json:"certificateId"`
	CertPEM           string    `json:"certPem,omitempty"`
	ChainPEM          string    `json:"chainPem,omitempty"`
	FullChainPEM      string    `json:"fullChainPem,omitempty"`
	PrivateKeyRef     string    `json:"privateKeyRef"`
	FingerprintSHA256 string    `json:"fingerprintSha256"`
	SerialNumber      string    `json:"serialNumber"`
	NotBefore         string    `json:"notBefore"`
	NotAfter          string    `json:"notAfter"`
	CreatedAt         time.Time `json:"createdAt"`
}

type ACMEAccount struct {
	ID                         string    `json:"id"`
	Name                       string    `json:"name"`
	Email                      string    `json:"email"`
	DirectoryURL               string    `json:"directoryUrl"`
	Status                     string    `json:"status"`
	PrivateKeyRef              string    `json:"privateKeyRef"`
	ExternalAccountBindingJSON string    `json:"externalAccountBinding,omitempty"`
	CreatedAt                  time.Time `json:"createdAt"`
	UpdatedAt                  time.Time `json:"updatedAt"`
}

type CertificateBinding struct {
	ID            string    `json:"id"`
	CertificateID string    `json:"certificateId"`
	TargetType    string    `json:"targetType"` // "lb", "ingress"
	TargetID      string    `json:"targetId"`
	Hostname      string    `json:"hostname"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type CertRequest struct {
	Project          string
	AccountID        string
	Name             string
	CommonName       string
	SANs             []string
	Issuer           string
	ValidationMethod string
	ACMEAccountName  string
	AutoRenew        bool
}

type ImportCertRequest struct {
	Project   string
	AccountID string
	Name      string
	CertPEM   string
	KeyPEM    string
	ChainPEM  string
}

type CertResult struct {
	CertPEM      string
	ChainPEM     string
	FullChainPEM string
	KeyPEM       string
	NotBefore    string
	NotAfter     string
	Fingerprint  string
	SerialNumber string
}
