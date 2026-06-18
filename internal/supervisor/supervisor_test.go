package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"capper/internal/manager"
	"capper/internal/store"
	"capper/internal/types"
)

// insertDeadRunningInstance inserts a running instance whose PID does not exist.
func insertDeadRunningInstance(t *testing.T, st *store.Store, id, name string, policy types.RestartPolicy) types.Instance {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "inst-"+id)
	rootfs := filepath.Join(dir, "rootfs")
	if err := os.MkdirAll(rootfs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	inst := types.Instance{
		ID:            id,
		Name:          name,
		Image:         "test.cap",
		Status:        types.StatusRunning,
		PID:           99999999,
		CreatedAt:     now,
		StartedAt:     now,
		RootFSPath:    rootfs,
		Command:       "/bin/test",
		RestartPolicy: policy,
	}
	if err := st.InsertInstance(inst); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := st.WriteInstanceJSON(inst); err != nil {
		t.Fatalf("write json: %v", err)
	}
	return inst
}

// TestSupervisorMarksStoppedInstances verifies that the supervisor flips a
// "running" instance whose PID is dead to "stopped" within one tick.
func TestSupervisorMarksStoppedInstances(t *testing.T) {
	dir := t.TempDir()
	paths := store.NewPaths(filepath.Join(dir, "state"))
	st, err := store.Open(paths)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Insert a fake running instance with a PID that does not exist.
	instDir := filepath.Join(dir, "inst-abc")
	rootfs := filepath.Join(instDir, "rootfs")
	if err := os.MkdirAll(rootfs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	inst := types.Instance{
		ID:         "abc123",
		Name:       "test-instance",
		Image:      "hello.cap",
		Status:     types.StatusRunning,
		PID:        99999999, // no such PID
		CreatedAt:  now,
		StartedAt:  now,
		RootFSPath: rootfs,
		Command:    "/bin/hello",
	}
	if err := st.InsertInstance(inst); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := st.WriteInstanceJSON(inst); err != nil {
		t.Fatalf("write json: %v", err)
	}

	// Build a supervisor with a short interval.
	sup := New(st, manager.InstanceManager{Store: st})
	sup.Interval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go sup.Run(ctx)
	<-ctx.Done()

	updated, err := st.ResolveInstance("abc123")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if updated.Status != types.StatusStopped {
		t.Errorf("expected stopped, got %s", updated.Status)
	}
	if updated.StoppedAt == nil {
		t.Error("expected StoppedAt to be set")
	}
}

// TestSupervisorIgnoresAliveInstances verifies that a genuinely running PID
// (the test process itself) is left in "running" state by the supervisor.
func TestSupervisorIgnoresAliveInstances(t *testing.T) {
	dir := t.TempDir()
	paths := store.NewPaths(filepath.Join(dir, "state"))
	st, err := store.Open(paths)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	instDir := filepath.Join(dir, "inst-alive")
	rootfs := filepath.Join(instDir, "rootfs")
	if err := os.MkdirAll(rootfs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	inst := types.Instance{
		ID:         "alive1",
		Name:       "alive-instance",
		Image:      "hello.cap",
		Status:     types.StatusRunning,
		PID:        os.Getpid(), // test process is alive
		CreatedAt:  now,
		StartedAt:  now,
		RootFSPath: rootfs,
		Command:    "/bin/hello",
	}
	if err := st.InsertInstance(inst); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := st.WriteInstanceJSON(inst); err != nil {
		t.Fatalf("write json: %v", err)
	}

	sup := New(st, manager.InstanceManager{Store: st})
	sup.Interval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go sup.Run(ctx)
	<-ctx.Done()

	updated, err := st.ResolveInstance("alive1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if updated.Status != types.StatusRunning {
		t.Errorf("expected running, got %s", updated.Status)
	}
}

// TestSupervisorRateLimitStopsRestarting verifies that when an instance has
// already hit maxSessionRestarts in the stats map, the supervisor marks the
// instance policy as "never" instead of attempting another restart.
func TestSupervisorRateLimitStopsRestarting(t *testing.T) {
	dir := t.TempDir()
	paths := store.NewPaths(filepath.Join(dir, "state"))
	st, err := store.Open(paths)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	inst := insertDeadRunningInstance(t, st, "ratelimit1", "rate-instance", types.RestartAlways)

	sup := New(st, manager.InstanceManager{Store: st})
	sup.Interval = 50 * time.Millisecond
	// Pre-fill the session restart count to the maximum so the next tick
	// hits the rate-limit branch instead of attempting a real restart.
	sup.stats[inst.ID] = maxSessionRestarts

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go sup.Run(ctx)
	<-ctx.Done()

	updated, err := st.ResolveInstance(inst.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if updated.Status == types.StatusRunning {
		t.Error("expected instance to be stopped, still running")
	}
	if updated.RestartPolicy != types.RestartNever {
		t.Errorf("expected policy=never after rate limit, got %s", updated.RestartPolicy)
	}
}

// TestSupervisorRestartFailureMarksInstanceFailed verifies HR-02: when the
// restart attempt fails (no loader/runner configured), the instance is marked
// StatusFailed and policy is set to RestartNever.
func TestSupervisorRestartFailureMarksInstanceFailed(t *testing.T) {
	dir := t.TempDir()
	paths := store.NewPaths(filepath.Join(dir, "state"))
	st, err := store.Open(paths)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	inst := insertDeadRunningInstance(t, st, "failrestart1", "fail-instance", types.RestartAlways)

	// InstanceManager with no Loader or Runner — Run() will fail to resolve the image.
	sup := New(st, manager.InstanceManager{Store: st})
	sup.Interval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go sup.Run(ctx)
	<-ctx.Done()

	updated, err := st.ResolveInstance(inst.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if updated.Status != types.StatusFailed {
		t.Errorf("expected StatusFailed after restart failure, got %s", updated.Status)
	}
	if updated.RestartPolicy != types.RestartNever {
		t.Errorf("expected policy=never after restart failure, got %s", updated.RestartPolicy)
	}
}

// TestSupervisorNeverPolicyNotRestarted verifies that an instance with
// RestartNever is not restarted when it dies.
func TestSupervisorNeverPolicyNotRestarted(t *testing.T) {
	dir := t.TempDir()
	paths := store.NewPaths(filepath.Join(dir, "state"))
	st, err := store.Open(paths)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	insertDeadRunningInstance(t, st, "never1", "never-instance", types.RestartNever)

	sup := New(st, manager.InstanceManager{Store: st})
	sup.Interval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	go sup.Run(ctx)
	<-ctx.Done()

	updated, err := st.ResolveInstance("never1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// Should be stopped (not running), not failed (we didn't try to restart).
	if updated.Status != types.StatusStopped {
		t.Errorf("expected stopped, got %s", updated.Status)
	}
	if updated.RestartPolicy != types.RestartNever {
		t.Errorf("expected policy still never, got %s", updated.RestartPolicy)
	}
}
