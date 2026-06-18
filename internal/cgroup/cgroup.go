// Package cgroup manages cgroup v2 lifecycle for Capper instances.
// All operations degrade gracefully when cgroup v2 is unavailable or
// when the caller lacks permission to write controller files.
package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"capper/internal/types"
)

const (
	root        = "/sys/fs/cgroup"
	capperGroup = "capper"
)

// Manager handles one instance's cgroup directory.
type Manager struct {
	Path string
}

// Available reports whether cgroup v2 (unified hierarchy) is present.
func Available() bool {
	_, err := os.Stat(filepath.Join(root, "cgroup.controllers"))
	return err == nil
}

// New creates a cgroup for the given instance ID and returns the manager.
// Returns nil, nil when cgroup v2 is unavailable or not writable (graceful no-op).
func New(instanceID string) (*Manager, error) {
	if !Available() {
		return nil, nil
	}
	group := filepath.Join(root, capperGroup)
	if err := os.MkdirAll(group, 0o755); err != nil {
		// Non-fatal: cgroup setup is optional.
		return nil, nil
	}
	// Enable controllers for the per-instance leaves. Without this the child
	// cgroup has no memory.max/pids.max files and limit writes silently no-op.
	enableControllers(group)
	path := filepath.Join(group, instanceID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, nil
	}
	return &Manager{Path: path}, nil
}

// enableControllers turns on the controllers Capper limits (memory, pids, cpu)
// in the group's subtree_control so child cgroups expose memory.max etc. Only
// controllers actually delegated to the group (listed in cgroup.controllers) are
// requested. Best-effort: requires root + cgroup-v2 delegation from the parent.
func enableControllers(group string) {
	avail, err := os.ReadFile(filepath.Join(group, "cgroup.controllers"))
	if err != nil {
		return
	}
	have := map[string]bool{}
	for _, c := range strings.Fields(string(avail)) {
		have[c] = true
	}
	var want []string
	for _, c := range []string{"memory", "pids", "cpu"} {
		if have[c] {
			want = append(want, "+"+c)
		}
	}
	if len(want) == 0 {
		return
	}
	_ = os.WriteFile(filepath.Join(group, "cgroup.subtree_control"), []byte(strings.Join(want, " ")), 0o644)
}

// Open returns a Manager for an existing cgroup directory.
// Returns nil if the directory does not exist.
func Open(instanceID string) *Manager {
	path := filepath.Join(root, capperGroup, instanceID)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return &Manager{Path: path}
}

// Apply writes resource limits to the cgroup controller files.
// Individual limit writes are best-effort: a failure to set one limit does not
// prevent others from being applied.
func (m *Manager) Apply(limits types.ResourceLimits) []error {
	var errs []error
	if limits.MemoryBytes > 0 {
		if err := m.write("memory.max", fmt.Sprintf("%d\n", limits.MemoryBytes)); err != nil {
			errs = append(errs, fmt.Errorf("memory.max: %w", err))
		}
	}
	if limits.MaxProcesses > 0 {
		if err := m.write("pids.max", fmt.Sprintf("%d\n", limits.MaxProcesses)); err != nil {
			errs = append(errs, fmt.Errorf("pids.max: %w", err))
		}
	}
	if limits.CPUCount > 0 {
		// cgroup v2 cpu.max: $QUOTA $PERIOD (microseconds). One full CPU ≈ 100ms/100ms.
		const period = 100000
		quota := limits.CPUCount * period
		if err := m.write("cpu.max", fmt.Sprintf("%d %d\n", quota, period)); err != nil {
			errs = append(errs, fmt.Errorf("cpu.max: %w", err))
		}
	}
	return errs
}

// AddPID moves pid into this cgroup.
func (m *Manager) AddPID(pid int) error {
	return m.write("cgroup.procs", strconv.Itoa(pid)+"\n")
}

// PIDs returns the live PIDs in this cgroup.
func (m *Manager) PIDs() []int {
	data, err := os.ReadFile(filepath.Join(m.Path, "cgroup.procs"))
	if err != nil {
		return nil
	}
	var out []int
	for _, line := range strings.Fields(string(data)) {
		if pid, err := strconv.Atoi(line); err == nil {
			out = append(out, pid)
		}
	}
	return out
}

// Remove deletes the cgroup directory. The cgroup must be empty (no live PIDs)
// before the kernel will allow removal.
func (m *Manager) Remove() error {
	return os.Remove(m.Path)
}

func (m *Manager) write(file, value string) error {
	return os.WriteFile(filepath.Join(m.Path, file), []byte(value), 0o644)
}
