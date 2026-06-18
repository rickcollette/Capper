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
	path := filepath.Join(root, capperGroup, instanceID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		// Non-fatal: cgroup setup is optional.
		return nil, nil
	}
	return &Manager{Path: path}, nil
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
