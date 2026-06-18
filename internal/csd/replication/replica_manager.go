package replication

import (
	"context"
	"fmt"
	"sync"
	"time"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"
)

// ReplicaState is the runtime state of a single replica peer.
type ReplicaState struct {
	ReplicaID  string
	VolumeID   string
	NodeID     string
	Addr       string
	LastSeq    int64
	Status     string // "active", "lagging", "failed"
	LastSyncAt time.Time
}

// ReplicaManager drives outbound journal streaming to replica peers.
// For each (volume, peer) pair it runs a goroutine that streams committed
// journal entries to the peer's replication endpoint.
type ReplicaManager struct {
	store    *csdstore.Store
	mu       sync.Mutex
	sessions map[string]context.CancelFunc // key = replicaID
}

func NewReplicaManager(store *csdstore.Store) *ReplicaManager {
	return &ReplicaManager{
		store:    store,
		sessions: make(map[string]context.CancelFunc),
	}
}

// StartReplica begins streaming journal entries for volumeID to the peer at
// addr. If a session for this replicaID already exists it is a no-op.
func (rm *ReplicaManager) StartReplica(ctx context.Context, replicaID, volumeID, addr string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if _, ok := rm.sessions[replicaID]; ok {
		return nil
	}
	rctx, cancel := context.WithCancel(ctx)
	rm.sessions[replicaID] = cancel
	go rm.runSession(rctx, replicaID, volumeID, addr)
	return nil
}

// StopReplica cancels the streaming session for replicaID.
func (rm *ReplicaManager) StopReplica(replicaID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if cancel, ok := rm.sessions[replicaID]; ok {
		cancel()
		delete(rm.sessions, replicaID)
	}
}

// StopAll cancels all streaming sessions.
func (rm *ReplicaManager) StopAll() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for id, cancel := range rm.sessions {
		cancel()
		delete(rm.sessions, id)
	}
}

// runSession manages one replica stream with exponential backoff on failure.
func (rm *ReplicaManager) runSession(ctx context.Context, replicaID, volumeID, addr string) {
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		err := rm.streamOnce(ctx, replicaID, volumeID, addr)
		if ctx.Err() != nil {
			return
		}
		_ = err
		// Back off before reconnecting.
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// HasQuorum returns true when more than half of the registered replicas for
// volumeID are currently active. Callers must check this before any mutation.
func (rm *ReplicaManager) HasQuorum(volumeID string) (bool, error) {
	replicas, err := rm.store.Replicas.ListByVolume(volumeID)
	if err != nil {
		return false, err
	}
	if len(replicas) == 0 {
		return true, nil // single-node: always quorate
	}
	active := 0
	for _, r := range replicas {
		if r.Status == "active" {
			active++
		}
	}
	return active > len(replicas)/2, nil
}

// streamOnce opens a single streaming connection to addr and streams journal
// entries until it fails or ctx is done. The addr parameter is reserved for
// the QUIC transport; currently replication runs in-process via a pipe.
func (rm *ReplicaManager) streamOnce(ctx context.Context, replicaID, volumeID, addr string) error {
	_ = addr

	afterSeq, err := rm.store.Journal.MaxSeq(volumeID)
	if err != nil {
		return fmt.Errorf("replica %s: get max seq: %w", replicaID, err)
	}

	pr, pw := newPipe()
	streamer := NewStreamer(rm.store, volumeID)
	receiver := NewReceiver(rm.store, volumeID)

	errCh := make(chan error, 2)
	go func() { errCh <- streamer.Stream(ctx, afterSeq, pw) }()
	go func() { errCh <- receiver.Receive(ctx, pr) }()

	err = <-errCh
	// Cancel the other half.
	_ = pw.Close()
	_ = pr.Close()
	<-errCh
	return err
}

// SyncStatus returns the current replication status for a volume.
func (rm *ReplicaManager) SyncStatus(volumeID string) ([]csd.VolumeReplica, error) {
	return rm.store.Replicas.ListByVolume(volumeID)
}

// ---- ReconcileLoop ---------------------------------------------------------

// ReconcileLoop periodically checks csd_volume_replicas and starts/stops
// streaming sessions to match the desired replica set.
func (rm *ReplicaManager) ReconcileLoop(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rm.reconcile(ctx)
		}
	}
}

func (rm *ReplicaManager) reconcile(ctx context.Context) {
	replicas, err := rm.store.Replicas.ListAll()
	if err != nil {
		return
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	active := make(map[string]bool)
	for _, r := range replicas {
		active[r.ID] = true
		if _, running := rm.sessions[r.ID]; !running && r.Addr != "" {
			rctx, cancel := context.WithCancel(ctx)
			rm.sessions[r.ID] = cancel
			go rm.runSession(rctx, r.ID, r.VolumeID, r.Addr)
		}
	}
	// Stop sessions whose replica records no longer exist.
	for id, cancel := range rm.sessions {
		if !active[id] {
			cancel()
			delete(rm.sessions, id)
		}
	}
}
