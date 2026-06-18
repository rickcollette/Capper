package csdfuse

import (
	"context"
	"syscall"

	csdclient "capper/internal/csd/client"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// CSDFileHandle is an open file handle backed by the CSD client.
type CSDFileHandle struct {
	client  *csdclient.Client
	inodeID string
	flags   uint32
}

var (
	_ fs.FileReader    = (*CSDFileHandle)(nil)
	_ fs.FileWriter    = (*CSDFileHandle)(nil)
	_ fs.FileFlusher   = (*CSDFileHandle)(nil)
	_ fs.FileGetattrer = (*CSDFileHandle)(nil)
)

func (fh *CSDFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	data, err := fh.client.Read(ctx, fh.inodeID, off, len(dest))
	if err != nil {
		return nil, syscall.EIO
	}
	return fuse.ReadResultData(data), 0
}

func (fh *CSDFileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	if err := fh.client.Write(ctx, fh.inodeID, off, data); err != nil {
		return 0, syscall.EIO
	}
	return uint32(len(data)), 0
}

func (fh *CSDFileHandle) Flush(ctx context.Context) syscall.Errno {
	if err := fh.client.Fsync(ctx, fh.inodeID); err != nil {
		return syscall.EIO
	}
	return 0
}

func (fh *CSDFileHandle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	inode, err := fh.client.Getattr(ctx, fh.inodeID)
	if err != nil {
		return syscall.EIO
	}
	fillAttr(inode, out)
	return 0
}
