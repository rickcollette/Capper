package host

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Store handles host persistence.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by an already-initialised database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InitSchema creates the hosts and provision_images tables. Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS hosts (
			id             TEXT PRIMARY KEY,
			hostname       TEXT NOT NULL UNIQUE,
			roles          TEXT NOT NULL DEFAULT '[]',
			labels         TEXT NOT NULL DEFAULT '{}',
			os             TEXT NOT NULL DEFAULT '',
			arch           TEXT NOT NULL DEFAULT '',
			kernel_version TEXT NOT NULL DEFAULT '',
			cpu_count      INTEGER NOT NULL DEFAULT 0,
			memory_bytes   INTEGER NOT NULL DEFAULT 0,
			addresses      TEXT NOT NULL DEFAULT '[]',
			status         TEXT NOT NULL DEFAULT 'ready',
			registered_at  TEXT NOT NULL,
			last_seen      TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS provision_images (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL UNIQUE,
			version    TEXT NOT NULL DEFAULT '',
			path       TEXT NOT NULL DEFAULT '',
			checksum   TEXT NOT NULL DEFAULT '',
			arch       TEXT NOT NULL DEFAULT 'amd64',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS provision_jobs (
			id           TEXT PRIMARY KEY,
			host_id      TEXT NOT NULL,
			image_id     TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'pending',
			method       TEXT NOT NULL DEFAULT 'pxe',
			started_at   TEXT NOT NULL DEFAULT '',
			completed_at TEXT NOT NULL DEFAULT '',
			created_at   TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("host: schema: %w", err)
		}
	}
	return nil
}

// Upsert inserts or replaces a host record.
func (s *Store) Upsert(h Host) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if h.RegisteredAt == "" {
		h.RegisteredAt = now
	}
	h.LastSeen = now

	roles, _ := json.Marshal(h.Roles)
	labels, _ := json.Marshal(h.Labels)
	addrs, _ := json.Marshal(h.Addresses)

	_, err := s.db.Exec(`
		INSERT INTO hosts(id, hostname, roles, labels, os, arch, kernel_version,
		                  cpu_count, memory_bytes, addresses, status, registered_at, last_seen)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
		  hostname=excluded.hostname, roles=excluded.roles, labels=excluded.labels,
		  os=excluded.os, arch=excluded.arch, kernel_version=excluded.kernel_version,
		  cpu_count=excluded.cpu_count, memory_bytes=excluded.memory_bytes,
		  addresses=excluded.addresses, status=excluded.status, last_seen=excluded.last_seen`,
		h.ID, h.Hostname, string(roles), string(labels),
		h.OS, h.Arch, h.KernelVersion, h.CPUCount, h.MemoryBytes,
		string(addrs), h.Status, h.RegisteredAt, h.LastSeen,
	)
	return err
}

// UpdateSeen bumps last_seen for a host.
func (s *Store) UpdateSeen(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE hosts SET last_seen=? WHERE id=?`, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("host %q not found", id)
	}
	return nil
}

// UpdateStatus sets the status of a host.
func (s *Store) UpdateStatus(id, status string) error {
	res, err := s.db.Exec(`UPDATE hosts SET status=? WHERE id=?`, status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("host %q not found", id)
	}
	return nil
}

// SetLabels replaces all labels on a host.
func (s *Store) SetLabels(id string, labels map[string]string) error {
	b, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE hosts SET labels=? WHERE id=?`, string(b), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("host %q not found", id)
	}
	return nil
}

// Get returns a host by ID or hostname.
func (s *Store) Get(nameOrID string) (Host, error) {
	row := s.db.QueryRow(
		`SELECT id, hostname, roles, labels, os, arch, kernel_version,
		        cpu_count, memory_bytes, addresses, status, registered_at, last_seen
		 FROM hosts WHERE id=? OR hostname=? LIMIT 1`,
		nameOrID, nameOrID,
	)
	return scanHost(row)
}

// List returns all hosts.
func (s *Store) List() ([]Host, error) {
	rows, err := s.db.Query(
		`SELECT id, hostname, roles, labels, os, arch, kernel_version,
		        cpu_count, memory_bytes, addresses, status, registered_at, last_seen
		 FROM hosts ORDER BY registered_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ---- provision images -------------------------------------------------------

// ProvisionImage is a disk image used to bootstrap a physical host via PXE.
type ProvisionImage struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Checksum  string `json:"checksum"`
	Arch      string `json:"arch"`
	CreatedAt string `json:"createdAt"`
}

// ProvisionJob tracks the status of a host provisioning operation.
type ProvisionJob struct {
	ID          string `json:"id"`
	HostID      string `json:"hostId"`
	ImageID     string `json:"imageId"`
	Status      string `json:"status"`
	Method      string `json:"method"`
	StartedAt   string `json:"startedAt,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

func (s *Store) CreateProvisionImage(img ProvisionImage) (ProvisionImage, error) {
	if img.ID == "" {
		img.ID = "pimg_" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if img.CreatedAt == "" {
		img.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if img.Arch == "" {
		img.Arch = "amd64"
	}
	_, err := s.db.Exec(
		`INSERT INTO provision_images (id, name, version, path, checksum, arch, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		img.ID, img.Name, img.Version, img.Path, img.Checksum, img.Arch, img.CreatedAt,
	)
	return img, err
}

func (s *Store) GetProvisionImage(nameOrID string) (ProvisionImage, error) {
	var img ProvisionImage
	err := s.db.QueryRow(
		`SELECT id, name, version, path, checksum, arch, created_at FROM provision_images WHERE id=? OR name=?`,
		nameOrID, nameOrID,
	).Scan(&img.ID, &img.Name, &img.Version, &img.Path, &img.Checksum, &img.Arch, &img.CreatedAt)
	if err != nil {
		return img, fmt.Errorf("provision image %q not found", nameOrID)
	}
	return img, nil
}

func (s *Store) ListProvisionImages() ([]ProvisionImage, error) {
	rows, err := s.db.Query(
		`SELECT id, name, version, path, checksum, arch, created_at FROM provision_images ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProvisionImage
	for rows.Next() {
		var img ProvisionImage
		if err := rows.Scan(&img.ID, &img.Name, &img.Version, &img.Path, &img.Checksum, &img.Arch, &img.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, rows.Err()
}

func (s *Store) DeleteProvisionImage(nameOrID string) error {
	img, err := s.GetProvisionImage(nameOrID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM provision_images WHERE id=?`, img.ID)
	return err
}

// CreateProvisionJob starts a provisioning job for a host with the given image.
// method is "pxe" (default), "iso", or "netboot".
func (s *Store) CreateProvisionJob(hostID, imageID, method string) (ProvisionJob, error) {
	if method == "" {
		method = "pxe"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	job := ProvisionJob{
		ID:        "pjob_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		HostID:    hostID,
		ImageID:   imageID,
		Status:    "pending",
		Method:    method,
		CreatedAt: now,
	}
	_, err := s.db.Exec(
		`INSERT INTO provision_jobs (id, host_id, image_id, status, method, started_at, completed_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.HostID, job.ImageID, job.Status, job.Method, "", "", job.CreatedAt,
	)
	return job, err
}

func (s *Store) UpdateProvisionJob(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	switch status {
	case "running":
		_, err := s.db.Exec(`UPDATE provision_jobs SET status=?, started_at=? WHERE id=?`, status, now, id)
		return err
	case "complete", "failed":
		_, err := s.db.Exec(`UPDATE provision_jobs SET status=?, completed_at=? WHERE id=?`, status, now, id)
		return err
	default:
		_, err := s.db.Exec(`UPDATE provision_jobs SET status=? WHERE id=?`, status, id)
		return err
	}
}

func (s *Store) ListProvisionJobs(hostID string) ([]ProvisionJob, error) {
	var rows *sql.Rows
	var err error
	if hostID != "" {
		rows, err = s.db.Query(
			`SELECT id, host_id, image_id, status, method, started_at, completed_at, created_at
			 FROM provision_jobs WHERE host_id=? ORDER BY created_at DESC`, hostID,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, host_id, image_id, status, method, started_at, completed_at, created_at
			 FROM provision_jobs ORDER BY created_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProvisionJob
	for rows.Next() {
		var j ProvisionJob
		if err := rows.Scan(&j.ID, &j.HostID, &j.ImageID, &j.Status, &j.Method, &j.StartedAt, &j.CompletedAt, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ---- helpers ----------------------------------------------------------------

type rowScanner interface{ Scan(dest ...any) error }

func scanHost(s rowScanner) (Host, error) {
	var h Host
	var roles, labels, addrs string
	err := s.Scan(&h.ID, &h.Hostname, &roles, &labels,
		&h.OS, &h.Arch, &h.KernelVersion,
		&h.CPUCount, &h.MemoryBytes, &addrs,
		&h.Status, &h.RegisteredAt, &h.LastSeen)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Host{}, fmt.Errorf("host not found")
		}
		return Host{}, err
	}
	_ = json.Unmarshal([]byte(roles), &h.Roles)
	_ = json.Unmarshal([]byte(labels), &h.Labels)
	_ = json.Unmarshal([]byte(addrs), &h.Addresses)
	return h, nil
}
