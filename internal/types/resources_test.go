package types

import "testing"

func TestResourceOverridesApplyFieldByField(t *testing.T) {
	base := ResourceLimits{
		MemoryBytes:   128,
		CPUTimeSecs:   10,
		MaxProcesses:  20,
		FileSizeBytes: 30,
	}
	override := ResourceOverrides{
		Limits: ResourceLimits{
			CPUTimeSecs: 60,
		},
		CPUTimeSet: true,
	}
	got := override.Apply(base)
	if got.MemoryBytes != base.MemoryBytes {
		t.Fatalf("memory was unexpectedly overridden: %#v", got)
	}
	if got.CPUTimeSecs != 60 {
		t.Fatalf("cpu was not overridden: %#v", got)
	}
	if got.MaxProcesses != base.MaxProcesses {
		t.Fatalf("pids were unexpectedly overridden: %#v", got)
	}
	if got.FileSizeBytes != base.FileSizeBytes {
		t.Fatalf("file size was unexpectedly overridden: %#v", got)
	}
}
