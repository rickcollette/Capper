package loader

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	"capper/internal/types"
)

func TestVerifyChecksumsDetectsMismatch(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "capsule.json")
	if err := os.WriteFile(file, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(filepath.Join(dir, "checksums.json"), types.Checksums{
		Algorithm: "sha256",
		Files: map[string]string{
			"capsule.json": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChecksums(dir); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestVerifyChecksumsRequiresRootFSAndRejectsUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "capsule.json"), []byte("capsule"))
	writeFile(t, filepath.Join(dir, "rootfs.tar.zst"), []byte("rootfs"))
	capsuleDigest, err := FileDigest(filepath.Join(dir, "capsule.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteJSON(filepath.Join(dir, "checksums.json"), types.Checksums{
		Algorithm: "sha256",
		Files: map[string]string{
			"capsule.json": capsuleDigest,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChecksums(dir); err == nil {
		t.Fatal("expected missing rootfs checksum error")
	}

	rootDigest, err := FileDigest(filepath.Join(dir, "rootfs.tar.zst"))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(filepath.Join(dir, "checksums.json"), types.Checksums{
		Algorithm: "sha256",
		Files: map[string]string{
			"capsule.json":   capsuleDigest,
			"rootfs.tar.zst": rootDigest,
			"../host":        rootDigest,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChecksums(dir); err == nil {
		t.Fatal("expected unsafe checksum path error")
	}
}

func TestVerifyManifestDigestsRequiresMatchingRootFSDigest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "rootfs.tar.zst"), []byte("rootfs"))
	digest, err := FileDigest(filepath.Join(dir, "rootfs.tar.zst"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := types.CapsuleManifest{RootFS: types.RootFSInfo{Archive: "rootfs.tar.zst", Digest: digest}}
	if err := VerifyManifestDigests(dir, manifest); err != nil {
		t.Fatal(err)
	}
	manifest.RootFS.Digest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	if err := VerifyManifestDigests(dir, manifest); err == nil {
		t.Fatal("expected digest mismatch")
	}
}

func TestExtractTarRejectsLinksAndUnsafeTypes(t *testing.T) {
	cases := []struct {
		name string
		hdr  *tar.Header
	}{
		{name: "symlink", hdr: &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/tmp/outside"}},
		{name: "hardlink", hdr: &tar.Header{Name: "link", Typeflag: tar.TypeLink, Linkname: "target"}},
		{name: "fifo", hdr: &tar.Header{Name: "fifo", Typeflag: tar.TypeFifo}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, cleanup := tarWithHeader(t, tc.hdr, nil)
			defer cleanup()
			if err := ExtractTar(reader, t.TempDir()); err == nil {
				t.Fatalf("expected rejection for %s", tc.name)
			}
		})
	}
}

func TestExtractTarRejectsPathTraversal(t *testing.T) {
	cases := []string{
		"../escape",
		"../../escape",
		"subdir/../../escape",
		"/absolute/path",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			reader, cleanup := tarWithHeader(t, &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: 4}, []byte("evil"))
			defer cleanup()
			if err := ExtractTar(reader, t.TempDir()); err == nil {
				t.Fatalf("expected rejection for path traversal: %q", name)
			}
		})
	}
}

func TestExtractTarDoesNotFollowExistingSymlink(t *testing.T) {
	dest := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Symlink(outside, filepath.Join(dest, "victim")); err != nil {
		t.Fatal(err)
	}
	reader, cleanup := tarWithHeader(t, &tar.Header{Name: "victim", Typeflag: tar.TypeReg, Mode: 0o644, Size: 4}, []byte("evil"))
	defer cleanup()
	if err := ExtractTar(reader, dest); err == nil {
		t.Fatal("expected O_NOFOLLOW rejection")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("outside target was created or unexpected error: %v", err)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func tarWithHeader(t *testing.T, hdr *tar.Header, data []byte) (io.Reader, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.tar")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(file)
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if len(data) > 0 {
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	read, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return read, func() { _ = read.Close() }
}
