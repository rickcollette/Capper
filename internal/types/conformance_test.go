package types

import (
	"os"
	"path/filepath"
	"testing"
)

const conformanceDir = "../../testdata/conformance"

func TestConformanceValidCreateConfig(t *testing.T) {
	path := filepath.Join(conformanceDir, "valid", "create-config.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("fixture not found")
	}
	cfg, err := LoadCreateConfig(path)
	if err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if cfg.Name == "" || len(cfg.Entrypoint) == 0 {
		t.Fatal("valid config loaded with empty required fields")
	}
}

func TestConformanceInvalidCreateConfigs(t *testing.T) {
	cases := []struct {
		file string
	}{
		{"create-config-no-name.json"},
		{"create-config-no-entrypoint.json"},
		{"create-config-unknown-field.json"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join(conformanceDir, "invalid", tc.file)
			if _, err := os.Stat(path); err != nil {
				t.Skip("fixture not found")
			}
			_, err := LoadCreateConfig(path)
			if err == nil {
				t.Fatalf("expected LoadCreateConfig to reject %s but it succeeded", tc.file)
			}
		})
	}
}
