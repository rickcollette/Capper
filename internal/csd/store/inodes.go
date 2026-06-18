package csdstore

import (
	"database/sql"
	"fmt"
	"strings"

	"capper/internal/csd"
)

type InodeStore struct {
	db *sql.DB
}

const inodeCols = `id, volume_id, parent_id, name, inode_type, size_bytes,
	mode_bits, uid, gid, link_count, version, extent_root, inline_data,
	created_at, modified_at, accessed_at`

func (s *InodeStore) Insert(n csd.Inode) error {
	_, err := s.db.Exec(`
		INSERT INTO csd_inodes
			(id, volume_id, parent_id, name, inode_type, size_bytes,
			 mode_bits, uid, gid, link_count, version, extent_root, inline_data,
			 created_at, modified_at, accessed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		n.ID, n.VolumeID, n.ParentID, n.Name, n.Type, n.SizeBytes,
		n.ModeBits, n.UID, n.GID, n.LinkCount, n.Version, n.ExtentRoot, n.InlineData,
		n.CreatedAt, n.ModifiedAt, n.AccessedAt,
	)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return fmt.Errorf("%w: inode %q in parent %q", csd.ErrAlreadyExists, n.Name, n.ParentID)
	}
	return err
}

func (s *InodeStore) Get(id string) (csd.Inode, error) {
	row := s.db.QueryRow(`SELECT `+inodeCols+` FROM csd_inodes WHERE id=?`, id)
	return scanInode(row.Scan)
}

func (s *InodeStore) GetRoot(volumeID string) (csd.Inode, error) {
	row := s.db.QueryRow(`SELECT `+inodeCols+` FROM csd_inodes WHERE volume_id=? AND parent_id='' AND name='/' LIMIT 1`, volumeID)
	return scanInode(row.Scan)
}

func (s *InodeStore) Lookup(volumeID, parentID, name string) (csd.Inode, error) {
	row := s.db.QueryRow(`SELECT `+inodeCols+` FROM csd_inodes WHERE volume_id=? AND parent_id=? AND name=?`,
		volumeID, parentID, name)
	return scanInode(row.Scan)
}

func (s *InodeStore) Children(volumeID, parentID string) ([]csd.Inode, error) {
	rows, err := s.db.Query(`SELECT `+inodeCols+` FROM csd_inodes WHERE volume_id=? AND parent_id=? ORDER BY name`,
		volumeID, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []csd.Inode
	for rows.Next() {
		n, err := scanInode(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *InodeStore) Update(n csd.Inode) error {
	_, err := s.db.Exec(`
		UPDATE csd_inodes SET
			name=?, parent_id=?, inode_type=?, size_bytes=?,
			mode_bits=?, uid=?, gid=?, link_count=?, version=version+1,
			extent_root=?, inline_data=?, modified_at=?, accessed_at=?
		WHERE id=?`,
		n.Name, n.ParentID, n.Type, n.SizeBytes,
		n.ModeBits, n.UID, n.GID, n.LinkCount,
		n.ExtentRoot, n.InlineData, n.ModifiedAt, n.AccessedAt,
		n.ID,
	)
	return err
}

func (s *InodeStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM csd_inodes WHERE id=?`, id)
	return err
}

func (s *InodeStore) Move(id, newParentID, newName string) error {
	_, err := s.db.Exec(`UPDATE csd_inodes SET parent_id=?, name=? WHERE id=?`, newParentID, newName, id)
	return err
}

// ---- helpers ----------------------------------------------------------------

type scanFn func(dest ...any) error

func scanInode(scan scanFn) (csd.Inode, error) {
	var n csd.Inode
	err := scan(
		&n.ID, &n.VolumeID, &n.ParentID, &n.Name, &n.Type, &n.SizeBytes,
		&n.ModeBits, &n.UID, &n.GID, &n.LinkCount, &n.Version, &n.ExtentRoot, &n.InlineData,
		&n.CreatedAt, &n.ModifiedAt, &n.AccessedAt,
	)
	if err == sql.ErrNoRows {
		return n, csd.ErrNotFound
	}
	return n, err
}
