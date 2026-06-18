package agent

import (
	"os"
	"testing"
)

func TestCollectNodeMetrics(t *testing.T) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		t.Skip("/proc not available — skipping host metrics test")
	}
	a := &Agent{nodeID: "node-test"}
	samples := a.collectNodeMetrics()
	if len(samples) == 0 {
		t.Fatal("expected at least one metric sample on a Linux host")
	}

	seen := map[string]bool{}
	for _, s := range samples {
		if s.ResourceType != "node" || s.ResourceID != "node-test" {
			t.Errorf("sample has wrong target: %+v", s)
		}
		if s.MetricName == "" {
			t.Errorf("sample has empty metric name: %+v", s)
		}
		// Percentages must be within a sane range.
		if (s.MetricName == "cpu.percent" || s.MetricName == "memory.used_percent" ||
			s.MetricName == "disk.used_percent") && (s.Value < 0 || s.Value > 100) {
			t.Errorf("%s out of range: %v", s.MetricName, s.Value)
		}
		seen[s.MetricName] = true
	}
	// Memory should always be readable on Linux.
	if !seen["memory.used_percent"] {
		t.Error("expected memory.used_percent metric")
	}
}
