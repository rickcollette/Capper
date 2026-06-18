package cgroup

import (
	"os"
	"testing"

	"capper/internal/types"
)

func TestAvailable(t *testing.T) {
	// Just verify the call doesn't panic. The result depends on the host.
	_ = Available()
}

func TestNewReturnsNilWhenUnavailable(t *testing.T) {
	if Available() && os.Geteuid() == 0 {
		t.Skip("cgroup v2 available as root; skipping graceful-degradation test")
	}
	mgr, err := New("test-instance-" + t.Name())
	if err != nil {
		t.Fatalf("expected nil error on graceful degradation, got: %v", err)
	}
	if mgr != nil && !Available() {
		t.Fatal("expected nil manager when cgroup v2 unavailable")
	}
}

func TestManagerFullLifecycle(t *testing.T) {
	if !Available() {
		t.Skip("cgroup v2 not available")
	}
	if os.Geteuid() != 0 {
		t.Skip("cgroup write requires root")
	}

	id := "capper-test-" + t.Name()
	mgr, err := New(id)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if mgr == nil {
		t.Fatal("expected non-nil manager as root with cgroup v2")
	}
	defer mgr.Remove()

	limits := types.ResourceLimits{
		MemoryBytes:  64 * 1024 * 1024,
		MaxProcesses: 32,
	}
	errs := mgr.Apply(limits)
	if len(errs) != 0 {
		t.Fatalf("Apply errors: %v", errs)
	}

	if open := Open(id); open == nil {
		t.Fatal("Open returned nil for existing cgroup")
	}
}
