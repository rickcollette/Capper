package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const cgroupBase = "/sys/fs/cgroup"

// InstanceMetrics holds live resource usage for a capsule instance.
type InstanceMetrics struct {
	InstanceID   string `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	CPUUsageUs   uint64 `json:"cpuUsageMicros"`  // cumulative microseconds
	MemoryBytes  uint64 `json:"memoryBytes"`     // current RSS
	PIDCount     int    `json:"pidCount"`
}

// ReadInstance reads cgroup v2 metrics for the given instance ID.
// Returns zero-value metrics (not an error) if the cgroup does not exist.
func ReadInstance(instanceID, instanceName string) InstanceMetrics {
	m := InstanceMetrics{InstanceID: instanceID, InstanceName: instanceName}
	cgPath := filepath.Join(cgroupBase, "capper", instanceID)
	if _, err := os.Stat(cgPath); err != nil {
		return m
	}
	m.CPUUsageUs = readCPUUsage(cgPath)
	m.MemoryBytes = readMemoryCurrent(cgPath)
	m.PIDCount = readPIDCurrent(cgPath)
	return m
}

func readCPUUsage(cgPath string) uint64 {
	// cpu.stat has lines like "usage_usec 12345"
	data, err := os.ReadFile(filepath.Join(cgPath, "cpu.stat"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "usage_usec ") {
			v, _ := strconv.ParseUint(strings.TrimPrefix(line, "usage_usec "), 10, 64)
			return v
		}
	}
	return 0
}

func readMemoryCurrent(cgPath string) uint64 {
	data, err := os.ReadFile(filepath.Join(cgPath, "memory.current"))
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	return v
}

func readPIDCurrent(cgPath string) int {
	data, err := os.ReadFile(filepath.Join(cgPath, "pids.current"))
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return v
}

// HumanCPU formats CPU microseconds as a percentage string (over 1s window).
// Without a window this just returns the raw µs value.
func HumanCPU(usec uint64) string {
	return fmt.Sprintf("%dµs", usec)
}

// HumanMemory formats bytes in human-readable form.
func HumanMemory(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fGiB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fMiB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fKiB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
