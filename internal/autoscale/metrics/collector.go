// Package metrics provides metric collection for the autoscaler.
// It reads live cgroup metrics for each instance in a group and computes
// derived group-level aggregates used by the evaluators.
package metrics

import (
	"context"
	"fmt"
	"time"

	"capper/internal/autoscale"
	"capper/internal/lb"
	cgroupmetrics "capper/internal/metrics"
	"capper/internal/types"
)

// InstanceLister returns all known instances.
type InstanceLister func() ([]types.Instance, error)

// GroupInstanceLister returns the instance IDs belonging to a group.
type GroupInstanceLister func(groupID string) ([]string, error)

// LBStatsFetcher returns live LB stats.
type LBStatsFetcher func() []lb.LBStat

// Collector samples per-instance cgroup metrics and aggregates them into
// group-level metrics stored in the metric_samples table.
type Collector struct {
	listInstances    InstanceLister
	listGroupInsts   GroupInstanceLister
	lbStats          LBStatsFetcher

	// instanceMemoryMax is the memory limit (bytes) per instance type; used to
	// convert memory bytes into a percentage. If 0, percentage = 0.
	instanceMemoryMax func(instanceID string) uint64
}

// NewCollector creates a Collector. lbStats may be nil if no LB integration.
func NewCollector(
	listInstances InstanceLister,
	listGroupInsts GroupInstanceLister,
	lbStats LBStatsFetcher,
) *Collector {
	return &Collector{
		listInstances:  listInstances,
		listGroupInsts: listGroupInsts,
		lbStats:        lbStats,
	}
}

// SetMemoryMaxFunc registers a function that returns the memory limit (bytes)
// for an instance, used to compute memory percentage.
func (c *Collector) SetMemoryMaxFunc(fn func(id string) uint64) {
	c.instanceMemoryMax = fn
}

// GroupMetrics queries live cgroup metrics for all running instances in groupID
// and returns aggregated GroupMetrics.
func (c *Collector) GroupMetrics(_ context.Context, groupID string) (autoscale.GroupMetrics, error) {
	// Get instance IDs for this group.
	instanceIDs, err := c.listGroupInsts(groupID)
	if err != nil {
		return autoscale.GroupMetrics{}, fmt.Errorf("autoscale/metrics: list group instances: %w", err)
	}
	if len(instanceIDs) == 0 {
		return autoscale.GroupMetrics{GroupID: groupID}, nil
	}

	// Build a set for O(1) lookup.
	idSet := make(map[string]struct{}, len(instanceIDs))
	for _, id := range instanceIDs {
		idSet[id] = struct{}{}
	}

	// Read all running instances, filter to group members.
	allInsts, err := c.listInstances()
	if err != nil {
		return autoscale.GroupMetrics{}, fmt.Errorf("autoscale/metrics: list instances: %w", err)
	}

	var totalCPUPercent, totalMemPercent float64
	healthyCount := 0
	for _, inst := range allInsts {
		if _, ok := idSet[inst.ID]; !ok {
			continue
		}
		if inst.Status != types.StatusRunning {
			continue
		}
		m := cgroupmetrics.ReadInstance(inst.ID, inst.Name)
		// CPU: use raw microseconds as a proxy for load.
		// A single CPU at 100% over 1 second = 1_000_000 µs.
		// We treat the raw µs value as "load units" for relative comparisons.
		totalCPUPercent += float64(m.CPUUsageUs) / 1_000_000 * 100

		if c.instanceMemoryMax != nil {
			maxMem := c.instanceMemoryMax(inst.ID)
			if maxMem > 0 {
				totalMemPercent += float64(m.MemoryBytes) / float64(maxMem) * 100
			}
		}
		healthyCount++
	}

	gm := autoscale.GroupMetrics{
		GroupID:         groupID,
		HealthyReplicas: healthyCount,
		TotalReplicas:   len(instanceIDs),
	}
	if healthyCount > 0 {
		gm.CPUAvgPercent = totalCPUPercent / float64(healthyCount)
		gm.MemoryAvgPercent = totalMemPercent / float64(healthyCount)
	}

	// LB-based metrics.
	if c.lbStats != nil {
		stats := c.lbStats()
		var totalConns int64
		var totalReqs uint64
		for _, s := range stats {
			totalConns += s.ActiveConns
			totalReqs += s.TotalRequests
		}
		if healthyCount > 0 {
			gm.ActiveConnsPerInst = float64(totalConns) / float64(healthyCount)
		}
		gm.RequestsPerSec = float64(totalReqs) / float64(time.Minute/time.Second)
	}

	return gm, nil
}

// QueryMetric resolves a named metric for a group into a float64 value.
// Supported metrics: group_cpu_avg_percent, group_memory_avg_percent,
// group_active_connections_per_instance, group_healthy_replicas.
func (c *Collector) QueryMetric(ctx context.Context, groupID, metricName string, queueDepthFn func(name string) (int64, error)) (float64, error) {
	switch metricName {
	case autoscale.MetricQueueDepth:
		// Caller supplies a queue depth function.
		return 0, fmt.Errorf("autoscale/metrics: queue depth requires queue name — use QueryQueueMetric")

	case autoscale.MetricHealthyReplicas:
		gm, err := c.GroupMetrics(ctx, groupID)
		if err != nil {
			return 0, err
		}
		return float64(gm.HealthyReplicas), nil

	default:
		gm, err := c.GroupMetrics(ctx, groupID)
		if err != nil {
			return 0, err
		}
		switch metricName {
		case autoscale.MetricCPUAvgPercent:
			return gm.CPUAvgPercent, nil
		case autoscale.MetricMemoryAvgPercent:
			return gm.MemoryAvgPercent, nil
		case autoscale.MetricActiveConnsPerInst:
			return gm.ActiveConnsPerInst, nil
		case autoscale.MetricRequestsPerSec:
			return gm.RequestsPerSec, nil
		default:
			return 0, fmt.Errorf("autoscale/metrics: unknown metric %q", metricName)
		}
	}
}
