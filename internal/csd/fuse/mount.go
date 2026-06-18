package csdfuse

import (
	"fmt"
	"os"
	"time"

	csdclient "capper/internal/csd/client"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Mount manages a FUSE mount point for a single CSD volume.
type Mount struct {
	client    *csdclient.Client
	mountPath string
	server    *fuse.Server
}

// NewMount creates a Mount (not yet mounted).
func NewMount(client *csdclient.Client, mountPath string) *Mount {
	return &Mount{client: client, mountPath: mountPath}
}

// Mount mounts the CSD volume at m.mountPath using FUSE.
// EnsureRoot must have been called on the client's volume before mounting.
func (m *Mount) Mount() error {
	if err := os.MkdirAll(m.mountPath, 0o755); err != nil {
		return fmt.Errorf("csd/fuse: create mount dir %s: %w", m.mountPath, err)
	}
	rootNode := newNode(m.client, m.client.RootInodeID())
	ttl := time.Second
	opts := &fs.Options{
		AttrTimeout:  &ttl,
		EntryTimeout: &ttl,
		MountOptions: fuse.MountOptions{
			AllowOther: true,
			FsName:     "capper-csd",
			Name:       "csd",
		},
	}
	srv, err := fs.Mount(m.mountPath, rootNode, opts)
	if err != nil {
		return fmt.Errorf("csd/fuse: mount %s: %w", m.mountPath, err)
	}
	m.server = srv
	go m.server.Wait()
	return nil
}

// Unmount unmounts the FUSE filesystem and releases the mount point.
func (m *Mount) Unmount() error {
	if m.server == nil {
		return nil
	}
	return m.server.Unmount()
}
