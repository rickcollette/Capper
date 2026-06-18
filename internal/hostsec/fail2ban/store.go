package fail2ban

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Store persists the admin-managed persistent blocklist: (jail, ip) bans that
// Capper re-applies whenever fail2ban restarts or drops them.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over an open database.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the blocklist table. Safe to call repeatedly.
func (s *Store) InitSchema() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS fail2ban_blocklist (
		id         TEXT PRIMARY KEY,
		jail       TEXT NOT NULL,
		ip         TEXT NOT NULL,
		reason     TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		UNIQUE(jail, ip)
	)`)
	return err
}

// BlocklistEntry is one persistent ban.
type BlocklistEntry struct {
	ID        string `json:"id"`
	Jail      string `json:"jail"`
	IP        string `json:"ip"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// AddBlocklist records a persistent ban (idempotent on jail+ip).
func (s *Store) AddBlocklist(jail, ip, reason string) (BlocklistEntry, error) {
	e := BlocklistEntry{ID: "f2bbl_" + uuid.NewString(), Jail: jail, IP: ip, Reason: reason,
		CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	_, err := s.db.Exec(`INSERT INTO fail2ban_blocklist (id, jail, ip, reason, created_at)
		VALUES (?,?,?,?,?) ON CONFLICT(jail, ip) DO UPDATE SET reason=excluded.reason`,
		e.ID, e.Jail, e.IP, e.Reason, e.CreatedAt)
	if err != nil {
		return BlocklistEntry{}, err
	}
	return e, nil
}

// ListBlocklist returns all persistent bans.
func (s *Store) ListBlocklist() ([]BlocklistEntry, error) {
	rows, err := s.db.Query(`SELECT id, jail, ip, reason, created_at FROM fail2ban_blocklist ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BlocklistEntry
	for rows.Next() {
		var e BlocklistEntry
		if err := rows.Scan(&e.ID, &e.Jail, &e.IP, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RemoveBlocklist deletes a persistent ban by ID and returns the removed entry.
func (s *Store) RemoveBlocklist(id string) (BlocklistEntry, error) {
	var e BlocklistEntry
	err := s.db.QueryRow(`SELECT id, jail, ip, reason, created_at FROM fail2ban_blocklist WHERE id=?`, id).
		Scan(&e.ID, &e.Jail, &e.IP, &e.Reason, &e.CreatedAt)
	if err != nil {
		return BlocklistEntry{}, err
	}
	_, err = s.db.Exec(`DELETE FROM fail2ban_blocklist WHERE id=?`, id)
	return e, err
}
