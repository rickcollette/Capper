package csdbackend

import "context"

// Backend stores and retrieves extent data blobs by key.
// Implementations must be safe for concurrent use.
type Backend interface {
	PutExtent(ctx context.Context, volumeID, key string, data []byte) error
	GetExtent(ctx context.Context, volumeID, key string) ([]byte, error)
	DeleteExtent(ctx context.Context, volumeID, key string) error
	HasExtent(ctx context.Context, volumeID, key string) bool
	ListExtents(ctx context.Context, volumeID string) ([]string, error)
}
