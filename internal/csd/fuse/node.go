package csdfuse

import (
	"context"
	"os"
	"syscall"
	"time"

	"capper/internal/csd"
	csdclient "capper/internal/csd/client"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// CSDNode is a single inode in the CSD FUSE tree.
// It embeds fs.Inode (required by go-fuse v2).
type CSDNode struct {
	fs.Inode
	client  *csdclient.Client
	inodeID string  // CSD inode ID
}

// Compile-time interface checks.
var (
	_ fs.NodeGetattrer = (*CSDNode)(nil)
	_ fs.NodeLookuper  = (*CSDNode)(nil)
	_ fs.NodeReaddirer = (*CSDNode)(nil)
	_ fs.NodeCreater   = (*CSDNode)(nil)
	_ fs.NodeMkdirer   = (*CSDNode)(nil)
	_ fs.NodeUnlinker  = (*CSDNode)(nil)
	_ fs.NodeRmdirer   = (*CSDNode)(nil)
	_ fs.NodeRenamer   = (*CSDNode)(nil)
	_ fs.NodeOpener    = (*CSDNode)(nil)
)

func newNode(client *csdclient.Client, inodeID string) *CSDNode {
	return &CSDNode{client: client, inodeID: inodeID}
}

func (n *CSDNode) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	inode, err := n.client.Getattr(ctx, n.inodeID)
	if err != nil {
		return syscall.EIO
	}
	fillAttr(inode, out)
	return 0
}

func (n *CSDNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child, err := n.client.Lookup(ctx, n.inodeID, name)
	if err == csd.ErrNotFound {
		return nil, syscall.ENOENT
	}
	if err != nil {
		return nil, syscall.EIO
	}
	childNode := newNode(n.client, child.ID)
	fillEntryOut(child, out)
	stable := fs.StableAttr{
		Mode: modeFromInode(child),
		Ino:  stableIno(child.ID),
	}
	return n.NewInode(ctx, childNode, stable), 0
}

func (n *CSDNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	children, err := n.client.Readdir(ctx, n.inodeID)
	if err != nil {
		return nil, syscall.EIO
	}
	entries := make([]fuse.DirEntry, 0, len(children)+2)
	entries = append(entries,
		fuse.DirEntry{Name: ".", Mode: fuse.S_IFDIR, Ino: stableIno(n.inodeID)},
		fuse.DirEntry{Name: "..", Mode: fuse.S_IFDIR, Ino: 1},
	)
	for _, c := range children {
		entries = append(entries, fuse.DirEntry{
			Name: c.Name,
			Mode: modeFromInode(c),
			Ino:  stableIno(c.ID),
		})
	}
	return fs.NewListDirStream(entries), 0
}

func (n *CSDNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	child, err := n.client.Create(ctx, n.inodeID, name, mode)
	if err != nil {
		return nil, nil, 0, syscall.EIO
	}
	fillEntryOut(child, out)
	childNode := newNode(n.client, child.ID)
	stable := fs.StableAttr{Mode: fuse.S_IFREG, Ino: stableIno(child.ID)}
	inode := n.NewInode(ctx, childNode, stable)
	fh := &CSDFileHandle{client: n.client, inodeID: child.ID, flags: flags}
	return inode, fh, fuse.FOPEN_KEEP_CACHE, 0
}

func (n *CSDNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child, err := n.client.Mkdir(ctx, n.inodeID, name, mode)
	if err != nil {
		return nil, syscall.EIO
	}
	fillEntryOut(child, out)
	childNode := newNode(n.client, child.ID)
	stable := fs.StableAttr{Mode: fuse.S_IFDIR, Ino: stableIno(child.ID)}
	return n.NewInode(ctx, childNode, stable), 0
}

func (n *CSDNode) Unlink(ctx context.Context, name string) syscall.Errno {
	child, err := n.client.Lookup(ctx, n.inodeID, name)
	if err == csd.ErrNotFound {
		return syscall.ENOENT
	}
	if err != nil {
		return syscall.EIO
	}
	if err := n.client.Unlink(ctx, n.inodeID, child.ID, name); err != nil {
		return syscall.EIO
	}
	return 0
}

func (n *CSDNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	return n.Unlink(ctx, name)
}

func (n *CSDNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, _ uint32) syscall.Errno {
	child, err := n.client.Lookup(ctx, n.inodeID, name)
	if err == csd.ErrNotFound {
		return syscall.ENOENT
	}
	if err != nil {
		return syscall.EIO
	}
	np, ok := newParent.(*CSDNode)
	if !ok {
		return syscall.EIO
	}
	if err := n.client.Rename(ctx, child.ID, n.inodeID, name, np.inodeID, newName); err != nil {
		return syscall.EIO
	}
	return 0
}

func (n *CSDNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &CSDFileHandle{client: n.client, inodeID: n.inodeID, flags: flags}, fuse.FOPEN_KEEP_CACHE, 0
}

// ---- helpers ----------------------------------------------------------------

func fillAttr(inode csd.Inode, out *fuse.AttrOut) {
	out.Attr.Ino = stableIno(inode.ID)
	out.Attr.Size = uint64(inode.SizeBytes)
	out.Attr.Mode = modeFromInode(inode)
	out.Attr.Uid = inode.UID
	out.Attr.Gid = inode.GID
	out.Attr.Nlink = uint32(inode.LinkCount)
	parseTime(inode.ModifiedAt, &out.Attr.Mtime, &out.Attr.Mtimensec)
	parseTime(inode.AccessedAt, &out.Attr.Atime, &out.Attr.Atimensec)
	parseTime(inode.CreatedAt, &out.Attr.Ctime, &out.Attr.Ctimensec)
	out.AttrValid = 1
}

func fillEntryOut(inode csd.Inode, out *fuse.EntryOut) {
	fillAttr(inode, &fuse.AttrOut{Attr: out.Attr})
	out.Attr.Ino = stableIno(inode.ID)
	out.Attr.Size = uint64(inode.SizeBytes)
	out.Attr.Mode = modeFromInode(inode)
	out.Attr.Uid = inode.UID
	out.Attr.Gid = inode.GID
	out.Attr.Nlink = uint32(inode.LinkCount)
	parseTime(inode.ModifiedAt, &out.Attr.Mtime, &out.Attr.Mtimensec)
	out.AttrValid = 1
	out.EntryValid = 1
}

func modeFromInode(inode csd.Inode) uint32 {
	switch inode.Type {
	case csd.InodeDir:
		return fuse.S_IFDIR | inode.ModeBits
	case csd.InodeSymlink:
		return fuse.S_IFLNK | inode.ModeBits
	default:
		return fuse.S_IFREG | inode.ModeBits
	}
}

// stableIno converts a CSD UUID string to a stable uint64 inode number.
// Uses FNV-1a to avoid collisions without a full map.
func stableIno(id string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(id); i++ {
		h ^= uint64(id[i])
		h *= 1099511628211
	}
	if h < 2 {
		h += 2 // FUSE reserves 0 and 1
	}
	return h
}

func parseTime(s string, sec *uint64, nsec *uint32) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return
	}
	*sec = uint64(t.Unix())
	*nsec = uint32(t.Nanosecond())
}

// ---- file mode mask --------------------------------------------------------
// os.ModeDir is already defined; we just need its numeric value for FUSE.
const _ = os.ModeDir
