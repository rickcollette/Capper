package store

import (
	"os"
	"path/filepath"

	"capper/internal/types"
)

func (s *Store) UpsertImage(img types.ImageRecord) error {
	_, err := s.DB.Exec(`
		INSERT INTO images (id, name, version, path, created_at, size_bytes, digest)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			id=excluded.id,
			version=excluded.version,
			path=excluded.path,
			created_at=excluded.created_at,
			size_bytes=excluded.size_bytes,
			digest=excluded.digest
	`, img.ID, img.Name, img.Version, img.Path, img.CreatedAt, img.SizeBytes, img.Digest)
	return err
}

func (s *Store) GetImage(name string) (*types.ImageRecord, error) {
	row := s.DB.QueryRow(`SELECT id, name, version, path, created_at, size_bytes, digest FROM images WHERE name = ?`, name)
	var img types.ImageRecord
	if err := row.Scan(&img.ID, &img.Name, &img.Version, &img.Path, &img.CreatedAt, &img.SizeBytes, &img.Digest); err != nil {
		return nil, err
	}
	img.Path = s.ResolveImagePath(img.Path)
	return &img, nil
}

func (s *Store) DeleteImage(name string) error {
	_, err := s.DB.Exec(`DELETE FROM images WHERE name = ?`, name)
	return err
}

func (s *Store) ListImages() ([]types.ImageRecord, error) {
	rows, err := s.DB.Query(`SELECT id, name, version, path, created_at, size_bytes, digest FROM images ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]types.ImageRecord, 0)
	for rows.Next() {
		var img types.ImageRecord
		if err := rows.Scan(&img.ID, &img.Name, &img.Version, &img.Path, &img.CreatedAt, &img.SizeBytes, &img.Digest); err != nil {
			return nil, err
		}
		img.Path = s.ResolveImagePath(img.Path)
		out = append(out, img)
	}
	return out, rows.Err()
}

func (s *Store) ResolveImagePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	candidates := []string{
		filepath.Join(filepath.Dir(s.Paths.Root), path),
		filepath.Join(s.Paths.Root, path),
		path,
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}
