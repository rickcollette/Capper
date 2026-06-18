// capinit is the Capper guest-side init binary.
//
// It runs INSIDE a capsule as /sbin/capinit. On start it:
//  1. Reads CAPPER_METADATA_URL (or falls back to http://169.254.169.254/capper/v1).
//  2. Reads CAPPER_METADATA_TOKEN_FILE to obtain the instance token.
//  3. Fetches meta-data and network-data from capmeta.
//  4. Applies hostname, writes /etc/hosts and /etc/resolv.conf.
//  5. Execs the original entrypoint (passed as arguments after "--").
//
// Debug subcommands:
//
//	capinit fetch <endpoint>  — print raw endpoint response
//	capinit validate          — check metadata integrity (signature)
//	capinit status            — print resolved metadata summary
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
)

const defaultMetadataURL = "http://169.254.169.254/capper/v1"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "fetch":
			if len(os.Args) < 3 {
				fatal("usage: capinit fetch <endpoint>")
			}
			runFetch(os.Args[2])
			return
		case "validate":
			runValidate()
			return
		case "status":
			runStatus()
			return
		}
	}

	baseURL := metadataURL()
	token := readToken()
	required := os.Getenv("CAPPER_METADATA_REQUIRED") == "true"
	signRequired := os.Getenv("CAPPER_METADATA_SIGN_REQUIRED") == "true"

	meta, metaErr := fetchJSONWithError(baseURL+"/meta-data", token)
	netData, netErr := fetchJSONWithError(baseURL+"/network-data", token)

	if required && (metaErr != nil || netErr != nil) {
		// Fail-closed: abort if metadata unavailable and required.
		if metaErr != nil {
			fatal("metadata unavailable (fail-closed): %v", metaErr)
		}
		fatal("network-data unavailable (fail-closed): %v", netErr)
	}

	// Signature verification when signing is required.
	if signRequired {
		if err := verifySignature(baseURL, token, meta, netData); err != nil {
			fatal("signature verification failed (fail-closed): %v", err)
		}
	}

	applyHostname(meta)
	writeHosts(meta, netData)
	writeResolvConf(netData)

	// Exec entrypoint — everything after "--" or after capinit itself.
	args := entrypointArgs()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "capinit: no entrypoint; exiting")
		os.Exit(0)
	}
	if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
		fatal("capinit: exec %s: %v", args[0], err)
	}
}

// ---- subcommands ------------------------------------------------------------

func runFetch(endpoint string) {
	baseURL := metadataURL()
	token := readToken()
	url := baseURL + "/" + strings.TrimPrefix(endpoint, "/")
	body, err := doRequest(url, token)
	if err != nil {
		fatal("fetch %s: %v", endpoint, err)
	}
	fmt.Println(string(body))
}

func runValidate() {
	baseURL := metadataURL()
	token := readToken()

	metaBytes, err := doRequest(baseURL+"/meta-data", token)
	if err != nil {
		fatal("validate: fetch meta-data: %v", err)
	}
	netBytes, err := doRequest(baseURL+"/network-data", token)
	if err != nil {
		fatal("validate: fetch network-data: %v", err)
	}
	sigBytes, err := doRequest(baseURL+"/signature", token)
	if err != nil {
		fatal("validate: fetch signature: %v", err)
	}

	// Parse the signature bundle.
	var bundle struct {
		Documents map[string]string `json:"documents"` // name -> hex(SHA-256)
		Signature string            `json:"signature"`
	}
	if err := json.Unmarshal(sigBytes, &bundle); err != nil {
		fatal("validate: parse signature: %v", err)
	}

	// Verify SHA-256 hashes of fetched documents.
	check := func(name string, data []byte) {
		expected, ok := bundle.Documents[name]
		if !ok {
			fmt.Printf("validate: %s: no hash in bundle\n", name)
			return
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(data))
		if actual == expected {
			fmt.Printf("validate: %s OK (%s)\n", name, actual[:12])
		} else {
			fmt.Printf("validate: %s MISMATCH expected=%s got=%s\n", name, expected[:12], actual[:12])
		}
	}
	check("meta-data", metaBytes)
	check("network-data", netBytes)
	fmt.Printf("validate: signature=%s\n", bundle.Signature[:min(16, len(bundle.Signature))])
}

func runStatus() {
	baseURL := metadataURL()
	token := readToken()
	meta := fetchJSON(baseURL+"/meta-data", token)

	fmt.Printf("Instance ID:   %s\n", strField(meta, "instanceId"))
	fmt.Printf("Hostname:      %s\n", strField(meta, "hostname"))
	fmt.Printf("Project:       %s\n", strField(meta, "project"))
	fmt.Printf("Metadata URL:  %s\n", baseURL)
	fmt.Printf("Token:         %s\n", tokenStatus())
}

// ---- helpers ----------------------------------------------------------------

func metadataURL() string {
	if u := os.Getenv("CAPPER_METADATA_URL"); u != "" {
		return strings.TrimSuffix(u, "/")
	}
	return defaultMetadataURL
}

func readToken() string {
	path := os.Getenv("CAPPER_METADATA_TOKEN_FILE")
	if path == "" {
		path = "/run/capper/metadata-token"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func tokenStatus() string {
	path := os.Getenv("CAPPER_METADATA_TOKEN_FILE")
	if path == "" {
		path = "/run/capper/metadata-token"
	}
	if _, err := os.Stat(path); err == nil {
		return "present (" + path + ")"
	}
	return "missing"
}

func doRequest(url, token string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("X-Capper-Token", token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func fetchJSON(url, token string) map[string]any {
	m, _ := fetchJSONWithError(url, token)
	return m
}

func fetchJSONWithError(url, token string) (map[string]any, error) {
	body, err := doRequest(url, token)
	if err != nil {
		return map[string]any{}, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return map[string]any{}, err
	}
	return out, nil
}

// verifySignature fetches /signature and checks SHA-256 hashes of meta/network docs.
func verifySignature(baseURL, token string, meta, netData map[string]any) error {
	sigBytes, err := doRequest(baseURL+"/signature", token)
	if err != nil {
		return fmt.Errorf("fetch signature: %w", err)
	}
	var bundle struct {
		Documents map[string]string `json:"documents"`
	}
	if err := json.Unmarshal(sigBytes, &bundle); err != nil {
		return fmt.Errorf("parse signature bundle: %w", err)
	}

	check := func(name string, data map[string]any) error {
		expected, ok := bundle.Documents[name]
		if !ok {
			return fmt.Errorf("no hash for %q in signature bundle", name)
		}
		raw, _ := json.Marshal(data)
		actual := fmt.Sprintf("%x", sha256.Sum256(raw))
		if actual != expected {
			return fmt.Errorf("%s hash mismatch: expected %s got %s", name, expected[:12], actual[:12])
		}
		return nil
	}
	if err := check("meta-data", meta); err != nil {
		return err
	}
	return check("network-data", netData)
}

func applyHostname(meta map[string]any) {
	hostname := strField(meta, "hostname")
	if hostname == "" {
		return
	}
	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		fmt.Fprintf(os.Stderr, "capinit: sethostname %s: %v\n", hostname, err)
	}
	// Persist to /etc/hostname so a later interactive shell (a fresh namespace
	// that reads the file rather than inheriting the live UTS hostname) agrees.
	_ = os.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0o644)
}

func writeHosts(meta, network map[string]any) {
	hostname := strField(meta, "hostname")
	ip := strField(meta, "networkIp")
	if ip == "" {
		if ifaces, ok := network["interfaces"].([]any); ok && len(ifaces) > 0 {
			if iface, ok := ifaces[0].(map[string]any); ok {
				ip, _ = iface["ip"].(string)
			}
		}
	}
	content := "127.0.0.1 localhost\n::1 localhost\n"
	if ip != "" && hostname != "" {
		content += fmt.Sprintf("%s %s\n", ip, hostname)
	}
	_ = os.WriteFile("/etc/hosts", []byte(content), 0o644)
}

func writeResolvConf(network map[string]any) {
	dns := strField(network, "dns")
	if dns == "" {
		return
	}
	content := fmt.Sprintf("nameserver %s\n", dns)
	_ = os.WriteFile("/etc/resolv.conf", []byte(content), 0o644)
}

func entrypointArgs() []string {
	args := os.Args[1:]
	for i, a := range args {
		if a == "--" {
			return args[i+1:]
		}
	}
	return args
}

func strField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "capinit: "+format+"\n", args...)
	os.Exit(1)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
