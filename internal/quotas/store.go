package quotas

import (
	"database/sql"
)

// Store persists quota limits and resource usage.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by an already-initialised database.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the quota and usage tables. Safe to call multiple times.
func (s *Store) InitSchema() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS account_quotas (
		account_id    TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		quota_limit   INTEGER NOT NULL DEFAULT 100,
		PRIMARY KEY (account_id, resource_type)
	)`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS resource_usage (
		account_id    TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id   TEXT NOT NULL,
		metric_name   TEXT NOT NULL DEFAULT 'count',
		value         INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (account_id, resource_type, resource_id)
	)`)
	return err
}

// SeedDefaults inserts default quota rows for all known keys for an account.
// Existing rows are left untouched (INSERT OR IGNORE).
func (s *Store) SeedDefaults(accountID string) error {
	for k, v := range DefaultQuotas {
		_, err := s.db.Exec(
			`INSERT OR IGNORE INTO account_quotas (account_id, resource_type, quota_limit) VALUES (?,?,?)`,
			accountID, k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetQuota returns the configured limit for an account/resource pair.
// Falls back to DefaultQuotas then 100 if no row exists.
func (s *Store) GetQuota(accountID, resourceType string) (int64, error) {
	var limit int64
	err := s.db.QueryRow(
		`SELECT quota_limit FROM account_quotas WHERE account_id=? AND resource_type=?`,
		accountID, resourceType).Scan(&limit)
	if err == sql.ErrNoRows {
		if def, ok := DefaultQuotas[resourceType]; ok {
			return int64(def), nil
		}
		return 100, nil
	}
	return limit, err
}

// SetQuota upserts the limit for an account/resource pair.
func (s *Store) SetQuota(accountID, resourceType string, limit int64) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO account_quotas (account_id, resource_type, quota_limit) VALUES (?,?,?)`,
		accountID, resourceType, limit)
	return err
}

// CurrentUsage returns the sum of all tracked values for an account/resource pair.
func (s *Store) CurrentUsage(accountID, resourceType string) (int64, error) {
	var total int64
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(value),0) FROM resource_usage WHERE account_id=? AND resource_type=?`,
		accountID, resourceType).Scan(&total)
	return total, err
}

// RecordUsage adds delta to the value for a specific resource instance.
func (s *Store) RecordUsage(accountID, resourceType, resourceID string, delta int64) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO resource_usage (account_id, resource_type, resource_id, metric_name, value)
		 VALUES (?,?,?,'count',COALESCE((SELECT value FROM resource_usage WHERE account_id=? AND resource_type=? AND resource_id=?),0)+?)`,
		accountID, resourceType, resourceID, accountID, resourceType, resourceID, delta)
	return err
}

// ReleaseUsage removes usage tracking for a specific resource instance.
func (s *Store) ReleaseUsage(accountID, resourceType, resourceID string) error {
	_, err := s.db.Exec(
		`DELETE FROM resource_usage WHERE account_id=? AND resource_type=? AND resource_id=?`,
		accountID, resourceType, resourceID)
	return err
}

// ListQuotas returns all configured quota rows for an account.
func (s *Store) ListQuotas(accountID string) ([]Quota, error) {
	rows, err := s.db.Query(
		`SELECT account_id, resource_type, quota_limit FROM account_quotas WHERE account_id=?`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Quota
	for rows.Next() {
		var q Quota
		if err := rows.Scan(&q.AccountID, &q.ResourceType, &q.Limit); err != nil {
			return nil, err
		}
		result = append(result, q)
	}
	return result, rows.Err()
}
