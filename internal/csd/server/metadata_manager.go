package csdserver

import (
	"context"
	"fmt"
	"time"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"

	"github.com/google/uuid"
)

type MetadataManager struct {
	store   *csdstore.Store
	journal *JournalManager
}

func NewMetadataManager(store *csdstore.Store, journal *JournalManager) *MetadataManager {
	return &MetadataManager{store: store, journal: journal}
}

// EnsureRoot creates the root directory inode for volumeID if it does not exist.
func (mm *MetadataManager) EnsureRoot(ctx context.Context, volumeID string) (csd.Inode, error) {
	root, err := mm.store.Inodes.GetRoot(volumeID)
	if err == nil {
		return root, nil
	}
	if err != csd.ErrNotFound {
		return csd.Inode{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	root = csd.Inode{
		ID:         "root-" + volumeID,
		VolumeID:   volumeID,
		ParentID:   "",
		Name:       "/",
		Type:       csd.InodeDir,
		ModeBits:   0o755,
		LinkCount:  2,
		Version:    1,
		CreatedAt:  now,
		ModifiedAt: now,
		AccessedAt: now,
	}
	if err := mm.store.Inodes.Insert(root); err != nil {
		return csd.Inode{}, err
	}
	return root, nil
}

func (mm *MetadataManager) Lookup(_ context.Context, volumeID, parentID, name string) (csd.Inode, error) {
	return mm.store.Inodes.Lookup(volumeID, parentID, name)
}

func (mm *MetadataManager) Getattr(_ context.Context, inodeID string) (csd.Inode, error) {
	return mm.store.Inodes.Get(inodeID)
}

func (mm *MetadataManager) Readdir(_ context.Context, volumeID, dirID string) ([]csd.Inode, error) {
	return mm.store.Inodes.Children(volumeID, dirID)
}

type CreateReq struct {
	VolumeID string
	ParentID string
	Name     string
	ModeBits uint32
	UID      uint32
	GID      uint32
	ClientID string
}

func (mm *MetadataManager) Create(ctx context.Context, req CreateReq) (csd.Inode, error) {
	return mm.mknode(ctx, req, csd.InodeFile)
}

func (mm *MetadataManager) Mkdir(ctx context.Context, req CreateReq) (csd.Inode, error) {
	return mm.mknode(ctx, req, csd.InodeDir)
}

func (mm *MetadataManager) mknode(ctx context.Context, req CreateReq, nodeType string) (csd.Inode, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	n := csd.Inode{
		ID:         uuid.New().String(),
		VolumeID:   req.VolumeID,
		ParentID:   req.ParentID,
		Name:       req.Name,
		Type:       nodeType,
		ModeBits:   req.ModeBits,
		UID:        req.UID,
		GID:        req.GID,
		LinkCount:  1,
		Version:    1,
		CreatedAt:  now,
		ModifiedAt: now,
		AccessedAt: now,
	}
	op := csd.JournalCreate
	if nodeType == csd.InodeDir {
		op = csd.JournalMkdir
	}
	seq, err := mm.journal.Append(ctx, req.VolumeID, req.ClientID, "", op, n.ID, map[string]any{
		"name": req.Name, "parent_id": req.ParentID, "mode_bits": req.ModeBits,
	})
	if err != nil {
		return csd.Inode{}, err
	}
	if err := mm.store.Inodes.Insert(n); err != nil {
		return csd.Inode{}, err
	}
	if err := mm.journal.Commit(ctx, req.VolumeID, seq); err != nil {
		return csd.Inode{}, err
	}
	return n, nil
}

type UnlinkReq struct {
	VolumeID string
	InodeID  string
	ParentID string
	Name     string
	ClientID string
}

func (mm *MetadataManager) Unlink(ctx context.Context, req UnlinkReq) error {
	n, err := mm.store.Inodes.Get(req.InodeID)
	if err != nil {
		return err
	}
	seq, err := mm.journal.Append(ctx, req.VolumeID, req.ClientID, "", csd.JournalUnlink, req.InodeID, map[string]any{
		"inode_id": req.InodeID, "parent_id": req.ParentID, "name": req.Name,
	})
	if err != nil {
		return err
	}
	n.LinkCount--
	if n.LinkCount <= 0 {
		if err := mm.store.Inodes.Delete(req.InodeID); err != nil {
			return err
		}
	} else {
		if err := mm.store.Inodes.Update(n); err != nil {
			return err
		}
	}
	return mm.journal.Commit(ctx, req.VolumeID, seq)
}

type RenameReq struct {
	VolumeID    string
	InodeID     string
	OldParentID string
	OldName     string
	NewParentID string
	NewName     string
	ClientID    string
}

func (mm *MetadataManager) Rename(ctx context.Context, req RenameReq) error {
	seq, err := mm.journal.Append(ctx, req.VolumeID, req.ClientID, "", csd.JournalRename, req.InodeID, map[string]any{
		"old_parent_id": req.OldParentID, "old_name": req.OldName,
		"new_parent_id": req.NewParentID, "new_name": req.NewName,
	})
	if err != nil {
		return err
	}
	if err := mm.store.Inodes.Move(req.InodeID, req.NewParentID, req.NewName); err != nil {
		return err
	}
	return mm.journal.Commit(ctx, req.VolumeID, seq)
}

type TruncateReq struct {
	VolumeID string
	InodeID  string
	NewSize  int64
	ClientID string
}

func (mm *MetadataManager) Truncate(ctx context.Context, req TruncateReq) error {
	n, err := mm.store.Inodes.Get(req.InodeID)
	if err != nil {
		return err
	}
	seq, err := mm.journal.Append(ctx, req.VolumeID, req.ClientID, "", csd.JournalTruncate, req.InodeID, map[string]any{
		"new_size": req.NewSize,
	})
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	n.SizeBytes = req.NewSize
	n.ModifiedAt = now
	if err := mm.store.Inodes.Update(n); err != nil {
		return err
	}
	return mm.journal.Commit(ctx, req.VolumeID, seq)
}

// apply re-executes a journal entry (used during replay).
func (mm *MetadataManager) apply(ctx context.Context, e csd.JournalEntry) error {
	switch e.Operation {
	case csd.JournalCreate, csd.JournalMkdir:
		nodeType := csd.InodeFile
		if e.Operation == csd.JournalMkdir {
			nodeType = csd.InodeDir
		}
		name, _ := e.Payload["name"].(string)
		parentID, _ := e.Payload["parent_id"].(string)
		modeBitsF, _ := e.Payload["mode_bits"].(float64)
		now := time.Now().UTC().Format(time.RFC3339)
		n := csd.Inode{
			ID:       e.InodeID,
			VolumeID: e.VolumeID,
			ParentID: parentID,
			Name:     name,
			Type:     nodeType,
			ModeBits: uint32(modeBitsF),
			LinkCount: 1,
			Version:   1,
			CreatedAt: now, ModifiedAt: now, AccessedAt: now,
		}
		err := mm.store.Inodes.Insert(n)
		if err != nil && err != csd.ErrAlreadyExists {
			return err
		}
	case csd.JournalUnlink:
		n, err := mm.store.Inodes.Get(e.InodeID)
		if err == csd.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		n.LinkCount--
		if n.LinkCount <= 0 {
			return mm.store.Inodes.Delete(e.InodeID)
		}
		return mm.store.Inodes.Update(n)
	case csd.JournalRename:
		newParent, _ := e.Payload["new_parent_id"].(string)
		newName, _ := e.Payload["new_name"].(string)
		return mm.store.Inodes.Move(e.InodeID, newParent, newName)
	case csd.JournalTruncate:
		n, err := mm.store.Inodes.Get(e.InodeID)
		if err != nil {
			return err
		}
		sizeF, _ := e.Payload["new_size"].(float64)
		n.SizeBytes = int64(sizeF)
		n.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
		return mm.store.Inodes.Update(n)
	}
	return fmt.Errorf("metadata: unknown operation %q in journal replay", e.Operation)
}
