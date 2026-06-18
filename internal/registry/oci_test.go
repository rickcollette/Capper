package registry_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"capper/internal/registry"
)

func openOCIHandler(t *testing.T) *registry.OCIHandler {
	t.Helper()
	mgr := openManager(t)
	return registry.NewOCIHandler(mgr)
}

// seedImage pushes a real (tiny) image into the registry so OCI handler queries work.
func seedImage(t *testing.T, mgr *registry.Manager, regName, imgName, version string) registry.RegistryImage {
	t.Helper()
	if _, err := mgr.Init(regName); err != nil {
		t.Fatalf("Init registry: %v", err)
	}
	// Write a tiny tar file as fake image content.
	tmp := filepath.Join(t.TempDir(), "layer.tar")
	if err := os.WriteFile(tmp, []byte("fake-oci-layer"), 0o600); err != nil {
		t.Fatalf("write layer: %v", err)
	}
	img, err := mgr.Push(regName, imgName, version, tmp)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	return img
}

// TestOCIVersionCheck verifies GET /v2/ returns 200 with the API version header.
func TestOCIVersionCheck(t *testing.T) {
	h := openOCIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v2/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Docker-Distribution-API-Version"); got != "registry/2.0" {
		t.Errorf("API version header = %q want %q", got, "registry/2.0")
	}
}

// TestOCIManifestGet verifies GET /v2/<reg>/<img>/manifests/<tag> returns a manifest.
func TestOCIManifestGet(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	seedImage(t, mgr, "testreg", "myapp", "v1.0")

	req := httptest.NewRequest(http.MethodGet, "/v2/testreg/myapp/manifests/v1.0", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", rr.Code, rr.Body)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/vnd.oci.image.manifest.v1+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var manifest map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest["schemaVersion"].(float64) != 2 {
		t.Errorf("schemaVersion = %v want 2", manifest["schemaVersion"])
	}
}

// TestOCIManifestHead verifies HEAD returns headers without body.
func TestOCIManifestHead(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	seedImage(t, mgr, "testreg", "myapp", "v2.0")

	req := httptest.NewRequest(http.MethodHead, "/v2/testreg/myapp/manifests/v2.0", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if len(body) != 0 {
		t.Errorf("HEAD should have empty body, got %q", body)
	}
}

// TestOCITagList verifies GET /v2/<reg>/<img>/tags/list returns all tags.
func TestOCITagList(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	seedImage(t, mgr, "listreg", "nginx", "1.24")
	seedImage(t, mgr, "listreg", "nginx", "1.25")

	req := httptest.NewRequest(http.MethodGet, "/v2/listreg/nginx/tags/list", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", rr.Code, rr.Body)
	}
	var resp struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(resp.Tags), resp.Tags)
	}
}

// TestOCIManifestNotFound returns 404 for an unknown image.
func TestOCIManifestNotFound(t *testing.T) {
	h := openOCIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v2/noreg/noimage/manifests/latest", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// TestOCIBlobPushReturns405 verifies that PUT/POST to blobs returns 405.
func TestOCIBlobPushReturns405(t *testing.T) {
	h := openOCIHandler(t)
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		req := httptest.NewRequest(method, "/v2/reg/img/blobs/uploads/", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s blobs: expected 405, got %d", method, rr.Code)
		}
	}
}

// TestOCICatalog verifies GET /v2/_catalog returns all repositories.
func TestOCICatalog(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	seedImage(t, mgr, "reg-a", "app", "v1")
	seedImage(t, mgr, "reg-a", "worker", "v1")
	seedImage(t, mgr, "reg-b", "proxy", "v1")

	req := httptest.NewRequest(http.MethodGet, "/v2/_catalog", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body)
	}
	var resp struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Repositories) != 3 {
		t.Errorf("expected 3 repos, got %d: %v", len(resp.Repositories), resp.Repositories)
	}
}

// TestOCIBlobGetByDigest verifies GET /v2/<reg>/<img>/blobs/<digest> serves the image file.
func TestOCIBlobGetByDigest(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	img := seedImage(t, mgr, "blobreg", "myapp", "v1")

	req := httptest.NewRequest(http.MethodGet, "/v2/blobreg/myapp/blobs/sha256:"+img.Digest, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body)
	}
	if got := rr.Header().Get("Docker-Content-Digest"); got != "sha256:"+img.Digest {
		t.Errorf("Docker-Content-Digest = %q want sha256:%s", got, img.Digest)
	}
	body, _ := io.ReadAll(rr.Body)
	if len(body) == 0 {
		t.Error("expected non-empty blob body")
	}
}

// TestOCIBlobHeadByDigest verifies HEAD returns headers with empty body.
func TestOCIBlobHeadByDigest(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	img := seedImage(t, mgr, "blobhreg", "myapp", "v1")

	req := httptest.NewRequest(http.MethodHead, "/v2/blobhreg/myapp/blobs/sha256:"+img.Digest, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if len(body) != 0 {
		t.Errorf("HEAD blob should have empty body, got %d bytes", len(body))
	}
}

// TestOCIBlobNotFound returns 404 for an unknown digest.
func TestOCIBlobNotFound(t *testing.T) {
	h := openOCIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v2/reg/img/blobs/sha256:deadbeef", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// TestOCIManifestDelete verifies DELETE removes the image.
func TestOCIManifestDelete(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	seedImage(t, mgr, "delreg", "myapp", "v1")

	req := httptest.NewRequest(http.MethodDelete, "/v2/delreg/myapp/manifests/v1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d — body: %s", rr.Code, rr.Body)
	}

	// Confirm subsequent GET returns 404.
	req2 := httptest.NewRequest(http.MethodGet, "/v2/delreg/myapp/manifests/v1", nil)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNotFound {
		t.Errorf("after delete: expected 404, got %d", rr2.Code)
	}
}

// TestOCIManifestDeleteByDigest verifies DELETE by digest reference.
func TestOCIManifestDeleteByDigest(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	img := seedImage(t, mgr, "digestdelreg", "myapp", "v1")

	req := httptest.NewRequest(http.MethodDelete, "/v2/digestdelreg/myapp/manifests/sha256:"+img.Digest, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d — body: %s", rr.Code, rr.Body)
	}
}

// TestOCIErrorFormat verifies errors are returned in OCI spec JSON format.
func TestOCIErrorFormat(t *testing.T) {
	h := openOCIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v2/noreg/noimg/manifests/latest", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	var errResp struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("error response is not valid JSON: %v", err)
	}
	if len(errResp.Errors) == 0 {
		t.Error("expected at least one error in OCI error response")
	}
	if errResp.Errors[0].Code == "" {
		t.Error("error code should not be empty")
	}
}

// TestOCIDistributionVersionHeader verifies all responses carry the required header.
func TestOCIDistributionVersionHeader(t *testing.T) {
	mgr := openManager(t)
	h := registry.NewOCIHandler(mgr)
	seedImage(t, mgr, "hdreg", "app", "v1")

	endpoints := []string{
		"/v2/",
		"/v2/_catalog",
		"/v2/hdreg/app/manifests/v1",
		"/v2/hdreg/app/tags/list",
	}
	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if got := rr.Header().Get("Docker-Distribution-API-Version"); got != "registry/2.0" {
			t.Errorf("%s: Docker-Distribution-API-Version = %q want registry/2.0", ep, got)
		}
	}
}
