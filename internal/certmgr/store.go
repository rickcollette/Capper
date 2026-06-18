package certmgr

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) InitSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS certificates (
            id                TEXT PRIMARY KEY,
            project           TEXT NOT NULL,
            account_id        TEXT NOT NULL DEFAULT 'acct_local',
            name              TEXT NOT NULL,
            common_name       TEXT NOT NULL,
            sans_json         TEXT NOT NULL DEFAULT '[]',
            issuer            TEXT NOT NULL DEFAULT 'letsencrypt',
            status            TEXT NOT NULL DEFAULT 'pending',
            validation_method TEXT NOT NULL DEFAULT 'http-01',
            acme_account_id   TEXT NOT NULL DEFAULT '',
            active_version_id TEXT NOT NULL DEFAULT '',
            not_before        TEXT NOT NULL DEFAULT '',
            not_after         TEXT NOT NULL DEFAULT '',
            auto_renew        INTEGER NOT NULL DEFAULT 1,
            renew_after       TEXT NOT NULL DEFAULT '',
            failure_reason    TEXT NOT NULL DEFAULT '',
            created_at        TEXT NOT NULL,
            updated_at        TEXT NOT NULL,
            UNIQUE(project, name)
        )`,
		`CREATE TABLE IF NOT EXISTS certificate_versions (
            id                 TEXT PRIMARY KEY,
            certificate_id     TEXT NOT NULL REFERENCES certificates(id) ON DELETE CASCADE,
            cert_pem           TEXT NOT NULL,
            chain_pem          TEXT NOT NULL DEFAULT '',
            fullchain_pem      TEXT NOT NULL DEFAULT '',
            private_key_ref    TEXT NOT NULL,
            fingerprint_sha256 TEXT NOT NULL,
            serial_number      TEXT NOT NULL DEFAULT '',
            not_before         TEXT NOT NULL,
            not_after          TEXT NOT NULL,
            created_at         TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS acme_accounts (
            id                             TEXT PRIMARY KEY,
            name                           TEXT NOT NULL,
            email                          TEXT NOT NULL,
            directory_url                  TEXT NOT NULL,
            status                         TEXT NOT NULL DEFAULT 'active',
            private_key_ref                TEXT NOT NULL,
            external_account_binding_json  TEXT NOT NULL DEFAULT '{}',
            created_at                     TEXT NOT NULL,
            updated_at                     TEXT NOT NULL,
            UNIQUE(name, directory_url)
        )`,
		`CREATE TABLE IF NOT EXISTS certificate_bindings (
            id             TEXT PRIMARY KEY,
            certificate_id TEXT NOT NULL REFERENCES certificates(id) ON DELETE CASCADE,
            target_type    TEXT NOT NULL,
            target_id      TEXT NOT NULL,
            hostname       TEXT NOT NULL,
            status         TEXT NOT NULL DEFAULT 'active',
            created_at     TEXT NOT NULL,
            updated_at     TEXT NOT NULL,
            UNIQUE(target_type, target_id, hostname)
        )`,
		`CREATE TABLE IF NOT EXISTS acme_challenges (
            id                TEXT PRIMARY KEY,
            certificate_id    TEXT NOT NULL REFERENCES certificates(id) ON DELETE CASCADE,
            domain            TEXT NOT NULL,
            challenge_type    TEXT NOT NULL,
            token             TEXT NOT NULL,
            key_authorization TEXT NOT NULL,
            dns_record_name   TEXT NOT NULL DEFAULT '',
            dns_record_value  TEXT NOT NULL DEFAULT '',
            status            TEXT NOT NULL DEFAULT 'pending',
            expires_at        TEXT NOT NULL,
            created_at        TEXT NOT NULL,
            updated_at        TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS acme_rate_limits (
            id            TEXT PRIMARY KEY,
            domain_root   TEXT NOT NULL UNIQUE,
            issued_count  INTEGER NOT NULL DEFAULT 0,
            window_start  TEXT NOT NULL,
            last_error    TEXT NOT NULL DEFAULT '',
            backoff_until TEXT NOT NULL DEFAULT ''
        )`,
		`CREATE TABLE IF NOT EXISTS cert_private_keys (
            ref        TEXT PRIMARY KEY,
            encrypted  BLOB NOT NULL,
            created_at TEXT NOT NULL
        )`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("certmgr schema: %w", err)
		}
	}
	return nil
}

// --- Certificate CRUD ---

func (s *Store) CreateCertificate(c Certificate) (Certificate, error) {
	if c.ID == "" {
		c.ID = "cert_" + uuid.New().String()
	}
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now
	sansJSON, _ := json.Marshal(c.SANs)
	_, err := s.db.Exec(`INSERT INTO certificates
        (id, project, account_id, name, common_name, sans_json, issuer, status,
         validation_method, acme_account_id, active_version_id, not_before, not_after,
         auto_renew, renew_after, failure_reason, created_at, updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.Project, c.AccountID, c.Name, c.CommonName, string(sansJSON), c.Issuer, c.Status,
		c.ValidationMethod, c.ACMEAccountID, c.ActiveVersionID, c.NotBefore, c.NotAfter,
		boolToInt(c.AutoRenew), c.RenewAfter, c.FailureReason,
		now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return Certificate{}, err
	}
	return c, nil
}

func (s *Store) GetCertificate(nameOrID string) (Certificate, error) {
	row := s.db.QueryRow(`SELECT id, project, account_id, name, common_name, sans_json, issuer, status,
        validation_method, acme_account_id, active_version_id, not_before, not_after,
        auto_renew, renew_after, failure_reason, created_at, updated_at
        FROM certificates WHERE id=? OR name=?`, nameOrID, nameOrID)
	return scanCert(row)
}

func (s *Store) ListCertificates(project, accountID, status string) ([]Certificate, error) {
	conditions := []string{}
	args := []any{}
	if project != "" {
		conditions = append(conditions, "project=?")
		args = append(args, project)
	}
	if accountID != "" {
		conditions = append(conditions, "account_id=?")
		args = append(args, accountID)
	}
	if status != "" {
		conditions = append(conditions, "status=?")
		args = append(args, status)
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	rows, err := s.db.Query(`SELECT id, project, account_id, name, common_name, sans_json, issuer, status,
        validation_method, acme_account_id, active_version_id, not_before, not_after,
        auto_renew, renew_after, failure_reason, created_at, updated_at
        FROM certificates `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var certs []Certificate
	for rows.Next() {
		c, err := scanCert(rows)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

func (s *Store) UpdateCertificate(id string, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	setClauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)
	for k, v := range updates {
		setClauses = append(setClauses, k+"=?")
		args = append(args, v)
	}
	args = append(args, id)
	_, err := s.db.Exec(`UPDATE certificates SET `+strings.Join(setClauses, ",")+` WHERE id=?`, args...)
	return err
}

func (s *Store) DeleteCertificate(nameOrID string) error {
	_, err := s.db.Exec(`DELETE FROM certificates WHERE id=? OR name=?`, nameOrID, nameOrID)
	return err
}

// --- Certificate Version ---

func (s *Store) CreateCertVersion(v CertificateVersion) (CertificateVersion, error) {
	if v.ID == "" {
		v.ID = "certver_" + uuid.New().String()
	}
	v.CreatedAt = time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO certificate_versions
        (id, certificate_id, cert_pem, chain_pem, fullchain_pem, private_key_ref,
         fingerprint_sha256, serial_number, not_before, not_after, created_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		v.ID, v.CertificateID, v.CertPEM, v.ChainPEM, v.FullChainPEM, v.PrivateKeyRef,
		v.FingerprintSHA256, v.SerialNumber, v.NotBefore, v.NotAfter,
		v.CreatedAt.Format(time.RFC3339))
	return v, err
}

func (s *Store) GetCertVersion(id string) (CertificateVersion, error) {
	var v CertificateVersion
	var createdAt string
	err := s.db.QueryRow(`SELECT id, certificate_id, cert_pem, chain_pem, fullchain_pem, private_key_ref,
        fingerprint_sha256, serial_number, not_before, not_after, created_at
        FROM certificate_versions WHERE id=?`, id).Scan(
		&v.ID, &v.CertificateID, &v.CertPEM, &v.ChainPEM, &v.FullChainPEM, &v.PrivateKeyRef,
		&v.FingerprintSHA256, &v.SerialNumber, &v.NotBefore, &v.NotAfter, &createdAt)
	if err != nil {
		return CertificateVersion{}, err
	}
	v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return v, nil
}

// --- ACME Accounts ---

func (s *Store) CreateACMEAccount(a ACMEAccount) (ACMEAccount, error) {
	if a.ID == "" {
		a.ID = "acme_" + uuid.New().String()
	}
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.ExternalAccountBindingJSON == "" {
		a.ExternalAccountBindingJSON = "{}"
	}
	if a.PrivateKeyRef == "" {
		a.PrivateKeyRef = "acmekey_" + a.ID
	}
	_, err := s.db.Exec(`INSERT INTO acme_accounts
        (id, name, email, directory_url, status, private_key_ref, external_account_binding_json, created_at, updated_at)
        VALUES (?,?,?,?,?,?,?,?,?)`,
		a.ID, a.Name, a.Email, a.DirectoryURL, a.Status, a.PrivateKeyRef,
		a.ExternalAccountBindingJSON, now.Format(time.RFC3339), now.Format(time.RFC3339))
	return a, err
}

func (s *Store) GetACMEAccount(nameOrID string) (ACMEAccount, error) {
	var a ACMEAccount
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id, name, email, directory_url, status, private_key_ref,
        external_account_binding_json, created_at, updated_at
        FROM acme_accounts WHERE id=? OR name=?`, nameOrID, nameOrID).Scan(
		&a.ID, &a.Name, &a.Email, &a.DirectoryURL, &a.Status, &a.PrivateKeyRef,
		&a.ExternalAccountBindingJSON, &createdAt, &updatedAt)
	if err != nil {
		return ACMEAccount{}, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return a, nil
}

func (s *Store) ListACMEAccounts() ([]ACMEAccount, error) {
	rows, err := s.db.Query(`SELECT id, name, email, directory_url, status, private_key_ref,
        external_account_binding_json, created_at, updated_at FROM acme_accounts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []ACMEAccount
	for rows.Next() {
		var a ACMEAccount
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.DirectoryURL, &a.Status, &a.PrivateKeyRef,
			&a.ExternalAccountBindingJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (s *Store) DeleteACMEAccount(nameOrID string) error {
	_, err := s.db.Exec(`DELETE FROM acme_accounts WHERE id=? OR name=?`, nameOrID, nameOrID)
	return err
}

// --- Bindings ---

func (s *Store) CreateBinding(b CertificateBinding) (CertificateBinding, error) {
	if b.ID == "" {
		b.ID = "bind_" + uuid.New().String()
	}
	now := time.Now().UTC()
	b.CreatedAt = now
	b.UpdatedAt = now
	_, err := s.db.Exec(`INSERT OR REPLACE INTO certificate_bindings
        (id, certificate_id, target_type, target_id, hostname, status, created_at, updated_at)
        VALUES (?,?,?,?,?,?,?,?)`,
		b.ID, b.CertificateID, b.TargetType, b.TargetID, b.Hostname, b.Status,
		now.Format(time.RFC3339), now.Format(time.RFC3339))
	return b, err
}

func (s *Store) ListBindings(certID string) ([]CertificateBinding, error) {
	rows, err := s.db.Query(`SELECT id, certificate_id, target_type, target_id, hostname, status,
        created_at, updated_at FROM certificate_bindings WHERE certificate_id=? AND status='active'`, certID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bindings []CertificateBinding
	for rows.Next() {
		var b CertificateBinding
		var createdAt, updatedAt string
		if err := rows.Scan(&b.ID, &b.CertificateID, &b.TargetType, &b.TargetID,
			&b.Hostname, &b.Status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		bindings = append(bindings, b)
	}
	return bindings, rows.Err()
}

func (s *Store) DeleteBinding(id string) error {
	_, err := s.db.Exec(`UPDATE certificate_bindings SET status='inactive', updated_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *Store) ListBindingsByLB(lbID string) ([]CertificateBinding, error) {
	rows, err := s.db.Query(`SELECT id, certificate_id, target_type, target_id, hostname, status,
        created_at, updated_at FROM certificate_bindings WHERE target_id=? AND target_type='lb' AND status='active'`, lbID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bindings []CertificateBinding
	for rows.Next() {
		var b CertificateBinding
		var createdAt, updatedAt string
		if err := rows.Scan(&b.ID, &b.CertificateID, &b.TargetType, &b.TargetID,
			&b.Hostname, &b.Status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		bindings = append(bindings, b)
	}
	return bindings, rows.Err()
}

// --- Private Key Storage ---

func (s *Store) StorePrivateKey(ref string, encrypted []byte) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO cert_private_keys (ref, encrypted, created_at) VALUES (?,?,?)`,
		ref, encrypted, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) LoadPrivateKey(ref string) ([]byte, error) {
	var encrypted []byte
	err := s.db.QueryRow(`SELECT encrypted FROM cert_private_keys WHERE ref=?`, ref).Scan(&encrypted)
	return encrypted, err
}

// --- Rate Limits ---

func (s *Store) GetRateLimit(domainRoot string) (issuedCount int, windowStart string, backoffUntil string, err error) {
	err = s.db.QueryRow(`SELECT issued_count, window_start, backoff_until FROM acme_rate_limits WHERE domain_root=?`,
		domainRoot).Scan(&issuedCount, &windowStart, &backoffUntil)
	if err == sql.ErrNoRows {
		return 0, "", "", nil
	}
	return
}

func (s *Store) IncrementRateLimit(domainRoot string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO acme_rate_limits (id, domain_root, issued_count, window_start, last_error, backoff_until)
        VALUES (?, ?, 1, ?, '', '')
        ON CONFLICT(domain_root) DO UPDATE SET issued_count=issued_count+1`,
		"rl_"+domainRoot, domainRoot, now)
	return err
}

func (s *Store) SetBackoff(domainRoot, until, errMsg string) error {
	_, err := s.db.Exec(`INSERT INTO acme_rate_limits (id, domain_root, issued_count, window_start, last_error, backoff_until)
        VALUES (?, ?, 0, ?, ?, ?)
        ON CONFLICT(domain_root) DO UPDATE SET backoff_until=excluded.backoff_until, last_error=excluded.last_error`,
		"rl_"+domainRoot, domainRoot, time.Now().UTC().Format(time.RFC3339), errMsg, until)
	return err
}

// --- Renewal queries ---

func (s *Store) ListCertsDueForRenewal() ([]Certificate, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.Query(`SELECT id, project, account_id, name, common_name, sans_json, issuer, status,
        validation_method, acme_account_id, active_version_id, not_before, not_after,
        auto_renew, renew_after, failure_reason, created_at, updated_at
        FROM certificates
        WHERE auto_renew=1 AND status IN ('issued','attached') AND renew_after != '' AND renew_after <= ?`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var certs []Certificate
	for rows.Next() {
		c, err := scanCert(rows)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

func (s *Store) ListExpiredCerts() ([]Certificate, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.Query(`SELECT id, project, account_id, name, common_name, sans_json, issuer, status,
        validation_method, acme_account_id, active_version_id, not_before, not_after,
        auto_renew, renew_after, failure_reason, created_at, updated_at
        FROM certificates
        WHERE status IN ('issued','attached') AND not_after != '' AND not_after < ?`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var certs []Certificate
	for rows.Next() {
		c, err := scanCert(rows)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

// --- Helpers ---

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCert(row rowScanner) (Certificate, error) {
	var c Certificate
	var sansJSON string
	var autoRenew int
	var createdAt, updatedAt string
	err := row.Scan(&c.ID, &c.Project, &c.AccountID, &c.Name, &c.CommonName, &sansJSON, &c.Issuer, &c.Status,
		&c.ValidationMethod, &c.ACMEAccountID, &c.ActiveVersionID, &c.NotBefore, &c.NotAfter,
		&autoRenew, &c.RenewAfter, &c.FailureReason, &createdAt, &updatedAt)
	if err != nil {
		return Certificate{}, err
	}
	_ = json.Unmarshal([]byte(sansJSON), &c.SANs)
	c.AutoRenew = autoRenew == 1
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return c, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func fingerprintSHA256(certDER []byte) string {
	sum := sha256.Sum256(certDER)
	return hex.EncodeToString(sum[:])
}
