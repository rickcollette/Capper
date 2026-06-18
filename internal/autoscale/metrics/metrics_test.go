package metrics_test

import (
	"context"
	"testing"

	"capper/internal/autoscale/metrics"
	"capper/internal/types"
)

// TestNewCollector ensures the collector can be constructed without panicking.
func TestNewCollector(t *testing.T) {
	c := metrics.NewCollector(
		func() ([]types.Instance, error) { return nil, nil },
		func(groupID string) ([]string, error) { return nil, nil },
		nil, // no LB stats
	)
	if c == nil {
		t.Fatal("expected non-nil collector")
	}
}

// TestGroupMetrics_EmptyGroup returns zero-value metrics for a group with no instances.
func TestGroupMetrics_EmptyGroup(t *testing.T) {
	c := metrics.NewCollector(
		func() ([]types.Instance, error) { return nil, nil },
		func(groupID string) ([]string, error) { return []string{}, nil },
		nil,
	)
	gm, err := c.GroupMetrics(context.Background(), "empty-group")
	if err != nil {
		t.Fatalf("GroupMetrics: %v", err)
	}
	if gm.GroupID != "empty-group" {
		t.Errorf("GroupID: %q", gm.GroupID)
	}
	if gm.HealthyReplicas != 0 {
		t.Errorf("expected 0 healthy replicas, got %d", gm.HealthyReplicas)
	}
}

// TestGroupMetrics_HealthyInstances counts running instances as healthy.
func TestGroupMetrics_HealthyInstances(t *testing.T) {
	instances := []types.Instance{
		{ID: "inst-1", Status: types.StatusRunning},
		{ID: "inst-2", Status: types.StatusRunning},
		{ID: "inst-3", Status: types.StatusStopped},
	}
	c := metrics.NewCollector(
		func() ([]types.Instance, error) { return instances, nil },
		func(groupID string) ([]string, error) {
			return []string{"inst-1", "inst-2", "inst-3"}, nil
		},
		nil,
	)
	gm, err := c.GroupMetrics(context.Background(), "g1")
	if err != nil {
		t.Fatalf("GroupMetrics: %v", err)
	}
	if gm.HealthyReplicas != 2 {
		t.Errorf("expected 2 healthy replicas (running), got %d", gm.HealthyReplicas)
	}
	if gm.TotalReplicas != 3 {
		t.Errorf("expected 3 total replicas, got %d", gm.TotalReplicas)
	}
}
