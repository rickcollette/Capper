package csdstore

import (
	"database/sql"
	"time"

	"capper/internal/csd"
)

type LeaseStore struct {
	db *sql.DB
}

const leaseCols = `id, volume_id, inode_id, client_id, session_id, lease_type,
	range_start, range_end, epoch, expires_at, created_at`

func (s *LeaseStore) Insert(l csd.Lease) error {
	_, err := s.db.Exec(`
		INSERT INTO csd_leases (id, volume_id, inode_id, client_id, session_id, lease_type,
			range_start, range_end, epoch, expires_at, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		l.ID, l.VolumeID, l.InodeID, l.ClientID, l.SessionID, l.LeaseType,
		l.RangeStart, l.RangeEnd, l.Epoch,
		l.ExpiresAt.UTC().Format(time.RFC3339), l.CreatedAt,
	)
	return err
}

func (s *LeaseStore) Get(id string) (csd.Lease, error) {
	row := s.db.QueryRow(`SELECT `+leaseCols+` FROM csd_leases WHERE id=?`, id)
	return scanLease(row.Scan)
}

func (s *LeaseStore) ForVolume(volumeID string) ([]csd.Lease, error) {
	rows, err := s.db.Query(`SELECT `+leaseCols+` FROM csd_leases WHERE volume_id=?`, volumeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLeases(rows)
}

func (s *LeaseStore) ForInode(volumeID, inodeID string) ([]csd.Lease, error) {
	rows, err := s.db.Query(`SELECT `+leaseCols+` FROM csd_leases WHERE volume_id=? AND inode_id=?`,
		volumeID, inodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLeases(rows)
}

func (s *LeaseStore) ForClient(clientID string) ([]csd.Lease, error) {
	rows, err := s.db.Query(`SELECT `+leaseCols+` FROM csd_leases WHERE client_id=?`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLeases(rows)
}

func (s *LeaseStore) Renew(id string, expiresAt time.Time) error {
	_, err := s.db.Exec(`UPDATE csd_leases SET expires_at=? WHERE id=?`,
		expiresAt.UTC().Format(time.RFC3339), id)
	return err
}

func (s *LeaseStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM csd_leases WHERE id=?`, id)
	return err
}

func (s *LeaseStore) DeleteForClient(volumeID, clientID string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM csd_leases WHERE volume_id=? AND client_id=?`, volumeID, clientID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *LeaseStore) DeleteExpired() (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`DELETE FROM csd_leases WHERE expires_at < ?`, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ---- helpers ----------------------------------------------------------------

func scanLease(scan scanFn) (csd.Lease, error) {
	var l csd.Lease
	var expiresStr string
	err := scan(
		&l.ID, &l.VolumeID, &l.InodeID, &l.ClientID, &l.SessionID, &l.LeaseType,
		&l.RangeStart, &l.RangeEnd, &l.Epoch, &expiresStr, &l.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return l, csd.ErrNotFound
	}
	if err == nil {
		l.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr)
	}
	return l, err
}

func scanLeases(rows *sql.Rows) ([]csd.Lease, error) {
	var out []csd.Lease
	for rows.Next() {
		l, err := scanLease(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
