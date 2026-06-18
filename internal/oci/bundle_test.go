package oci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"capper/internal/types"
)

func TestGenerateWritesValidConfigJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "rootfs"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := types.CapsuleManifest{
		CapsuleVersion: "0.1",
		Name:           "hello",
		Version:        "0.1.0",
		Entrypoint:     []string{"/bin/sh"},
		Args:           []string{"-c", "echo hi"},
		Env:            map[string]string{"HOME": "/root"},
		WorkingDir:     "/",
		User:           types.UserConfig{UID: 1000, GID: 1000},
		Network:        types.NetworkConfig{Enabled: false},
		Resources:      types.ResourceLimits{MemoryBytes: 67108864, MaxProcesses: 64},
	}

	configPath, err := Generate(dir, manifest)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if configPath != filepath.Join(dir, "config.json") {
		t.Fatalf("unexpected config path: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("config.json is invalid JSON: %v", err)
	}
	if spec.OCIVersion != "1.0.0" {
		t.Fatalf("unexpected ociVersion: %s", spec.OCIVersion)
	}
	if spec.Process.User.UID != 1000 || spec.Process.User.GID != 1000 {
		t.Fatalf("unexpected user: %+v", spec.Process.User)
	}
	if spec.Linux.Resources.Memory == nil || spec.Linux.Resources.Memory.Limit != 67108864 {
		t.Fatalf("unexpected memory limit: %+v", spec.Linux.Resources.Memory)
	}
	if spec.Linux.Resources.Pids == nil || spec.Linux.Resources.Pids.Limit != 64 {
		t.Fatalf("unexpected pids limit: %+v", spec.Linux.Resources.Pids)
	}
	// Network namespace must be present when network is disabled.
	hasNet := false
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == "network" {
			hasNet = true
		}
	}
	if !hasNet {
		t.Fatal("expected network namespace when network disabled")
	}
}

func TestGenerateOmitsNetworkNamespaceWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	manifest := types.CapsuleManifest{
		Entrypoint: []string{"/bin/sh"},
		WorkingDir: "/",
		Network:    types.NetworkConfig{Enabled: true},
	}
	configPath, err := Generate(dir, manifest)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(configPath)
	var spec Spec
	_ = json.Unmarshal(data, &spec)
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == "network" {
			t.Fatal("should not have network namespace when network is enabled")
		}
	}
}
