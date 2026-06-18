package deploylimit

import (
	"fmt"
	"os"
	"strconv"
)

// MaxDeployments returns the host-wide cap on running capsule deployments
// (user instances + system-managed database instances). Override with
// CAPPER_MAX_DEPLOYMENTS; otherwise derive ~1 slot per 512 MiB host RAM
// (minimum 4, maximum 64).
func MaxDeployments() int64 {
	if v := os.Getenv("CAPPER_MAX_DEPLOYMENTS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	mem := hostMemBytes()
	n := mem / (512 << 20)
	if n < 4 {
		n = 4
	}
	if n > 64 {
		n = 64
	}
	return n
}

func hostMemBytes() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 4 << 30 // assume 4 GiB when unknown
	}
	for _, line := range splitLines(string(data)) {
		if len(line) > 10 && line[:9] == "MemTotal:" {
			var kb int64
			if _, err := fmt.Sscanf(line, "MemTotal: %d kB", &kb); err == nil {
				return kb * 1024
			}
		}
	}
	return 4 << 30
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
