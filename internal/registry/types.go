package registry

// Registry is a named local filesystem store for images and artifacts.
type Registry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Backend   string `json:"backend"`
	Path      string `json:"path"`
	CreatedAt string `json:"createdAt"`
}

const (
	BackendFilesystem = "filesystem"
)

// RegistryImage is a versioned .cap image stored in a registry.
type RegistryImage struct {
	ID         string `json:"id"`
	RegistryID string `json:"registryId"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Digest     string `json:"digest"`
	Path       string `json:"path"`
	Signed     bool   `json:"signed"`
	ScanStatus string `json:"scanStatus,omitempty"`
	CreatedAt  string `json:"createdAt"`

	// Populated by manager for display only; not stored.
	RegistryName string `json:"registryName,omitempty"`
}

const (
	ScanStatusPending = "pending"
	ScanStatusClean   = "clean"
	ScanStatusFailed  = "failed"
)

// Artifact is a versioned opaque binary stored in a registry.
type Artifact struct {
	ID           string            `json:"id"`
	RegistryID   string            `json:"registryId"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Type         string            `json:"type"`
	Digest       string            `json:"digest"`
	Path         string            `json:"path"`
	SizeBytes    int64             `json:"sizeBytes"`
	Labels       map[string]string `json:"labels,omitempty"`
	CreatedAt    string            `json:"createdAt"`

	RegistryName string `json:"registryName,omitempty"`
}

// ImageRef represents a parsed registry/name:version reference.
type ImageRef struct {
	Registry string
	Name     string
	Version  string
}

// ParseRef parses "registry/name:version" or "name:version".
// Version defaults to "latest" when omitted.
func ParseRef(ref string) ImageRef {
	r := ImageRef{Version: "latest"}
	// Split off registry prefix.
	slash := -1
	for i, c := range ref {
		if c == '/' {
			slash = i
			break
		}
	}
	if slash >= 0 {
		r.Registry = ref[:slash]
		ref = ref[slash+1:]
	}
	// Split name:version.
	if colon := lastIndex(ref, ':'); colon >= 0 {
		r.Name = ref[:colon]
		r.Version = ref[colon+1:]
	} else {
		r.Name = ref
	}
	return r
}

func lastIndex(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}
