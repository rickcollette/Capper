package csdserver

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"capper/internal/csd"
	csdbackend "capper/internal/csd/backend"
	csdstore "capper/internal/csd/store"

	"github.com/google/uuid"
)

type ExtentManager struct {
	store   *csdstore.Store
	backend csdbackend.Backend
	journal *JournalManager
}

func NewExtentManager(store *csdstore.Store, backend csdbackend.Backend, journal *JournalManager) *ExtentManager {
	return &ExtentManager{store: store, backend: backend, journal: journal}
}

type WriteReq struct {
	VolumeID  string
	InodeID   string
	Offset    int64
	Data      []byte
	ClientID  string
	SessionID string
	Epoch     int64
	OpSeq     int64
}

func (em *ExtentManager) Write(ctx context.Context, req WriteReq) error {
	if len(req.Data) == 0 {
		return nil
	}
	n, err := em.store.Inodes.Get(req.InodeID)
	if err != nil {
		return err
	}
	// Small file: store inline.
	if req.Offset == 0 && int64(len(req.Data)) < csd.InlineMaxBytes {
		seq, err := em.journal.Append(ctx, req.VolumeID, req.ClientID, req.SessionID, csd.JournalWrite, req.InodeID, map[string]any{
			"offset": req.Offset, "length": len(req.Data), "inline": true,
		})
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339)
		n.InlineData = req.Data
		n.SizeBytes = int64(len(req.Data))
		n.ModifiedAt = now
		if err := em.store.Inodes.Update(n); err != nil {
			return err
		}
		if err := em.journal.Commit(ctx, req.VolumeID, seq); err != nil {
			return err
		}
		return em.store.Volumes.UpdateUsed(req.VolumeID, int64(len(req.Data)))
	}

	// Large file: split into extents.
	data := req.Data
	offset := req.Offset
	totalWritten := int64(0)
	for len(data) > 0 {
		chunk := data
		if len(chunk) > csd.ExtentSize {
			chunk = data[:csd.ExtentSize]
		}
		data = data[len(chunk):]

		key := fmt.Sprintf("%s_%d", req.InodeID, offset)
		sum := sha256.Sum256(chunk)
		checksum := fmt.Sprintf("%x", sum[:8])

		if err := em.backend.PutExtent(ctx, req.VolumeID, key, chunk); err != nil {
			return fmt.Errorf("extent write: %w", err)
		}
		seq, err := em.journal.Append(ctx, req.VolumeID, req.ClientID, req.SessionID, csd.JournalWrite, req.InodeID, map[string]any{
			"offset": offset, "extent_key": key, "length": len(chunk),
		})
		if err != nil {
			return err
		}
		e := csd.Extent{
			ID:          uuid.New().String(),
			VolumeID:    req.VolumeID,
			InodeID:     req.InodeID,
			OffsetBytes: offset,
			LengthBytes: int64(len(chunk)),
			ObjectKey:   key,
			Checksum:    checksum,
			RefCount:    1,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		if err := em.store.Extents.Upsert(e); err != nil {
			return err
		}
		if err := em.journal.Commit(ctx, req.VolumeID, seq); err != nil {
			return err
		}
		offset += int64(len(chunk))
		totalWritten += int64(len(chunk))
	}

	// Update inode size if grown.
	if newEnd := req.Offset + totalWritten; newEnd > n.SizeBytes {
		n.SizeBytes = newEnd
	}
	n.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
	n.InlineData = nil // clear inline data if now using extents
	if err := em.store.Inodes.Update(n); err != nil {
		return err
	}
	return em.store.Volumes.UpdateUsed(req.VolumeID, totalWritten)
}

type ReadReq struct {
	VolumeID string
	InodeID  string
	Offset   int64
	Length   int
}

func (em *ExtentManager) Read(ctx context.Context, req ReadReq) ([]byte, error) {
	n, err := em.store.Inodes.Get(req.InodeID)
	if err != nil {
		return nil, err
	}
	// Inline case.
	if len(n.InlineData) > 0 {
		end := req.Offset + int64(req.Length)
		if end > int64(len(n.InlineData)) {
			end = int64(len(n.InlineData))
		}
		if req.Offset >= int64(len(n.InlineData)) {
			return []byte{}, nil
		}
		return n.InlineData[req.Offset:end], nil
	}
	// Extent case.
	end := req.Offset + int64(req.Length)
	extents, err := em.store.Extents.ForInodeRange(req.InodeID, req.Offset, end)
	if err != nil {
		return nil, err
	}
	result := make([]byte, req.Length)
	for _, e := range extents {
		data, err := em.backend.GetExtent(ctx, req.VolumeID, e.ObjectKey)
		if err != nil {
			return nil, err
		}
		// Copy the relevant slice into result.
		srcStart := req.Offset - e.OffsetBytes
		if srcStart < 0 {
			srcStart = 0
		}
		dstStart := e.OffsetBytes + srcStart - req.Offset
		copyLen := int64(len(data)) - srcStart
		if dstStart+copyLen > int64(len(result)) {
			copyLen = int64(len(result)) - dstStart
		}
		if copyLen > 0 {
			copy(result[dstStart:dstStart+copyLen], data[srcStart:srcStart+copyLen])
		}
	}
	// Trim to actual inode size.
	available := n.SizeBytes - req.Offset
	if available < 0 {
		available = 0
	}
	if int64(req.Length) > available {
		result = result[:available]
	}
	return result, nil
}

func (em *ExtentManager) DeleteForInode(ctx context.Context, volumeID, inodeID string) error {
	extents, err := em.store.Extents.DeleteForInode(inodeID)
	if err != nil {
		return err
	}
	for _, e := range extents {
		if e.RefCount <= 1 {
			_ = em.backend.DeleteExtent(ctx, volumeID, e.ObjectKey)
		}
	}
	return nil
}
