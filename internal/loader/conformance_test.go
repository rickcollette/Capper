package loader

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"
)

const conformanceDir = "../../testdata/conformance"

func TestConformanceInvalidCapsuleManifests(t *testing.T) {
	cases := []struct {
		file string
	}{
		{"capsule-wrong-version.json"},
		{"capsule-no-entrypoint.json"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join(conformanceDir, "invalid", tc.file)
			if _, err := os.Stat(path); err != nil {
				t.Skip("fixture not found")
			}
			_, err := ReadManifest(path)
			if err == nil {
				t.Fatalf("expected ReadManifest to reject %s but it succeeded", tc.file)
			}
		})
	}
}

func TestConformanceMaliciousTarPathTraversal(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"double-dot-prefix", "../escape"},
		{"absolute-path", "/etc/passwd"},
		{"nested-traversal", "a/../../escape"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, cleanup := tarWithHeader(t, &tar.Header{
				Name:     tc.path,
				Typeflag: tar.TypeReg,
				Mode:     0o644,
				Size:     4,
			}, []byte("evil"))
			defer cleanup()
			if err := ExtractTar(reader, t.TempDir()); err == nil {
				t.Fatalf("expected ExtractTar to reject path %q but it succeeded", tc.path)
			}
		})
	}
}

func TestConformanceMaliciousTarDeviceNodes(t *testing.T) {
	cases := []struct {
		name     string
		typeflag byte
	}{
		{"char-device", tar.TypeChar},
		{"block-device", tar.TypeBlock},
		{"fifo", tar.TypeFifo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, cleanup := tarWithHeader(t, &tar.Header{
				Name:     "device",
				Typeflag: tc.typeflag,
			}, nil)
			defer cleanup()
			if err := ExtractTar(reader, t.TempDir()); err == nil {
				t.Fatalf("expected ExtractTar to reject %s but it succeeded", tc.name)
			}
		})
	}
}

func TestConformanceMaliciousTarSymlinkEscape(t *testing.T) {
	reader, cleanup := tarWithHeader(t, &tar.Header{
		Name:     "link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/tmp/outside",
	}, nil)
	defer cleanup()
	if err := ExtractTar(reader, t.TempDir()); err == nil {
		t.Fatal("expected ExtractTar to reject symlink but it succeeded")
	}
}

func TestConformanceMaliciousTarHardlinkEscape(t *testing.T) {
	reader, cleanup := tarWithHeader(t, &tar.Header{
		Name:     "hardlink",
		Typeflag: tar.TypeLink,
		Linkname: "/etc/shadow",
	}, nil)
	defer cleanup()
	if err := ExtractTar(reader, t.TempDir()); err == nil {
		t.Fatal("expected ExtractTar to reject hardlink but it succeeded")
	}
}
