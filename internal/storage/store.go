package storage

import (
	"database/sql"
	"fmt"
	"strings"
)

// InitSchema creates the storage tables. Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS storage_volumes (
			id                   TEXT PRIMARY KEY,
			name                 TEXT NOT NULL UNIQUE,
			size_bytes           INTEGER NOT NULL DEFAULT 0,
			class                TEXT NOT NULL DEFAULT 'local',
			backend              TEXT NOT NULL DEFAULT 'directory',
			path                 TEXT NOT NULL,
			encrypted            INTEGER NOT NULL DEFAULT 0,
			attached_instance_id TEXT NOT NULL DEFAULT '',
			attached_path        TEXT NOT NULL DEFAULT '',
			created_at           TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_storage_volumes_attached ON storage_volumes(attached_instance_id);`,
		`CREATE TABLE IF NOT EXISTS storage_buckets (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL UNIQUE,
			backend     TEXT NOT NULL DEFAULT 'local',
			path        TEXT NOT NULL,
			versioning  INTEGER NOT NULL DEFAULT 0,
			encrypted   INTEGER NOT NULL DEFAULT 0,
			quota_bytes INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS storage_snapshots (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL UNIQUE,
			source_type TEXT NOT NULL,
			source_id   TEXT NOT NULL,
			path        TEXT NOT NULL,
			digest      TEXT NOT NULL DEFAULT '',
			size_bytes  INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("storage.InitSchema: %w", err)
		}
	}
	// Additive migrations for existing databases.
	for _, alter := range []string{
		`ALTER TABLE storage_buckets ADD COLUMN kms_key_name TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := db.Exec(alter); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("storage.InitSchema migrate: %w", err)
			}
		}
	}
	return nil
}

// Store provides CRUD for storage objects.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ---- volumes ----------------------------------------------------------------

func (s *Store) InsertVolume(v Volume) error {
	_, err := s.db.Exec(
		`INSERT INTO storage_volumes
			(id, name, size_bytes, class, backend, path, encrypted, attached_instance_id, attached_path, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		v.ID, v.Name, v.SizeBytes, v.Class, v.Backend, v.Path,
		boolInt(v.Encrypted), v.AttachedInstanceID, v.AttachedPath, v.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("storage: insert volume: %w", err)
	}
	return nil
}

func (s *Store) GetVolume(nameOrID string) (Volume, error) {
	row := s.db.QueryRow(
		`SELECT id, name, size_bytes, class, backend, path, encrypted, attached_instance_id, attached_path, created_at
		FROM storage_volumes WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanVolume(row)
}

func (s *Store) ListVolumes() ([]Volume, error) {
	rows, err := s.db.Query(
		`SELECT id, name, size_bytes, class, backend, path, encrypted, attached_instance_id, attached_path, created_at
		FROM storage_volumes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Volume
	for rows.Next() {
		v, err := scanVolume(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) UpdateVolumeAttachment(nameOrID, instanceID, attachedPath string) error {
	_, err := s.db.Exec(
		`UPDATE storage_volumes SET attached_instance_id=?, attached_path=? WHERE id=? OR name=?`,
		instanceID, attachedPath, nameOrID, nameOrID)
	return err
}

func (s *Store) DeleteVolume(nameOrID string) error {
	res, err := s.db.Exec(`DELETE FROM storage_volumes WHERE id=? OR name=?`, nameOrID, nameOrID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("storage: volume %q not found", nameOrID)
	}
	return nil
}

// ---- buckets ----------------------------------------------------------------

func (s *Store) InsertBucket(b Bucket) error {
	_, err := s.db.Exec(
		`INSERT INTO storage_buckets (id, name, backend, path, versioning, encrypted, quota_bytes, kms_key_name, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		b.ID, b.Name, b.Backend, b.Path, boolInt(b.Versioning), boolInt(b.Encrypted), b.QuotaBytes, b.KMSKeyName, b.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("storage: insert bucket: %w", err)
	}
	return nil
}

func (s *Store) GetBucket(nameOrID string) (Bucket, error) {
	row := s.db.QueryRow(
		`SELECT id, name, backend, path, versioning, encrypted, quota_bytes, kms_key_name, created_at
		FROM storage_buckets WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanBucket(row)
}

func (s *Store) ListBuckets() ([]Bucket, error) {
	rows, err := s.db.Query(
		`SELECT id, name, backend, path, versioning, encrypted, quota_bytes, kms_key_name, created_at
		FROM storage_buckets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Bucket
	for rows.Next() {
		b, err := scanBucket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBucket(id string) error {
	res, err := s.db.Exec(`DELETE FROM storage_buckets WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("storage: bucket not found")
	}
	return nil
}

// ---- snapshots --------------------------------------------------------------

func (s *Store) InsertSnapshot(snap Snapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO storage_snapshots (id, name, source_type, source_id, path, digest, size_bytes, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		snap.ID, snap.Name, snap.SourceType, snap.SourceID, snap.Path,
		snap.Digest, snap.SizeBytes, snap.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("storage: insert snapshot: %w", err)
	}
	return nil
}

func (s *Store) GetSnapshot(nameOrID string) (Snapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, name, source_type, source_id, path, digest, size_bytes, created_at
		FROM storage_snapshots WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanSnapshot(row)
}

func (s *Store) ListSnapshots(sourceID string) ([]Snapshot, error) {
	var rows *sql.Rows
	var err error
	if sourceID != "" {
		rows, err = s.db.Query(
			`SELECT id, name, source_type, source_id, path, digest, size_bytes, created_at
			FROM storage_snapshots WHERE source_id=? ORDER BY created_at DESC`,
			sourceID)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, source_type, source_id, path, digest, size_bytes, created_at
			FROM storage_snapshots ORDER BY created_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSnapshot(nameOrID string) error {
	res, err := s.db.Exec(`DELETE FROM storage_snapshots WHERE id=? OR name=?`, nameOrID, nameOrID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("storage: snapshot %q not found", nameOrID)
	}
	return nil
}

// ---- scanners ---------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanVolume(s rowScanner) (Volume, error) {
	var v Volume
	var encrypted int
	err := s.Scan(&v.ID, &v.Name, &v.SizeBytes, &v.Class, &v.Backend, &v.Path,
		&encrypted, &v.AttachedInstanceID, &v.AttachedPath, &v.CreatedAt)
	if err != nil {
		return Volume{}, fmt.Errorf("storage: scan volume: %w", err)
	}
	v.Encrypted = encrypted != 0
	return v, nil
}

func scanBucket(s rowScanner) (Bucket, error) {
	var b Bucket
	var versioning, encrypted int
	err := s.Scan(&b.ID, &b.Name, &b.Backend, &b.Path, &versioning, &encrypted, &b.QuotaBytes, &b.KMSKeyName, &b.CreatedAt)
	if err != nil {
		return Bucket{}, fmt.Errorf("storage: scan bucket: %w", err)
	}
	b.Versioning = versioning != 0
	b.Encrypted = encrypted != 0
	return b, nil
}

func scanSnapshot(s rowScanner) (Snapshot, error) {
	var snap Snapshot
	err := s.Scan(&snap.ID, &snap.Name, &snap.SourceType, &snap.SourceID,
		&snap.Path, &snap.Digest, &snap.SizeBytes, &snap.CreatedAt)
	if err != nil {
		return Snapshot{}, fmt.Errorf("storage: scan snapshot: %w", err)
	}
	return snap, nil
}

// ---- helpers ----------------------------------------------------------------

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "no rows")
}

// ---- storage shares ---------------------------------------------------------

// InitShareSchema creates the storage_shares table. Safe to call multiple times.
func InitShareSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS storage_shares (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL UNIQUE,
		host_path   TEXT NOT NULL,
		mount_path  TEXT NOT NULL,
		instance_id TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("storage.InitShareSchema: %w", err)
	}
	return nil
}

// Share represents a host filesystem path shared into an instance.
type Share struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	HostPath   string `json:"hostPath"`
	MountPath  string `json:"mountPath"`
	InstanceID string `json:"instanceId,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

func (s *Store) InsertShare(sh Share) error {
	_, err := s.db.Exec(
		`INSERT INTO storage_shares (id, name, host_path, mount_path, instance_id, created_at)
		VALUES (?,?,?,?,?,?)`,
		sh.ID, sh.Name, sh.HostPath, sh.MountPath, sh.InstanceID, sh.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("storage: insert share: %w", err)
	}
	return nil
}

func (s *Store) GetShare(nameOrID string) (Share, error) {
	row := s.db.QueryRow(
		`SELECT id, name, host_path, mount_path, instance_id, created_at
		FROM storage_shares WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanShare(row)
}

func (s *Store) ListShares() ([]Share, error) {
	rows, err := s.db.Query(
		`SELECT id, name, host_path, mount_path, instance_id, created_at
		FROM storage_shares ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Share
	for rows.Next() {
		sh, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

func (s *Store) DeleteShare(nameOrID string) error {
	res, err := s.db.Exec(`DELETE FROM storage_shares WHERE id=? OR name=?`, nameOrID, nameOrID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("storage: share %q not found", nameOrID)
	}
	return nil
}

func scanShare(s rowScanner) (Share, error) {
	var sh Share
	err := s.Scan(&sh.ID, &sh.Name, &sh.HostPath, &sh.MountPath, &sh.InstanceID, &sh.CreatedAt)
	if err != nil {
		return Share{}, fmt.Errorf("storage: scan share: %w", err)
	}
	return sh, nil
}
