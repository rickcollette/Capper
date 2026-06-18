package registry

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// InitTokenSchema creates the registry_tokens table. Safe to call multiple times.
func InitTokenSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS registry_tokens (
		id         TEXT PRIMARY KEY,
		registry   TEXT NOT NULL DEFAULT '',
		token      TEXT NOT NULL UNIQUE,
		expires_at TEXT NOT NULL,
		revoked    INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("registry.InitTokenSchema: %w", err)
	}
	return nil
}

// RegistryToken is a short-lived auth token for registry access.
type RegistryToken struct {
	ID        string `json:"id"`
	Registry  string `json:"registry,omitempty"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
	Revoked   bool   `json:"revoked,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// InsertToken stores a new registry token.
func InsertToken(db *sql.DB, t RegistryToken) error {
	_, err := db.Exec(
		`INSERT INTO registry_tokens (id, registry, token, expires_at, revoked, created_at)
		VALUES (?,?,?,?,?,?)`,
		t.ID, t.Registry, t.Token, t.ExpiresAt, boolInt(t.Revoked), t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("registry: insert token: %w", err)
	}
	return nil
}

// ValidateToken checks that a token exists, is not revoked, and has not expired.
func ValidateToken(db *sql.DB, token string) (RegistryToken, error) {
	row := db.QueryRow(
		`SELECT id, registry, token, expires_at, revoked, created_at
		FROM registry_tokens WHERE token=? LIMIT 1`,
		token)
	t, err := scanToken(row)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return RegistryToken{}, fmt.Errorf("registry: token not found")
		}
		return RegistryToken{}, err
	}
	if t.Revoked {
		return RegistryToken{}, fmt.Errorf("registry: token revoked")
	}
	exp, err := time.Parse(time.RFC3339, t.ExpiresAt)
	if err == nil && time.Now().UTC().After(exp) {
		return RegistryToken{}, fmt.Errorf("registry: token expired")
	}
	return t, nil
}

// RevokeToken marks a token as revoked.
func RevokeToken(db *sql.DB, token string) error {
	res, err := db.Exec(`UPDATE registry_tokens SET revoked=1 WHERE token=?`, token)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("registry: token not found")
	}
	return nil
}

// IssueToken creates a new random token for the given registry with the given TTL.
func IssueToken(db *sql.DB, registry string, ttl time.Duration) (RegistryToken, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return RegistryToken{}, err
	}
	t := RegistryToken{
		ID:        "rtok_" + hex.EncodeToString(raw[:5]),
		Registry:  registry,
		Token:     hex.EncodeToString(raw),
		ExpiresAt: time.Now().UTC().Add(ttl).Format(time.RFC3339),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := InsertToken(db, t); err != nil {
		return RegistryToken{}, err
	}
	return t, nil
}

func scanToken(s rowScanner) (RegistryToken, error) {
	var t RegistryToken
	var revoked int
	err := s.Scan(&t.ID, &t.Registry, &t.Token, &t.ExpiresAt, &revoked, &t.CreatedAt)
	if err != nil {
		return RegistryToken{}, fmt.Errorf("registry: scan token: %w", err)
	}
	t.Revoked = revoked != 0
	return t, nil
}
