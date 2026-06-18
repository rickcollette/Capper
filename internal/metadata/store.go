package metadata

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// Store persists instance metadata and access logs.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the metadata tables.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS instance_metadata (
			instance_id   TEXT PRIMARY KEY,
			hostname      TEXT NOT NULL DEFAULT '',
			project       TEXT NOT NULL DEFAULT '',
			labels        TEXT NOT NULL DEFAULT '{}',
			instance_type TEXT NOT NULL DEFAULT '',
			network_ip    TEXT NOT NULL DEFAULT '',
			gateway       TEXT NOT NULL DEFAULT '',
			dns           TEXT NOT NULL DEFAULT '',
			user_data     TEXT NOT NULL DEFAULT '',
			token_hash    TEXT NOT NULL DEFAULT '',
			created_at    TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS metadata_access_log (
			id          TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL,
			source_ip   TEXT NOT NULL,
			endpoint    TEXT NOT NULL,
			allowed     INTEGER NOT NULL DEFAULT 1,
			auth_status TEXT NOT NULL DEFAULT 'public',
			created_at  TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// Upsert creates or replaces the metadata record for an instance.
func (s *Store) Upsert(m InstanceMetadata) error {
	labels := "{}"
	if len(m.Labels) > 0 {
		b := make([]byte, 0, 64)
		b = append(b, '{')
		first := true
		for k, v := range m.Labels {
			if !first {
				b = append(b, ',')
			}
			b = append(b, fmt.Sprintf("%q:%q", k, v)...)
			first = false
		}
		b = append(b, '}')
		labels = string(b)
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO instance_metadata
		 (instance_id, hostname, project, labels, instance_type, network_ip, gateway, dns, user_data, token_hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.InstanceID, m.Hostname, m.Project, labels, m.InstanceType,
		m.NetworkIP, m.Gateway, m.DNS, m.UserData, m.TokenHash, m.CreatedAt,
	)
	return err
}

// Get returns the metadata record for an instance.
func (s *Store) Get(instanceID string) (InstanceMetadata, error) {
	row := s.db.QueryRow(
		`SELECT instance_id, hostname, project, network_ip, gateway, dns, user_data, token_hash, created_at
		 FROM instance_metadata WHERE instance_id=?`, instanceID,
	)
	var m InstanceMetadata
	if err := row.Scan(&m.InstanceID, &m.Hostname, &m.Project, &m.NetworkIP,
		&m.Gateway, &m.DNS, &m.UserData, &m.TokenHash, &m.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return InstanceMetadata{}, fmt.Errorf("metadata not found for %s", instanceID)
		}
		return InstanceMetadata{}, err
	}
	return m, nil
}

// GetByIP returns the metadata record for an instance by its network IP.
func (s *Store) GetByIP(networkIP string) (InstanceMetadata, error) {
	row := s.db.QueryRow(
		`SELECT instance_id, hostname, project, network_ip, gateway, dns, user_data, token_hash, created_at
		 FROM instance_metadata WHERE network_ip=? LIMIT 1`, networkIP,
	)
	var m InstanceMetadata
	if err := row.Scan(&m.InstanceID, &m.Hostname, &m.Project, &m.NetworkIP,
		&m.Gateway, &m.DNS, &m.UserData, &m.TokenHash, &m.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return InstanceMetadata{}, fmt.Errorf("no instance for IP %s", networkIP)
		}
		return InstanceMetadata{}, err
	}
	return m, nil
}

// Delete removes the metadata record for an instance.
func (s *Store) Delete(instanceID string) error {
	_, err := s.db.Exec(`DELETE FROM instance_metadata WHERE instance_id=?`, instanceID)
	return err
}

// LogAccess records a metadata access event.
func (s *Store) LogAccess(log AccessLog) error {
	if log.ID == "" {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		log.ID = "mlog_" + hex.EncodeToString(b)
	}
	if log.CreatedAt == "" {
		log.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO metadata_access_log (id, instance_id, source_ip, endpoint, allowed, auth_status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.InstanceID, log.SourceIP, log.Endpoint,
		boolInt(log.Allowed), log.AuthStatus, log.CreatedAt,
	)
	return err
}

// ListAccessLogs returns recent access log entries for an instance.
func (s *Store) ListAccessLogs(instanceID string, limit int) ([]AccessLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, instance_id, source_ip, endpoint, allowed, auth_status, created_at
		 FROM metadata_access_log WHERE instance_id=? ORDER BY created_at DESC LIMIT ?`,
		instanceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccessLog
	for rows.Next() {
		var l AccessLog
		var allowed int
		if err := rows.Scan(&l.ID, &l.InstanceID, &l.SourceIP, &l.Endpoint,
			&allowed, &l.AuthStatus, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.Allowed = allowed != 0
		out = append(out, l)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
