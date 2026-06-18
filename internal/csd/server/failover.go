package csdserver

import (
	"context"
	"time"

	"capper/internal/csd"
	"capper/internal/csd/replication"
)

const (
	failoverCheckInterval = 5 * time.Second
	// primaryStaleAfter is how long since last_seq update before we consider
	// the primary lost and trigger a new election.
	primaryStaleAfter = 30 * time.Second
)

// FailoverManager monitors the primary for each volume. When the primary has
// not updated its progress within primaryStaleAfter, it triggers an election
// via the ElectionManager.
type FailoverManager struct {
	server *Server
	nodeID string
	// elections holds one ElectionManager per volumeID.
	elections map[string]*replication.ElectionManager
}

func NewFailoverManager(srv *Server, nodeID string) *FailoverManager {
	return &FailoverManager{
		server:    srv,
		nodeID:    nodeID,
		elections: make(map[string]*replication.ElectionManager),
	}
}

// Run starts the failover watch loop.
func (f *FailoverManager) Run(ctx context.Context) {
	t := time.NewTicker(failoverCheckInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			f.check(ctx)
		}
	}
}

func (f *FailoverManager) check(ctx context.Context) {
	vols, err := f.server.Store.Volumes.List("")
	if err != nil {
		return
	}
	for _, v := range vols {
		f.checkVolume(ctx, v)
	}
}

func (f *FailoverManager) checkVolume(ctx context.Context, vol csd.Volume) {
	replicas, err := f.server.Store.Replicas.ListByVolume(vol.ID)
	if err != nil {
		return
	}
	primaryHealthy := false
	for _, r := range replicas {
		if r.Role != csd.ReplicaPrimary {
			continue
		}
		if r.NodeID == f.nodeID {
			primaryHealthy = true
			break
		}
		// Check if the primary's updated_at is recent enough.
		t, err := time.Parse(time.RFC3339, r.UpdatedAt)
		if err == nil && time.Since(t) < primaryStaleAfter {
			primaryHealthy = true
		}
	}
	if primaryHealthy {
		return
	}
	// Primary is gone — start an election for this volume.
	em := f.electionFor(vol.ID)
	go em.Run(ctx)
}

func (f *FailoverManager) electionFor(volumeID string) *replication.ElectionManager {
	if em, ok := f.elections[volumeID]; ok {
		return em
	}
	em := replication.NewElectionManager(f.server.Store, f.nodeID, volumeID)
	f.elections[volumeID] = em
	return em
}

// SetReadOnly transitions a volume to readonly mode when no primary is available
// and this node cannot win an election (e.g. insufficient quorum).
func (f *FailoverManager) SetReadOnly(volumeID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_ = now
	return f.server.Store.Volumes.UpdateStatus(volumeID, csd.StatusReadonly)
}
