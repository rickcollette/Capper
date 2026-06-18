package csdstore

import (
	"database/sql"

	"capper/internal/csd"
)

type SnapshotStore struct {
	db *sql.DB
}

const snapCols = `id, volume_id, name, root_version, status, consistent, size_bytes, created_at`

func (s *SnapshotStore) Insert(snap csd.Snapshot) error {
	_, err := s.db.Exec(`
		INSERT INTO csd_volume_snapshots (id, volume_id, name, root_version, status, consistent, size_bytes, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		snap.ID, snap.VolumeID, snap.Name, snap.RootVersion,
		snap.Status, boolInt(snap.Consistent), snap.SizeBytes, snap.CreatedAt,
	)
	return err
}

func (s *SnapshotStore) Get(volumeID, nameOrID string) (csd.Snapshot, error) {
	row := s.db.QueryRow(`SELECT `+snapCols+` FROM csd_volume_snapshots WHERE (id=? OR name=?) AND volume_id=? LIMIT 1`,
		nameOrID, nameOrID, volumeID)
	return scanSnapshot(row)
}

func (s *SnapshotStore) List(volumeID string) ([]csd.Snapshot, error) {
	rows, err := s.db.Query(`SELECT `+snapCols+` FROM csd_volume_snapshots WHERE volume_id=? ORDER BY created_at`, volumeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []csd.Snapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

func (s *SnapshotStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE csd_volume_snapshots SET status=? WHERE id=?`, status, id)
	return err
}

func (s *SnapshotStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM csd_volume_snapshots WHERE id=?`, id)
	return err
}

// ---- helpers ----------------------------------------------------------------

func scanSnapshot(scanner interface {
	Scan(...any) error
}) (csd.Snapshot, error) {
	var snap csd.Snapshot
	var consistent int
	err := scanner.Scan(
		&snap.ID, &snap.VolumeID, &snap.Name, &snap.RootVersion,
		&snap.Status, &consistent, &snap.SizeBytes, &snap.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return snap, csd.ErrNotFound
	}
	snap.Consistent = consistent != 0
	return snap, err
}
