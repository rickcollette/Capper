package replication

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

// ErrNoQuorum is returned when a write cannot proceed because fewer than
// half+1 of the configured replicas are reachable.
var ErrNoQuorum = errors.New("csd: no quorum — insufficient live replicas")

// Transport is the network interface used by ElectionManager and ReplicaManager
// to communicate with peer CSD nodes. In production this is backed by QUIC.
// In tests it can be replaced with an in-process implementation.
type Transport interface {
	// SendElection notifies peer that nodeID is starting an election for term.
	// Returns true if the peer yields (sends OK back).
	SendElection(ctx context.Context, peer, nodeID string, term uint64) (bool, error)
	// SendCoordinator broadcasts that leaderID has won the election for term.
	SendCoordinator(ctx context.Context, peer, leaderID string, term uint64) error
	// SendHeartbeat sends a leader heartbeat to peer for the given term.
	SendHeartbeat(ctx context.Context, peer string, term uint64) error
}

// FencingToken is a monotonically increasing counter that prevents stale leaders
// from writing after a new election completes. Each term change increments it.
type FencingToken struct {
	val atomic.Uint64
}

// Current returns the current fencing token value.
func (ft *FencingToken) Current() uint64 { return ft.val.Load() }

// Advance increments the fencing token, signalling a new election term.
func (ft *FencingToken) Advance() uint64 { return ft.val.Add(1) }

// Validate returns ErrStaleFence if token is older than the current value.
func (ft *FencingToken) Validate(token uint64) error {
	if cur := ft.val.Load(); token < cur {
		return &StaleFenceError{Got: token, Want: cur}
	}
	return nil
}

// StaleFenceError is returned when a write carries an outdated fencing token.
type StaleFenceError struct {
	Got  uint64
	Want uint64
}

func (e *StaleFenceError) Error() string {
	return fmt.Sprintf("csd: stale fencing token %d (current: %d) — possible split-brain", e.Got, e.Want)
}
