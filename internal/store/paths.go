package store

import (
	"os"
	"path/filepath"
)

type Paths struct {
	Root             string
	Images           string
	Instances        string
	Tmp              string
	DB               string
	StorageVolumes   string
	StorageBuckets   string
	StorageSnapshots string
	Registries       string
	ImportStaging    string
}

func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".capper"), nil
}

func NewPaths(root string) Paths {
	storage := filepath.Join(root, "storage")
	return Paths{
		Root:             root,
		Images:           filepath.Join(root, "images"),
		Instances:        filepath.Join(root, "instances"),
		Tmp:              filepath.Join(root, "tmp"),
		DB:               filepath.Join(root, "capper.db"),
		StorageVolumes:   filepath.Join(storage, "volumes"),
		StorageBuckets:   filepath.Join(storage, "objects"),
		StorageSnapshots: filepath.Join(storage, "snapshots"),
		Registries:       filepath.Join(root, "registries"),
		ImportStaging:    filepath.Join(root, "import-staging"),
	}
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.Root, p.Images, p.Instances, p.Tmp} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
