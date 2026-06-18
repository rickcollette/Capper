package resourcemon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Store persists the resource registry, config history, metrics, events,
// and alerts. Tables are prefixed rmon_ to avoid collisions with the existing
// internal/resource and internal/store event tables.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over an already-open database.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates all resourcemon tables and indexes. Safe to call repeatedly.
func (s *Store) InitSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS rmon_resources (
			id TEXT PRIMARY KEY,
			resource_type TEXT NOT NULL,
			name TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			account_id TEXT NOT NULL DEFAULT '',
			realm_id TEXT NOT NULL DEFAULT '',
			region_id TEXT NOT NULL DEFAULT '',
			zone_id TEXT NOT NULL DEFAULT '',
			node_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'unknown',
			health TEXT NOT NULL DEFAULT 'unknown',
			owner TEXT NOT NULL DEFAULT '',
			labels_json TEXT NOT NULL DEFAULT '{}',
			tags_json TEXT NOT NULL DEFAULT '{}',
			configuration_hash TEXT NOT NULL DEFAULT '',
			last_seen_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT NOT NULL DEFAULT '',
			UNIQUE(resource_type, project, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rmon_resources_type ON rmon_resources(resource_type, project)`,
		`CREATE TABLE IF NOT EXISTS rmon_resource_configs (
			id TEXT PRIMARY KEY,
			resource_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			desired_config_json TEXT NOT NULL DEFAULT '{}',
			observed_config_json TEXT NOT NULL DEFAULT '{}',
			last_applied_config_json TEXT NOT NULL DEFAULT '{}',
			config_hash TEXT NOT NULL DEFAULT '',
			drift_status TEXT NOT NULL DEFAULT 'unknown',
			drift_reason TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			UNIQUE(resource_id, version)
		)`,
		`CREATE TABLE IF NOT EXISTS rmon_metric_samples (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL DEFAULT '',
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			metric_name TEXT NOT NULL,
			value REAL NOT NULL,
			unit TEXT NOT NULL DEFAULT '',
			labels_json TEXT NOT NULL DEFAULT '{}',
			sampled_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rmon_metric_samples_lookup
			ON rmon_metric_samples(resource_type, resource_id, metric_name, sampled_at)`,
		`CREATE TABLE IF NOT EXISTS rmon_resource_events (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL DEFAULT '',
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			severity TEXT NOT NULL DEFAULT 'info',
			message TEXT NOT NULL DEFAULT '',
			details_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rmon_resource_events_lookup
			ON rmon_resource_events(resource_type, resource_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS rmon_alert_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			resource_type TEXT NOT NULL DEFAULT '',
			metric_name TEXT NOT NULL DEFAULT '',
			condition TEXT NOT NULL,
			threshold REAL NOT NULL DEFAULT 0,
			duration_seconds INTEGER NOT NULL DEFAULT 0,
			severity TEXT NOT NULL DEFAULT 'warning',
			enabled INTEGER NOT NULL DEFAULT 1,
			notification_target TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(project, name)
		)`,
		`CREATE TABLE IF NOT EXISTS rmon_alerts (
			id TEXT PRIMARY KEY,
			rule_id TEXT NOT NULL DEFAULT '',
			project TEXT NOT NULL DEFAULT '',
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			severity TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			opened_at TEXT NOT NULL,
			acknowledged_at TEXT NOT NULL DEFAULT '',
			resolved_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rmon_alerts_status ON rmon_alerts(status, project)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("resourcemon: schema: %w", err)
		}
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func mustJSON(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func parseJSON(s string) map[string]string {
	if s == "" {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]string{}
	}
	return m
}

// ---- resources -------------------------------------------------------------

// UpsertResource inserts or updates a resource by (resource_type, project, name).
func (s *Store) UpsertResource(r Resource) (Resource, error) {
	if r.ID == "" {
		r.ID = "res_" + uuid.NewString()
	}
	ts := now()
	if r.CreatedAt == "" {
		r.CreatedAt = ts
	}
	r.UpdatedAt = ts
	if r.Status == "" {
		r.Status = "unknown"
	}
	if r.Health == "" {
		r.Health = DeriveHealth(r.Status)
	}
	_, err := s.db.Exec(`
		INSERT INTO rmon_resources
			(id, resource_type, name, project, account_id, realm_id, region_id, zone_id, node_id,
			 status, health, owner, labels_json, tags_json, configuration_hash, last_seen_at,
			 created_at, updated_at, deleted_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'')
		ON CONFLICT(resource_type, project, name) DO UPDATE SET
			account_id=excluded.account_id, realm_id=excluded.realm_id,
			region_id=excluded.region_id, zone_id=excluded.zone_id, node_id=excluded.node_id,
			status=excluded.status, health=excluded.health, owner=excluded.owner,
			labels_json=excluded.labels_json, tags_json=excluded.tags_json,
			configuration_hash=excluded.configuration_hash, last_seen_at=excluded.last_seen_at,
			updated_at=excluded.updated_at, deleted_at=''`,
		r.ID, r.ResourceType, r.Name, r.Project, r.AccountID, r.RealmID, r.RegionID, r.ZoneID, r.NodeID,
		r.Status, r.Health, r.Owner, mustJSON(r.Labels), mustJSON(r.Tags), r.ConfigurationHash, r.LastSeenAt,
		r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return Resource{}, err
	}
	return s.GetResourceByKey(r.ResourceType, r.Project, r.Name)
}

func scanResource(row interface{ Scan(...any) error }) (Resource, error) {
	var r Resource
	var labels, tags string
	if err := row.Scan(&r.ID, &r.ResourceType, &r.Name, &r.Project, &r.AccountID,
		&r.RealmID, &r.RegionID, &r.ZoneID, &r.NodeID, &r.Status, &r.Health, &r.Owner,
		&labels, &tags, &r.ConfigurationHash, &r.LastSeenAt, &r.CreatedAt, &r.UpdatedAt, &r.DeletedAt); err != nil {
		return Resource{}, err
	}
	r.Labels = parseJSON(labels)
	r.Tags = parseJSON(tags)
	return r, nil
}

const resourceCols = `id, resource_type, name, project, account_id, realm_id, region_id, zone_id, node_id,
	status, health, owner, labels_json, tags_json, configuration_hash, last_seen_at, created_at, updated_at, deleted_at`

// GetResource returns a resource by ID.
func (s *Store) GetResource(id string) (Resource, error) {
	row := s.db.QueryRow(`SELECT `+resourceCols+` FROM rmon_resources WHERE id=?`, id)
	return scanResource(row)
}

// GetResourceByKey returns a resource by its natural key.
func (s *Store) GetResourceByKey(rtype, project, name string) (Resource, error) {
	row := s.db.QueryRow(`SELECT `+resourceCols+` FROM rmon_resources
		WHERE resource_type=? AND project=? AND name=?`, rtype, project, name)
	return scanResource(row)
}

// ListResources returns resources matching the filter (excludes soft-deleted).
func (s *Store) ListResources(f ResourceFilter) ([]Resource, error) {
	if f.Limit <= 0 {
		f.Limit = 500
	}
	conds := []string{"deleted_at=''"}
	args := []any{}
	add := func(col, val string) {
		if val != "" {
			conds = append(conds, col+"=?")
			args = append(args, val)
		}
	}
	add("project", f.Project)
	add("resource_type", f.ResourceType)
	add("status", f.Status)
	add("health", f.Health)
	add("region_id", f.RegionID)
	add("zone_id", f.ZoneID)
	add("node_id", f.NodeID)
	if f.Query != "" {
		conds = append(conds, "name LIKE ?")
		args = append(args, "%"+f.Query+"%")
	}
	args = append(args, f.Limit)
	rows, err := s.db.Query(`SELECT `+resourceCols+` FROM rmon_resources WHERE `+
		strings.Join(conds, " AND ")+` ORDER BY resource_type, name LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Resource
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkResourceDeleted soft-deletes a resource by ID.
func (s *Store) MarkResourceDeleted(id string) error {
	_, err := s.db.Exec(`UPDATE rmon_resources SET deleted_at=?, status='deleted', updated_at=? WHERE id=?`,
		now(), now(), id)
	return err
}

// ---- resource configs ------------------------------------------------------

// PutConfigVersion appends a new config version (auto-incrementing version).
func (s *Store) PutConfigVersion(c ResourceConfig) (ResourceConfig, error) {
	if c.ID == "" {
		c.ID = "cfg_" + uuid.NewString()
	}
	if c.CreatedAt == "" {
		c.CreatedAt = now()
	}
	var maxVer sql.NullInt64
	_ = s.db.QueryRow(`SELECT MAX(version) FROM rmon_resource_configs WHERE resource_id=?`, c.ResourceID).Scan(&maxVer)
	c.Version = int(maxVer.Int64) + 1
	if c.DriftStatus == "" {
		c.DriftStatus = DriftUnknown
	}
	_, err := s.db.Exec(`INSERT INTO rmon_resource_configs
		(id, resource_id, version, desired_config_json, observed_config_json, last_applied_config_json,
		 config_hash, drift_status, drift_reason, created_by, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.ResourceID, c.Version, jsonOrEmpty(c.DesiredConfigJSON), jsonOrEmpty(c.ObservedConfigJSON),
		jsonOrEmpty(c.LastAppliedJSON), c.ConfigHash, c.DriftStatus, c.DriftReason, c.CreatedBy, c.CreatedAt)
	if err != nil {
		return ResourceConfig{}, err
	}
	return c, nil
}

func jsonOrEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}

// LatestConfig returns the most recent config version for a resource.
func (s *Store) LatestConfig(resourceID string) (ResourceConfig, error) {
	row := s.db.QueryRow(`SELECT id, resource_id, version, desired_config_json, observed_config_json,
		last_applied_config_json, config_hash, drift_status, drift_reason, created_by, created_at
		FROM rmon_resource_configs WHERE resource_id=? ORDER BY version DESC LIMIT 1`, resourceID)
	return scanConfig(row)
}

// ConfigHistory returns all config versions for a resource, newest first.
func (s *Store) ConfigHistory(resourceID string, limit int) ([]ResourceConfig, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, resource_id, version, desired_config_json, observed_config_json,
		last_applied_config_json, config_hash, drift_status, drift_reason, created_by, created_at
		FROM rmon_resource_configs WHERE resource_id=? ORDER BY version DESC LIMIT ?`, resourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceConfig
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanConfig(row interface{ Scan(...any) error }) (ResourceConfig, error) {
	var c ResourceConfig
	err := row.Scan(&c.ID, &c.ResourceID, &c.Version, &c.DesiredConfigJSON, &c.ObservedConfigJSON,
		&c.LastAppliedJSON, &c.ConfigHash, &c.DriftStatus, &c.DriftReason, &c.CreatedBy, &c.CreatedAt)
	return c, err
}

// SetDriftStatus updates the drift fields on a config version.
func (s *Store) SetDriftStatus(configID, status, reason string) error {
	_, err := s.db.Exec(`UPDATE rmon_resource_configs SET drift_status=?, drift_reason=? WHERE id=?`,
		status, reason, configID)
	return err
}

// ---- metrics ---------------------------------------------------------------

// InsertSample records a metric sample.
func (s *Store) InsertSample(m MetricSample) error {
	if m.ID == "" {
		m.ID = "ms_" + uuid.NewString()
	}
	if m.SampledAt == "" {
		m.SampledAt = now()
	}
	_, err := s.db.Exec(`INSERT INTO rmon_metric_samples
		(id, project, resource_type, resource_id, metric_name, value, unit, labels_json, sampled_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Project, m.ResourceType, m.ResourceID, m.MetricName, m.Value, m.Unit,
		mustJSON(m.Labels), m.SampledAt)
	return err
}

// QuerySamples returns samples matching the query, oldest first (time series).
func (s *Store) QuerySamples(q MetricQuery) ([]MetricSample, error) {
	if q.Limit <= 0 {
		q.Limit = 1000
	}
	conds := []string{"resource_type=?", "resource_id=?", "metric_name=?"}
	args := []any{q.ResourceType, q.ResourceID, q.MetricName}
	if q.Since != "" {
		conds = append(conds, "sampled_at>=?")
		args = append(args, q.Since)
	}
	args = append(args, q.Limit)
	rows, err := s.db.Query(`SELECT id, project, resource_type, resource_id, metric_name, value, unit,
		labels_json, sampled_at FROM rmon_metric_samples WHERE `+strings.Join(conds, " AND ")+
		` ORDER BY sampled_at ASC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MetricSample
	for rows.Next() {
		var m MetricSample
		var labels string
		if err := rows.Scan(&m.ID, &m.Project, &m.ResourceType, &m.ResourceID, &m.MetricName,
			&m.Value, &m.Unit, &labels, &m.SampledAt); err != nil {
			return nil, err
		}
		m.Labels = parseJSON(labels)
		out = append(out, m)
	}
	return out, rows.Err()
}

// LatestSample returns the most recent sample for a resource/metric, or
// ok=false when none exist.
func (s *Store) LatestSample(resourceType, resourceID, metricName string) (MetricSample, bool) {
	var m MetricSample
	var labels string
	err := s.db.QueryRow(`SELECT id, project, resource_type, resource_id, metric_name, value, unit,
		labels_json, sampled_at FROM rmon_metric_samples
		WHERE resource_type=? AND resource_id=? AND metric_name=?
		ORDER BY sampled_at DESC LIMIT 1`, resourceType, resourceID, metricName).Scan(
		&m.ID, &m.Project, &m.ResourceType, &m.ResourceID, &m.MetricName, &m.Value, &m.Unit, &labels, &m.SampledAt)
	if err != nil {
		return MetricSample{}, false
	}
	m.Labels = parseJSON(labels)
	return m, true
}

// MetricNames returns distinct metric names for a resource.
func (s *Store) MetricNames(resourceType, resourceID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT metric_name FROM rmon_metric_samples
		WHERE resource_type=? AND resource_id=? ORDER BY metric_name`, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ---- resource events -------------------------------------------------------

// RecordEvent appends a resource event.
func (s *Store) RecordEvent(e ResourceEvent) error {
	if e.ID == "" {
		e.ID = "rev_" + uuid.NewString()
	}
	if e.CreatedAt == "" {
		e.CreatedAt = now()
	}
	if e.Severity == "" {
		e.Severity = "info"
	}
	_, err := s.db.Exec(`INSERT INTO rmon_resource_events
		(id, project, resource_type, resource_id, event_type, severity, message, details_json, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		e.ID, e.Project, e.ResourceType, e.ResourceID, e.EventType, e.Severity, e.Message,
		jsonOrEmpty(e.DetailsJSON), e.CreatedAt)
	return err
}

// ListEventsByResource returns events for a resource, newest first.
func (s *Store) ListEventsByResource(resourceType, resourceID string, limit int) ([]ResourceEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, project, resource_type, resource_id, event_type, severity,
		message, details_json, created_at FROM rmon_resource_events
		WHERE resource_type=? AND resource_id=? ORDER BY created_at DESC LIMIT ?`,
		resourceType, resourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// ListEvents returns events filtered by project and/or severity, newest first.
func (s *Store) ListEvents(project, severity string, limit int) ([]ResourceEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	conds := []string{"1=1"}
	args := []any{}
	if project != "" {
		conds = append(conds, "project=?")
		args = append(args, project)
	}
	if severity != "" {
		conds = append(conds, "severity=?")
		args = append(args, severity)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT id, project, resource_type, resource_id, event_type, severity,
		message, details_json, created_at FROM rmon_resource_events WHERE `+strings.Join(conds, " AND ")+
		` ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]ResourceEvent, error) {
	var out []ResourceEvent
	for rows.Next() {
		var e ResourceEvent
		if err := rows.Scan(&e.ID, &e.Project, &e.ResourceType, &e.ResourceID, &e.EventType,
			&e.Severity, &e.Message, &e.DetailsJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
