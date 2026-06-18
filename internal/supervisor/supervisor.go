// Package supervisor provides a background watcher that reconciles running
// instance state and enforces restart policies.
//
// The Supervisor polls the instance store on a configurable interval (default
// 5 s), checks whether each "running" instance PID is still alive, marks dead
// instances as "stopped", and re-runs any instance whose RestartPolicy is
// "always" or "on-failure".
//
// The Supervisor is intentionally simple: it does not own the child processes
// it watches (they were started by `capper run` before the daemon came up), so
// it cannot wait(2) on them. Exit codes are therefore unavailable; for
// "on-failure" policy the restart decision falls back to the "always" behaviour
// (restart unconditionally) with a note written to the instance's stderr log.
package supervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"capper/internal/manager"
	"capper/internal/runtime"
	"capper/internal/store"
	"capper/internal/types"
)

const (
	defaultInterval = 5 * time.Second
	// maxSessionRestarts caps the number of restart attempts per original
	// instance ID within a single daemon session to prevent restart storms.
	maxSessionRestarts = 10
)

// Supervisor watches running instances and applies restart policies.
type Supervisor struct {
	Store    *store.Store
	Manager  manager.InstanceManager
	Interval time.Duration

	// Stats tracks restart counts per instance ID for this daemon session.
	stats map[string]int
}

// New creates a Supervisor with the default poll interval.
func New(st *store.Store, mgr manager.InstanceManager) *Supervisor {
	return &Supervisor{
		Store:    st,
		Manager:  mgr,
		Interval: defaultInterval,
		stats:    make(map[string]int),
	}
}

// Run starts the supervisor loop and blocks until ctx is cancelled.
func (s *Supervisor) Run(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Supervisor) tick() {
	instances, err := s.Store.ListInstances()
	if err != nil {
		return
	}
	for i := range instances {
		inst := &instances[i]
		if inst.Status != types.StatusRunning {
			continue
		}
		if runtime.Alive(inst.PID) {
			continue
		}
		s.markStopped(inst)
		s.maybeRestart(inst)
	}
}

func (s *Supervisor) markStopped(inst *types.Instance) {
	now := time.Now().UTC().Format(time.RFC3339)
	inst.Status = types.StatusStopped
	inst.StoppedAt = &now
	_ = s.Store.UpdateInstance(*inst)
	_ = s.Store.WriteInstanceJSON(*inst)
}

func (s *Supervisor) maybeRestart(inst *types.Instance) {
	policy := inst.RestartPolicy
	if policy != types.RestartAlways && policy != types.RestartOnFailure {
		return
	}

	// Enforce per-session restart cap to prevent restart storms (LR-05).
	if s.stats[inst.ID] >= maxSessionRestarts {
		appendLog(inst, fmt.Sprintf("supervisor: max restart limit (%d) reached, setting policy to never\n", maxSessionRestarts))
		inst.RestartPolicy = types.RestartNever
		_ = s.Store.UpdateInstance(*inst)
		_ = s.Store.WriteInstanceJSON(*inst)
		return
	}

	// on-failure: we cannot determine the exit code since we do not own the
	// process; treat it the same as "always" and note the limitation.
	if policy == types.RestartOnFailure {
		appendLog(inst, "supervisor: on-failure restart (exit code unavailable, restarting unconditionally)\n")
	}

	newInst, err := s.Manager.Run(inst.Image, types.ResourceOverrides{}, manager.RunOptions{
		RestartPolicy: policy,
	})
	if err != nil {
		// HR-02: on restart failure mark the instance as failed so the user
		// can see it and the daemon stops attempting further restarts.
		appendLog(inst, fmt.Sprintf("supervisor: restart failed: %v\n", err))
		inst.Status = types.StatusFailed
		inst.RestartPolicy = types.RestartNever
		_ = s.Store.UpdateInstance(*inst)
		_ = s.Store.WriteInstanceJSON(*inst)
		return
	}
	// HR-03: use the session restart counter (not inst.RestartCount + 1) so
	// the count is accurate regardless of what was persisted previously.
	s.stats[inst.ID]++
	newInst.RestartCount = s.stats[inst.ID]
	_ = s.Store.UpdateInstance(*newInst)
	_ = s.Store.WriteInstanceJSON(*newInst)
}

// Stats returns the number of restarts performed per original instance ID
// during this daemon session.
func (s *Supervisor) Stats() map[string]int {
	out := make(map[string]int, len(s.stats))
	for k, v := range s.stats {
		out[k] = v
	}
	return out
}

func appendLog(inst *types.Instance, msg string) {
	if inst.RootFSPath == "" {
		return
	}
	logPath := filepath.Join(filepath.Dir(inst.RootFSPath), "stderr.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(msg)
}
