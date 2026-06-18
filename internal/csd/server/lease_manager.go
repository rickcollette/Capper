package csdserver

import (
	"context"
	"fmt"
	"time"

	"capper/internal/csd"
	"capper/internal/csd/replication"
	csdstore "capper/internal/csd/store"

	"github.com/google/uuid"
)

const leaseExpireCheck = 5 * time.Second

type LeaseManager struct {
	store  *csdstore.Store
	fence  replication.FencingToken
}

func NewLeaseManager(store *csdstore.Store) *LeaseManager {
	return &LeaseManager{store: store}
}

type LeaseRequest struct {
	VolumeID   string
	InodeID    string
	ClientID   string
	SessionID  string
	LeaseType  string
	RangeStart int64
	RangeEnd   int64
	Epoch      int64
}

func (lm *LeaseManager) Acquire(_ context.Context, req LeaseRequest) (csd.Lease, error) {
	existing, err := lm.store.Leases.ForInode(req.VolumeID, req.InodeID)
	if err != nil {
		return csd.Lease{}, err
	}
	for _, ex := range existing {
		if ex.ClientID == req.ClientID {
			continue // same client, allow re-acquire
		}
		if err := lm.checkConflict(req, ex); err != nil {
			return csd.Lease{}, err
		}
	}
	now := time.Now().UTC()
	l := csd.Lease{
		ID:         uuid.New().String(),
		VolumeID:   req.VolumeID,
		InodeID:    req.InodeID,
		ClientID:   req.ClientID,
		SessionID:  req.SessionID,
		LeaseType:  req.LeaseType,
		RangeStart: req.RangeStart,
		RangeEnd:   req.RangeEnd,
		Epoch:      req.Epoch,
		ExpiresAt:  now.Add(csd.LeaseTTL),
		CreatedAt:  now.UTC().Format(time.RFC3339),
	}
	if err := lm.store.Leases.Insert(l); err != nil {
		return csd.Lease{}, err
	}
	return l, nil
}

func (lm *LeaseManager) Renew(_ context.Context, leaseID, clientID string) error {
	l, err := lm.store.Leases.Get(leaseID)
	if err != nil {
		return err
	}
	if l.ClientID != clientID {
		return csd.ErrAccessDenied
	}
	return lm.store.Leases.Renew(leaseID, time.Now().Add(csd.LeaseTTL))
}

func (lm *LeaseManager) Release(_ context.Context, leaseID, clientID string) error {
	l, err := lm.store.Leases.Get(leaseID)
	if err != nil {
		return err
	}
	if l.ClientID != clientID {
		return csd.ErrAccessDenied
	}
	return lm.store.Leases.Delete(leaseID)
}

func (lm *LeaseManager) Revoke(_ context.Context, volumeID, clientID string) error {
	_, err := lm.store.Leases.DeleteForClient(volumeID, clientID)
	return err
}

// ExpireLoop runs in a goroutine and periodically removes expired leases.
func (lm *LeaseManager) ExpireLoop(ctx context.Context) {
	t := time.NewTicker(leaseExpireCheck)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = lm.store.Leases.DeleteExpired()
		}
	}
}

// ValidateFence returns an error if token is older than the current fencing token,
// which indicates a write from a stale leader (possible split-brain scenario).
func (lm *LeaseManager) ValidateFence(token uint64) error {
	return lm.fence.Validate(token)
}

// AdvanceFence increments the fencing token, signalling a new election term.
// Must be called whenever a new leader is elected.
func (lm *LeaseManager) AdvanceFence() uint64 {
	return lm.fence.Advance()
}

// CurrentFence returns the current fencing token value.
func (lm *LeaseManager) CurrentFence() uint64 {
	return lm.fence.Current()
}

// checkConflict returns ErrLeaseConflict if req conflicts with an existing lease.
func (lm *LeaseManager) checkConflict(req LeaseRequest, ex csd.Lease) error {
	if ex.LeaseType == csd.LeaseExclusive || req.LeaseType == csd.LeaseExclusive {
		return fmt.Errorf("%w: exclusive lease held by %s", csd.ErrLeaseConflict, ex.ClientID)
	}
	if req.LeaseType == csd.LeaseRead && ex.LeaseType == csd.LeaseRead {
		return nil
	}
	// write ∩ read or write ∩ write — check range overlap
	if rangesOverlap(req.RangeStart, req.RangeEnd, ex.RangeStart, ex.RangeEnd) {
		return fmt.Errorf("%w: conflicting %s lease held by %s on range [%d,%d]",
			csd.ErrLeaseConflict, ex.LeaseType, ex.ClientID, ex.RangeStart, ex.RangeEnd)
	}
	return nil
}

func rangesOverlap(s1, e1, s2, e2 int64) bool {
	// -1 means unbounded end
	if e1 == -1 || e2 == -1 {
		return true
	}
	return s1 < e2 && s2 < e1
}
