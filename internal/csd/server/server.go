package csdserver

import (
	"context"

	csdbackend "capper/internal/csd/backend"
	"capper/internal/csd/replication"
	csdstore "capper/internal/csd/store"
)

// Server is the CSD local server. It owns all the managers and provides the
// in-process API used by the FUSE client (Phase 3) and CSDP listener (Phase 4).
type Server struct {
	Store     *csdstore.Store
	Backend   csdbackend.Backend
	Volumes   *VolumeManager
	Metadata  *MetadataManager
	Journal   *JournalManager
	Leases    *LeaseManager
	Extents   *ExtentManager
	Snapshots *SnapshotManager
	Replicas  *replication.ReplicaManager
	Failover  *FailoverManager
}

// NewServer wires all managers together.
func NewServer(store *csdstore.Store, backend csdbackend.Backend) *Server {
	journal := NewJournalManager(store)
	leases := NewLeaseManager(store)
	meta := NewMetadataManager(store, journal)
	extents := NewExtentManager(store, backend, journal)
	volumes := NewVolumeManager(store)
	snapshots := NewSnapshotManager(store)
	replicas := replication.NewReplicaManager(store)
	s := &Server{
		Store:     store,
		Backend:   backend,
		Volumes:   volumes,
		Metadata:  meta,
		Journal:   journal,
		Leases:    leases,
		Extents:   extents,
		Snapshots: snapshots,
		Replicas:  replicas,
	}
	s.Failover = NewFailoverManager(s, "local")
	return s
}

// Start replays pending journal entries for all volumes, then starts background
// goroutines for lease expiry, journal checkpointing, snapshot GC, and replica
// reconciliation.
func (s *Server) Start(ctx context.Context) error {
	vols, err := s.Store.Volumes.List("")
	if err != nil {
		return err
	}
	for _, v := range vols {
		if err := s.Journal.Replay(ctx, s.Metadata, v.ID); err != nil {
			// Log and continue — a failed replay is degraded, not fatal.
			continue
		}
		if _, err := s.Metadata.EnsureRoot(ctx, v.ID); err != nil {
			continue
		}
	}
	go s.Leases.ExpireLoop(ctx)
	go s.Journal.CheckpointLoop(ctx, s.Volumes)
	go s.Snapshots.GCLoop(ctx)
	go s.Replicas.ReconcileLoop(ctx)
	go s.Failover.Run(ctx)
	return nil
}
