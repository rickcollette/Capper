package registry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Manager provides high-level registry lifecycle operations.
type Manager struct {
	store    *Store
	rootPath string // ~/.local/share/capper/registries
}

// NewManager returns a Manager backed by the given store.
// rootPath is the directory under which all registry subdirectories are created.
func NewManager(s *Store, rootPath string) *Manager {
	return &Manager{store: s, rootPath: rootPath}
}

// EnsureRoot creates the registry root directory if it does not exist.
func (m *Manager) EnsureRoot() error {
	return os.MkdirAll(m.rootPath, 0o700)
}

// ---- registries -------------------------------------------------------------

// Init creates a registry with the given name if it doesn't already exist.
// This is idempotent: calling it a second time returns the existing registry.
func (m *Manager) Init(name string) (Registry, error) {
	if name == "" {
		return Registry{}, fmt.Errorf("registry: name is required")
	}
	existing, err := m.store.GetRegistry(name)
	if err == nil {
		return existing, nil
	}
	if !isNotFound(err) {
		return Registry{}, err
	}
	regPath := filepath.Join(m.rootPath, name)
	for _, sub := range []string{"images", "artifacts"} {
		if err := os.MkdirAll(filepath.Join(regPath, sub), 0o700); err != nil {
			return Registry{}, fmt.Errorf("registry: create %s dir: %w", sub, err)
		}
	}
	r := Registry{
		ID:        newID("reg"),
		Name:      strings.ToLower(name),
		Backend:   BackendFilesystem,
		Path:      regPath,
		CreatedAt: now(),
	}
	if err := m.store.InsertRegistry(r); err != nil {
		return Registry{}, err
	}
	return r, nil
}

// GetRegistry returns a registry by name or ID.
func (m *Manager) GetRegistry(nameOrID string) (Registry, error) {
	r, err := m.store.GetRegistry(nameOrID)
	if err != nil {
		return Registry{}, fmt.Errorf("registry: %q not found: %w", nameOrID, err)
	}
	return r, nil
}

// ListRegistries returns all registries.
func (m *Manager) ListRegistries() ([]Registry, error) {
	return m.store.ListRegistries()
}

// DeleteRegistry removes a registry's directory and all its records.
func (m *Manager) DeleteRegistry(nameOrID string) error {
	r, err := m.store.GetRegistry(nameOrID)
	if err != nil {
		return fmt.Errorf("registry: %q not found", nameOrID)
	}
	if err := os.RemoveAll(r.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("registry: remove dir: %w", err)
	}
	return m.store.DeleteRegistry(r.ID)
}

// GC removes top-level name directories inside a registry that have no
// corresponding record. For images it scans registries/<name>/images/<NAME>/;
// for artifacts registries/<name>/artifacts/<NAME>/.
func (m *Manager) GC(registryNameOrID string) (int, error) {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return 0, fmt.Errorf("registry: %q not found", registryNameOrID)
	}

	// Build sets of known name-level directories.
	knownImages := make(map[string]bool)
	imgs, _ := m.store.ListImages(r.ID)
	for _, img := range imgs {
		knownImages[img.Name] = true
	}

	knownArtifacts := make(map[string]bool)
	arts, _ := m.store.ListArtifacts(r.ID)
	for _, a := range arts {
		knownArtifacts[a.Name] = true
	}

	removed := 0
	for _, pair := range []struct {
		sub   string
		known map[string]bool
	}{
		{"images", knownImages},
		{"artifacts", knownArtifacts},
	} {
		dir := filepath.Join(r.Path, pair.sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !pair.known[e.Name()] {
				if err := os.RemoveAll(filepath.Join(dir, e.Name())); err == nil {
					removed++
				}
			}
		}
	}
	return removed, nil
}

// ---- push / pull ------------------------------------------------------------

// Push copies a .cap image file into the registry and records its metadata.
func (m *Manager) Push(registryNameOrID, imageName, version, srcPath string) (RegistryImage, error) {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	destDir := filepath.Join(r.Path, "images", imageName, version)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return RegistryImage{}, fmt.Errorf("registry: create image dir: %w", err)
	}
	destPath := filepath.Join(destDir, filepath.Base(srcPath))
	size, digest, err := copyWithDigest(srcPath, destPath)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry: copy image: %w", err)
	}
	_ = size
	img := RegistryImage{
		ID:         newID("rimg"),
		RegistryID: r.ID,
		Name:       imageName,
		Version:    version,
		Digest:     digest,
		Path:       destPath,
		CreatedAt:  now(),
	}
	if err := m.store.UpsertImage(img); err != nil {
		return RegistryImage{}, err
	}
	img.RegistryName = r.Name
	return img, nil
}

// Pull copies an image from the registry to destPath.
func (m *Manager) Pull(registryNameOrID, imageName, version, destPath string) (RegistryImage, error) {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	img, err := m.store.GetImage(r.ID, imageName, version)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry: image %s:%s not found in %s", imageName, version, r.Name)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return RegistryImage{}, err
	}
	if _, _, err := copyWithDigest(img.Path, destPath); err != nil {
		return RegistryImage{}, fmt.Errorf("registry: pull image: %w", err)
	}
	return img, nil
}

// TagImage creates or updates a version alias for an image already in the registry.
func (m *Manager) TagImage(registryNameOrID, imageName, srcVersion, newVersion string) (RegistryImage, error) {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	src, err := m.store.GetImage(r.ID, imageName, srcVersion)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry: source %s:%s not found", imageName, srcVersion)
	}
	destDir := filepath.Join(r.Path, "images", imageName, newVersion)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return RegistryImage{}, err
	}
	destPath := filepath.Join(destDir, filepath.Base(src.Path))
	if _, _, err := copyWithDigest(src.Path, destPath); err != nil {
		return RegistryImage{}, fmt.Errorf("registry: tag copy: %w", err)
	}
	tagged := RegistryImage{
		ID:           newID("rimg"),
		RegistryID:   r.ID,
		Name:         imageName,
		Version:      newVersion,
		Digest:       src.Digest,
		Path:         destPath,
		Signed:       src.Signed,
		CreatedAt:    now(),
		RegistryName: r.Name,
	}
	if err := m.store.UpsertImage(tagged); err != nil {
		return RegistryImage{}, err
	}
	return tagged, nil
}

// ListImages returns all images in a registry, or all images when registryNameOrID is empty.
func (m *Manager) ListImages(registryNameOrID string) ([]RegistryImage, error) {
	if registryNameOrID == "" {
		return m.store.ListImages("")
	}
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return nil, fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	return m.store.ListImages(r.ID)
}

// DeleteImage removes a specific image version from a registry.
func (m *Manager) DeleteImage(registryNameOrID, imageName, version string) error {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	img, err := m.store.GetImage(r.ID, imageName, version)
	if err != nil {
		return fmt.Errorf("registry: image %s:%s not found", imageName, version)
	}
	if err := os.Remove(img.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("registry: remove image file: %w", err)
	}
	return m.store.DeleteImage(r.ID, imageName, version)
}

// ---- artifacts --------------------------------------------------------------

// PutArtifact copies a file into the registry as a versioned artifact.
func (m *Manager) PutArtifact(registryNameOrID, name, version, artifactType, srcPath string, labels map[string]string) (Artifact, error) {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return Artifact{}, fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	destDir := filepath.Join(r.Path, "artifacts", name, version)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return Artifact{}, fmt.Errorf("registry: create artifact dir: %w", err)
	}
	destPath := filepath.Join(destDir, filepath.Base(srcPath))
	size, digest, err := copyWithDigest(srcPath, destPath)
	if err != nil {
		return Artifact{}, fmt.Errorf("registry: copy artifact: %w", err)
	}
	if artifactType == "" {
		artifactType = inferType(srcPath)
	}
	a := Artifact{
		ID:           newID("art"),
		RegistryID:   r.ID,
		Name:         name,
		Version:      version,
		Type:         artifactType,
		Digest:       digest,
		Path:         destPath,
		SizeBytes:    size,
		Labels:       labels,
		CreatedAt:    now(),
		RegistryName: r.Name,
	}
	if err := m.store.UpsertArtifact(a); err != nil {
		return Artifact{}, err
	}
	return a, nil
}

// GetArtifact copies an artifact from the registry to destPath.
func (m *Manager) GetArtifact(registryNameOrID, name, version, destPath string) (Artifact, error) {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return Artifact{}, fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	a, err := m.store.GetArtifact(r.ID, name, version)
	if err != nil {
		return Artifact{}, fmt.Errorf("registry: artifact %s:%s not found in %s", name, version, r.Name)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if _, _, err := copyWithDigest(a.Path, destPath); err != nil {
		return Artifact{}, fmt.Errorf("registry: get artifact: %w", err)
	}
	return a, nil
}

// ListArtifacts returns all artifacts in a registry.
func (m *Manager) ListArtifacts(registryNameOrID string) ([]Artifact, error) {
	if registryNameOrID == "" {
		return m.store.ListArtifacts("")
	}
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return nil, fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	return m.store.ListArtifacts(r.ID)
}

// DeleteArtifact removes a specific artifact version from a registry.
func (m *Manager) DeleteArtifact(registryNameOrID, name, version string) error {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	a, err := m.store.GetArtifact(r.ID, name, version)
	if err != nil {
		return fmt.Errorf("registry: artifact %s:%s not found", name, version)
	}
	if err := os.Remove(a.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("registry: remove artifact file: %w", err)
	}
	return m.store.DeleteArtifact(r.ID, name, version)
}

// ---- filesystem helpers -----------------------------------------------------

func copyWithDigest(src, dest string) (int64, string, error) {
	sf, err := os.Open(src)
	if err != nil {
		return 0, "", err
	}
	defer sf.Close()
	df, err := os.Create(dest)
	if err != nil {
		return 0, "", err
	}
	defer df.Close()
	h := sha256.New()
	mw := io.MultiWriter(df, h)
	n, err := io.Copy(mw, sf)
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

func inferType(path string) string {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(name, ".cap"):
		return "cap"
	case strings.HasSuffix(name, ".tar.zst"):
		return "tar.zst"
	case strings.HasSuffix(name, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(name, ".tar"):
		return "tar"
	case strings.HasSuffix(name, ".zip"):
		return "zip"
	default:
		return "binary"
	}
}

// ---- helpers ----------------------------------------------------------------

func newID(prefix string) string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
