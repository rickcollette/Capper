package csdstore

import (
	"database/sql"

	"capper/internal/csd"
)

type ReplicaStore struct {
	db *sql.DB
}

const replicaCols = `id, volume_id, role, realm_id, region_id, zone_id, node_id, addr,
	backend_type, backend_path, status, lag_bytes, last_seq, created_at, updated_at`

func (s *ReplicaStore) Insert(r csd.Replica) error {
	_, err := s.db.Exec(`
		INSERT INTO csd_volume_replicas
			(id, volume_id, role, realm_id, region_id, zone_id, node_id, addr,
			 backend_type, backend_path, status, lag_bytes, last_seq, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.VolumeID, r.Role, r.RealmID, r.RegionID, r.ZoneID, r.NodeID, r.Addr,
		r.BackendType, r.BackendPath, r.Status, r.LagBytes, r.LastSeq, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

func (s *ReplicaStore) Get(id string) (csd.Replica, error) {
	row := s.db.QueryRow(`SELECT `+replicaCols+` FROM csd_volume_replicas WHERE id=?`, id)
	return scanReplica(row)
}

func (s *ReplicaStore) ListByVolume(volumeID string) ([]csd.Replica, error) {
	rows, err := s.db.Query(`SELECT `+replicaCols+` FROM csd_volume_replicas WHERE volume_id=? ORDER BY role`, volumeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReplicas(rows)
}

func (s *ReplicaStore) ListAll() ([]csd.Replica, error) {
	rows, err := s.db.Query(`SELECT ` + replicaCols + ` FROM csd_volume_replicas ORDER BY volume_id, role`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReplicas(rows)
}

func (s *ReplicaStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE csd_volume_replicas SET status=? WHERE id=?`, status, id)
	return err
}

func (s *ReplicaStore) UpdateProgress(id string, lastSeq, lagBytes int64, updatedAt string) error {
	_, err := s.db.Exec(`
		UPDATE csd_volume_replicas SET last_seq=?, lag_bytes=?, updated_at=? WHERE id=?`,
		lastSeq, lagBytes, updatedAt, id)
	return err
}

func (s *ReplicaStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM csd_volume_replicas WHERE id=?`, id)
	return err
}

// ---- helpers ----------------------------------------------------------------

func scanReplica(row interface{ Scan(...any) error }) (csd.Replica, error) {
	var r csd.Replica
	err := row.Scan(
		&r.ID, &r.VolumeID, &r.Role, &r.RealmID, &r.RegionID, &r.ZoneID, &r.NodeID, &r.Addr,
		&r.BackendType, &r.BackendPath, &r.Status, &r.LagBytes, &r.LastSeq, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return r, csd.ErrNotFound
	}
	return r, err
}

func scanReplicas(rows *sql.Rows) ([]csd.Replica, error) {
	var out []csd.Replica
	for rows.Next() {
		r, err := scanReplica(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
