package csdserver

import (
	"context"
	"fmt"
	"time"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"

	"github.com/google/uuid"
)

const gcInterval = 30 * time.Second

type SnapshotManager struct {
	store *csdstore.Store
}

func NewSnapshotManager(store *csdstore.Store) *SnapshotManager {
	return &SnapshotManager{store: store}
}

// Create takes a crash-consistent snapshot of volumeID.
// It records the current max committed journal seq as the root_version,
// and increments the ref_count of all extents so CoW is triggered on next write.
func (sm *SnapshotManager) Create(ctx context.Context, volumeID, name string) (csd.Snapshot, error) {
	seq, err := sm.store.Journal.MaxSeq(volumeID)
	if err != nil {
		return csd.Snapshot{}, fmt.Errorf("snapshot: get journal seq: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	snap := csd.Snapshot{
		ID:          uuid.New().String(),
		VolumeID:    volumeID,
		Name:        name,
		RootVersion: seq,
		Status:      csd.StatusAvailable,
		Consistent:  true,
		CreatedAt:   now,
	}
	// Increment ref_count for all extents so writes trigger CoW.
	extents, err := sm.store.Extents.ForVolume(volumeID)
	if err != nil {
		return csd.Snapshot{}, err
	}
	for _, e := range extents {
		if err := sm.store.Extents.IncrRef(e.ID); err != nil {
			return csd.Snapshot{}, fmt.Errorf("snapshot: incr extent ref: %w", err)
		}
		snap.SizeBytes += e.LengthBytes
	}
	if err := sm.store.Snapshots.Insert(snap); err != nil {
		return csd.Snapshot{}, err
	}
	return snap, nil
}

func (sm *SnapshotManager) List(_ context.Context, volumeID string) ([]csd.Snapshot, error) {
	snaps, err := sm.store.Snapshots.List(volumeID)
	if snaps == nil {
		snaps = []csd.Snapshot{}
	}
	return snaps, err
}

func (sm *SnapshotManager) Delete(_ context.Context, volumeID, nameOrID string) error {
	snap, err := sm.store.Snapshots.Get(volumeID, nameOrID)
	if err != nil {
		return err
	}
	// Decrement ref counts — GC loop will clean up zero-ref extents.
	extents, err := sm.store.Extents.ForVolume(volumeID)
	if err != nil {
		return err
	}
	for _, e := range extents {
		if e.RefCount > 1 {
			if err := sm.store.Extents.DecrRef(e.ID); err != nil {
				return fmt.Errorf("snapshot: decr extent ref: %w", err)
			}
		}
	}
	return sm.store.Snapshots.Delete(snap.ID)
}

// GCLoop runs periodically and deletes extents with ref_count <= 0.
func (sm *SnapshotManager) GCLoop(ctx context.Context) {
	t := time.NewTicker(gcInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = sm.store.Extents.DeleteOrphans()
		}
	}
}
