package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// OCIHandler implements the OCI Distribution Spec v2 HTTP API.
//
// Mount at /v2/ to provide compatibility with docker pull, skopeo, crane, etc.
//
// Supported endpoints:
//
//	GET  /v2/                                    — API version check
//	GET  /v2/_catalog                            — list repositories
//	GET  /v2/<reg>/<img>/tags/list               — list image tags
//	GET  /v2/<reg>/<img>/manifests/<ref>         — fetch manifest by tag or digest
//	HEAD /v2/<reg>/<img>/manifests/<ref>         — manifest existence check
//	DELETE /v2/<reg>/<img>/manifests/<ref>       — delete image by tag or digest
//	GET  /v2/<reg>/<img>/blobs/<digest>          — fetch layer blob by digest
//	HEAD /v2/<reg>/<img>/blobs/<digest>          — blob existence check
type OCIHandler struct {
	mgr *Manager
}

// NewOCIHandler creates an OCI Distribution Spec handler backed by the given Manager.
func NewOCIHandler(mgr *Manager) *OCIHandler {
	return &OCIHandler{mgr: mgr}
}

// ServeHTTP routes OCI Distribution API requests.
func (h *OCIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v2")
	path = strings.TrimPrefix(path, "/")

	// GET /v2/ — API version check (required by spec).
	if path == "" || path == "/" {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
		return
	}

	// GET /v2/_catalog — list all repositories.
	if path == "_catalog" {
		h.handleCatalog(w, r)
		return
	}

	// Expect: <registry>/<image>/<resource>[/<ref>]
	parts := strings.SplitN(path, "/", 4)
	if len(parts) < 4 {
		ociError(w, http.StatusNotFound, "NAME_UNKNOWN", "repository not found")
		return
	}
	registryName := parts[0]
	imageName := parts[1]
	resource := parts[2]
	ref := parts[3]

	switch resource {
	case "manifests":
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			h.handleManifestGet(w, r, registryName, imageName, ref)
		case http.MethodDelete:
			h.handleManifestDelete(w, r, registryName, imageName, ref)
		default:
			ociError(w, http.StatusMethodNotAllowed, "UNSUPPORTED", "method not allowed")
		}
	case "tags":
		if ref == "list" && r.Method == http.MethodGet {
			h.handleTagList(w, r, registryName, imageName)
		} else {
			ociError(w, http.StatusNotFound, "NAME_UNKNOWN", "not found")
		}
	case "blobs":
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			h.handleBlobGet(w, r, registryName, imageName, ref)
		default:
			ociError(w, http.StatusMethodNotAllowed, "UNSUPPORTED",
				"blob push not supported via OCI API — use capper registry push")
		}
	default:
		ociError(w, http.StatusNotFound, "NAME_UNKNOWN", "not found")
	}
}

// handleCatalog serves GET /v2/_catalog — lists all <registry>/<image> names.
func (h *OCIHandler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ociError(w, http.StatusMethodNotAllowed, "UNSUPPORTED", "method not allowed")
		return
	}
	regs, err := h.mgr.ListRegistries()
	if err != nil {
		ociError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	var repos []string
	for _, reg := range regs {
		images, err := h.mgr.ListImages(reg.ID)
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, img := range images {
			key := reg.Name + "/" + img.Name
			if !seen[key] {
				repos = append(repos, key)
				seen[key] = true
			}
		}
	}
	if repos == nil {
		repos = []string{}
	}
	writeOCIJSON(w, http.StatusOK, map[string]any{"repositories": repos})
}

// handleManifestGet serves GET/HEAD /v2/<reg>/<img>/manifests/<ref>.
func (h *OCIHandler) handleManifestGet(w http.ResponseWriter, r *http.Request, registryName, imageName, ref string) {
	img, err := h.resolveImage(registryName, imageName, ref)
	if err != nil {
		ociError(w, http.StatusNotFound, "MANIFEST_UNKNOWN", "manifest unknown")
		return
	}
	manifest := buildManifest(img, registryName)
	body, _ := json.Marshal(manifest)
	w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.Header().Set("Docker-Content-Digest", "sha256:"+img.Digest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// handleManifestDelete serves DELETE /v2/<reg>/<img>/manifests/<ref>.
func (h *OCIHandler) handleManifestDelete(w http.ResponseWriter, r *http.Request, registryName, imageName, ref string) {
	img, err := h.resolveImage(registryName, imageName, ref)
	if err != nil {
		ociError(w, http.StatusNotFound, "MANIFEST_UNKNOWN", "manifest unknown")
		return
	}
	if err := h.mgr.DeleteImage(registryName, imageName, img.Version); err != nil {
		ociError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleTagList serves GET /v2/<reg>/<img>/tags/list.
func (h *OCIHandler) handleTagList(w http.ResponseWriter, r *http.Request, registryName, imageName string) {
	reg, err := h.mgr.GetRegistry(registryName)
	if err != nil {
		ociError(w, http.StatusNotFound, "NAME_UNKNOWN", "repository not found")
		return
	}
	images, err := h.mgr.ListImages(reg.ID)
	if err != nil {
		ociError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	var tags []string
	for _, img := range images {
		if img.Name == imageName {
			tags = append(tags, img.Version)
		}
	}
	if tags == nil {
		tags = []string{}
	}
	writeOCIJSON(w, http.StatusOK, map[string]any{
		"name": registryName + "/" + imageName,
		"tags": tags,
	})
}

// handleBlobGet serves GET/HEAD /v2/<reg>/<img>/blobs/<digest>.
// The digest must match the sha256 digest of an image stored in the registry.
func (h *OCIHandler) handleBlobGet(w http.ResponseWriter, r *http.Request, registryName, imageName, digest string) {
	img, err := h.resolveByDigest(registryName, imageName, digest)
	if err != nil {
		ociError(w, http.StatusNotFound, "BLOB_UNKNOWN", "blob unknown")
		return
	}
	fi, err := os.Stat(img.Path)
	if err != nil {
		ociError(w, http.StatusNotFound, "BLOB_UNKNOWN", "blob file not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	f, err := os.Open(img.Path)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}

// ---- helpers ----------------------------------------------------------------

// resolveImage finds an image by tag, digest, or "latest" pseudo-tag.
func (h *OCIHandler) resolveImage(registryName, imageName, ref string) (RegistryImage, error) {
	reg, err := h.mgr.GetRegistry(registryName)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry not found")
	}
	images, err := h.mgr.ListImages(reg.ID)
	if err != nil {
		return RegistryImage{}, err
	}
	// Normalise digest reference (strip "sha256:" prefix for comparison).
	digestRef := strings.TrimPrefix(ref, "sha256:")
	for _, img := range images {
		if img.Name != imageName {
			continue
		}
		if img.Version == ref || img.Digest == digestRef || (ref == "latest") {
			return img, nil
		}
	}
	return RegistryImage{}, fmt.Errorf("not found")
}

// resolveByDigest finds an image whose stored digest matches the requested digest.
func (h *OCIHandler) resolveByDigest(registryName, imageName, digest string) (RegistryImage, error) {
	reg, err := h.mgr.GetRegistry(registryName)
	if err != nil {
		return RegistryImage{}, fmt.Errorf("registry not found")
	}
	images, err := h.mgr.ListImages(reg.ID)
	if err != nil {
		return RegistryImage{}, err
	}
	clean := strings.TrimPrefix(digest, "sha256:")
	for _, img := range images {
		if img.Name == imageName && img.Digest == clean {
			return img, nil
		}
	}
	return RegistryImage{}, fmt.Errorf("blob not found")
}

// buildManifest constructs an OCI image manifest for the given image record.
func buildManifest(img RegistryImage, registryName string) map[string]any {
	return map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config": map[string]any{
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest":    "sha256:" + img.Digest,
			"size":      0,
		},
		"layers": []map[string]any{
			{
				"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
				"digest":    "sha256:" + img.Digest,
				"size":      0,
			},
		},
		"annotations": map[string]string{
			"org.opencontainers.image.ref.name": img.Name + ":" + img.Version,
			"org.capper.registry":               registryName,
			"org.capper.image.id":               img.ID,
		},
	}
}

// ociError writes an OCI-spec-compliant error response.
func ociError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(status)
	body, _ := json.Marshal(map[string]any{
		"errors": []map[string]any{
			{"code": code, "message": message, "detail": nil},
		},
	})
	_, _ = w.Write(body)
}

// writeOCIJSON writes a JSON response with the Distribution API version header.
func writeOCIJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(status)
	body, _ := json.Marshal(v)
	_, _ = w.Write(body)
}
