package csdstore

import (
	"database/sql"
	"fmt"
	"strings"
)

// InitSchema creates all CSD tables. Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS csd_volumes (
			id                TEXT PRIMARY KEY,
			project           TEXT NOT NULL DEFAULT '',
			name              TEXT NOT NULL,
			mode              TEXT NOT NULL DEFAULT 'shared-fs',
			size_bytes        INTEGER NOT NULL,
			used_bytes        INTEGER NOT NULL DEFAULT 0,
			status            TEXT NOT NULL DEFAULT 'creating',
			realm_id          TEXT NOT NULL DEFAULT '',
			region_id         TEXT NOT NULL DEFAULT '',
			zone_id           TEXT NOT NULL DEFAULT '',
			storage_class     TEXT NOT NULL DEFAULT 'local',
			replica_count     INTEGER NOT NULL DEFAULT 1,
			primary_node_id   TEXT NOT NULL DEFAULT '',
			epoch             INTEGER NOT NULL DEFAULT 1,
			encrypted         INTEGER NOT NULL DEFAULT 0,
			encryption_key_id TEXT NOT NULL DEFAULT '',
			created_at        TEXT NOT NULL,
			updated_at        TEXT NOT NULL,
			UNIQUE(project, name)
		)`,
		`CREATE TABLE IF NOT EXISTS csd_volume_attachments (
			id          TEXT PRIMARY KEY,
			volume_id   TEXT NOT NULL REFERENCES csd_volumes(id),
			instance_id TEXT NOT NULL,
			node_id     TEXT NOT NULL DEFAULT '',
			mount_path  TEXT NOT NULL,
			access_mode TEXT NOT NULL DEFAULT 'rw',
			client_id   TEXT NOT NULL DEFAULT '',
			lease_epoch INTEGER NOT NULL DEFAULT 0,
			status      TEXT NOT NULL DEFAULT 'pending',
			attached_at TEXT NOT NULL,
			updated_at  TEXT NOT NULL,
			UNIQUE(volume_id, instance_id, mount_path)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_csd_attachments_instance ON csd_volume_attachments(instance_id)`,
		`CREATE TABLE IF NOT EXISTS csd_inodes (
			id          TEXT PRIMARY KEY,
			volume_id   TEXT NOT NULL REFERENCES csd_volumes(id),
			parent_id   TEXT NOT NULL DEFAULT '',
			name        TEXT NOT NULL,
			inode_type  TEXT NOT NULL DEFAULT 'file',
			size_bytes  INTEGER NOT NULL DEFAULT 0,
			mode_bits   INTEGER NOT NULL DEFAULT 420,
			uid         INTEGER NOT NULL DEFAULT 0,
			gid         INTEGER NOT NULL DEFAULT 0,
			link_count  INTEGER NOT NULL DEFAULT 1,
			version     INTEGER NOT NULL DEFAULT 1,
			extent_root TEXT NOT NULL DEFAULT '',
			inline_data BLOB,
			created_at  TEXT NOT NULL,
			modified_at TEXT NOT NULL,
			accessed_at TEXT NOT NULL,
			UNIQUE(volume_id, parent_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_csd_inodes_volume ON csd_inodes(volume_id)`,
		`CREATE INDEX IF NOT EXISTS idx_csd_inodes_parent ON csd_inodes(volume_id, parent_id)`,
		`CREATE TABLE IF NOT EXISTS csd_extents (
			id           TEXT PRIMARY KEY,
			volume_id    TEXT NOT NULL REFERENCES csd_volumes(id),
			inode_id     TEXT NOT NULL REFERENCES csd_inodes(id),
			offset_bytes INTEGER NOT NULL,
			length_bytes INTEGER NOT NULL,
			object_key   TEXT NOT NULL,
			checksum     TEXT NOT NULL DEFAULT '',
			ref_count    INTEGER NOT NULL DEFAULT 1,
			created_at   TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_csd_extents_inode ON csd_extents(inode_id, offset_bytes)`,
		`CREATE TABLE IF NOT EXISTS csd_leases (
			id          TEXT PRIMARY KEY,
			volume_id   TEXT NOT NULL REFERENCES csd_volumes(id),
			inode_id    TEXT NOT NULL,
			client_id   TEXT NOT NULL,
			session_id  TEXT NOT NULL DEFAULT '',
			lease_type  TEXT NOT NULL,
			range_start INTEGER NOT NULL DEFAULT 0,
			range_end   INTEGER NOT NULL DEFAULT -1,
			epoch       INTEGER NOT NULL,
			expires_at  TEXT NOT NULL,
			created_at  TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_csd_leases_inode  ON csd_leases(volume_id, inode_id)`,
		`CREATE INDEX IF NOT EXISTS idx_csd_leases_client ON csd_leases(client_id)`,
		`CREATE TABLE IF NOT EXISTS csd_journal (
			id           TEXT PRIMARY KEY,
			volume_id    TEXT NOT NULL REFERENCES csd_volumes(id),
			seq          INTEGER NOT NULL,
			client_id    TEXT NOT NULL,
			session_id   TEXT NOT NULL DEFAULT '',
			operation    TEXT NOT NULL,
			inode_id     TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '{}',
			checksum     TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'pending',
			created_at   TEXT NOT NULL,
			committed_at TEXT NOT NULL DEFAULT '',
			UNIQUE(volume_id, seq)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_csd_journal_vol_seq ON csd_journal(volume_id, seq)`,
		`CREATE TABLE IF NOT EXISTS csd_volume_replicas (
			id           TEXT PRIMARY KEY,
			volume_id    TEXT NOT NULL REFERENCES csd_volumes(id),
			role         TEXT NOT NULL DEFAULT 'secondary',
			realm_id     TEXT NOT NULL DEFAULT '',
			region_id    TEXT NOT NULL DEFAULT '',
			zone_id      TEXT NOT NULL DEFAULT '',
			node_id      TEXT NOT NULL DEFAULT '',
			addr         TEXT NOT NULL DEFAULT '',
			backend_type TEXT NOT NULL DEFAULT 'local',
			backend_path TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'pending',
			lag_bytes    INTEGER NOT NULL DEFAULT 0,
			last_seq     INTEGER NOT NULL DEFAULT 0,
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS csd_volume_snapshots (
			id           TEXT PRIMARY KEY,
			volume_id    TEXT NOT NULL REFERENCES csd_volumes(id),
			name         TEXT NOT NULL,
			root_version INTEGER NOT NULL,
			status       TEXT NOT NULL DEFAULT 'creating',
			consistent   INTEGER NOT NULL DEFAULT 1,
			size_bytes   INTEGER NOT NULL DEFAULT 0,
			created_at   TEXT NOT NULL,
			UNIQUE(volume_id, name)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("csdstore.InitSchema: %w", err)
		}
	}
	// Additive migrations — ignore "duplicate column" errors.
	for _, alter := range []string{} {
		if _, err := db.Exec(alter); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("csdstore.InitSchema migrate: %w", err)
			}
		}
	}
	return nil
}
