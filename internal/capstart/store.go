package capstart

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// InitSchema creates the CapStart tables using the SQLite/CapDB dialect used by
// the rest of Capper's control-plane store.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS capstart_recipes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL DEFAULT '',
			tags_json TEXT NOT NULL DEFAULT '[]',
			schema_json TEXT NOT NULL DEFAULT '{}',
			content_json TEXT NOT NULL DEFAULT '{}',
			checksum TEXT NOT NULL DEFAULT '',
			is_builtin INTEGER NOT NULL DEFAULT 0,
			is_community INTEGER NOT NULL DEFAULT 0,
			author_id TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT,
			UNIQUE(name, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_recipes_name ON capstart_recipes(name)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_recipes_category ON capstart_recipes(category)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_recipes_builtin ON capstart_recipes(is_builtin)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_recipes_created ON capstart_recipes(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS capstart_recipe_executions (
			id TEXT PRIMARY KEY,
			recipe_id TEXT NOT NULL,
			vm_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			config_json TEXT NOT NULL DEFAULT '{}',
			started_at TEXT,
			completed_at TEXT,
			error_message TEXT,
			logs TEXT,
			metadata_json TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(recipe_id) REFERENCES capstart_recipes(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_exec_recipe ON capstart_recipe_executions(recipe_id)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_exec_vm ON capstart_recipe_executions(vm_id)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_exec_status ON capstart_recipe_executions(status)`,
		`CREATE TABLE IF NOT EXISTS capstart_isos (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			version TEXT NOT NULL DEFAULT '',
			os_type TEXT NOT NULL DEFAULT '',
			architecture TEXT NOT NULL DEFAULT '',
			file_size INTEGER NOT NULL DEFAULT 0,
			checksum TEXT NOT NULL DEFAULT '',
			checksum_type TEXT NOT NULL DEFAULT '',
			storage_path TEXT NOT NULL DEFAULT '',
			is_verified INTEGER NOT NULL DEFAULT 0,
			url TEXT,
			metadata_json TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_isos_name ON capstart_isos(name)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_isos_os ON capstart_isos(os_type)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_isos_verified ON capstart_isos(is_verified)`,
		`CREATE TABLE IF NOT EXISTS capstart_installation_jobs (
			id TEXT PRIMARY KEY,
			iso_id TEXT NOT NULL,
			vm_id TEXT NOT NULL,
			status TEXT NOT NULL,
			booted_at TEXT,
			started_at TEXT,
			completed_at TEXT,
			error_message TEXT,
			installer_logs TEXT,
			timeout INTEGER NOT NULL DEFAULT 3600,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(iso_id) REFERENCES capstart_isos(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_jobs_iso ON capstart_installation_jobs(iso_id)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_jobs_vm ON capstart_installation_jobs(vm_id)`,
		`CREATE INDEX IF NOT EXISTS idx_capstart_jobs_status ON capstart_installation_jobs(status)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("capstart schema: %w", err)
		}
	}
	return nil
}

// RecipeStore handles recipe persistence.
type RecipeStore struct {
	db *sql.DB
}

func NewRecipeStore(db *sql.DB) *RecipeStore {
	return &RecipeStore{db: db}
}

func (rs *RecipeStore) CreateRecipe(recipe *Recipe) error {
	if recipe.ID == "" {
		recipe.ID = uuid.New().String()
	}
	setRecipeDefaults(recipe)
	if recipe.Checksum == "" {
		hash := sha256.Sum256(recipe.Content)
		recipe.Checksum = hex.EncodeToString(hash[:])
	}
	tags, err := json.Marshal(recipe.Tags)
	if err != nil {
		return err
	}
	_, err = rs.db.Exec(`
		INSERT INTO capstart_recipes
			(id, name, version, title, description, category, author, tags_json, schema_json, content_json, checksum, is_builtin, is_community, author_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		recipe.ID, recipe.Name, recipe.Version, recipe.Title, recipe.Description,
		recipe.Category, recipe.Author, string(tags), rawOrObject(recipe.Schema),
		rawOrObject(recipe.Content), recipe.Checksum, boolInt(recipe.IsBuiltin),
		boolInt(recipe.IsCommunity), recipe.AuthorID, fmtTime(recipe.CreatedAt), fmtTime(recipe.UpdatedAt))
	return err
}

func (rs *RecipeStore) GetRecipe(id string) (*Recipe, error) {
	row := rs.db.QueryRow(`
		SELECT id, name, version, title, description, category, author, tags_json, schema_json, content_json, checksum, is_builtin, is_community, author_id, created_at, updated_at
		FROM capstart_recipes
		WHERE id = ? AND deleted_at IS NULL`, id)
	return scanRecipe(row)
}

func (rs *RecipeStore) GetRecipeByName(name, version string) (*Recipe, error) {
	row := rs.db.QueryRow(`
		SELECT id, name, version, title, description, category, author, tags_json, schema_json, content_json, checksum, is_builtin, is_community, author_id, created_at, updated_at
		FROM capstart_recipes
		WHERE name = ? AND version = ? AND deleted_at IS NULL`, name, version)
	return scanRecipe(row)
}

func (rs *RecipeStore) ListRecipes(category string, isBuiltin *bool, offset, limit int) ([]*Recipe, error) {
	query := `
		SELECT id, name, version, title, description, category, author, tags_json, schema_json, content_json, checksum, is_builtin, is_community, author_id, created_at, updated_at
		FROM capstart_recipes
		WHERE deleted_at IS NULL`
	args := []any{}
	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}
	if isBuiltin != nil {
		query += " AND is_builtin = ?"
		args = append(args, boolInt(*isBuiltin))
	}
	query += " ORDER BY is_builtin DESC, created_at DESC"
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}
	rows, err := rs.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Recipe
	for rows.Next() {
		recipe, err := scanRecipe(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, recipe)
	}
	return out, rows.Err()
}

func (rs *RecipeStore) UpdateRecipe(recipe *Recipe) error {
	recipe.UpdatedAt = time.Now().UTC()
	tags, err := json.Marshal(recipe.Tags)
	if err != nil {
		return err
	}
	result, err := rs.db.Exec(`
		UPDATE capstart_recipes
		SET title = ?, description = ?, category = ?, tags_json = ?, schema_json = ?, content_json = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`,
		recipe.Title, recipe.Description, recipe.Category, string(tags), rawOrObject(recipe.Schema),
		rawOrObject(recipe.Content), fmtTime(recipe.UpdatedAt), recipe.ID)
	if err != nil {
		return err
	}
	return requireRows(result, "recipe not found")
}

func (rs *RecipeStore) DeleteRecipe(id string) error {
	result, err := rs.db.Exec(`UPDATE capstart_recipes SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, fmtTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	return requireRows(result, "recipe not found")
}

// RecipeExecutionStore handles recipe execution persistence.
type RecipeExecutionStore struct {
	db *sql.DB
}

func NewRecipeExecutionStore(db *sql.DB) *RecipeExecutionStore {
	return &RecipeExecutionStore{db: db}
}

func (res *RecipeExecutionStore) CreateExecution(execution *RecipeExecution) error {
	if execution.ID == "" {
		execution.ID = uuid.New().String()
	}
	if execution.Status == "" {
		execution.Status = "pending"
	}
	if execution.CreatedAt.IsZero() {
		execution.CreatedAt = time.Now().UTC()
	}
	if execution.UpdatedAt.IsZero() {
		execution.UpdatedAt = execution.CreatedAt
	}
	_, err := res.db.Exec(`
		INSERT INTO capstart_recipe_executions
			(id, recipe_id, vm_id, status, config_json, started_at, completed_at, error_message, logs, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		execution.ID, execution.RecipeID, execution.VMID, execution.Status,
		rawOrObject(execution.Config), optTime(execution.StartedAt), optTime(execution.CompletedAt),
		execution.ErrorMessage, execution.Logs, rawOrNull(execution.Metadata),
		fmtTime(execution.CreatedAt), fmtTime(execution.UpdatedAt))
	return err
}

func (res *RecipeExecutionStore) GetExecution(id string) (*RecipeExecution, error) {
	row := res.db.QueryRow(`
		SELECT id, recipe_id, vm_id, status, config_json, started_at, completed_at, error_message, logs, metadata_json, created_at, updated_at
		FROM capstart_recipe_executions WHERE id = ?`, id)
	return scanExecution(row)
}

func (res *RecipeExecutionStore) UpdateExecution(execution *RecipeExecution) error {
	execution.UpdatedAt = time.Now().UTC()
	result, err := res.db.Exec(`
		UPDATE capstart_recipe_executions
		SET status = ?, started_at = ?, completed_at = ?, error_message = ?, logs = ?, metadata_json = ?, updated_at = ?
		WHERE id = ?`,
		execution.Status, optTime(execution.StartedAt), optTime(execution.CompletedAt),
		execution.ErrorMessage, execution.Logs, rawOrNull(execution.Metadata),
		fmtTime(execution.UpdatedAt), execution.ID)
	if err != nil {
		return err
	}
	return requireRows(result, "execution not found")
}

func (res *RecipeExecutionStore) ListExecutionsByVM(vmID string) ([]*RecipeExecution, error) {
	rows, err := res.db.Query(`
		SELECT id, recipe_id, vm_id, status, config_json, started_at, completed_at, error_message, logs, metadata_json, created_at, updated_at
		FROM capstart_recipe_executions WHERE vm_id = ? ORDER BY created_at DESC`, vmID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*RecipeExecution
	for rows.Next() {
		exec, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, exec)
	}
	return out, rows.Err()
}

// ISOStore handles ISO persistence.
type ISOStore struct {
	db *sql.DB
}

func NewISOStore(db *sql.DB) *ISOStore {
	return &ISOStore{db: db}
}

func (is *ISOStore) CreateISO(iso *ISO) error {
	if iso.ID == "" {
		iso.ID = uuid.New().String()
	}
	if iso.CreatedAt.IsZero() {
		iso.CreatedAt = time.Now().UTC()
	}
	if iso.UpdatedAt.IsZero() {
		iso.UpdatedAt = iso.CreatedAt
	}
	_, err := is.db.Exec(`
		INSERT INTO capstart_isos
			(id, name, version, os_type, architecture, file_size, checksum, checksum_type, storage_path, is_verified, url, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		iso.ID, iso.Name, iso.Version, iso.OSType, iso.Architecture, iso.FileSize,
		iso.Checksum, iso.ChecksumType, iso.StoragePath, boolInt(iso.IsVerified),
		iso.URL, rawOrNull(iso.Metadata), fmtTime(iso.CreatedAt), fmtTime(iso.UpdatedAt))
	return err
}

func (is *ISOStore) GetISO(id string) (*ISO, error) {
	row := is.db.QueryRow(`
		SELECT id, name, version, os_type, architecture, file_size, checksum, checksum_type, storage_path, is_verified, url, metadata_json, created_at, updated_at, deleted_at
		FROM capstart_isos WHERE id = ? AND deleted_at IS NULL`, id)
	return scanISO(row)
}

func (is *ISOStore) ListISOs(osType string, offset, limit int) ([]*ISO, error) {
	query := `
		SELECT id, name, version, os_type, architecture, file_size, checksum, checksum_type, storage_path, is_verified, url, metadata_json, created_at, updated_at, deleted_at
		FROM capstart_isos WHERE deleted_at IS NULL`
	args := []any{}
	if osType != "" {
		query += " AND os_type = ?"
		args = append(args, osType)
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}
	rows, err := is.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ISO
	for rows.Next() {
		iso, err := scanISO(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, iso)
	}
	return out, rows.Err()
}

func (is *ISOStore) UpdateISO(iso *ISO) error {
	iso.UpdatedAt = time.Now().UTC()
	result, err := is.db.Exec(`
		UPDATE capstart_isos
		SET is_verified = ?, storage_path = ?, checksum = ?, checksum_type = ?, metadata_json = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`,
		boolInt(iso.IsVerified), iso.StoragePath, iso.Checksum, iso.ChecksumType,
		rawOrNull(iso.Metadata), fmtTime(iso.UpdatedAt), iso.ID)
	if err != nil {
		return err
	}
	return requireRows(result, "ISO not found")
}

func (is *ISOStore) DeleteISO(id string) error {
	result, err := is.db.Exec(`UPDATE capstart_isos SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, fmtTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	return requireRows(result, "ISO not found")
}

type InstallationJobStore struct {
	db *sql.DB
}

func NewInstallationJobStore(db *sql.DB) *InstallationJobStore {
	return &InstallationJobStore{db: db}
}

func (js *InstallationJobStore) CreateJob(job *InstallationJob) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.Status == "" {
		job.Status = "pending"
	}
	if job.Timeout == 0 {
		job.Timeout = 3600
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = job.CreatedAt
	}
	_, err := js.db.Exec(`
		INSERT INTO capstart_installation_jobs
			(id, iso_id, vm_id, status, booted_at, started_at, completed_at, error_message, installer_logs, timeout, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.ISOID, job.VMID, job.Status, optTime(job.BootedAt),
		optTime(job.StartedAt), optTime(job.CompletedAt), job.ErrorMessage,
		job.InstallerLogs, job.Timeout, fmtTime(job.CreatedAt), fmtTime(job.UpdatedAt))
	return err
}

func (js *InstallationJobStore) GetJob(id string) (*InstallationJob, error) {
	row := js.db.QueryRow(`
		SELECT id, iso_id, vm_id, status, booted_at, started_at, completed_at, error_message, installer_logs, timeout, created_at, updated_at
		FROM capstart_installation_jobs WHERE id = ?`, id)
	return scanJob(row)
}

func (js *InstallationJobStore) UpdateJob(job *InstallationJob) error {
	job.UpdatedAt = time.Now().UTC()
	result, err := js.db.Exec(`
		UPDATE capstart_installation_jobs
		SET status = ?, booted_at = ?, started_at = ?, completed_at = ?, error_message = ?, installer_logs = ?, timeout = ?, updated_at = ?
		WHERE id = ?`,
		job.Status, optTime(job.BootedAt), optTime(job.StartedAt), optTime(job.CompletedAt),
		job.ErrorMessage, job.InstallerLogs, job.Timeout, fmtTime(job.UpdatedAt), job.ID)
	if err != nil {
		return err
	}
	return requireRows(result, "installation job not found")
}

func setRecipeDefaults(recipe *Recipe) {
	if recipe.Author == "" {
		recipe.Author = "CapperVM Team"
	}
	if recipe.CreatedAt.IsZero() {
		recipe.CreatedAt = time.Now().UTC()
	}
	if recipe.UpdatedAt.IsZero() {
		recipe.UpdatedAt = recipe.CreatedAt
	}
	if len(recipe.Schema) == 0 {
		recipe.Schema = json.RawMessage(`{}`)
	}
	if len(recipe.Content) == 0 {
		recipe.Content = json.RawMessage(`{}`)
	}
	if recipe.Tags == nil {
		recipe.Tags = []string{}
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecipe(row rowScanner) (*Recipe, error) {
	var recipe Recipe
	var tags, schema, content, created, updated string
	var isBuiltin, isCommunity int
	if err := row.Scan(&recipe.ID, &recipe.Name, &recipe.Version, &recipe.Title,
		&recipe.Description, &recipe.Category, &recipe.Author, &tags, &schema,
		&content, &recipe.Checksum, &isBuiltin, &isCommunity, &recipe.AuthorID,
		&created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("recipe not found")
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(tags), &recipe.Tags)
	recipe.Schema = json.RawMessage(schema)
	recipe.Content = json.RawMessage(content)
	recipe.IsBuiltin = isBuiltin != 0
	recipe.IsCommunity = isCommunity != 0
	recipe.CreatedAt = parseTime(created)
	recipe.UpdatedAt = parseTime(updated)
	return &recipe, nil
}

func scanExecution(row rowScanner) (*RecipeExecution, error) {
	var execution RecipeExecution
	var config, metadata sql.NullString
	var started, completed, created, updated sql.NullString
	if err := row.Scan(&execution.ID, &execution.RecipeID, &execution.VMID,
		&execution.Status, &config, &started, &completed, &execution.ErrorMessage,
		&execution.Logs, &metadata, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("execution not found")
		}
		return nil, err
	}
	execution.Config = rawFromNull(config, `{}`)
	execution.Metadata = rawFromNull(metadata, ``)
	execution.StartedAt = nullTimePtr(started)
	execution.CompletedAt = nullTimePtr(completed)
	execution.CreatedAt = parseNullTime(created)
	execution.UpdatedAt = parseNullTime(updated)
	return &execution, nil
}

func scanISO(row rowScanner) (*ISO, error) {
	var iso ISO
	var url, metadata, deleted sql.NullString
	var created, updated string
	var verified int
	if err := row.Scan(&iso.ID, &iso.Name, &iso.Version, &iso.OSType,
		&iso.Architecture, &iso.FileSize, &iso.Checksum, &iso.ChecksumType,
		&iso.StoragePath, &verified, &url, &metadata, &created, &updated,
		&deleted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("ISO not found")
		}
		return nil, err
	}
	iso.IsVerified = verified != 0
	if url.Valid {
		iso.URL = &url.String
	}
	iso.Metadata = rawFromNull(metadata, ``)
	iso.CreatedAt = parseTime(created)
	iso.UpdatedAt = parseTime(updated)
	iso.DeletedAt = nullTimePtr(deleted)
	return &iso, nil
}

func scanJob(row rowScanner) (*InstallationJob, error) {
	var job InstallationJob
	var booted, started, completed, created, updated sql.NullString
	if err := row.Scan(&job.ID, &job.ISOID, &job.VMID, &job.Status, &booted,
		&started, &completed, &job.ErrorMessage, &job.InstallerLogs, &job.Timeout,
		&created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("installation job not found")
		}
		return nil, err
	}
	job.BootedAt = nullTimePtr(booted)
	job.StartedAt = nullTimePtr(started)
	job.CompletedAt = nullTimePtr(completed)
	job.CreatedAt = parseNullTime(created)
	job.UpdatedAt = parseNullTime(updated)
	return &job, nil
}

func requireRows(result sql.Result, notFound string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New(notFound)
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func rawOrObject(raw json.RawMessage) string {
	if len(raw) == 0 {
		return `{}`
	}
	return string(raw)
}

func rawOrNull(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func rawFromNull(ns sql.NullString, fallback string) json.RawMessage {
	if !ns.Valid || ns.String == "" {
		if fallback == "" {
			return nil
		}
		return json.RawMessage(fallback)
	}
	return json.RawMessage(ns.String)
}

func fmtTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func optTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return fmtTime(*t)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func parseNullTime(ns sql.NullString) time.Time {
	if !ns.Valid {
		return time.Time{}
	}
	return parseTime(ns.String)
}

func nullTimePtr(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseTime(ns.String)
	if t.IsZero() {
		return nil
	}
	return &t
}
