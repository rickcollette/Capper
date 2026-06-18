package registry

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// InitSchema creates the registry tables. Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS registries (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL UNIQUE,
			backend    TEXT NOT NULL DEFAULT 'filesystem',
			path       TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS registry_images (
			id          TEXT PRIMARY KEY,
			registry_id TEXT NOT NULL,
			name        TEXT NOT NULL,
			version     TEXT NOT NULL,
			digest      TEXT NOT NULL DEFAULT '',
			path        TEXT NOT NULL,
			signed      INTEGER NOT NULL DEFAULT 0,
			scan_status TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			UNIQUE(registry_id, name, version)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_registry_images_name ON registry_images(registry_id, name);`,
		`CREATE TABLE IF NOT EXISTS registry_artifacts (
			id             TEXT PRIMARY KEY,
			registry_id    TEXT NOT NULL,
			name           TEXT NOT NULL,
			version        TEXT NOT NULL,
			type           TEXT NOT NULL DEFAULT '',
			digest         TEXT NOT NULL DEFAULT '',
			path           TEXT NOT NULL,
			size_bytes     INTEGER NOT NULL DEFAULT 0,
			labels_json    TEXT NOT NULL DEFAULT '{}',
			created_at     TEXT NOT NULL,
			UNIQUE(registry_id, name, version)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_registry_artifacts_name ON registry_artifacts(registry_id, name);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("registry.InitSchema: %w", err)
		}
	}
	return nil
}

// Store provides CRUD for registry objects.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ---- registries -------------------------------------------------------------

func (s *Store) InsertRegistry(r Registry) error {
	_, err := s.db.Exec(
		`INSERT INTO registries (id, name, backend, path, created_at)
		VALUES (?,?,?,?,?)`,
		r.ID, r.Name, r.Backend, r.Path, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("registry: insert registry: %w", err)
	}
	return nil
}

func (s *Store) GetRegistry(nameOrID string) (Registry, error) {
	row := s.db.QueryRow(
		`SELECT id, name, backend, path, created_at
		FROM registries WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanRegistry(row)
}

func (s *Store) ListRegistries() ([]Registry, error) {
	rows, err := s.db.Query(
		`SELECT id, name, backend, path, created_at
		FROM registries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Registry
	for rows.Next() {
		r, err := scanRegistry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRegistry(nameOrID string) error {
	r, err := s.GetRegistry(nameOrID)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM registry_images WHERE registry_id=?`, r.ID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM registry_artifacts WHERE registry_id=?`, r.ID); err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM registries WHERE id=?`, r.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("registry: registry %q not found", nameOrID)
	}
	return nil
}

// ---- registry images --------------------------------------------------------

func (s *Store) UpsertImage(img RegistryImage) error {
	_, err := s.db.Exec(
		`INSERT INTO registry_images (id, registry_id, name, version, digest, path, signed, scan_status, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(registry_id, name, version) DO UPDATE SET
			digest=excluded.digest, path=excluded.path, signed=excluded.signed,
			scan_status=excluded.scan_status, created_at=excluded.created_at`,
		img.ID, img.RegistryID, img.Name, img.Version, img.Digest, img.Path,
		boolInt(img.Signed), img.ScanStatus, img.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("registry: upsert image: %w", err)
	}
	return nil
}

func (s *Store) GetImage(registryID, name, version string) (RegistryImage, error) {
	row := s.db.QueryRow(
		`SELECT i.id, i.registry_id, i.name, i.version, i.digest, i.path, i.signed, i.scan_status, i.created_at, r.name
		FROM registry_images i
		JOIN registries r ON r.id = i.registry_id
		WHERE i.registry_id=? AND i.name=? AND i.version=? LIMIT 1`,
		registryID, name, version)
	return scanImage(row)
}

func (s *Store) ListImages(registryID string) ([]RegistryImage, error) {
	var rows *sql.Rows
	var err error
	if registryID != "" {
		rows, err = s.db.Query(
			`SELECT i.id, i.registry_id, i.name, i.version, i.digest, i.path, i.signed, i.scan_status, i.created_at, r.name
			FROM registry_images i
			JOIN registries r ON r.id = i.registry_id
			WHERE i.registry_id=? ORDER BY i.name, i.version`,
			registryID)
	} else {
		rows, err = s.db.Query(
			`SELECT i.id, i.registry_id, i.name, i.version, i.digest, i.path, i.signed, i.scan_status, i.created_at, r.name
			FROM registry_images i
			JOIN registries r ON r.id = i.registry_id
			ORDER BY r.name, i.name, i.version`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RegistryImage
	for rows.Next() {
		img, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, rows.Err()
}

func (s *Store) DeleteImage(registryID, name, version string) error {
	res, err := s.db.Exec(
		`DELETE FROM registry_images WHERE registry_id=? AND name=? AND version=?`,
		registryID, name, version)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("registry: image %s:%s not found in registry", name, version)
	}
	return nil
}

func (s *Store) UpdateImageScanStatus(registryID, name, version, status string) error {
	_, err := s.db.Exec(
		`UPDATE registry_images SET scan_status=? WHERE registry_id=? AND name=? AND version=?`,
		status, registryID, name, version)
	return err
}

// ---- artifacts --------------------------------------------------------------

func (s *Store) UpsertArtifact(a Artifact) error {
	labels, _ := json.Marshal(a.Labels)
	_, err := s.db.Exec(
		`INSERT INTO registry_artifacts
			(id, registry_id, name, version, type, digest, path, size_bytes, labels_json, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(registry_id, name, version) DO UPDATE SET
			type=excluded.type, digest=excluded.digest, path=excluded.path,
			size_bytes=excluded.size_bytes, labels_json=excluded.labels_json, created_at=excluded.created_at`,
		a.ID, a.RegistryID, a.Name, a.Version, a.Type, a.Digest, a.Path,
		a.SizeBytes, string(labels), a.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("registry: upsert artifact: %w", err)
	}
	return nil
}

func (s *Store) GetArtifact(registryID, name, version string) (Artifact, error) {
	row := s.db.QueryRow(
		`SELECT a.id, a.registry_id, a.name, a.version, a.type, a.digest, a.path, a.size_bytes, a.labels_json, a.created_at, r.name
		FROM registry_artifacts a
		JOIN registries r ON r.id = a.registry_id
		WHERE a.registry_id=? AND a.name=? AND a.version=? LIMIT 1`,
		registryID, name, version)
	return scanArtifact(row)
}

func (s *Store) ListArtifacts(registryID string) ([]Artifact, error) {
	var rows *sql.Rows
	var err error
	if registryID != "" {
		rows, err = s.db.Query(
			`SELECT a.id, a.registry_id, a.name, a.version, a.type, a.digest, a.path, a.size_bytes, a.labels_json, a.created_at, r.name
			FROM registry_artifacts a
			JOIN registries r ON r.id = a.registry_id
			WHERE a.registry_id=? ORDER BY a.name, a.version`,
			registryID)
	} else {
		rows, err = s.db.Query(
			`SELECT a.id, a.registry_id, a.name, a.version, a.type, a.digest, a.path, a.size_bytes, a.labels_json, a.created_at, r.name
			FROM registry_artifacts a
			JOIN registries r ON r.id = a.registry_id
			ORDER BY r.name, a.name, a.version`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Artifact
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) DeleteArtifact(registryID, name, version string) error {
	res, err := s.db.Exec(
		`DELETE FROM registry_artifacts WHERE registry_id=? AND name=? AND version=?`,
		registryID, name, version)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("registry: artifact %s:%s not found in registry", name, version)
	}
	return nil
}

// ---- scanners ---------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRegistry(s rowScanner) (Registry, error) {
	var r Registry
	err := s.Scan(&r.ID, &r.Name, &r.Backend, &r.Path, &r.CreatedAt)
	if err != nil {
		return Registry{}, fmt.Errorf("registry: scan registry: %w", err)
	}
	return r, nil
}

func scanImage(s rowScanner) (RegistryImage, error) {
	var img RegistryImage
	var signed int
	err := s.Scan(&img.ID, &img.RegistryID, &img.Name, &img.Version,
		&img.Digest, &img.Path, &signed, &img.ScanStatus, &img.CreatedAt, &img.RegistryName)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry: scan image: %w", err)
	}
	img.Signed = signed != 0
	return img, nil
}

func scanArtifact(s rowScanner) (Artifact, error) {
	var a Artifact
	var labelsJSON string
	err := s.Scan(&a.ID, &a.RegistryID, &a.Name, &a.Version, &a.Type,
		&a.Digest, &a.Path, &a.SizeBytes, &labelsJSON, &a.CreatedAt, &a.RegistryName)
	if err != nil {
		return Artifact{}, fmt.Errorf("registry: scan artifact: %w", err)
	}
	json.Unmarshal([]byte(labelsJSON), &a.Labels)
	return a, nil
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
