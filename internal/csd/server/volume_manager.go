package csdserver

import (
	"context"
	"fmt"
	"time"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"

	"github.com/google/uuid"
)

// VolumeManager handles volume and attachment lifecycle.
type VolumeManager struct {
	store *csdstore.Store
}

// NewVolumeManager returns a VolumeManager backed by store.
func NewVolumeManager(store *csdstore.Store) *VolumeManager {
	return &VolumeManager{store: store}
}

// CreateVolumeOpts are the inputs for creating a new CSD volume.
type CreateVolumeOpts struct {
	Project      string
	Name         string
	Mode         string
	SizeBytes    int64
	StorageClass string
	ReplicaCount int
	Encrypted    bool
	EncKeyID     string
}

// AttachOpts are the inputs for attaching a volume to an instance.
type AttachOpts struct {
	VolumeID   string
	InstanceID string
	NodeID     string
	MountPath  string
	AccessMode string
}

func (m *VolumeManager) Create(_ context.Context, opts CreateVolumeOpts) (csd.Volume, error) {
	if opts.Name == "" {
		return csd.Volume{}, fmt.Errorf("csd: volume name is required")
	}
	if opts.SizeBytes <= 0 {
		return csd.Volume{}, fmt.Errorf("csd: size must be > 0")
	}
	mode := opts.Mode
	if mode == "" {
		mode = csd.ModeSharedFS
	}
	switch mode {
	case csd.ModeSharedFS, csd.ModeSingleWriter, csd.ModeSharedBlock:
	default:
		return csd.Volume{}, fmt.Errorf("csd: unknown mode %q", mode)
	}
	class := opts.StorageClass
	if class == "" {
		class = "local"
	}
	replicas := opts.ReplicaCount
	if replicas <= 0 {
		replicas = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	v := csd.Volume{
		ID:              uuid.New().String(),
		Project:         opts.Project,
		Name:            opts.Name,
		Mode:            mode,
		SizeBytes:       opts.SizeBytes,
		UsedBytes:       0,
		Status:          csd.StatusAvailable,
		StorageClass:    class,
		ReplicaCount:    replicas,
		Epoch:           1,
		Encrypted:       opts.Encrypted,
		EncryptionKeyID: opts.EncKeyID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := m.store.Volumes.Insert(v); err != nil {
		return csd.Volume{}, err
	}
	return v, nil
}

func (m *VolumeManager) Get(_ context.Context, idOrName, project string) (csd.Volume, error) {
	return m.store.Volumes.Get(idOrName, project)
}

func (m *VolumeManager) List(_ context.Context, project string) ([]csd.Volume, error) {
	vols, err := m.store.Volumes.List(project)
	if vols == nil {
		vols = []csd.Volume{}
	}
	return vols, err
}

func (m *VolumeManager) Delete(_ context.Context, idOrName, project string) error {
	v, err := m.store.Volumes.Get(idOrName, project)
	if err != nil {
		return err
	}
	n, err := m.store.Attachments.CountByVolume(v.ID)
	if err != nil {
		return err
	}
	if n > 0 {
		return csd.ErrVolumeActive
	}
	return m.store.Volumes.Delete(v.ID)
}

func (m *VolumeManager) Attach(_ context.Context, opts AttachOpts) (csd.Attachment, error) {
	v, err := m.store.Volumes.Get(opts.VolumeID, "")
	if err != nil {
		return csd.Attachment{}, err
	}
	mode := opts.AccessMode
	if mode == "" {
		mode = csd.AccessRW
	}
	if mode != csd.AccessRO && mode != csd.AccessRW {
		return csd.Attachment{}, fmt.Errorf("csd: unknown access mode %q", mode)
	}
	// Single-writer: only one RW attachment allowed at a time.
	if v.Mode == csd.ModeSingleWriter && mode == csd.AccessRW {
		atts, err := m.store.Attachments.ListByVolume(v.ID)
		if err != nil {
			return csd.Attachment{}, err
		}
		for _, a := range atts {
			if a.AccessMode == csd.AccessRW {
				return csd.Attachment{}, fmt.Errorf("%w: single-writer volume already has an rw attachment", csd.ErrLeaseConflict)
			}
		}
	}
	mp := opts.MountPath
	if mp == "" {
		mp = "/mnt/csd"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	a := csd.Attachment{
		ID:         uuid.New().String(),
		VolumeID:   v.ID,
		InstanceID: opts.InstanceID,
		NodeID:     opts.NodeID,
		MountPath:  mp,
		AccessMode: mode,
		Status:     csd.StatusAttached,
		AttachedAt: now,
		UpdatedAt:  now,
	}
	if err := m.store.Attachments.Insert(a); err != nil {
		return csd.Attachment{}, err
	}
	return a, nil
}

func (m *VolumeManager) Detach(_ context.Context, idOrName, instanceID string) error {
	v, err := m.store.Volumes.Get(idOrName, "")
	if err != nil {
		return err
	}
	a, err := m.store.Attachments.GetByVolumeInstance(v.ID, instanceID)
	if err != nil {
		return err
	}
	return m.store.Attachments.Delete(a.ID)
}

func (m *VolumeManager) DetachAll(_ context.Context, instanceID string) error {
	return m.store.Attachments.DeleteByInstance(instanceID)
}

func (m *VolumeManager) ListAttachments(_ context.Context, idOrName string) ([]csd.Attachment, error) {
	v, err := m.store.Volumes.Get(idOrName, "")
	if err != nil {
		return nil, err
	}
	atts, err := m.store.Attachments.ListByVolume(v.ID)
	if atts == nil {
		atts = []csd.Attachment{}
	}
	return atts, err
}

func (m *VolumeManager) UpdateUsage(_ context.Context, volumeID string, delta int64) error {
	return m.store.Volumes.UpdateUsed(volumeID, delta)
}
