package compute

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// InitSchema creates the compute tables. Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS compute_hosts (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL UNIQUE,
			address      TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'ready',
			labels_json  TEXT NOT NULL DEFAULT '{}',
			cpu_count    INTEGER NOT NULL DEFAULT 0,
			memory_bytes INTEGER NOT NULL DEFAULT 0,
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS compute_templates (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL UNIQUE,
			image         TEXT NOT NULL,
			runtime       TEXT NOT NULL DEFAULT '',
			document_json TEXT NOT NULL DEFAULT '{}',
			created_at    TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS compute_groups (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL UNIQUE,
			template_id  TEXT NOT NULL,
			min_size     INTEGER NOT NULL DEFAULT 0,
			desired_size INTEGER NOT NULL DEFAULT 1,
			max_size     INTEGER NOT NULL DEFAULT 1,
			status       TEXT NOT NULL DEFAULT 'active',
			created_at   TEXT NOT NULL,
			FOREIGN KEY(template_id) REFERENCES compute_templates(id)
		);`,
		`CREATE TABLE IF NOT EXISTS compute_snapshots (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL UNIQUE,
			instance_id TEXT NOT NULL,
			path        TEXT NOT NULL,
			digest      TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS compute_group_instances (
			group_id    TEXT NOT NULL,
			instance_id TEXT NOT NULL,
			created_at  TEXT NOT NULL,
			PRIMARY KEY (group_id, instance_id)
		);`,
		`CREATE TABLE IF NOT EXISTS compute_instance_types (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL UNIQUE,
			family       TEXT NOT NULL DEFAULT 'compute',
			cpu_count    INTEGER NOT NULL DEFAULT 1,
			memory_bytes INTEGER NOT NULL DEFAULT 0,
			disk_bytes   INTEGER NOT NULL DEFAULT 0,
			pid_limit    INTEGER NOT NULL DEFAULT 256,
			gpu_eligible INTEGER NOT NULL DEFAULT 0,
			gpu_count    INTEGER NOT NULL DEFAULT 0,
			locked       INTEGER NOT NULL DEFAULT 0,
			description  TEXT NOT NULL DEFAULT '',
			created_at   TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS compute_gpu_devices (
			id                   TEXT PRIMARY KEY,
			vendor               TEXT NOT NULL DEFAULT '',
			model                TEXT NOT NULL DEFAULT '',
			memory_bytes         INTEGER NOT NULL DEFAULT 0,
			status               TEXT NOT NULL DEFAULT 'available',
			device_path          TEXT NOT NULL DEFAULT '',
			assigned_instance_id TEXT NOT NULL DEFAULT '',
			created_at           TEXT NOT NULL,
			updated_at           TEXT NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("compute.InitSchema: %w", err)
		}
	}
	_, _ = db.Exec(`ALTER TABLE compute_instance_types ADD COLUMN disk_bytes INTEGER NOT NULL DEFAULT 0`)
	return nil
}

// Store provides CRUD for compute objects.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ---- hosts ------------------------------------------------------------------

func (s *Store) UpsertHost(h Host) error {
	labels, _ := json.Marshal(h.Labels)
	_, err := s.db.Exec(
		`INSERT INTO compute_hosts (id, name, address, status, labels_json, cpu_count, memory_bytes, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET
			address=excluded.address, status=excluded.status, labels_json=excluded.labels_json,
			cpu_count=excluded.cpu_count, memory_bytes=excluded.memory_bytes, updated_at=excluded.updated_at`,
		h.ID, h.Name, h.Address, h.Status, string(labels), h.CPUCount, h.MemoryBytes, h.CreatedAt, h.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("compute: upsert host: %w", err)
	}
	return nil
}

func (s *Store) GetHost(nameOrID string) (Host, error) {
	row := s.db.QueryRow(
		`SELECT id, name, address, status, labels_json, cpu_count, memory_bytes, created_at, updated_at
		FROM compute_hosts WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanHost(row)
}

func (s *Store) ListHosts() ([]Host, error) {
	rows, err := s.db.Query(
		`SELECT id, name, address, status, labels_json, cpu_count, memory_bytes, created_at, updated_at
		FROM compute_hosts ORDER BY name`)
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

func (s *Store) UpdateHostStatus(nameOrID, status, updatedAt string) error {
	_, err := s.db.Exec(
		`UPDATE compute_hosts SET status=?, updated_at=? WHERE id=? OR name=?`,
		status, updatedAt, nameOrID, nameOrID)
	return err
}

// ---- templates --------------------------------------------------------------

func (s *Store) InsertTemplate(t Template) error {
	doc, _ := json.Marshal(t.Doc)
	_, err := s.db.Exec(
		`INSERT INTO compute_templates (id, name, image, runtime, document_json, created_at)
		VALUES (?,?,?,?,?,?)`,
		t.ID, t.Name, t.Image, t.Runtime, string(doc), t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("compute: insert template: %w", err)
	}
	return nil
}

func (s *Store) GetTemplate(nameOrID string) (Template, error) {
	row := s.db.QueryRow(
		`SELECT id, name, image, runtime, document_json, created_at
		FROM compute_templates WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanTemplate(row)
}

func (s *Store) ListTemplates() ([]Template, error) {
	rows, err := s.db.Query(
		`SELECT id, name, image, runtime, document_json, created_at
		FROM compute_templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteTemplate(nameOrID string) error {
	res, err := s.db.Exec(`DELETE FROM compute_templates WHERE id=? OR name=?`, nameOrID, nameOrID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("compute: template %q not found", nameOrID)
	}
	return nil
}

// ---- groups -----------------------------------------------------------------

func (s *Store) InsertGroup(g Group) error {
	_, err := s.db.Exec(
		`INSERT INTO compute_groups (id, name, template_id, min_size, desired_size, max_size, status, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		g.ID, g.Name, g.TemplateID, g.MinSize, g.DesiredSize, g.MaxSize, g.Status, g.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("compute: insert group: %w", err)
	}
	return nil
}

func (s *Store) GetGroup(nameOrID string) (Group, error) {
	row := s.db.QueryRow(
		`SELECT g.id, g.name, g.template_id, t.name, g.min_size, g.desired_size, g.max_size, g.status, g.created_at
		FROM compute_groups g
		LEFT JOIN compute_templates t ON t.id = g.template_id
		WHERE g.id=? OR g.name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanGroup(row)
}

func (s *Store) ListGroups() ([]Group, error) {
	rows, err := s.db.Query(
		`SELECT g.id, g.name, g.template_id, t.name, g.min_size, g.desired_size, g.max_size, g.status, g.created_at
		FROM compute_groups g
		LEFT JOIN compute_templates t ON t.id = g.template_id
		ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) UpdateGroupDesired(nameOrID string, desired int) error {
	_, err := s.db.Exec(
		`UPDATE compute_groups SET desired_size=? WHERE id=? OR name=?`,
		desired, nameOrID, nameOrID)
	return err
}

func (s *Store) UpdateGroupStatus(nameOrID, status string) error {
	_, err := s.db.Exec(
		`UPDATE compute_groups SET status=? WHERE id=? OR name=?`,
		status, nameOrID, nameOrID)
	return err
}

func (s *Store) DeleteGroup(nameOrID string) error {
	g, err := s.GetGroup(nameOrID)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM compute_group_instances WHERE group_id=?`, g.ID); err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM compute_groups WHERE id=?`, g.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("compute: group %q not found", nameOrID)
	}
	return nil
}

// ---- group instances --------------------------------------------------------

func (s *Store) AddGroupInstance(groupID, instanceID, createdAt string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO compute_group_instances (group_id, instance_id, created_at)
		VALUES (?,?,?)`,
		groupID, instanceID, createdAt)
	return err
}

func (s *Store) RemoveGroupInstance(groupID, instanceID string) error {
	_, err := s.db.Exec(
		`DELETE FROM compute_group_instances WHERE group_id=? AND instance_id=?`,
		groupID, instanceID)
	return err
}

func (s *Store) ListGroupInstances(groupID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT instance_id FROM compute_group_instances WHERE group_id=? ORDER BY created_at`,
		groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ---- snapshots --------------------------------------------------------------

func (s *Store) InsertSnapshot(snap Snapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO compute_snapshots (id, name, instance_id, path, digest, created_at)
		VALUES (?,?,?,?,?,?)`,
		snap.ID, snap.Name, snap.InstanceID, snap.Path, snap.Digest, snap.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("compute: insert snapshot: %w", err)
	}
	return nil
}

func (s *Store) GetSnapshot(nameOrID string) (Snapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, name, instance_id, path, digest, created_at
		FROM compute_snapshots WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanSnapshot(row)
}

func (s *Store) ListSnapshots(instanceID string) ([]Snapshot, error) {
	var rows *sql.Rows
	var err error
	if instanceID != "" {
		rows, err = s.db.Query(
			`SELECT id, name, instance_id, path, digest, created_at
			FROM compute_snapshots WHERE instance_id=? ORDER BY created_at DESC`,
			instanceID)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, instance_id, path, digest, created_at
			FROM compute_snapshots ORDER BY created_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSnapshot(nameOrID string) error {
	res, err := s.db.Exec(`DELETE FROM compute_snapshots WHERE id=? OR name=?`, nameOrID, nameOrID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("compute: snapshot %q not found", nameOrID)
	}
	return nil
}

// ---- scanners ---------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHost(s rowScanner) (Host, error) {
	var h Host
	var labelsJSON string
	err := s.Scan(&h.ID, &h.Name, &h.Address, &h.Status, &labelsJSON, &h.CPUCount, &h.MemoryBytes, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return Host{}, fmt.Errorf("compute: scan host: %w", err)
	}
	json.Unmarshal([]byte(labelsJSON), &h.Labels)
	return h, nil
}

func scanTemplate(s rowScanner) (Template, error) {
	var t Template
	var docJSON string
	err := s.Scan(&t.ID, &t.Name, &t.Image, &t.Runtime, &docJSON, &t.CreatedAt)
	if err != nil {
		return Template{}, fmt.Errorf("compute: scan template: %w", err)
	}
	json.Unmarshal([]byte(docJSON), &t.Doc)
	return t, nil
}

func scanGroup(s rowScanner) (Group, error) {
	var g Group
	var templateName sql.NullString
	err := s.Scan(&g.ID, &g.Name, &g.TemplateID, &templateName, &g.MinSize, &g.DesiredSize, &g.MaxSize, &g.Status, &g.CreatedAt)
	if err != nil {
		return Group{}, fmt.Errorf("compute: scan group: %w", err)
	}
	if templateName.Valid {
		g.TemplateName = templateName.String
	}
	return g, nil
}

func scanSnapshot(s rowScanner) (Snapshot, error) {
	var snap Snapshot
	err := s.Scan(&snap.ID, &snap.Name, &snap.InstanceID, &snap.Path, &snap.Digest, &snap.CreatedAt)
	if err != nil {
		return Snapshot{}, fmt.Errorf("compute: scan snapshot: %w", err)
	}
	return snap, nil
}

// ---- helpers ----------------------------------------------------------------

// TemplateInUse reports whether any group references this template ID.
func (s *Store) TemplateInUse(templateID string) (bool, error) {
	var n int
	row := s.db.QueryRow(`SELECT COUNT(*) FROM compute_groups WHERE template_id=?`, templateID)
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// ---- instance types ---------------------------------------------------------

func (s *Store) UpsertInstanceType(it InstanceType) error {
	_, err := s.db.Exec(
		`INSERT INTO compute_instance_types
			(id, name, family, cpu_count, memory_bytes, disk_bytes, pid_limit, gpu_eligible, gpu_count, locked, description, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET
			family=excluded.family,
			cpu_count=excluded.cpu_count,
			memory_bytes=excluded.memory_bytes,
			disk_bytes=excluded.disk_bytes,
			pid_limit=excluded.pid_limit,
			gpu_eligible=excluded.gpu_eligible,
			gpu_count=excluded.gpu_count,
			description=excluded.description`,
		it.ID, it.Name, it.Family, it.CPUCount, it.MemoryBytes, it.DiskBytes, it.PIDLimit,
		boolInt(it.GPUEligible), it.GPUCount, boolInt(it.Locked), it.Description, it.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("compute: upsert instance type: %w", err)
	}
	return nil
}

func (s *Store) GetInstanceType(nameOrID string) (InstanceType, error) {
	row := s.db.QueryRow(
		`SELECT id, name, family, cpu_count, memory_bytes, disk_bytes, pid_limit, gpu_eligible, gpu_count, locked, description, created_at
		FROM compute_instance_types WHERE id=? OR name=? LIMIT 1`,
		nameOrID, nameOrID)
	return scanInstanceType(row)
}

func (s *Store) ListInstanceTypes() ([]InstanceType, error) {
	rows, err := s.db.Query(
		`SELECT id, name, family, cpu_count, memory_bytes, disk_bytes, pid_limit, gpu_eligible, gpu_count, locked, description, created_at
		FROM compute_instance_types ORDER BY family, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InstanceType
	for rows.Next() {
		it, err := scanInstanceType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) DeleteInstanceType(nameOrID string) error {
	res, err := s.db.Exec(`DELETE FROM compute_instance_types WHERE (id=? OR name=?) AND locked=0`, nameOrID, nameOrID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Check if it exists but is locked
		var locked int
		_ = s.db.QueryRow(`SELECT locked FROM compute_instance_types WHERE id=? OR name=?`, nameOrID, nameOrID).Scan(&locked)
		if locked == 1 {
			return fmt.Errorf("compute: instance type %q is locked and cannot be deleted", nameOrID)
		}
		return fmt.Errorf("compute: instance type %q not found", nameOrID)
	}
	return nil
}

func (s *Store) DeprecateInstanceType(nameOrID string) (InstanceType, error) {
	it, err := s.GetInstanceType(nameOrID)
	if err != nil {
		return InstanceType{}, err
	}
	if !strings.Contains(it.Description, "[deprecated]") {
		if it.Description != "" {
			it.Description += " "
		}
		it.Description += "[deprecated]"
	}
	it.Locked = true
	if err := s.UpsertInstanceType(it); err != nil {
		return InstanceType{}, err
	}
	return it, nil
}

func scanInstanceType(s rowScanner) (InstanceType, error) {
	var it InstanceType
	var gpuEligible, locked int
	err := s.Scan(&it.ID, &it.Name, &it.Family, &it.CPUCount, &it.MemoryBytes, &it.DiskBytes, &it.PIDLimit,
		&gpuEligible, &it.GPUCount, &locked, &it.Description, &it.CreatedAt)
	if err != nil {
		return InstanceType{}, fmt.Errorf("compute: scan instance type: %w", err)
	}
	it.GPUEligible = gpuEligible != 0
	it.Locked = locked != 0
	return it, nil
}

// ---- GPU devices ------------------------------------------------------------

func (s *Store) UpsertGPUDevice(g GPUDevice) error {
	_, err := s.db.Exec(
		`INSERT INTO compute_gpu_devices
			(id, vendor, model, memory_bytes, status, device_path, assigned_instance_id, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			vendor=excluded.vendor,
			model=excluded.model,
			memory_bytes=excluded.memory_bytes,
			status=excluded.status,
			device_path=excluded.device_path,
			assigned_instance_id=excluded.assigned_instance_id,
			updated_at=excluded.updated_at`,
		g.ID, g.Vendor, g.Model, g.MemoryBytes, g.Status, g.DevicePath,
		g.AssignedInstanceID, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("compute: upsert gpu device: %w", err)
	}
	return nil
}

func (s *Store) GetGPUDevice(id string) (GPUDevice, error) {
	row := s.db.QueryRow(
		`SELECT id, vendor, model, memory_bytes, status, device_path, assigned_instance_id, created_at, updated_at
		FROM compute_gpu_devices WHERE id=? LIMIT 1`, id)
	return scanGPUDevice(row)
}

func (s *Store) ListGPUDevices() ([]GPUDevice, error) {
	rows, err := s.db.Query(
		`SELECT id, vendor, model, memory_bytes, status, device_path, assigned_instance_id, created_at, updated_at
		FROM compute_gpu_devices ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GPUDevice
	for rows.Next() {
		g, err := scanGPUDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) UpdateGPUStatus(id, status, assignedInstanceID, updatedAt string) error {
	res, err := s.db.Exec(
		`UPDATE compute_gpu_devices SET status=?, assigned_instance_id=?, updated_at=? WHERE id=?`,
		status, assignedInstanceID, updatedAt, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("compute: GPU device %q not found", id)
	}
	return nil
}

func (s *Store) DeleteGPUDevice(id string) error {
	res, err := s.db.Exec(`DELETE FROM compute_gpu_devices WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("compute: GPU device %q not found", id)
	}
	return nil
}

func scanGPUDevice(s rowScanner) (GPUDevice, error) {
	var g GPUDevice
	err := s.Scan(&g.ID, &g.Vendor, &g.Model, &g.MemoryBytes, &g.Status,
		&g.DevicePath, &g.AssignedInstanceID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return GPUDevice{}, fmt.Errorf("compute: scan gpu device: %w", err)
	}
	return g, nil
}

// ---- helpers ----------------------------------------------------------------

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// groupNotFoundError is a sentinel returned when scanGroup fails due to no rows.
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "no rows")
}
