package database

import (
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS managed_databases (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			project     TEXT NOT NULL DEFAULT 'default',
			engine      TEXT NOT NULL,
			version     TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL,
			network_id  TEXT NOT NULL DEFAULT '',
			instance_id TEXT NOT NULL DEFAULT '',
			volume_id   TEXT NOT NULL DEFAULT '',
			secret_name TEXT NOT NULL DEFAULT '',
			dns_name    TEXT NOT NULL DEFAULT '',
			port        INTEGER NOT NULL DEFAULT 0,
			primary_id  TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
		`CREATE TABLE IF NOT EXISTS db_backups (
			id         TEXT PRIMARY KEY,
			db_id      TEXT NOT NULL,
			project    TEXT NOT NULL DEFAULT 'default',
			type       TEXT NOT NULL DEFAULT 'full',
			path       TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL DEFAULT 'pending',
			size_bytes INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	// Additive migrations.
	_, _ = db.Exec(`ALTER TABLE managed_databases ADD COLUMN primary_id TEXT NOT NULL DEFAULT ''`)
	return nil
}

func (s *Store) Insert(db ManagedDB) error {
	_, err := s.db.Exec(
		`INSERT INTO managed_databases
		 (id, name, project, engine, version, status, network_id, instance_id, volume_id, secret_name, dns_name, port, primary_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		db.ID, db.Name, db.Project, db.Engine, db.Version, db.Status,
		db.NetworkID, db.InstanceID, db.VolumeID, db.SecretName, db.DNSName, db.Port, db.PrimaryID, db.CreatedAt,
	)
	return err
}

func (s *Store) Get(nameOrID, project string) (ManagedDB, error) {
	var row *sql.Row
	if project == "" {
		row = s.db.QueryRow(
			`SELECT id, name, project, engine, version, status, network_id, instance_id, volume_id, secret_name, dns_name, port, primary_id, created_at
			 FROM managed_databases WHERE id=? OR name=? LIMIT 1`,
			nameOrID, nameOrID,
		)
	} else {
		row = s.db.QueryRow(
			`SELECT id, name, project, engine, version, status, network_id, instance_id, volume_id, secret_name, dns_name, port, primary_id, created_at
			 FROM managed_databases WHERE (id=? OR name=?) AND project=? LIMIT 1`,
			nameOrID, nameOrID, project,
		)
	}
	return scanDB(row)
}

func (s *Store) List(project string) ([]ManagedDB, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if project == "" {
		rows, err = s.db.Query(
			`SELECT id, name, project, engine, version, status, network_id, instance_id, volume_id, secret_name, dns_name, port, primary_id, created_at
			 FROM managed_databases ORDER BY name`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, project, engine, version, status, network_id, instance_id, volume_id, secret_name, dns_name, port, primary_id, created_at
			 FROM managed_databases WHERE project=? ORDER BY name`,
			project,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ManagedDB
	for rows.Next() {
		d, err := scanDB(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateInstanceID(id, instanceID string, status DBStatus) error {
	_, err := s.db.Exec(`UPDATE managed_databases SET instance_id=?, status=? WHERE id=?`, instanceID, status, id)
	return err
}

func (s *Store) UpdateStatus(id string, status DBStatus) error {
	_, err := s.db.Exec(`UPDATE managed_databases SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) ClearPrimaryID(id string) error {
	_, err := s.db.Exec(`UPDATE managed_databases SET primary_id='' WHERE id=?`, id)
	return err
}

func (s *Store) Delete(nameOrID, project string) error {
	var res sql.Result
	var err error
	if project == "" {
		res, err = s.db.Exec(`DELETE FROM managed_databases WHERE id=? OR name=?`, nameOrID, nameOrID)
	} else {
		res, err = s.db.Exec(`DELETE FROM managed_databases WHERE (id=? OR name=?) AND project=?`, nameOrID, nameOrID, project)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("managed database %q not found", nameOrID)
	}
	return nil
}

// ---- db backups -------------------------------------------------------------

func (s *Store) InsertBackup(b DBBackup) error {
	if b.ID == "" {
		b.ID = "dbk_" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if b.CreatedAt == "" {
		b.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO db_backups (id, db_id, project, type, path, status, size_bytes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.DBID, b.Project, b.Type, b.Path, b.Status, b.SizeBytes, b.CreatedAt,
	)
	return err
}

func (s *Store) UpdateBackupStatus(id, status string, sizeBytes int64) error {
	_, err := s.db.Exec(
		`UPDATE db_backups SET status=?, size_bytes=? WHERE id=?`,
		status, sizeBytes, id,
	)
	return err
}

func (s *Store) ListBackups(dbID string) ([]DBBackup, error) {
	var rows *sql.Rows
	var err error
	if dbID != "" {
		rows, err = s.db.Query(
			`SELECT id, db_id, project, type, path, status, size_bytes, created_at
			 FROM db_backups WHERE db_id=? ORDER BY created_at DESC`, dbID,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, db_id, project, type, path, status, size_bytes, created_at
			 FROM db_backups ORDER BY created_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DBBackup
	for rows.Next() {
		var b DBBackup
		if err := rows.Scan(&b.ID, &b.DBID, &b.Project, &b.Type, &b.Path, &b.Status, &b.SizeBytes, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBackup(id string) error {
	_, err := s.db.Exec(`DELETE FROM db_backups WHERE id=?`, id)
	return err
}

// ---- helpers ----------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDB(s rowScanner) (ManagedDB, error) {
	var d ManagedDB
	if err := s.Scan(
		&d.ID, &d.Name, &d.Project, &d.Engine, &d.Version, &d.Status,
		&d.NetworkID, &d.InstanceID, &d.VolumeID, &d.SecretName, &d.DNSName, &d.Port, &d.PrimaryID, &d.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return ManagedDB{}, fmt.Errorf("managed database not found")
		}
		return ManagedDB{}, fmt.Errorf("managed database: scan: %w", err)
	}
	return d, nil
}

func newID() string {
	return "db_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
