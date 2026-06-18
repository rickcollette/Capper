package csdserver

import (
	"context"
	"fmt"
	"time"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"

	"github.com/google/uuid"
)

const (
	journalKeepLast = 1000
	checkpointEvery = 60 * time.Second
)

type JournalManager struct {
	store *csdstore.Store
}

func NewJournalManager(store *csdstore.Store) *JournalManager {
	return &JournalManager{store: store}
}

// Append inserts a pending journal entry and returns its assigned seq number.
func (jm *JournalManager) Append(_ context.Context, volumeID, clientID, sessionID, operation, inodeID string, payload map[string]any) (int64, error) {
	seq, err := jm.store.Journal.NextSeq(volumeID)
	if err != nil {
		return 0, fmt.Errorf("journal: next seq: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	e := csd.JournalEntry{
		ID:        uuid.New().String(),
		VolumeID:  volumeID,
		Seq:       seq,
		ClientID:  clientID,
		SessionID: sessionID,
		Operation: operation,
		InodeID:   inodeID,
		Payload:   payload,
		Status:    "pending",
		CreatedAt: now,
	}
	if err := jm.store.Journal.Append(e); err != nil {
		return 0, err
	}
	return seq, nil
}

// Commit marks the entry at seq as committed.
func (jm *JournalManager) Commit(_ context.Context, volumeID string, seq int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return jm.store.Journal.Commit(volumeID, seq, now)
}

// Replay re-applies all pending entries in seq order.
// Called on startup to recover from a crash before a clean shutdown.
func (jm *JournalManager) Replay(ctx context.Context, mm *MetadataManager, volumeID string) error {
	pending, err := jm.store.Journal.Pending(volumeID)
	if err != nil {
		return err
	}
	for _, e := range pending {
		if err := mm.apply(ctx, e); err != nil {
			// best-effort mark committed on replay failure (avoid re-replaying)
			now := time.Now().UTC().Format(time.RFC3339)
			_ = jm.store.Journal.Commit(volumeID, e.Seq, now)
			continue
		}
		if err := jm.Commit(ctx, volumeID, e.Seq); err != nil {
			return err
		}
	}
	return nil
}

// Checkpoint deletes committed entries older than keepLast per volume.
func (jm *JournalManager) Checkpoint(_ context.Context, volumeID string) error {
	max, err := jm.store.Journal.MaxSeq(volumeID)
	if err != nil {
		return err
	}
	if max <= journalKeepLast {
		return nil
	}
	return jm.store.Journal.Truncate(volumeID, max-journalKeepLast)
}

// CheckpointLoop runs Checkpoint for all volumes every checkpointEvery interval.
func (jm *JournalManager) CheckpointLoop(ctx context.Context, vm *VolumeManager) {
	t := time.NewTicker(checkpointEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			vols, err := vm.List(ctx, "")
			if err != nil {
				continue
			}
			for _, v := range vols {
				_ = jm.Checkpoint(ctx, v.ID)
			}
		}
	}
}
