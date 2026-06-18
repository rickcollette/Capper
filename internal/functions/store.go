package functions

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Store persists functions, versions, triggers, and invocations.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over an open database.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the function tables. Safe to call repeatedly.
func (s *Store) InitSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS functions (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			name TEXT NOT NULL,
			runtime TEXT NOT NULL,
			handler TEXT NOT NULL DEFAULT '',
			image TEXT NOT NULL DEFAULT '',
			command_json TEXT NOT NULL DEFAULT '[]',
			package_id TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '1',
			status TEXT NOT NULL DEFAULT 'created',
			memory_bytes INTEGER NOT NULL DEFAULT 268435456,
			cpu_units INTEGER NOT NULL DEFAULT 250,
			timeout_ms INTEGER NOT NULL DEFAULT 30000,
			concurrency INTEGER NOT NULL DEFAULT 10,
			min_scale INTEGER NOT NULL DEFAULT 0,
			max_scale INTEGER NOT NULL DEFAULT 10,
			isolation TEXT NOT NULL DEFAULT 'container',
			env_json TEXT NOT NULL DEFAULT '{}',
			labels_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(project, name)
		)`,
		`CREATE TABLE IF NOT EXISTS function_versions (
			id TEXT PRIMARY KEY,
			function_id TEXT NOT NULL,
			version TEXT NOT NULL,
			package_id TEXT NOT NULL DEFAULT '',
			image TEXT NOT NULL DEFAULT '',
			config_json TEXT NOT NULL DEFAULT '{}',
			digest TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			created_at TEXT NOT NULL,
			UNIQUE(function_id, version)
		)`,
		`CREATE TABLE IF NOT EXISTS function_triggers (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			function_id TEXT NOT NULL,
			type TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			pattern_json TEXT NOT NULL DEFAULT '{}',
			retry_policy_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			labels_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS function_invocations (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			function_id TEXT NOT NULL,
			function_version TEXT NOT NULL DEFAULT '',
			trigger_id TEXT NOT NULL DEFAULT '',
			request_id TEXT NOT NULL DEFAULT '',
			principal TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			started_at TEXT NOT NULL DEFAULT '',
			ended_at TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			result_ref TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_function_invocations_fn
			ON function_invocations(function_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_function_triggers_fn
			ON function_triggers(function_id)`,
		`CREATE INDEX IF NOT EXISTS idx_function_triggers_source
			ON function_triggers(type, source)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("functions: schema: %w", err)
		}
	}
	return nil
}

func nowTS() string { return time.Now().UTC().Format(time.RFC3339) }

func encJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func decMap(s string) map[string]string {
	m := map[string]string{}
	if s != "" {
		_ = json.Unmarshal([]byte(s), &m)
	}
	return m
}

func decStrSlice(s string) []string {
	var out []string
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

// ---- functions -------------------------------------------------------------

// CreateFunction inserts a new function with defaults applied.
func (s *Store) CreateFunction(f Function) (Function, error) {
	if f.ID == "" {
		f.ID = "fn_" + uuid.NewString()
	}
	ts := nowTS()
	f.CreatedAt, f.UpdatedAt = ts, ts
	if f.Version == "" {
		f.Version = "1"
	}
	if f.Status == "" {
		f.Status = StatusCreated
	}
	if f.MemoryBytes == 0 {
		f.MemoryBytes = DefaultMemoryBytes
	}
	if f.CPUUnits == 0 {
		f.CPUUnits = DefaultCPUUnits
	}
	if f.TimeoutMS == 0 {
		f.TimeoutMS = DefaultTimeoutMS
	}
	if f.Concurrency == 0 {
		f.Concurrency = DefaultConcurrency
	}
	if f.MaxScale == 0 {
		f.MaxScale = DefaultMaxScale
	}
	if f.Isolation == "" {
		f.Isolation = "container"
	}
	_, err := s.db.Exec(`INSERT INTO functions
		(id, project, name, runtime, handler, image, command_json, package_id, version, status,
		 memory_bytes, cpu_units, timeout_ms, concurrency, min_scale, max_scale, isolation,
		 env_json, labels_json, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.ID, f.Project, f.Name, f.Runtime, f.Handler, f.Image, encJSON(f.Command), f.PackageID,
		f.Version, f.Status, f.MemoryBytes, f.CPUUnits, f.TimeoutMS, f.Concurrency, f.MinScale,
		f.MaxScale, f.Isolation, encJSON(f.Env), encJSON(f.Labels), f.CreatedAt, f.UpdatedAt)
	if err != nil {
		return Function{}, err
	}
	return f, nil
}

const fnCols = `id, project, name, runtime, handler, image, command_json, package_id, version, status,
	memory_bytes, cpu_units, timeout_ms, concurrency, min_scale, max_scale, isolation, env_json,
	labels_json, created_at, updated_at`

func scanFunction(row interface{ Scan(...any) error }) (Function, error) {
	var f Function
	var cmd, env, labels string
	if err := row.Scan(&f.ID, &f.Project, &f.Name, &f.Runtime, &f.Handler, &f.Image, &cmd,
		&f.PackageID, &f.Version, &f.Status, &f.MemoryBytes, &f.CPUUnits, &f.TimeoutMS, &f.Concurrency,
		&f.MinScale, &f.MaxScale, &f.Isolation, &env, &labels, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return Function{}, err
	}
	f.Command = decStrSlice(cmd)
	f.Env = decMap(env)
	f.Labels = decMap(labels)
	return f, nil
}

// GetFunction returns a function by ID.
func (s *Store) GetFunction(id string) (Function, error) {
	return scanFunction(s.db.QueryRow(`SELECT `+fnCols+` FROM functions WHERE id=?`, id))
}

// GetFunctionByName returns a function by project + name.
func (s *Store) GetFunctionByName(project, name string) (Function, error) {
	return scanFunction(s.db.QueryRow(`SELECT `+fnCols+` FROM functions WHERE project=? AND name=?`, project, name))
}

// ListFunctions returns all functions, optionally scoped to a project.
func (s *Store) ListFunctions(project string) ([]Function, error) {
	q := `SELECT ` + fnCols + ` FROM functions`
	var args []any
	if project != "" {
		q += " WHERE project=?"
		args = append(args, project)
	}
	q += " ORDER BY name"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Function
	for rows.Next() {
		f, err := scanFunction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// UpdateFunction applies whitelisted field changes.
func (s *Store) UpdateFunction(id string, fields map[string]any) error {
	allowed := map[string]bool{
		"runtime": true, "handler": true, "image": true, "status": true, "memory_bytes": true,
		"cpu_units": true, "timeout_ms": true, "concurrency": true, "min_scale": true,
		"max_scale": true, "isolation": true,
	}
	var sets []string
	var args []any
	for k, v := range fields {
		if allowed[k] {
			sets = append(sets, k+"=?")
			args = append(args, v)
		}
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at=?")
	args = append(args, nowTS(), id)
	_, err := s.db.Exec(`UPDATE functions SET `+strings.Join(sets, ", ")+` WHERE id=?`, args...)
	return err
}

// SetFunctionStatus updates a function's status.
func (s *Store) SetFunctionStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE functions SET status=?, updated_at=? WHERE id=?`, status, nowTS(), id)
	return err
}

// DeleteFunction removes a function and its versions/triggers/invocations.
func (s *Store) DeleteFunction(id string) error {
	for _, q := range []string{
		`DELETE FROM function_versions WHERE function_id=?`,
		`DELETE FROM function_triggers WHERE function_id=?`,
		`DELETE FROM function_invocations WHERE function_id=?`,
		`DELETE FROM functions WHERE id=?`,
	} {
		if _, err := s.db.Exec(q, id); err != nil {
			return err
		}
	}
	return nil
}

// ---- versions --------------------------------------------------------------

// AddVersion publishes a new version (auto-incrementing numeric version).
func (s *Store) AddVersion(v FunctionVersion) (FunctionVersion, error) {
	if v.ID == "" {
		v.ID = "fnv_" + uuid.NewString()
	}
	if v.CreatedAt == "" {
		v.CreatedAt = nowTS()
	}
	if v.Status == "" {
		v.Status = "active"
	}
	if v.Version == "" {
		var maxVer sql.NullInt64
		_ = s.db.QueryRow(`SELECT MAX(CAST(version AS INTEGER)) FROM function_versions WHERE function_id=?`,
			v.FunctionID).Scan(&maxVer)
		v.Version = fmt.Sprintf("%d", maxVer.Int64+1)
	}
	_, err := s.db.Exec(`INSERT INTO function_versions
		(id, function_id, version, package_id, image, config_json, digest, status, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		v.ID, v.FunctionID, v.Version, v.PackageID, v.Image, jsonOr(v.ConfigJSON), v.Digest, v.Status, v.CreatedAt)
	if err != nil {
		return FunctionVersion{}, err
	}
	return v, nil
}

func jsonOr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}

// ListVersions returns versions for a function, newest first.
func (s *Store) ListVersions(functionID string) ([]FunctionVersion, error) {
	rows, err := s.db.Query(`SELECT id, function_id, version, package_id, image, config_json, digest,
		status, created_at FROM function_versions WHERE function_id=?
		ORDER BY CAST(version AS INTEGER) DESC`, functionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FunctionVersion
	for rows.Next() {
		var v FunctionVersion
		if err := rows.Scan(&v.ID, &v.FunctionID, &v.Version, &v.PackageID, &v.Image, &v.ConfigJSON,
			&v.Digest, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ---- triggers --------------------------------------------------------------

// AddTrigger binds an event source to a function.
func (s *Store) AddTrigger(t Trigger) (Trigger, error) {
	if t.ID == "" {
		t.ID = "fntr_" + uuid.NewString()
	}
	ts := nowTS()
	t.CreatedAt, t.UpdatedAt = ts, ts
	enabled := 0
	if t.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(`INSERT INTO function_triggers
		(id, project, function_id, type, source, pattern_json, retry_policy_json, enabled, labels_json, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Project, t.FunctionID, t.Type, t.Source, jsonOr(t.PatternJSON), jsonOr(t.RetryPolicyJSON),
		enabled, encJSON(t.Labels), t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return Trigger{}, err
	}
	return t, nil
}

func scanTrigger(row interface{ Scan(...any) error }) (Trigger, error) {
	var t Trigger
	var labels string
	var enabled int
	if err := row.Scan(&t.ID, &t.Project, &t.FunctionID, &t.Type, &t.Source, &t.PatternJSON,
		&t.RetryPolicyJSON, &enabled, &labels, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return Trigger{}, err
	}
	t.Enabled = enabled == 1
	t.Labels = decMap(labels)
	return t, nil
}

const trigCols = `id, project, function_id, type, source, pattern_json, retry_policy_json, enabled,
	labels_json, created_at, updated_at`

// ListTriggers returns triggers for a function.
func (s *Store) ListTriggers(functionID string) ([]Trigger, error) {
	rows, err := s.db.Query(`SELECT `+trigCols+` FROM function_triggers WHERE function_id=?`, functionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Trigger
	for rows.Next() {
		t, err := scanTrigger(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TriggersBySource returns enabled triggers matching a type + source (used by
// the event router to fan an event out to its bound functions).
func (s *Store) TriggersBySource(triggerType, source string) ([]Trigger, error) {
	rows, err := s.db.Query(`SELECT `+trigCols+` FROM function_triggers
		WHERE type=? AND source=? AND enabled=1`, triggerType, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Trigger
	for rows.Next() {
		t, err := scanTrigger(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteTrigger removes a trigger.
func (s *Store) DeleteTrigger(id string) error {
	_, err := s.db.Exec(`DELETE FROM function_triggers WHERE id=?`, id)
	return err
}

// ---- invocations -----------------------------------------------------------

// StartInvocation records a pending/running invocation and returns it.
func (s *Store) StartInvocation(inv Invocation) (Invocation, error) {
	if inv.ID == "" {
		inv.ID = "fninv_" + uuid.NewString()
	}
	ts := nowTS()
	inv.CreatedAt = ts
	if inv.StartedAt == "" {
		inv.StartedAt = ts
	}
	if inv.Status == "" {
		inv.Status = InvocationRunning
	}
	_, err := s.db.Exec(`INSERT INTO function_invocations
		(id, project, function_id, function_version, trigger_id, request_id, principal, source,
		 status, started_at, ended_at, duration_ms, error, result_ref, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		inv.ID, inv.Project, inv.FunctionID, inv.FunctionVersion, inv.TriggerID, inv.RequestID,
		inv.Principal, inv.Source, inv.Status, inv.StartedAt, inv.EndedAt, inv.DurationMS,
		inv.Error, inv.Result, inv.CreatedAt)
	if err != nil {
		return Invocation{}, err
	}
	return inv, nil
}

// FinishInvocation updates an invocation's terminal status.
func (s *Store) FinishInvocation(id, status string, durationMS int64, errMsg, result string) error {
	_, err := s.db.Exec(`UPDATE function_invocations
		SET status=?, ended_at=?, duration_ms=?, error=?, result_ref=? WHERE id=?`,
		status, nowTS(), durationMS, errMsg, result, id)
	return err
}

// ListInvocations returns invocations for a function, newest first.
func (s *Store) ListInvocations(functionID string, limit int) ([]Invocation, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, project, function_id, function_version, trigger_id, request_id,
		principal, source, status, started_at, ended_at, duration_ms, error, result_ref, created_at
		FROM function_invocations WHERE function_id=? ORDER BY created_at DESC LIMIT ?`, functionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invocation
	for rows.Next() {
		var inv Invocation
		if err := rows.Scan(&inv.ID, &inv.Project, &inv.FunctionID, &inv.FunctionVersion, &inv.TriggerID,
			&inv.RequestID, &inv.Principal, &inv.Source, &inv.Status, &inv.StartedAt, &inv.EndedAt,
			&inv.DurationMS, &inv.Error, &inv.Result, &inv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}
