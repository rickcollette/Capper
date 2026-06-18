package bottle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// templateRe matches {{ expression }} placeholders.
var templateRe = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// ParseSpec parses raw JSON bytes into a BottleSpec.
func ParseSpec(data []byte) (BottleSpec, error) {
	var spec BottleSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return BottleSpec{}, fmt.Errorf("bottle parse: %w", err)
	}
	return spec, nil
}

// LoadSpec reads and parses a bottle JSON file from disk.
func LoadSpec(path string) (BottleSpec, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BottleSpec{}, nil, fmt.Errorf("bottle: read %s: %w", path, err)
	}
	spec, err := ParseSpec(data)
	if err != nil {
		return BottleSpec{}, nil, err
	}
	return spec, data, nil
}

// Digest returns the SHA-256 hex digest of raw bottle bytes.
func Digest(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// RenderTemplate replaces {{ key }} placeholders in s using the provided
// context map. Unknown keys are left as-is (empty string fallback).
func RenderTemplate(s string, ctx map[string]string) string {
	return templateRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := strings.TrimSpace(match[2 : len(match)-2])
		if v, ok := ctx[inner]; ok {
			return v
		}
		// Unknown expression — return empty so callers can detect it.
		return ""
	})
}

// BuildContext assembles the template context from a bottle spec and
// user-supplied parameters. It merges defaults then overrides.
func BuildContext(spec BottleSpec, params map[string]string) map[string]string {
	ctx := make(map[string]string)
	ctx["metadata.name"] = spec.Metadata.Name
	ctx["metadata.version"] = spec.Metadata.Version
	ctx["metadata.displayName"] = spec.Metadata.DisplayName
	if spec.Spec.Build != nil {
		ctx["build.outputImage"] = spec.Spec.Build.OutputImage
	}
	// Apply parameter defaults first.
	for key, pspec := range spec.Spec.Parameters {
		ctx["parameters."+key] = pspec.Default
	}
	// Apply caller-supplied values.
	for key, val := range params {
		ctx["parameters."+key] = val
	}
	return ctx
}
