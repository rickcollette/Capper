package network

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Store persists network records and IP leases.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by an already-initialised database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InitSchema creates the networks and network_leases tables.
// Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS networks (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			project    TEXT NOT NULL DEFAULT '',
			mode       TEXT NOT NULL DEFAULT 'nat',
			subnet     TEXT NOT NULL,
			gateway    TEXT NOT NULL,
			bridge     TEXT NOT NULL,
			labels     TEXT NOT NULL DEFAULT '{}',
			status     TEXT NOT NULL DEFAULT 'pending',
			error_msg  TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS networks_name_project ON networks(name, project)`,
		`CREATE TABLE IF NOT EXISTS network_leases (
			network_id  TEXT NOT NULL,
			instance_id TEXT NOT NULL,
			ip          TEXT NOT NULL,
			mac         TEXT NOT NULL,
			created_at  TEXT NOT NULL,
			PRIMARY KEY(network_id, instance_id)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS network_leases_ip ON network_leases(network_id, ip)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("network: schema: %w", err)
		}
	}
	// Migration: add error_msg column to existing databases (ignore if already present).
	_, _ = db.Exec(`ALTER TABLE networks ADD COLUMN error_msg TEXT NOT NULL DEFAULT ''`)
	return nil
}

// Insert stores a new network record.
func (s *Store) Insert(n Network) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if n.CreatedAt == "" {
		n.CreatedAt = now
	}
	labels, _ := json.Marshal(n.Labels)
	_, err := s.db.Exec(
		`INSERT INTO networks(id, name, project, mode, subnet, gateway, bridge, labels, status, error_msg, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		n.ID, n.Name, n.Project, n.Mode, n.Subnet, n.Gateway, n.Bridge,
		string(labels), n.Status, n.Error, n.CreatedAt,
	)
	return err
}

// UpdateStatus sets the status field of the given network.
func (s *Store) UpdateStatus(id, status string) error {
	return s.UpdateStatusError(id, status, "")
}

// UpdateStatusError sets the status and error_msg fields of the given network.
func (s *Store) UpdateStatusError(id, status, errMsg string) error {
	res, err := s.db.Exec(`UPDATE networks SET status=?, error_msg=? WHERE id=?`, status, errMsg, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("network %q not found", id)
	}
	return nil
}

// Get returns the network with the given ID or name+project.
func (s *Store) Get(nameOrID, project string) (Network, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, mode, subnet, gateway, bridge, labels, status, error_msg, created_at
			 FROM networks WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, mode, subnet, gateway, bridge, labels, status, error_msg, created_at
			 FROM networks WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	}
	return scanNetwork(row)
}

// List returns all networks optionally filtered by project.
func (s *Store) List(project string) ([]Network, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, mode, subnet, gateway, bridge, labels, status, error_msg, created_at
			 FROM networks ORDER BY created_at`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, mode, subnet, gateway, bridge, labels, status, error_msg, created_at
			 FROM networks WHERE project=? ORDER BY created_at`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Network, 0)
	for rows.Next() {
		n, err := scanNetwork(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Delete removes the network record. Fails if active leases exist.
func (s *Store) Delete(id string) error {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM network_leases WHERE network_id=?`, id).Scan(&count)
	if count > 0 {
		return fmt.Errorf("network %q has %d active lease(s); disconnect all instances first", id, count)
	}
	res, err := s.db.Exec(`DELETE FROM networks WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("network %q not found", id)
	}
	return nil
}

// ---- leases -----------------------------------------------------------------

// InsertLease records an IP/MAC assignment.
func (s *Store) InsertLease(l NetworkLease) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if l.CreatedAt == "" {
		l.CreatedAt = now
	}
	_, err := s.db.Exec(
		`INSERT INTO network_leases(network_id, instance_id, ip, mac, created_at)
		 VALUES(?,?,?,?,?)`,
		l.NetworkID, l.InstanceID, l.IP, l.MAC, l.CreatedAt,
	)
	return err
}

// DeleteLease removes the lease for the given instance on the given network.
func (s *Store) DeleteLease(networkID, instanceID string) error {
	_, err := s.db.Exec(
		`DELETE FROM network_leases WHERE network_id=? AND instance_id=?`,
		networkID, instanceID,
	)
	return err
}

// LeasesForNetwork returns all leases on a network.
func (s *Store) LeasesForNetwork(networkID string) ([]NetworkLease, error) {
	rows, err := s.db.Query(
		`SELECT network_id, instance_id, ip, mac, created_at
		 FROM network_leases WHERE network_id=? ORDER BY created_at`,
		networkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLeases(rows)
}

// LeasesForInstance returns all leases held by the given instance.
func (s *Store) LeasesForInstance(instanceID string) ([]NetworkLease, error) {
	rows, err := s.db.Query(
		`SELECT network_id, instance_id, ip, mac, created_at
		 FROM network_leases WHERE instance_id=? ORDER BY created_at`,
		instanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLeases(rows)
}

// AllocatedIPs returns the set of IPs already assigned on a network.
func (s *Store) AllocatedIPs(networkID string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT ip FROM network_leases WHERE network_id=?`, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		out[ip] = true
	}
	return out, rows.Err()
}

// ---- helpers ----------------------------------------------------------------

type rowScanner interface{ Scan(...any) error }

func scanNetwork(s rowScanner) (Network, error) {
	var n Network
	var labels string
	err := s.Scan(&n.ID, &n.Name, &n.Project, &n.Mode, &n.Subnet, &n.Gateway,
		&n.Bridge, &labels, &n.Status, &n.Error, &n.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Network{}, fmt.Errorf("network not found")
		}
		return Network{}, err
	}
	_ = json.Unmarshal([]byte(labels), &n.Labels)
	return n, nil
}

func scanLeases(rows *sql.Rows) ([]NetworkLease, error) {
	out := make([]NetworkLease, 0)
	for rows.Next() {
		var l NetworkLease
		if err := rows.Scan(&l.NetworkID, &l.InstanceID, &l.IP, &l.MAC, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
