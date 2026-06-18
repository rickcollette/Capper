package registry

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ImageScanResult summarises one scan pass on a registry image.
type ImageScanResult struct {
	Type     string `json:"type"`
	Status   string `json:"status"` // pass, warn, fail, unavailable
	Detail   string `json:"detail"`
	Findings int    `json:"findings"`
}

// ScanImage runs static checks on the named image and updates its scan_status.
// The returned results describe individual check outcomes.
func (m *Manager) ScanImage(registryNameOrID, imageName, version string) ([]ImageScanResult, string, error) {
	r, err := m.store.GetRegistry(registryNameOrID)
	if err != nil {
		return nil, "", fmt.Errorf("registry: %q not found", registryNameOrID)
	}
	img, err := m.store.GetImage(r.ID, imageName, version)
	if err != nil {
		return nil, "", fmt.Errorf("registry: image %s:%s not found in %s", imageName, version, r.Name)
	}

	results := runImageStaticScans(img.Path, img.Digest)

	// Determine overall status.
	status := "clean"
	for _, res := range results {
		if res.Status == "fail" {
			status = "critical"
			break
		}
		if res.Status == "warn" && status == "clean" {
			status = "failed"
		}
	}

	if err := m.store.UpdateImageScanStatus(r.ID, imageName, version, status); err != nil {
		return results, status, fmt.Errorf("registry: update scan status: %w", err)
	}
	return results, status, nil
}

// runImageStaticScans runs all available checks on a .cap image file.
func runImageStaticScans(path, expectedDigest string) []ImageScanResult {
	if path == "" {
		return unavailableResults("image has no stored path")
	}
	if _, err := os.Stat(path); err != nil {
		return unavailableResults(fmt.Sprintf("image file %q not accessible: %v", path, err))
	}
	return []ImageScanResult{
		imgScanDigest(path, expectedDigest),
		imgScanSignature(path),
		imgScanSBOM(path),
		imgScanVuln(path),
		imgScanSecrets(path),
	}
}

func unavailableResults(detail string) []ImageScanResult {
	return []ImageScanResult{
		{Type: "digest", Status: "fail", Detail: detail, Findings: 1},
		{Type: "signature", Status: "fail", Detail: detail, Findings: 1},
		{Type: "sbom", Status: "warn", Detail: detail, Findings: 1},
		{Type: "vuln", Status: "warn", Detail: detail, Findings: 1},
		{Type: "secrets", Status: "warn", Detail: detail, Findings: 1},
	}
}

func imgScanDigest(path, expectedDigest string) ImageScanResult {
	if expectedDigest == "" {
		return ImageScanResult{Type: "digest", Status: "warn", Detail: "no expected digest recorded", Findings: 1}
	}
	f, err := os.Open(path)
	if err != nil {
		return ImageScanResult{Type: "digest", Status: "fail", Detail: err.Error(), Findings: 1}
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ImageScanResult{Type: "digest", Status: "fail", Detail: err.Error(), Findings: 1}
	}
	actual := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if actual != expectedDigest {
		return ImageScanResult{Type: "digest", Status: "fail",
			Detail: fmt.Sprintf("digest mismatch: got %s, expected %s", actual, expectedDigest), Findings: 1}
	}
	return ImageScanResult{Type: "digest", Status: "pass", Detail: "digest matches record"}
}

func imgScanSignature(path string) ImageScanResult {
	if _, err := readCapTarEntry(path, "signature.json", 1<<20); err != nil {
		return ImageScanResult{Type: "signature", Status: "warn", Detail: "signature.json not present in image", Findings: 1}
	}
	return ImageScanResult{Type: "signature", Status: "pass", Detail: "signature.json present"}
}

func imgScanSBOM(path string) ImageScanResult {
	if _, err := readCapTarEntry(path, "attestations/sbom.spdx.json", 8<<20); err != nil {
		return ImageScanResult{Type: "sbom", Status: "warn", Detail: "embedded SPDX SBOM not found", Findings: 1}
	}
	return ImageScanResult{Type: "sbom", Status: "pass", Detail: "embedded SPDX SBOM present"}
}

func imgScanVuln(path string) ImageScanResult {
	if _, err := exec.LookPath("trivy"); err != nil {
		return ImageScanResult{Type: "vuln", Status: "warn",
			Detail: "trivy not installed; vulnerability scan unavailable", Findings: 1}
	}
	cmd := exec.Command("trivy", "fs", "--quiet", "--exit-code", "1",
		"--severity", "CRITICAL,HIGH", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ImageScanResult{Type: "vuln", Status: "fail",
			Detail: strings.TrimSpace(string(out)), Findings: 1}
	}
	return ImageScanResult{Type: "vuln", Status: "pass",
		Detail: "trivy found no high or critical vulnerabilities"}
}

func imgScanSecrets(path string) ImageScanResult {
	findings := 0
	_ = walkCapTarEntries(path, 1<<20, func(_ string, data []byte) {
		lower := bytes.ToLower(data)
		if bytes.Contains(data, []byte("-----BEGIN PRIVATE KEY-----")) ||
			bytes.Contains(data, []byte("AKIA")) ||
			bytes.Contains(lower, []byte("password=")) ||
			bytes.Contains(lower, []byte("secret=")) {
			findings++
		}
	})
	if findings > 0 {
		return ImageScanResult{Type: "secrets", Status: "fail",
			Detail: fmt.Sprintf("%d potential secret(s) found", findings), Findings: findings}
	}
	return ImageScanResult{Type: "secrets", Status: "pass", Detail: "no secrets detected"}
}

// readCapTarEntry extracts the named entry from a .cap (tar) file.
func readCapTarEntry(capPath, entryName string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(capPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == entryName || strings.TrimPrefix(hdr.Name, "./") == entryName {
			return io.ReadAll(io.LimitReader(tr, maxBytes))
		}
	}
	return nil, fmt.Errorf("entry %q not found", entryName)
}

// walkCapTarEntries calls fn for every regular file entry in a .cap (tar) file
// that does not exceed maxBytes in size.
func walkCapTarEntries(capPath string, maxBytes int64, fn func(name string, data []byte)) error {
	f, err := os.Open(capPath)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || hdr.Size > maxBytes {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxBytes))
		if err != nil {
			continue
		}
		fn(hdr.Name, data)
	}
	return nil
}
