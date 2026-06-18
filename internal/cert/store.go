package cert

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Store persists certificate records in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the certs table if it does not exist.
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS certs (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		project     TEXT NOT NULL DEFAULT 'default',
		common_name TEXT NOT NULL,
		dns_names   TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'valid',
		cert_pem    TEXT NOT NULL,
		issued_at   TEXT NOT NULL,
		expires_at  TEXT NOT NULL,
		revoked_at  TEXT NOT NULL DEFAULT ''
	)`)
	return err
}

func (s *Store) Insert(r CertRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO certs (id, name, project, common_name, dns_names, status, cert_pem, issued_at, expires_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Project, r.CommonName, strings.Join(r.DNSNames, ","),
		r.Status, r.CertPEM, r.IssuedAt, r.ExpiresAt, r.RevokedAt,
	)
	return err
}

func (s *Store) Get(nameOrID, project string) (CertRecord, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, common_name, dns_names, status, cert_pem, issued_at, expires_at, revoked_at
			 FROM certs WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, common_name, dns_names, status, cert_pem, issued_at, expires_at, revoked_at
			 FROM certs WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	}
	return scanCert(row)
}

func (s *Store) List(project string) ([]CertRecord, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, common_name, dns_names, status, cert_pem, issued_at, expires_at, revoked_at
			 FROM certs ORDER BY issued_at DESC`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, common_name, dns_names, status, cert_pem, issued_at, expires_at, revoked_at
			 FROM certs WHERE project=? ORDER BY issued_at DESC`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CertRecord
	for rows.Next() {
		r, err := scanCert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Revoke(nameOrID, project string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var res sql.Result
	var err error
	if project == "" {
		res, err = s.db.Exec(
			`UPDATE certs SET status='revoked', revoked_at=? WHERE (id=? OR name=?) AND status='valid'`,
			now, nameOrID, nameOrID,
		)
	} else {
		res, err = s.db.Exec(
			`UPDATE certs SET status='revoked', revoked_at=? WHERE (id=? OR name=?) AND project=? AND status='valid'`,
			now, nameOrID, nameOrID, project,
		)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("cert %q not found or already revoked", nameOrID)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCert(s rowScanner) (CertRecord, error) {
	var r CertRecord
	var dnsNames string
	if err := s.Scan(&r.ID, &r.Name, &r.Project, &r.CommonName, &dnsNames,
		&r.Status, &r.CertPEM, &r.IssuedAt, &r.ExpiresAt, &r.RevokedAt); err != nil {
		if err == sql.ErrNoRows {
			return CertRecord{}, fmt.Errorf("cert not found")
		}
		return CertRecord{}, fmt.Errorf("cert: scan: %w", err)
	}
	if dnsNames != "" {
		r.DNSNames = strings.Split(dnsNames, ",")
	}
	return r, nil
}

func newID() string {
	return "cert_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
