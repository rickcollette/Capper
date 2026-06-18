package csdbackend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalBackend stores extents as files under a root directory.
// Layout: <root>/extents/<volume_id>/<object_key>
type LocalBackend struct {
	root string
}

// NewLocalBackend returns a backend that stores extents under root.
func NewLocalBackend(root string) (*LocalBackend, error) {
	if err := os.MkdirAll(filepath.Join(root, "extents"), 0o755); err != nil {
		return nil, fmt.Errorf("csd/local: create extents dir: %w", err)
	}
	return &LocalBackend{root: root}, nil
}

func (b *LocalBackend) extentPath(volumeID, key string) string {
	// Sanitize key to prevent path traversal.
	safe := strings.ReplaceAll(key, "/", "_")
	return filepath.Join(b.root, "extents", volumeID, safe)
}

func (b *LocalBackend) PutExtent(_ context.Context, volumeID, key string, data []byte) error {
	dir := filepath.Join(b.root, "extents", volumeID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("csd/local: mkdir %s: %w", dir, err)
	}
	tmp := b.extentPath(volumeID, key) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("csd/local: write extent %s/%s: %w", volumeID, key, err)
	}
	if err := os.Rename(tmp, b.extentPath(volumeID, key)); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("csd/local: commit extent %s/%s: %w", volumeID, key, err)
	}
	return nil
}

func (b *LocalBackend) GetExtent(_ context.Context, volumeID, key string) ([]byte, error) {
	data, err := os.ReadFile(b.extentPath(volumeID, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("csd/local: extent %s/%s not found", volumeID, key)
	}
	return data, err
}

func (b *LocalBackend) DeleteExtent(_ context.Context, volumeID, key string) error {
	err := os.Remove(b.extentPath(volumeID, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (b *LocalBackend) HasExtent(_ context.Context, volumeID, key string) bool {
	_, err := os.Stat(b.extentPath(volumeID, key))
	return err == nil
}

func (b *LocalBackend) ListExtents(_ context.Context, volumeID string) ([]string, error) {
	dir := filepath.Join(b.root, "extents", volumeID)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			keys = append(keys, e.Name())
		}
	}
	return keys, nil
}
