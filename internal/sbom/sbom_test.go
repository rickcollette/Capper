package sbom

import (
	"archive/tar"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"capper/internal/types"
)

var testManifest = types.CapsuleManifest{
	CapsuleVersion: "0.1",
	Name:           "hello",
	Version:        "1.0.0",
	Entrypoint:     []string{"/bin/hello"},
	Resources:      types.ResourceLimits{MemoryBytes: 64 * 1024 * 1024},
}

func TestGenerateSPDX(t *testing.T) {
	doc := GenerateSPDX(testManifest, "sha256:abc123")
	if doc.SPDXVersion != "SPDX-2.3" {
		t.Errorf("expected SPDX-2.3, got %s", doc.SPDXVersion)
	}
	if len(doc.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(doc.Packages))
	}
	pkg := doc.Packages[0]
	if pkg.Name != "hello" {
		t.Errorf("expected package name hello, got %s", pkg.Name)
	}
	if len(pkg.Checksums) != 1 || pkg.Checksums[0].Algorithm != "SHA256" {
		t.Errorf("expected SHA256 checksum, got %+v", pkg.Checksums)
	}
}

func TestGenerateSPDXNoDigest(t *testing.T) {
	doc := GenerateSPDX(testManifest, "")
	if len(doc.Packages[0].Checksums) != 0 {
		t.Error("expected no checksums when digest is empty")
	}
}

func TestGenerateProvenance(t *testing.T) {
	prov := GenerateProvenance(testManifest, "hello.cap", "sha256:abc123")
	if prov.Type != "https://in-toto.io/Statement/v0.1" {
		t.Errorf("unexpected type: %s", prov.Type)
	}
	if len(prov.Subject) != 1 || prov.Subject[0].Name != "hello.cap" {
		t.Errorf("unexpected subject: %+v", prov.Subject)
	}
	digest, ok := prov.Subject[0].Digest["sha256"]
	if !ok || digest != "abc123" {
		t.Errorf("expected digest sha256:abc123, got %+v", prov.Subject[0].Digest)
	}
}

func TestEmbedAndExtract(t *testing.T) {
	dir := t.TempDir()
	capPath := filepath.Join(dir, "test.cap")
	buildTestCap(t, capPath)

	data := []byte(`{"test":true}` + "\n")
	if err := EmbedInCap(capPath, capPath, EntryNameSBOM, data); err != nil {
		t.Fatalf("EmbedInCap: %v", err)
	}

	got, err := ExtractFromCap(capPath, EntryNameSBOM)
	if err != nil {
		t.Fatalf("ExtractFromCap: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("round-trip mismatch: want %q got %q", data, got)
	}
}

func TestEmbedReplaces(t *testing.T) {
	dir := t.TempDir()
	capPath := filepath.Join(dir, "test.cap")
	buildTestCap(t, capPath)

	first := []byte(`{"version":1}` + "\n")
	second := []byte(`{"version":2}` + "\n")

	if err := EmbedInCap(capPath, capPath, EntryNameProvenance, first); err != nil {
		t.Fatal(err)
	}
	if err := EmbedInCap(capPath, capPath, EntryNameProvenance, second); err != nil {
		t.Fatal(err)
	}

	got, err := ExtractFromCap(capPath, EntryNameProvenance)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(second) {
		t.Errorf("expected second version, got %q", got)
	}
}

func TestExtractNotFound(t *testing.T) {
	dir := t.TempDir()
	capPath := filepath.Join(dir, "test.cap")
	buildTestCap(t, capPath)

	_, err := ExtractFromCap(capPath, "nonexistent.json")
	if err == nil {
		t.Error("expected error for missing entry")
	}
}

func TestMarshalJSON(t *testing.T) {
	doc := GenerateSPDX(testManifest, "")
	data, err := MarshalJSON(doc)
	if err != nil {
		t.Fatal(err)
	}
	// Verify it round-trips.
	var check Document
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if check.SPDXVersion != "SPDX-2.3" {
		t.Errorf("unexpected version after round-trip: %s", check.SPDXVersion)
	}
}

func buildTestCap(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tw := tar.NewWriter(f)
	for _, entry := range []struct{ name, content string }{
		{"capsule.json", `{"capsuleVersion":"0.1","name":"hello","version":"1.0.0"}`},
		{"checksums.json", `{"algorithm":"sha256","files":{}}`},
	} {
		data := []byte(entry.content)
		_ = tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(data))})
		_, _ = tw.Write(data)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
}
