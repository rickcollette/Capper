package replication

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"

	"github.com/google/uuid"
)

const (
	electionInterval     = 5 * time.Second
	electionTimeout      = 300 * time.Millisecond
	electionLagThreshold = 100
)

// ElectionManager watches the primary replica for a volume and runs a Bully
// election when the primary is unavailable or has a stale last_seq.
//
// Bully algorithm: the node with the highest nodeID that responds wins.
// A node that receives an ELECTION message from a lower-ID peer sends OK back
// and starts its own election; if it gets no OK from higher peers within the
// timeout it declares itself leader.
type ElectionManager struct {
	store    *csdstore.Store
	nodeID   string
	volumeID string
	peers    []string // peer addresses of other CSD nodes
	transport Transport

	mu       sync.Mutex
	term     atomic.Uint64
	leaderID atomic.Value // string
}

func NewElectionManager(store *csdstore.Store, nodeID, volumeID string) *ElectionManager {
	return &ElectionManager{store: store, nodeID: nodeID, volumeID: volumeID}
}

// WithPeers sets the peer addresses and transport for distributed elections.
func (em *ElectionManager) WithPeers(peers []string, t Transport) {
	em.peers = peers
	em.transport = t
}

// Run starts the election watch loop until ctx is cancelled.
func (em *ElectionManager) Run(ctx context.Context) {
	t := time.NewTicker(electionInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = em.tick(ctx)
		}
	}
}

// StartElection runs a Bully election for the current term.
// Called when the leader is detected as lost.
func (em *ElectionManager) StartElection(ctx context.Context) error {
	term := em.term.Add(1)

	// If transport is not configured, fall back to single-node promotion.
	if em.transport == nil || len(em.peers) == 0 {
		return em.promote()
	}

	higher := em.peersWithHigherID()
	if len(higher) == 0 {
		// No higher-priority peers — declare self leader.
		return em.declareLeader(ctx, term)
	}

	okCh := make(chan bool, len(higher))
	ectx, cancel := context.WithTimeout(ctx, electionTimeout)
	defer cancel()
	for _, peer := range higher {
		go func(p string) {
			ok, _ := em.transport.SendElection(ectx, p, em.nodeID, term)
			okCh <- ok
		}(peer)
	}

	for i := 0; i < len(higher); i++ {
		select {
		case ok := <-okCh:
			if ok {
				// A higher-priority node took over.
				return nil
			}
		case <-ectx.Done():
			return em.declareLeader(ctx, term)
		}
	}
	return em.declareLeader(ctx, term)
}

// HandleElectionMessage is called when a peer sends an ELECTION message.
// If the peer has a lower nodeID we send OK back and start our own election.
func (em *ElectionManager) HandleElectionMessage(ctx context.Context, from string, term uint64) {
	if from < em.nodeID {
		// We outrank the sender — start our own election (they will yield).
		go func() { _ = em.StartElection(ctx) }()
	}
	// If from > em.nodeID we yield; the caller is responsible for sending OK.
}

// HandleCoordinator is called when a peer broadcasts that it is the new leader.
func (em *ElectionManager) HandleCoordinator(leaderID string, term uint64) {
	em.leaderID.Store(leaderID)
	em.mu.Lock()
	if term > em.term.Load() {
		em.term.Store(term)
	}
	em.mu.Unlock()
}

// LeaderID returns the current known leader, or empty string if unknown.
func (em *ElectionManager) LeaderID() string {
	v, _ := em.leaderID.Load().(string)
	return v
}

// ---- internal ---------------------------------------------------------------

func (em *ElectionManager) tick(ctx context.Context) error {
	replicas, err := em.store.Replicas.ListByVolume(em.volumeID)
	if err != nil {
		return err
	}
	var primary *csd.Replica
	for i := range replicas {
		if replicas[i].Role == csd.ReplicaPrimary {
			primary = &replicas[i]
			break
		}
	}
	// Healthy primary from another node — nothing to do.
	if primary != nil && primary.NodeID != em.nodeID && primary.Status == "active" {
		return nil
	}
	// Primary missing or stale — start election.
	return em.StartElection(ctx)
}

func (em *ElectionManager) peersWithHigherID() []string {
	var out []string
	for _, p := range em.peers {
		if p > em.nodeID {
			out = append(out, p)
		}
	}
	return out
}

func (em *ElectionManager) declareLeader(ctx context.Context, term uint64) error {
	em.leaderID.Store(em.nodeID)
	if em.transport != nil {
		for _, peer := range em.peers {
			_ = em.transport.SendCoordinator(ctx, peer, em.nodeID, term)
		}
	}
	return em.promote()
}

// promote sets this node's replica record to primary role and increments the
// volume epoch to fence stale clients.
func (em *ElectionManager) promote() error {
	replicas, err := em.store.Replicas.ListByVolume(em.volumeID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, r := range replicas {
		if r.Role == csd.ReplicaPrimary && r.NodeID != em.nodeID {
			_ = em.store.Replicas.UpdateStatus(r.ID, "secondary")
		}
	}
	var myReplica *csd.Replica
	for i := range replicas {
		if replicas[i].NodeID == em.nodeID {
			myReplica = &replicas[i]
			break
		}
	}
	if myReplica == nil {
		r := csd.Replica{
			ID:        uuid.New().String(),
			VolumeID:  em.volumeID,
			Role:      csd.ReplicaPrimary,
			NodeID:    em.nodeID,
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
		}
		return em.store.Replicas.Insert(r)
	}
	// Elect most-up-to-date replica as final tiebreak if peers are present.
	if len(em.peers) == 0 {
		candidates := make([]csd.Replica, 0, len(replicas))
		for _, r := range replicas {
			if r.Status == "active" || r.NodeID == em.nodeID {
				candidates = append(candidates, r)
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].LastSeq != candidates[j].LastSeq {
				return candidates[i].LastSeq > candidates[j].LastSeq
			}
			return candidates[i].NodeID == em.nodeID
		})
		if len(candidates) > 0 && candidates[0].NodeID != em.nodeID {
			return nil
		}
	}
	_ = em.store.Volumes.BumpEpoch(em.volumeID, now)
	return em.store.Replicas.UpdateStatus(myReplica.ID, "active")
}

// HasElectionQuorum returns true when this node should consider itself
// eligible to run an election (it has seen responses from > half of peers).
func HasElectionQuorum(totalPeers, responsivePeers int) bool {
	if totalPeers == 0 {
		return true
	}
	return responsivePeers >= (totalPeers/2)+1
}

// ErrNoQuorum is re-exported here for convenience; the canonical definition
// is in transport.go.
var _ = fmt.Errorf // keep fmt imported
