package health

import (
	"context"

	"capper/internal/types"
)

// InstanceLister is a subset of store.Store used by the reconciler.
// Defined here to avoid import cycles.
type InstanceLister interface {
	ListInstances() ([]types.Instance, error)
}

// Reconciler runs health checks for all running instances that have a
// HealthCheck defined.
type Reconciler struct {
	lister      InstanceLister
	health      *Store
	// OnUnhealthy is an optional callback invoked when an instance transitions
	// to unhealthy. It receives the instance ID so callers (e.g. the daemon)
	// can remove the instance from active LB backends.
	OnUnhealthy func(instanceID string)
}

func NewReconciler(lister InstanceLister, health *Store) *Reconciler {
	return &Reconciler{lister: lister, health: health}
}

func (r *Reconciler) Name() string { return "health-check" }

func (r *Reconciler) Reconcile(ctx context.Context) error {
	instances, err := r.lister.ListInstances()
	if err != nil {
		return err
	}
	for _, inst := range instances {
		if inst.Status != types.StatusRunning {
			continue
		}
		if inst.HealthCheck == nil || inst.HealthCheck.Port == 0 {
			continue
		}
		ip := inst.NetworkIP
		if ip == "" {
			ip = "127.0.0.1"
		}
		hc := inst.HealthCheck
		timeout := hc.Timeout
		if timeout <= 0 {
			timeout = 5
		}
		var result Result
		if hc.Protocol == "http" {
			path := hc.Path
			if path == "" {
				path = "/"
			}
			result = CheckHTTP(inst.ID, ip, path, hc.Port, timeout)
		} else {
			result = CheckTCP(inst.ID, ip, hc.Port, timeout)
		}
		_ = r.health.Upsert(result)
		if result.Status == "unhealthy" && r.OnUnhealthy != nil {
			r.OnUnhealthy(inst.ID)
		}
	}
	return nil
}
