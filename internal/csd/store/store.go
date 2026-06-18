package csdstore

import "database/sql"

// Store bundles the CSD sub-stores that share a single SQLite DB handle.
type Store struct {
	db          *sql.DB
	Volumes     *VolumeStore
	Attachments *AttachmentStore
	Inodes      *InodeStore
	Extents     *ExtentStore
	Leases      *LeaseStore
	Journal     *JournalStore
	Snapshots   *SnapshotStore
	Replicas    *ReplicaStore
}

func New(db *sql.DB) *Store {
	s := &Store{db: db}
	s.Volumes = &VolumeStore{db: db}
	s.Attachments = &AttachmentStore{db: db}
	s.Inodes = &InodeStore{db: db}
	s.Extents = &ExtentStore{db: db}
	s.Leases = &LeaseStore{db: db}
	s.Journal = &JournalStore{db: db}
	s.Snapshots = &SnapshotStore{db: db}
	s.Replicas = &ReplicaStore{db: db}
	return s
}
