package csdclient

import (
	"context"
	"fmt"

	"capper/internal/csd"
	csdserver "capper/internal/csd/server"
)

// Client is the in-process CSD client (Phase 3). It calls the server directly.
// Phase 4 will replace the direct calls with CSDP/QUIC transport.
type Client struct {
	VolumeID   string
	InstanceID string
	ClientID   string
	server     *csdserver.Server
	// epoch tracks the last known volume epoch. When a call returns
	// ErrStaleEpoch the client refreshes its epoch and re-acquires leases.
	epoch int64
}

// NewClient returns a client for the given volume, bound to a local server.
func NewClient(volumeID, instanceID, nodeID string, srv *csdserver.Server) *Client {
	return &Client{
		VolumeID:   volumeID,
		InstanceID: instanceID,
		ClientID:   fmt.Sprintf("csdc_%s_%s", nodeID, instanceID),
		server:     srv,
	}
}

func (c *Client) RootInodeID() string {
	return "root-" + c.VolumeID
}

func (c *Client) EnsureRoot(ctx context.Context) (csd.Inode, error) {
	return c.server.Metadata.EnsureRoot(ctx, c.VolumeID)
}

func (c *Client) Lookup(ctx context.Context, parentID, name string) (csd.Inode, error) {
	return c.server.Metadata.Lookup(ctx, c.VolumeID, parentID, name)
}

func (c *Client) Getattr(ctx context.Context, inodeID string) (csd.Inode, error) {
	return c.server.Metadata.Getattr(ctx, inodeID)
}

func (c *Client) Readdir(ctx context.Context, dirID string) ([]csd.Inode, error) {
	return c.server.Metadata.Readdir(ctx, c.VolumeID, dirID)
}

func (c *Client) Create(ctx context.Context, parentID, name string, mode uint32) (csd.Inode, error) {
	return c.server.Metadata.Create(ctx, csdserver.CreateReq{
		VolumeID: c.VolumeID, ParentID: parentID, Name: name,
		ModeBits: mode, ClientID: c.ClientID,
	})
}

func (c *Client) Mkdir(ctx context.Context, parentID, name string, mode uint32) (csd.Inode, error) {
	return c.server.Metadata.Mkdir(ctx, csdserver.CreateReq{
		VolumeID: c.VolumeID, ParentID: parentID, Name: name,
		ModeBits: mode, ClientID: c.ClientID,
	})
}

func (c *Client) Unlink(ctx context.Context, parentID, inodeID, name string) error {
	return c.server.Metadata.Unlink(ctx, csdserver.UnlinkReq{
		VolumeID: c.VolumeID, InodeID: inodeID,
		ParentID: parentID, Name: name, ClientID: c.ClientID,
	})
}

func (c *Client) Rename(ctx context.Context, inodeID, oldParent, oldName, newParent, newName string) error {
	return c.server.Metadata.Rename(ctx, csdserver.RenameReq{
		VolumeID:    c.VolumeID,
		InodeID:     inodeID,
		OldParentID: oldParent, OldName: oldName,
		NewParentID: newParent, NewName: newName,
		ClientID: c.ClientID,
	})
}

func (c *Client) Truncate(ctx context.Context, inodeID string, size int64) error {
	return c.server.Metadata.Truncate(ctx, csdserver.TruncateReq{
		VolumeID: c.VolumeID, InodeID: inodeID, NewSize: size, ClientID: c.ClientID,
	})
}

func (c *Client) Read(ctx context.Context, inodeID string, offset int64, size int) ([]byte, error) {
	return c.server.Extents.Read(ctx, csdserver.ReadReq{
		VolumeID: c.VolumeID, InodeID: inodeID, Offset: offset, Length: size,
	})
}

func (c *Client) Write(ctx context.Context, inodeID string, offset int64, data []byte) error {
	return c.server.Extents.Write(ctx, csdserver.WriteReq{
		VolumeID:  c.VolumeID,
		InodeID:   inodeID,
		Offset:    offset,
		Data:      data,
		ClientID:  c.ClientID,
	})
}

func (c *Client) Fsync(ctx context.Context, inodeID string) error {
	// Phase 3: in-process, writes are already committed synchronously.
	return nil
}

// RefreshEpoch re-reads the volume epoch from the store. It is called
// automatically when a server call returns ErrStaleEpoch, and the FUSE
// layer retries the operation with the updated epoch.
func (c *Client) RefreshEpoch(ctx context.Context) error {
	v, err := c.server.Store.Volumes.Get(c.VolumeID, "")
	if err != nil {
		return fmt.Errorf("client: refresh epoch: %w", err)
	}
	c.epoch = v.Epoch
	// Re-acquire any write leases we held under the old epoch.
	_ = c.server.Leases.Revoke(ctx, c.VolumeID, c.ClientID)
	return nil
}

// Epoch returns the client's current view of the volume epoch.
func (c *Client) Epoch() int64 { return c.epoch }
