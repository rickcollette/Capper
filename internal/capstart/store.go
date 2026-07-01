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

// RecipeStore handles recipe persistence
type RecipeStore struct {
	db *sql.DB
}

// NewRecipeStore creates a new recipe store
func NewRecipeStore(db *sql.DB) *RecipeStore {
	return &RecipeStore{db: db}
}

// CreateRecipe creates a new recipe in the database
func (rs *RecipeStore) CreateRecipe(recipe *Recipe) error {
	if recipe.ID == "" {
		recipe.ID = uuid.New().String()
	}
	if recipe.CreatedAt.IsZero() {
		recipe.CreatedAt = time.Now()
	}
	if recipe.UpdatedAt.IsZero() {
		recipe.UpdatedAt = time.Now()
	}

	// Calculate checksum if not provided
	if recipe.Checksum == "" {
		hash := sha256.Sum256(recipe.Content)
		recipe.Checksum = hex.EncodeToString(hash[:])
	}

	query := `
		INSERT INTO recipes (id, name, version, title, description, category, author, tags, schema, content, checksum, is_builtin, is_community, author_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	_, err := rs.db.Exec(query,
		recipe.ID,
		recipe.Name,
		recipe.Version,
		recipe.Title,
		recipe.Description,
		recipe.Category,
		recipe.Author,
		recipe.Tags,
		recipe.Schema,
		recipe.Content,
		recipe.Checksum,
		recipe.IsBuiltin,
		recipe.IsCommunity,
		recipe.AuthorID,
		recipe.CreatedAt,
		recipe.UpdatedAt,
	)

	return err
}

// GetRecipe retrieves a recipe by ID
func (rs *RecipeStore) GetRecipe(id string) (*Recipe, error) {
	query := `
		SELECT id, name, version, title, description, category, author, tags, schema, content, checksum, is_builtin, is_community, author_id, created_at, updated_at
		FROM recipes
		WHERE id = $1 AND deleted_at IS NULL
	`

	recipe := &Recipe{}
	err := rs.db.QueryRow(query, id).Scan(
		&recipe.ID,
		&recipe.Name,
		&recipe.Version,
		&recipe.Title,
		&recipe.Description,
		&recipe.Category,
		&recipe.Author,
		pq.Array(&recipe.Tags),
		&recipe.Schema,
		&recipe.Content,
		&recipe.Checksum,
		&recipe.IsBuiltin,
		&recipe.IsCommunity,
		&recipe.AuthorID,
		&recipe.CreatedAt,
		&recipe.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("recipe not found")
	}

	return recipe, err
}

// GetRecipeByName retrieves a recipe by name and version
func (rs *RecipeStore) GetRecipeByName(name, version string) (*Recipe, error) {
	query := `
		SELECT id, name, version, title, description, category, author, tags, schema, content, checksum, is_builtin, is_community, author_id, created_at, updated_at
		FROM recipes
		WHERE name = $1 AND version = $2 AND deleted_at IS NULL
	`

	recipe := &Recipe{}
	err := rs.db.QueryRow(query, name, version).Scan(
		&recipe.ID,
		&recipe.Name,
		&recipe.Version,
		&recipe.Title,
		&recipe.Description,
		&recipe.Category,
		&recipe.Author,
		pq.Array(&recipe.Tags),
		&recipe.Schema,
		&recipe.Content,
		&recipe.Checksum,
		&recipe.IsBuiltin,
		&recipe.IsCommunity,
		&recipe.AuthorID,
		&recipe.CreatedAt,
		&recipe.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("recipe not found")
	}

	return recipe, err
}

// ListRecipes retrieves all recipes with optional filtering
func (rs *RecipeStore) ListRecipes(category string, isBuiltin *bool, offset, limit int) ([]*Recipe, error) {
	query := `
		SELECT id, name, version, title, description, category, author, tags, schema, content, checksum, is_builtin, is_community, author_id, created_at, updated_at
		FROM recipes
		WHERE deleted_at IS NULL
	`

	args := []interface{}{}
	argIndex := 1

	if category != "" {
		query += fmt.Sprintf(" AND category = $%d", argIndex)
		args = append(args, category)
		argIndex++
	}

	if isBuiltin != nil {
		query += fmt.Sprintf(" AND is_builtin = $%d", argIndex)
		args = append(args, *isBuiltin)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
		args = append(args, limit, offset)
	}

	rows, err := rs.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recipes := []*Recipe{}
	for rows.Next() {
		recipe := &Recipe{}
		err := rows.Scan(
			&recipe.ID,
			&recipe.Name,
			&recipe.Version,
			&recipe.Title,
			&recipe.Description,
			&recipe.Category,
			&recipe.Author,
			pq.Array(&recipe.Tags),
			&recipe.Schema,
			&recipe.Content,
			&recipe.Checksum,
			&recipe.IsBuiltin,
			&recipe.IsCommunity,
			&recipe.AuthorID,
			&recipe.CreatedAt,
			&recipe.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		recipes = append(recipes, recipe)
	}

	return recipes, rows.Err()
}

// UpdateRecipe updates a recipe
func (rs *RecipeStore) UpdateRecipe(recipe *Recipe) error {
	recipe.UpdatedAt = time.Now()

	query := `
		UPDATE recipes
		SET title = $1, description = $2, category = $3, tags = $4, updated_at = $5
		WHERE id = $6 AND deleted_at IS NULL
	`

	result, err := rs.db.Exec(query,
		recipe.Title,
		recipe.Description,
		recipe.Category,
		recipe.Tags,
		recipe.UpdatedAt,
		recipe.ID,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("recipe not found")
	}

	return nil
}

// DeleteRecipe soft-deletes a recipe
func (rs *RecipeStore) DeleteRecipe(id string) error {
	query := `
		UPDATE recipes
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := rs.db.Exec(query, time.Now(), id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("recipe not found")
	}

	return nil
}

// RecipeExecutionStore handles recipe execution persistence
type RecipeExecutionStore struct {
	db *sql.DB
}

// NewRecipeExecutionStore creates a new execution store
func NewRecipeExecutionStore(db *sql.DB) *RecipeExecutionStore {
	return &RecipeExecutionStore{db: db}
}

// CreateExecution creates a new recipe execution
func (res *RecipeExecutionStore) CreateExecution(execution *RecipeExecution) error {
	if execution.ID == "" {
		execution.ID = uuid.New().String()
	}
	if execution.CreatedAt.IsZero() {
		execution.CreatedAt = time.Now()
	}
	if execution.UpdatedAt.IsZero() {
		execution.UpdatedAt = time.Now()
	}

	query := `
		INSERT INTO recipe_executions (id, recipe_id, vm_id, status, config, started_at, completed_at, error_message, logs, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := res.db.Exec(query,
		execution.ID,
		execution.RecipeID,
		execution.VMID,
		execution.Status,
		execution.Config,
		execution.StartedAt,
		execution.CompletedAt,
		execution.ErrorMessage,
		execution.Logs,
		execution.Metadata,
		execution.CreatedAt,
		execution.UpdatedAt,
	)

	return err
}

// GetExecution retrieves an execution by ID
func (res *RecipeExecutionStore) GetExecution(id string) (*RecipeExecution, error) {
	query := `
		SELECT id, recipe_id, vm_id, status, config, started_at, completed_at, error_message, logs, metadata, created_at, updated_at
		FROM recipe_executions
		WHERE id = $1
	`

	execution := &RecipeExecution{}
	err := res.db.QueryRow(query, id).Scan(
		&execution.ID,
		&execution.RecipeID,
		&execution.VMID,
		&execution.Status,
		&execution.Config,
		&execution.StartedAt,
		&execution.CompletedAt,
		&execution.ErrorMessage,
		&execution.Logs,
		&execution.Metadata,
		&execution.CreatedAt,
		&execution.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("execution not found")
	}

	return execution, err
}

// UpdateExecution updates an execution status
func (res *RecipeExecutionStore) UpdateExecution(execution *RecipeExecution) error {
	execution.UpdatedAt = time.Now()

	query := `
		UPDATE recipe_executions
		SET status = $1, started_at = $2, completed_at = $3, error_message = $4, logs = $5, metadata = $6, updated_at = $7
		WHERE id = $8
	`

	result, err := res.db.Exec(query,
		execution.Status,
		execution.StartedAt,
		execution.CompletedAt,
		execution.ErrorMessage,
		execution.Logs,
		execution.Metadata,
		execution.UpdatedAt,
		execution.ID,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("execution not found")
	}

	return nil
}

// ListExecutionsByVM retrieves all executions for a VM
func (res *RecipeExecutionStore) ListExecutionsByVM(vmID string) ([]*RecipeExecution, error) {
	query := `
		SELECT id, recipe_id, vm_id, status, config, started_at, completed_at, error_message, logs, metadata, created_at, updated_at
		FROM recipe_executions
		WHERE vm_id = $1
		ORDER BY created_at DESC
	`

	rows, err := res.db.Query(query, vmID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executions := []*RecipeExecution{}
	for rows.Next() {
		execution := &RecipeExecution{}
		err := rows.Scan(
			&execution.ID,
			&execution.RecipeID,
			&execution.VMID,
			&execution.Status,
			&execution.Config,
			&execution.StartedAt,
			&execution.CompletedAt,
			&execution.ErrorMessage,
			&execution.Logs,
			&execution.Metadata,
			&execution.CreatedAt,
			&execution.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		executions = append(executions, execution)
	}

	return executions, rows.Err()
}

// ISOStore handles ISO persistence
type ISOStore struct {
	db *sql.DB
}

// NewISOStore creates a new ISO store
func NewISOStore(db *sql.DB) *ISOStore {
	return &ISOStore{db: db}
}

// CreateISO creates a new ISO record
func (is *ISOStore) CreateISO(iso *ISO) error {
	if iso.ID == "" {
		iso.ID = uuid.New().String()
	}
	if iso.CreatedAt.IsZero() {
		iso.CreatedAt = time.Now()
	}
	if iso.UpdatedAt.IsZero() {
		iso.UpdatedAt = time.Now()
	}

	query := `
		INSERT INTO isos (id, name, version, os_type, architecture, file_size, checksum, checksum_type, storage_path, is_verified, url, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := is.db.Exec(query,
		iso.ID,
		iso.Name,
		iso.Version,
		iso.OSType,
		iso.Architecture,
		iso.FileSize,
		iso.Checksum,
		iso.ChecksumType,
		iso.StoragePath,
		iso.IsVerified,
		iso.URL,
		iso.Metadata,
		iso.CreatedAt,
		iso.UpdatedAt,
	)

	return err
}

// GetISO retrieves an ISO by ID
func (is *ISOStore) GetISO(id string) (*ISO, error) {
	query := `
		SELECT id, name, version, os_type, architecture, file_size, checksum, checksum_type, storage_path, is_verified, url, metadata, created_at, updated_at
		FROM isos
		WHERE id = $1 AND deleted_at IS NULL
	`

	iso := &ISO{}
	err := is.db.QueryRow(query, id).Scan(
		&iso.ID,
		&iso.Name,
		&iso.Version,
		&iso.OSType,
		&iso.Architecture,
		&iso.FileSize,
		&iso.Checksum,
		&iso.ChecksumType,
		&iso.StoragePath,
		&iso.IsVerified,
		&iso.URL,
		&iso.Metadata,
		&iso.CreatedAt,
		&iso.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("ISO not found")
	}

	return iso, err
}

// ListISOs retrieves all ISOs
func (is *ISOStore) ListISOs(osType string, offset, limit int) ([]*ISO, error) {
	query := `
		SELECT id, name, version, os_type, architecture, file_size, checksum, checksum_type, storage_path, is_verified, url, metadata, created_at, updated_at
		FROM isos
		WHERE deleted_at IS NULL
	`

	args := []interface{}{}
	argIndex := 1

	if osType != "" {
		query += fmt.Sprintf(" AND os_type = $%d", argIndex)
		args = append(args, osType)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
		args = append(args, limit, offset)
	}

	rows, err := is.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	isos := []*ISO{}
	for rows.Next() {
		iso := &ISO{}
		err := rows.Scan(
			&iso.ID,
			&iso.Name,
			&iso.Version,
			&iso.OSType,
			&iso.Architecture,
			&iso.FileSize,
			&iso.Checksum,
			&iso.ChecksumType,
			&iso.StoragePath,
			&iso.IsVerified,
			&iso.URL,
			&iso.Metadata,
			&iso.CreatedAt,
			&iso.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		isos = append(isos, iso)
	}

	return isos, rows.Err()
}

// UpdateISO updates an ISO record
func (is *ISOStore) UpdateISO(iso *ISO) error {
	iso.UpdatedAt = time.Now()

	query := `
		UPDATE isos
		SET is_verified = $1, updated_at = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	result, err := is.db.Exec(query, iso.IsVerified, iso.UpdatedAt, iso.ID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("ISO not found")
	}

	return nil
}

// DeleteISO soft-deletes an ISO
func (is *ISOStore) DeleteISO(id string) error {
	query := `
		UPDATE isos
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := is.db.Exec(query, time.Now(), id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("ISO not found")
	}

	return nil
}

// Note: This file uses github.com/lib/pq for PostgreSQL array support
// Import statement needed: "github.com/lib/pq"
