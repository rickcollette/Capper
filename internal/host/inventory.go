package host

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Inventory captures the current host's hardware and OS info.
func Inventory() Host {
	hostname, _ := os.Hostname()
	return Host{
		Hostname:      hostname,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		KernelVersion: kernelVersion(),
		CPUCount:      runtime.NumCPU(),
		MemoryBytes:   totalMemory(),
		Addresses:     localAddresses(),
		Status:        StatusReady,
	}
}

func kernelVersion() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "unknown"
	}
	// format: "Linux version 6.9.3 (#1 SMP ...)"
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		return fields[2]
	}
	return strings.TrimSpace(string(data))
}

func totalMemory() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

func localAddresses() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			ip := ipnet.IP.String()
			if ip != "127.0.0.1" && ip != "::1" {
				out = append(out, ip)
			}
		}
	}
	return out
}

// kernelSemver parses "major.minor.patch" from a kernel version string.
// Returns (major, minor, patch) as integers; all zero on parse failure.
func kernelSemver(version string) (int, int, int) {
	// Strip any distro suffix after the third segment (e.g. "6.9.3-060903")
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 3 {
		return 0, 0, 0
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	// patch may have a suffix like "3-generic"; strip it
	patchStr := strings.FieldsFunc(parts[2], func(r rune) bool {
		return r == '-' || r == '+'
	})
	var patch int
	if len(patchStr) > 0 {
		patch, _ = strconv.Atoi(patchStr[0])
	}
	return major, minor, patch
}

// kernelAtLeast returns true if the running kernel is >= major.minor.patch.
func kernelAtLeast(major, minor, patch int) bool {
	kv := kernelVersion()
	ma, mi, pa := kernelSemver(kv)
	if ma != major {
		return ma > major
	}
	if mi != minor {
		return mi > minor
	}
	return pa >= patch
}

// diskFreeBytes returns available bytes on the filesystem containing path.
func diskFreeBytes(path string) (int64, error) {
	var stat syscallStatfs
	if err := statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return stat.available(), nil
}
