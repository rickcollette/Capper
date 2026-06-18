package csdstore

import (
	"database/sql"

	"capper/internal/csd"
)

type ExtentStore struct {
	db *sql.DB
}

const extentCols = `id, volume_id, inode_id, offset_bytes, length_bytes, object_key, checksum, ref_count, created_at`

func (s *ExtentStore) Insert(e csd.Extent) error {
	_, err := s.db.Exec(`
		INSERT INTO csd_extents (id, volume_id, inode_id, offset_bytes, length_bytes, object_key, checksum, ref_count, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		e.ID, e.VolumeID, e.InodeID, e.OffsetBytes, e.LengthBytes, e.ObjectKey, e.Checksum, e.RefCount, e.CreatedAt,
	)
	return err
}

func (s *ExtentStore) Upsert(e csd.Extent) error {
	_, err := s.db.Exec(`
		INSERT INTO csd_extents (id, volume_id, inode_id, offset_bytes, length_bytes, object_key, checksum, ref_count, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			length_bytes=excluded.length_bytes, object_key=excluded.object_key,
			checksum=excluded.checksum`,
		e.ID, e.VolumeID, e.InodeID, e.OffsetBytes, e.LengthBytes, e.ObjectKey, e.Checksum, e.RefCount, e.CreatedAt,
	)
	return err
}

func (s *ExtentStore) ForInode(inodeID string) ([]csd.Extent, error) {
	rows, err := s.db.Query(`SELECT `+extentCols+` FROM csd_extents WHERE inode_id=? ORDER BY offset_bytes`, inodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExtents(rows)
}

func (s *ExtentStore) ForInodeRange(inodeID string, start, end int64) ([]csd.Extent, error) {
	rows, err := s.db.Query(`
		SELECT `+extentCols+` FROM csd_extents
		WHERE inode_id=? AND offset_bytes < ? AND (offset_bytes + length_bytes) > ?
		ORDER BY offset_bytes`,
		inodeID, end, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExtents(rows)
}

func (s *ExtentStore) ForVolume(volumeID string) ([]csd.Extent, error) {
	rows, err := s.db.Query(`SELECT `+extentCols+` FROM csd_extents WHERE volume_id=?`, volumeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExtents(rows)
}

func (s *ExtentStore) DecrRef(id string) error {
	_, err := s.db.Exec(`UPDATE csd_extents SET ref_count=ref_count-1 WHERE id=?`, id)
	return err
}

func (s *ExtentStore) IncrRef(id string) error {
	_, err := s.db.Exec(`UPDATE csd_extents SET ref_count=ref_count+1 WHERE id=?`, id)
	return err
}

func (s *ExtentStore) DeleteOrphans() ([]csd.Extent, error) {
	rows, err := s.db.Query(`SELECT `+extentCols+` FROM csd_extents WHERE ref_count <= 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orphans, err := scanExtents(rows)
	if err != nil {
		return nil, err
	}
	for _, e := range orphans {
		if _, err := s.db.Exec(`DELETE FROM csd_extents WHERE id=?`, e.ID); err != nil {
			return nil, err
		}
	}
	return orphans, nil
}

func (s *ExtentStore) DeleteForInode(inodeID string) ([]csd.Extent, error) {
	rows, err := s.db.Query(`SELECT `+extentCols+` FROM csd_extents WHERE inode_id=?`, inodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	extents, err := scanExtents(rows)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE csd_extents SET ref_count=ref_count-1 WHERE inode_id=?`, inodeID); err != nil {
		return nil, err
	}
	return extents, nil
}

// ---- helpers ----------------------------------------------------------------

func scanExtents(rows *sql.Rows) ([]csd.Extent, error) {
	var out []csd.Extent
	for rows.Next() {
		var e csd.Extent
		if err := rows.Scan(
			&e.ID, &e.VolumeID, &e.InodeID, &e.OffsetBytes, &e.LengthBytes,
			&e.ObjectKey, &e.Checksum, &e.RefCount, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
