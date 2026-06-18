package replication

import (
	"context"
	"time"

	csdstore "capper/internal/csd/store"
)

const repairInterval = 60 * time.Second

// RepairReconciler detects degraded replicas and attempts to re-sync them by
// restarting their streaming sessions via the ReplicaManager.
type RepairReconciler struct {
	store   *csdstore.Store
	manager *ReplicaManager
}

func NewRepairReconciler(store *csdstore.Store, manager *ReplicaManager) *RepairReconciler {
	return &RepairReconciler{store: store, manager: manager}
}

// Run starts the repair loop. It periodically scans replica records for
// "failed" or "degraded" status and restarts their sessions.
func (r *RepairReconciler) Run(ctx context.Context) {
	t := time.NewTicker(repairInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.repair(ctx)
		}
	}
}

func (r *RepairReconciler) repair(ctx context.Context) {
	replicas, err := r.store.Replicas.ListAll()
	if err != nil {
		return
	}
	for _, rep := range replicas {
		if rep.Status != "failed" && rep.Status != "degraded" {
			continue
		}
		if rep.Addr == "" {
			continue
		}
		// Stop any existing session before restarting.
		r.manager.StopReplica(rep.ID)
		_ = r.manager.StartReplica(ctx, rep.ID, rep.VolumeID, rep.Addr)
		_ = r.store.Replicas.UpdateStatus(rep.ID, "repairing")
	}
}
