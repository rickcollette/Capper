package cli

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.3.0", "1.2.9", 1},
		{"2.0.0", "1.9.9", 1},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0-rc1", "1.0.0", 0}, // pre-release suffix ignored
		{"0.1.0", "0.2.0", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("hello"))
	if err := verifySHA256(p, hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("matching checksum should pass: %v", err)
	}
	if err := verifySHA256(p, "deadbeef"); err == nil {
		t.Fatal("mismatched checksum should fail")
	}
}

func TestExtractTarGzAndSingleSubdir(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.tgz")

	// Build a tar.gz with a single top-level dir containing bin/capper + VERSION.
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"pkg/VERSION":    "9.9.9\n",
		"pkg/bin/capper": "#!/bin/true\n",
	}
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()

	dest := filepath.Join(dir, "out")
	if err := extractTarGz(archive, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}
	root, err := singleSubdir(dest)
	if err != nil {
		t.Fatalf("singleSubdir: %v", err)
	}
	if filepath.Base(root) != "pkg" {
		t.Fatalf("expected single subdir 'pkg', got %s", root)
	}
	v, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil || string(v) != "9.9.9\n" {
		t.Fatalf("VERSION not extracted correctly: %q (%v)", v, err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "capper")); err != nil {
		t.Fatalf("bin/capper not extracted: %v", err)
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil.tgz")
	f, _ := os.Create(archive)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := "x"
	_ = tw.WriteHeader(&tar.Header{Name: "../escape", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte(body))
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()

	if err := extractTarGz(archive, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected path-traversal rejection")
	}
}
