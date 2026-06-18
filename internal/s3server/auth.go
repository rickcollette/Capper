package s3server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"capper/internal/secret"
)

// CredentialProvider maps access keys to secret keys.
type CredentialProvider interface {
	SecretKey(accessKey string) (string, error)
}

// StaticCredentials is a simple map[accessKey]secretKey provider.
type StaticCredentials map[string]string

func (s StaticCredentials) SecretKey(accessKey string) (string, error) {
	secret, ok := s[accessKey]
	if !ok {
		return "", fmt.Errorf("unknown access key")
	}
	return secret, nil
}

// SigV4Auth returns a gin middleware that validates AWS Signature Version 4.
// If cp is nil and productionMode is false, the request passes unauthenticated
// (dev/anonymous mode). If cp is nil and productionMode is true, every request
// is rejected with 503.
func SigV4Auth(cp CredentialProvider, productionMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cp == nil {
			if productionMode {
				writeS3Error(c, http.StatusServiceUnavailable, ErrCodeInternalError,
					"S3 credentials not configured; server is in production mode", "")
				c.Abort()
				return
			}
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if auth == "" {
			writeS3Error(c, http.StatusForbidden, ErrCodeAccessDenied, "Missing Authorization header", "")
			c.Abort()
			return
		}
		if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
			writeS3Error(c, http.StatusForbidden, ErrCodeAccessDenied, "Only AWS4-HMAC-SHA256 signatures are supported", "")
			c.Abort()
			return
		}
		pa, err := parseAuthHeader(auth)
		if err != nil {
			writeS3Error(c, http.StatusForbidden, ErrCodeAccessDenied, "Invalid Authorization header: "+err.Error(), "")
			c.Abort()
			return
		}
		secretKey, err := cp.SecretKey(pa.accessKey)
		if err != nil {
			writeS3Error(c, http.StatusForbidden, ErrCodeAccessDenied, "Invalid credentials", "")
			c.Abort()
			return
		}
		datetime := c.GetHeader("X-Amz-Date")
		if datetime == "" {
			writeS3Error(c, http.StatusForbidden, ErrCodeAccessDenied, "Missing X-Amz-Date header", "")
			c.Abort()
			return
		}
		if t, err := time.Parse("20060102T150405Z", datetime); err != nil || time.Since(t).Abs() > 15*time.Minute {
			writeS3Error(c, http.StatusForbidden, ErrCodeAccessDenied, "Request timestamp too skewed", "")
			c.Abort()
			return
		}
		if err := verifySigV4(c.Request, secretKey, datetime, pa); err != nil {
			writeS3Error(c, http.StatusForbidden, ErrCodeAccessDenied, "Signature mismatch", "")
			c.Abort()
			return
		}
		// Store the authenticated access key so object handlers can pass it to ObjectAuthorizer.
		c.Set("s3AccessKey", pa.accessKey)
		c.Next()
	}
}

type parsedAuth struct {
	accessKey     string
	dateScope     string // YYYYMMDD
	region        string
	service       string
	signedHeaders []string
	signature     string
}

// parseAuthHeader parses an AWS4-HMAC-SHA256 Authorization header.
func parseAuthHeader(auth string) (parsedAuth, error) {
	var pa parsedAuth
	rest := strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 ")
	// Split on ", " to get the three components
	for _, part := range strings.Split(rest, ", ") {
		part = strings.TrimSpace(part)
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "Credential":
			fields := strings.SplitN(v, "/", 5)
			if len(fields) < 5 {
				return pa, fmt.Errorf("invalid Credential format")
			}
			pa.accessKey = fields[0]
			pa.dateScope = fields[1]
			pa.region = fields[2]
			pa.service = fields[3]
		case "SignedHeaders":
			pa.signedHeaders = strings.Split(v, ";")
		case "Signature":
			pa.signature = v
		}
	}
	if pa.accessKey == "" || pa.signature == "" {
		return pa, fmt.Errorf("missing required Authorization fields")
	}
	return pa, nil
}

// verifySigV4 recomputes the SigV4 signature and compares it against the provided one.
func verifySigV4(r *http.Request, secretKey, datetime string, pa parsedAuth) error {
	// Canonical URI
	uri := r.URL.EscapedPath()
	if uri == "" {
		uri = "/"
	}
	// Canonical query string
	qs := canonicalQueryString(r.URL.Query())
	// Canonical headers + signed headers list
	ch := canonicalHeaders(r, pa.signedHeaders)
	sh := strings.Join(pa.signedHeaders, ";")
	// Payload hash — use header value or UNSIGNED-PAYLOAD
	bodyHash := r.Header.Get("X-Amz-Content-Sha256")
	if bodyHash == "" {
		bodyHash = "UNSIGNED-PAYLOAD"
	}
	canonReq := strings.Join([]string{r.Method, uri, qs, ch, sh, bodyHash}, "\n")
	credScope := strings.Join([]string{pa.dateScope, pa.region, pa.service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		datetime,
		credScope,
		hexSHA256([]byte(canonReq)),
	}, "\n")
	signingKey := deriveSigningKey(secretKey, pa.dateScope, pa.region, pa.service)
	expected := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	if expected != pa.signature {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func canonicalQueryString(vals url.Values) string {
	if len(vals) == 0 {
		return ""
	}
	type kv struct{ k, v string }
	var pairs []kv
	for k, vs := range vals {
		for _, v := range vs {
			pairs = append(pairs, kv{url.QueryEscape(k), url.QueryEscape(v)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.k + "=" + p.v
	}
	return strings.Join(parts, "&")
}

func canonicalHeaders(r *http.Request, signedHeaders []string) string {
	var sb strings.Builder
	for _, h := range signedHeaders {
		var val string
		if h == "host" {
			val = r.Host
			if val == "" {
				val = r.Header.Get("Host")
			}
		} else {
			val = strings.TrimSpace(r.Header.Get(h))
		}
		sb.WriteString(h)
		sb.WriteByte(':')
		sb.WriteString(val)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func deriveSigningKey(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func hexSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ---- IAM-backed credential store --------------------------------------------

// S3Credential represents an IAM service-account-linked S3 key pair.
type S3Credential struct {
	ID        string `json:"id"`
	AccountID string `json:"accountID"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// InitS3CredSchema creates the s3_credentials table if it does not exist.
func InitS3CredSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS s3_credentials (
		id TEXT PRIMARY KEY,
		account_id TEXT NOT NULL,
		access_key TEXT NOT NULL UNIQUE,
		secret_key TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`)
	return err
}

// GenerateS3Credential creates a new access/secret key pair for accountID,
// persists it (secret key encrypted with masterKey), and returns the credential
// record with the plaintext secret key for one-time display.
func GenerateS3Credential(db *sql.DB, masterKey []byte, accountID string) (S3Credential, error) {
	ak, err := randHex(10) // 20-char hex access key
	if err != nil {
		return S3Credential{}, err
	}
	sk, err := randHex(20) // 40-char hex secret key
	if err != nil {
		return S3Credential{}, err
	}
	encrypted, err := secret.Encrypt(masterKey, sk)
	if err != nil {
		return S3Credential{}, fmt.Errorf("s3: encrypt secret key: %w", err)
	}
	c := S3Credential{
		ID:        "s3cred_" + ak,
		AccountID: accountID,
		AccessKey: ak,
		SecretKey: sk, // plaintext returned to caller only; not stored
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err = db.Exec(
		`INSERT INTO s3_credentials (id, account_id, access_key, secret_key, created_at) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.AccountID, c.AccessKey, hex.EncodeToString(encrypted), c.CreatedAt,
	)
	return c, err
}

// ListS3Credentials returns all credentials for accountID.
func ListS3Credentials(db *sql.DB, accountID string) ([]S3Credential, error) {
	rows, err := db.Query(
		`SELECT id, account_id, access_key, secret_key, created_at FROM s3_credentials WHERE account_id=? ORDER BY created_at`,
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []S3Credential
	for rows.Next() {
		var c S3Credential
		if err := rows.Scan(&c.ID, &c.AccountID, &c.AccessKey, &c.SecretKey, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteS3Credential removes a credential by ID or access key.
func DeleteS3Credential(db *sql.DB, idOrKey string) error {
	_, err := db.Exec(`DELETE FROM s3_credentials WHERE id=? OR access_key=?`, idOrKey, idOrKey)
	return err
}

// IAMCredentialProvider implements CredentialProvider by looking up key pairs
// from the s3_credentials table. Secret keys are stored encrypted; masterKey
// is the AES-256 key used by secret.Encrypt/Decrypt.
type IAMCredentialProvider struct {
	db        *sql.DB
	masterKey []byte
}

// NewIAMCredentialProvider returns a provider backed by db. masterKey is the
// AES-256 encryption key used to decrypt stored secret keys.
func NewIAMCredentialProvider(db *sql.DB, masterKey []byte) *IAMCredentialProvider {
	return &IAMCredentialProvider{db: db, masterKey: masterKey}
}

func (p *IAMCredentialProvider) SecretKey(accessKey string) (string, error) {
	var encHex string
	err := p.db.QueryRow(
		`SELECT secret_key FROM s3_credentials WHERE access_key=?`, accessKey,
	).Scan(&encHex)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("unknown access key")
	}
	if err != nil {
		return "", err
	}
	enc, err := hex.DecodeString(encHex)
	if err != nil {
		return "", fmt.Errorf("s3: decode secret key: %w", err)
	}
	return secret.Decrypt(p.masterKey, enc)
}

func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
