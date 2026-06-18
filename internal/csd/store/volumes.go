package csdstore

import (
	"database/sql"
	"fmt"
	"strings"

	"capper/internal/csd"
)

type VolumeStore struct {
	db *sql.DB
}

func (s *VolumeStore) Insert(v csd.Volume) error {
	_, err := s.db.Exec(`
		INSERT INTO csd_volumes
			(id, project, name, mode, size_bytes, used_bytes, status,
			 realm_id, region_id, zone_id, storage_class, replica_count,
			 primary_node_id, epoch, encrypted, encryption_key_id, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		v.ID, v.Project, v.Name, v.Mode, v.SizeBytes, v.UsedBytes, v.Status,
		v.RealmID, v.RegionID, v.ZoneID, v.StorageClass, v.ReplicaCount,
		v.PrimaryNodeID, v.Epoch, boolInt(v.Encrypted), v.EncryptionKeyID,
		v.CreatedAt, v.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("%w: volume %q in project %q", csd.ErrAlreadyExists, v.Name, v.Project)
		}
		return err
	}
	return nil
}

func (s *VolumeStore) Get(idOrName, project string) (csd.Volume, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(`
			SELECT id, project, name, mode, size_bytes, used_bytes, status,
				realm_id, region_id, zone_id, storage_class, replica_count,
				primary_node_id, epoch, encrypted, encryption_key_id, created_at, updated_at
			FROM csd_volumes WHERE id=? OR name=? LIMIT 1`,
			idOrName, idOrName,
		)
	} else {
		row = s.db.QueryRow(`
			SELECT id, project, name, mode, size_bytes, used_bytes, status,
				realm_id, region_id, zone_id, storage_class, replica_count,
				primary_node_id, epoch, encrypted, encryption_key_id, created_at, updated_at
			FROM csd_volumes WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			idOrName, idOrName, project,
		)
	}
	return scanVolume(row)
}

func (s *VolumeStore) List(project string) ([]csd.Volume, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(`
			SELECT id, project, name, mode, size_bytes, used_bytes, status,
				realm_id, region_id, zone_id, storage_class, replica_count,
				primary_node_id, epoch, encrypted, encryption_key_id, created_at, updated_at
			FROM csd_volumes ORDER BY name`)
	} else {
		rows, err = s.db.Query(`
			SELECT id, project, name, mode, size_bytes, used_bytes, status,
				realm_id, region_id, zone_id, storage_class, replica_count,
				primary_node_id, epoch, encrypted, encryption_key_id, created_at, updated_at
			FROM csd_volumes WHERE project=? ORDER BY name`, project)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vols []csd.Volume
	for rows.Next() {
		v, err := scanVolumeRow(rows)
		if err != nil {
			return nil, err
		}
		vols = append(vols, v)
	}
	return vols, rows.Err()
}

func (s *VolumeStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE csd_volumes SET status=?, updated_at=datetime('now') WHERE id=?`, status, id)
	return err
}

func (s *VolumeStore) UpdateUsed(id string, delta int64) error {
	_, err := s.db.Exec(`
		UPDATE csd_volumes SET used_bytes=MAX(0, used_bytes+?), updated_at=datetime('now') WHERE id=?`,
		delta, id,
	)
	return err
}

func (s *VolumeStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM csd_volumes WHERE id=?`, id)
	return err
}

// BumpEpoch atomically increments the volume epoch and sets updated_at.
// Existing leases issued under the old epoch become stale after this call.
func (s *VolumeStore) BumpEpoch(id, updatedAt string) error {
	_, err := s.db.Exec(`UPDATE csd_volumes SET epoch=epoch+1, updated_at=? WHERE id=?`, updatedAt, id)
	return err
}

// ---- helpers ----------------------------------------------------------------

func scanVolume(row *sql.Row) (csd.Volume, error) {
	var v csd.Volume
	var enc int
	err := row.Scan(
		&v.ID, &v.Project, &v.Name, &v.Mode, &v.SizeBytes, &v.UsedBytes, &v.Status,
		&v.RealmID, &v.RegionID, &v.ZoneID, &v.StorageClass, &v.ReplicaCount,
		&v.PrimaryNodeID, &v.Epoch, &enc, &v.EncryptionKeyID, &v.CreatedAt, &v.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return v, csd.ErrNotFound
	}
	v.Encrypted = enc != 0
	return v, err
}

func scanVolumeRow(rows *sql.Rows) (csd.Volume, error) {
	var v csd.Volume
	var enc int
	err := rows.Scan(
		&v.ID, &v.Project, &v.Name, &v.Mode, &v.SizeBytes, &v.UsedBytes, &v.Status,
		&v.RealmID, &v.RegionID, &v.ZoneID, &v.StorageClass, &v.ReplicaCount,
		&v.PrimaryNodeID, &v.Epoch, &enc, &v.EncryptionKeyID, &v.CreatedAt, &v.UpdatedAt,
	)
	v.Encrypted = enc != 0
	return v, err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
