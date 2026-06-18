package csdstore

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"capper/internal/csd"
)

type JournalStore struct {
	db *sql.DB
}

const journalCols = `id, volume_id, seq, client_id, session_id, operation, inode_id,
	payload_json, checksum, status, created_at, committed_at`

func (s *JournalStore) Append(e csd.JournalEntry) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("csd journal: marshal payload: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO csd_journal
			(id, volume_id, seq, client_id, session_id, operation, inode_id,
			 payload_json, checksum, status, created_at, committed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.VolumeID, e.Seq, e.ClientID, e.SessionID, e.Operation, e.InodeID,
		string(payload), e.Checksum, e.Status, e.CreatedAt, e.CommittedAt,
	)
	return err
}

func (s *JournalStore) Commit(volumeID string, seq int64, committedAt string) error {
	_, err := s.db.Exec(`
		UPDATE csd_journal SET status='committed', committed_at=?
		WHERE volume_id=? AND seq=?`, committedAt, volumeID, seq)
	return err
}

func (s *JournalStore) Pending(volumeID string) ([]csd.JournalEntry, error) {
	rows, err := s.db.Query(`
		SELECT `+journalCols+` FROM csd_journal
		WHERE volume_id=? AND status='pending' ORDER BY seq`, volumeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJournal(rows)
}

func (s *JournalStore) Since(volumeID string, afterSeq int64) ([]csd.JournalEntry, error) {
	rows, err := s.db.Query(`
		SELECT `+journalCols+` FROM csd_journal
		WHERE volume_id=? AND status='committed' AND seq > ? ORDER BY seq`, volumeID, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJournal(rows)
}

func (s *JournalStore) MaxSeq(volumeID string) (int64, error) {
	var seq sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(seq) FROM csd_journal WHERE volume_id=?`, volumeID).Scan(&seq)
	if err != nil {
		return 0, err
	}
	return seq.Int64, nil
}

func (s *JournalStore) NextSeq(volumeID string) (int64, error) {
	max, err := s.MaxSeq(volumeID)
	return max + 1, err
}

func (s *JournalStore) Truncate(volumeID string, keepAfterSeq int64) error {
	_, err := s.db.Exec(`
		DELETE FROM csd_journal WHERE volume_id=? AND status='committed' AND seq <= ?`,
		volumeID, keepAfterSeq)
	return err
}

// ---- helpers ----------------------------------------------------------------

func scanJournal(rows *sql.Rows) ([]csd.JournalEntry, error) {
	var out []csd.JournalEntry
	for rows.Next() {
		var e csd.JournalEntry
		var payloadJSON string
		if err := rows.Scan(
			&e.ID, &e.VolumeID, &e.Seq, &e.ClientID, &e.SessionID, &e.Operation, &e.InodeID,
			&payloadJSON, &e.Checksum, &e.Status, &e.CreatedAt, &e.CommittedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(payloadJSON), &e.Payload)
		out = append(out, e)
	}
	return out, rows.Err()
}
