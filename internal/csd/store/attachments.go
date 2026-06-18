package csdstore

import (
	"database/sql"
	"fmt"
	"strings"

	"capper/internal/csd"
)

type AttachmentStore struct {
	db *sql.DB
}

func (s *AttachmentStore) Insert(a csd.Attachment) error {
	_, err := s.db.Exec(`
		INSERT INTO csd_volume_attachments
			(id, volume_id, instance_id, node_id, mount_path, access_mode,
			 client_id, lease_epoch, status, attached_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.VolumeID, a.InstanceID, a.NodeID, a.MountPath, a.AccessMode,
		a.ClientID, a.LeaseEpoch, a.Status, a.AttachedAt, a.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("%w: attachment for volume %q on instance %q at %q",
				csd.ErrAlreadyExists, a.VolumeID, a.InstanceID, a.MountPath)
		}
		return err
	}
	return nil
}

func (s *AttachmentStore) Get(id string) (csd.Attachment, error) {
	row := s.db.QueryRow(`
		SELECT id, volume_id, instance_id, node_id, mount_path, access_mode,
			client_id, lease_epoch, status, attached_at, updated_at
		FROM csd_volume_attachments WHERE id=?`, id)
	return scanAttachment(row)
}

func (s *AttachmentStore) GetByVolumeInstance(volumeID, instanceID string) (csd.Attachment, error) {
	row := s.db.QueryRow(`
		SELECT id, volume_id, instance_id, node_id, mount_path, access_mode,
			client_id, lease_epoch, status, attached_at, updated_at
		FROM csd_volume_attachments WHERE volume_id=? AND instance_id=? LIMIT 1`,
		volumeID, instanceID,
	)
	return scanAttachment(row)
}

func (s *AttachmentStore) ListByVolume(volumeID string) ([]csd.Attachment, error) {
	rows, err := s.db.Query(`
		SELECT id, volume_id, instance_id, node_id, mount_path, access_mode,
			client_id, lease_epoch, status, attached_at, updated_at
		FROM csd_volume_attachments WHERE volume_id=? ORDER BY attached_at`, volumeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAttachments(rows)
}

func (s *AttachmentStore) ListByInstance(instanceID string) ([]csd.Attachment, error) {
	rows, err := s.db.Query(`
		SELECT id, volume_id, instance_id, node_id, mount_path, access_mode,
			client_id, lease_epoch, status, attached_at, updated_at
		FROM csd_volume_attachments WHERE instance_id=? ORDER BY attached_at`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAttachments(rows)
}

func (s *AttachmentStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE csd_volume_attachments SET status=?, updated_at=datetime('now') WHERE id=?`, status, id)
	return err
}

func (s *AttachmentStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM csd_volume_attachments WHERE id=?`, id)
	return err
}

func (s *AttachmentStore) DeleteByInstance(instanceID string) error {
	_, err := s.db.Exec(`DELETE FROM csd_volume_attachments WHERE instance_id=?`, instanceID)
	return err
}

func (s *AttachmentStore) CountByVolume(volumeID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM csd_volume_attachments WHERE volume_id=?`, volumeID).Scan(&n)
	return n, err
}

// ---- helpers ----------------------------------------------------------------

func scanAttachment(row *sql.Row) (csd.Attachment, error) {
	var a csd.Attachment
	err := row.Scan(
		&a.ID, &a.VolumeID, &a.InstanceID, &a.NodeID, &a.MountPath, &a.AccessMode,
		&a.ClientID, &a.LeaseEpoch, &a.Status, &a.AttachedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return a, csd.ErrNotFound
	}
	return a, err
}

func scanAttachments(rows *sql.Rows) ([]csd.Attachment, error) {
	var out []csd.Attachment
	for rows.Next() {
		var a csd.Attachment
		if err := rows.Scan(
			&a.ID, &a.VolumeID, &a.InstanceID, &a.NodeID, &a.MountPath, &a.AccessMode,
			&a.ClientID, &a.LeaseEpoch, &a.Status, &a.AttachedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
