package types

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreateConfigDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capper.json")
	if err := os.WriteFile(path, []byte(`{
		"name": "hello",
		"version": "0.1.0",
		"rootfs": "./rootfs",
		"entrypoint": ["/bin/sh"]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadCreateConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkingDir != "/" || cfg.Shell != "/bin/sh" {
		t.Fatalf("defaults not applied: %#v", cfg)
	}
	if len(cfg.Args) != 0 {
		t.Fatalf("expected empty args, got %#v", cfg.Args)
	}
	if cfg.Env == nil {
		t.Fatal("expected env map")
	}
}

func TestLoadCreateConfigRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capper.json")
	if err := os.WriteFile(path, []byte(`{
		"name": "hello",
		"version": "0.1.0",
		"rootfs": "./rootfs",
		"entrypoint": ["/bin/sh"],
		"extra": true
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCreateConfig(path); err == nil {
		t.Fatal("expected unknown field error")
	}
}
