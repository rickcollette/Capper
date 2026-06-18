package hoststorage

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Store persists storage pools and capacity allocations.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over an open database.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the host-storage tables. Safe to call repeatedly.
func (s *Store) InitSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS storage_pools (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL UNIQUE,
			backend     TEXT NOT NULL DEFAULT 'directory',
			mountpoint  TEXT NOT NULL DEFAULT '',
			device      TEXT NOT NULL DEFAULT '',
			vg_name     TEXT NOT NULL DEFAULT '',
			total_bytes INTEGER NOT NULL DEFAULT 0,
			health      TEXT NOT NULL DEFAULT 'healthy',
			created_at  TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS storage_allocations (
			id         TEXT PRIMARY KEY,
			pool_id    TEXT NOT NULL,
			owner      TEXT NOT NULL DEFAULT '',
			name       TEXT NOT NULL,
			path       TEXT NOT NULL DEFAULT '',
			device     TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	// Lightweight migrations for pre-existing tables (ignore "duplicate column").
	for _, alter := range []string{
		`ALTER TABLE storage_pools ADD COLUMN backend TEXT NOT NULL DEFAULT 'directory'`,
		`ALTER TABLE storage_pools ADD COLUMN vg_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE storage_pools ADD COLUMN health TEXT NOT NULL DEFAULT 'healthy'`,
		`ALTER TABLE storage_allocations ADD COLUMN device TEXT NOT NULL DEFAULT ''`,
	} {
		_, _ = s.db.Exec(alter)
	}
	return nil
}

func nowTS() string { return time.Now().UTC().Format(time.RFC3339) }

// ---- pools -----------------------------------------------------------------

const poolCols = `id, name, backend, mountpoint, device, vg_name, total_bytes, health, created_at`

// InsertPool stores a pool record.
func (s *Store) InsertPool(p StoragePool) (StoragePool, error) {
	if p.ID == "" {
		p.ID = "spool_" + uuid.NewString()
	}
	if p.CreatedAt == "" {
		p.CreatedAt = nowTS()
	}
	if p.Backend == "" {
		p.Backend = BackendDirectory
	}
	if p.Health == "" {
		p.Health = PoolHealthy
	}
	_, err := s.db.Exec(`INSERT INTO storage_pools (`+poolCols+`) VALUES (?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Backend, p.Mountpoint, p.Device, p.VGName, p.TotalBytes, p.Health, p.CreatedAt)
	if err != nil {
		return StoragePool{}, err
	}
	return p, nil
}

func scanPool(row interface{ Scan(...any) error }) (StoragePool, error) {
	var p StoragePool
	err := row.Scan(&p.ID, &p.Name, &p.Backend, &p.Mountpoint, &p.Device, &p.VGName, &p.TotalBytes, &p.Health, &p.CreatedAt)
	return p, err
}

// UpdatePoolHealth updates a pool's total capacity and health (used by the reconciler).
func (s *Store) UpdatePoolHealth(id string, totalBytes int64, health string) error {
	_, err := s.db.Exec(`UPDATE storage_pools SET total_bytes=?, health=? WHERE id=?`, totalBytes, health, id)
	return err
}

// GetPool returns a pool by ID or name.
func (s *Store) GetPool(idOrName string) (StoragePool, error) {
	return scanPool(s.db.QueryRow(`SELECT `+poolCols+` FROM storage_pools WHERE id=? OR name=?`, idOrName, idOrName))
}

// ListPools returns all pools.
func (s *Store) ListPools() ([]StoragePool, error) {
	rows, err := s.db.Query(`SELECT ` + poolCols + ` FROM storage_pools ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoragePool
	for rows.Next() {
		p, err := scanPool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePool removes a pool (caller must ensure it has no allocations).
func (s *Store) DeletePool(id string) error {
	_, err := s.db.Exec(`DELETE FROM storage_pools WHERE id=?`, id)
	return err
}

// ---- allocations -----------------------------------------------------------

const allocCols = `id, pool_id, owner, name, path, device, size_bytes, created_at`

// InsertAllocation stores an allocation record.
func (s *Store) InsertAllocation(a Allocation) (Allocation, error) {
	if a.ID == "" {
		a.ID = "salloc_" + uuid.NewString()
	}
	if a.CreatedAt == "" {
		a.CreatedAt = nowTS()
	}
	_, err := s.db.Exec(`INSERT INTO storage_allocations (`+allocCols+`) VALUES (?,?,?,?,?,?,?,?)`,
		a.ID, a.PoolID, a.Owner, a.Name, a.Path, a.Device, a.SizeBytes, a.CreatedAt)
	if err != nil {
		return Allocation{}, err
	}
	return a, nil
}

func scanAllocation(row interface{ Scan(...any) error }) (Allocation, error) {
	var a Allocation
	err := row.Scan(&a.ID, &a.PoolID, &a.Owner, &a.Name, &a.Path, &a.Device, &a.SizeBytes, &a.CreatedAt)
	return a, err
}

// GetAllocationByOwner returns the allocation owned by owner, or sql.ErrNoRows.
func (s *Store) GetAllocationByOwner(owner string) (Allocation, error) {
	return scanAllocation(s.db.QueryRow(`SELECT `+allocCols+` FROM storage_allocations WHERE owner=? LIMIT 1`, owner))
}

// GetAllocation returns an allocation by ID.
func (s *Store) GetAllocation(id string) (Allocation, error) {
	return scanAllocation(s.db.QueryRow(`SELECT `+allocCols+` FROM storage_allocations WHERE id=?`, id))
}

// ListAllocations returns allocations, optionally filtered to a pool.
func (s *Store) ListAllocations(poolID string) ([]Allocation, error) {
	q := `SELECT ` + allocCols + ` FROM storage_allocations`
	var args []any
	if poolID != "" {
		q += ` WHERE pool_id=?`
		args = append(args, poolID)
	}
	q += ` ORDER BY created_at`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Allocation
	for rows.Next() {
		a, err := scanAllocation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteAllocation removes an allocation record.
func (s *Store) DeleteAllocation(id string) error {
	_, err := s.db.Exec(`DELETE FROM storage_allocations WHERE id=?`, id)
	return err
}

// AllocatedBytes returns the sum of all allocation sizes for a pool.
func (s *Store) AllocatedBytes(poolID string) (int64, error) {
	var total int64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(size_bytes),0) FROM storage_allocations WHERE pool_id=?`, poolID).Scan(&total)
	return total, err
}

// CountAllocations returns how many allocations a pool has.
func (s *Store) CountAllocations(poolID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM storage_allocations WHERE pool_id=?`, poolID).Scan(&n)
	return n, err
}
