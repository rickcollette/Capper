package replication

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"
)

// StreamEntry is a single journal entry delivered over the replication stream.
type StreamEntry struct {
	Seq       int64          `json:"seq"`
	VolumeID  string         `json:"volumeId"`
	Operation string         `json:"operation"`
	InodeID   string         `json:"inodeId,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	CommitAt  string         `json:"committedAt"`
}

// Streamer reads committed journal entries from a volume and delivers them to
// a replica via the supplied writer, starting at afterSeq+1.
// It blocks until ctx is cancelled or a write error occurs.
type Streamer struct {
	store    *csdstore.Store
	volumeID string
	interval time.Duration
}

func NewStreamer(store *csdstore.Store, volumeID string) *Streamer {
	return &Streamer{store: store, volumeID: volumeID, interval: 500 * time.Millisecond}
}

// Stream writes newline-delimited JSON StreamEntry records to w, starting after
// afterSeq. It polls the journal at s.interval and sends new entries as they
// are committed.
func (s *Streamer) Stream(ctx context.Context, afterSeq int64, w io.Writer) error {
	cur := afterSeq
	enc := json.NewEncoder(w)
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			entries, err := s.store.Journal.Since(s.volumeID, cur)
			if err != nil {
				return fmt.Errorf("stream: journal since %d: %w", cur, err)
			}
			for _, e := range entries {
				se := StreamEntry{
					Seq:       e.Seq,
					VolumeID:  e.VolumeID,
					Operation: e.Operation,
					InodeID:   e.InodeID,
					Payload:   e.Payload,
					CommitAt:  e.CommittedAt,
				}
				if err := enc.Encode(se); err != nil {
					return fmt.Errorf("stream: encode: %w", err)
				}
				cur = e.Seq
			}
		}
	}
}

// Receiver reads newline-delimited JSON StreamEntry records from r and applies
// them to the local journal store.
type Receiver struct {
	store    *csdstore.Store
	volumeID string
}

func NewReceiver(store *csdstore.Store, volumeID string) *Receiver {
	return &Receiver{store: store, volumeID: volumeID}
}

// Receive reads from r until EOF or ctx cancellation, applying each entry.
func (rc *Receiver) Receive(ctx context.Context, r io.Reader) error {
	dec := json.NewDecoder(r)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var se StreamEntry
		if err := dec.Decode(&se); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("receive: decode: %w", err)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		e := csd.JournalEntry{
			ID:          fmt.Sprintf("rep-%s-%d", se.VolumeID, se.Seq),
			VolumeID:    se.VolumeID,
			Seq:         se.Seq,
			Operation:   se.Operation,
			InodeID:     se.InodeID,
			Payload:     se.Payload,
			Status:      "pending",
			CreatedAt:   now,
		}
		if err := rc.store.Journal.Append(e); err != nil {
			// Duplicate seq is fine — idempotent.
			continue
		}
		_ = rc.store.Journal.Commit(se.VolumeID, se.Seq, now)
	}
}
