package capstart

import (
	"encoding/json"
	"time"
)

// Recipe represents a CapStart recipe for VM provisioning
type Recipe struct {
	ID          string          `db:"id" json:"id"`
	Name        string          `db:"name" json:"name"`
	Version     string          `db:"version" json:"version"`
	Title       string          `db:"title" json:"title"`
	Description string          `db:"description" json:"description"`
	Category    string          `db:"category" json:"category"`
	Author      string          `db:"author" json:"author"`
	Tags        []string        `db:"tags" json:"tags"`
	Schema      json.RawMessage `db:"schema" json:"schema"`
	Content     json.RawMessage `db:"content" json:"content"`
	Checksum    string          `db:"checksum" json:"checksum"`
	IsBuiltin   bool            `db:"is_builtin" json:"isBuiltin"`
	IsCommunity bool            `db:"is_community" json:"isCommunity"`
	AuthorID    *string         `db:"author_id" json:"authorID,omitempty"`
	CreatedAt   time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updatedAt"`
}

// RecipeExecution tracks a single recipe execution for a VM
type RecipeExecution struct {
	ID           string          `db:"id" json:"id"`
	RecipeID     string          `db:"recipe_id" json:"recipeID"`
	VMID         string          `db:"vm_id" json:"vmID"`
	Status       string          `db:"status" json:"status"` // pending, running, success, failed, cancelled
	Config       json.RawMessage `db:"config" json:"config"` // User-provided configuration
	StartedAt    *time.Time      `db:"started_at" json:"startedAt,omitempty"`
	CompletedAt  *time.Time      `db:"completed_at" json:"completedAt,omitempty"`
	ErrorMessage *string         `db:"error_message" json:"errorMessage,omitempty"`
	Logs         *string         `db:"logs" json:"logs,omitempty"`
	Metadata     json.RawMessage `db:"metadata" json:"metadata,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updatedAt"`
}

// ISO represents an uploaded OS installation ISO
type ISO struct {
	ID           string          `db:"id" json:"id"`
	Name         string          `db:"name" json:"name"`
	Version      string          `db:"version" json:"version"`
	OSType       string          `db:"os_type" json:"osType"` // linux, windows, etc.
	Architecture string          `db:"architecture" json:"architecture"`
	FileSize     int64           `db:"file_size" json:"fileSize"`
	Checksum     string          `db:"checksum" json:"checksum"`
	ChecksumType string          `db:"checksum_type" json:"checksumType"` // md5, sha256, etc.
	StoragePath  string          `db:"storage_path" json:"storagePath"`
	IsVerified   bool            `db:"is_verified" json:"isVerified"`
	URL          *string         `db:"url" json:"url,omitempty"` // For URL-based ISOs
	Metadata     json.RawMessage `db:"metadata" json:"metadata,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updatedAt"`
	DeletedAt    *time.Time      `db:"deleted_at" json:"deletedAt,omitempty"`
}

// InstallationJob tracks an OS installation from ISO
type InstallationJob struct {
	ID            string     `db:"id" json:"id"`
	ISOID         string     `db:"iso_id" json:"isoID"`
	VMID          string     `db:"vm_id" json:"vmID"`
	Status        string     `db:"status" json:"status"` // pending, running, installing, success, failed, cancelled
	BootedAt      *time.Time `db:"booted_at" json:"bootedAt,omitempty"`
	StartedAt     *time.Time `db:"started_at" json:"startedAt,omitempty"`
	CompletedAt   *time.Time `db:"completed_at" json:"completedAt,omitempty"`
	ErrorMessage  *string    `db:"error_message" json:"errorMessage,omitempty"`
	InstallerLogs *string    `db:"installer_logs" json:"installerLogs,omitempty"`
	Timeout       int        `db:"timeout" json:"timeout"` // seconds
	CreatedAt     time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updatedAt"`
}

// ValidationResult represents the result of recipe validation
type ValidationResult struct {
	Valid    bool                `json:"valid"`
	Errors   []ValidationError   `json:"errors"`
	Warnings []ValidationWarning `json:"warnings"`
	Metadata RecipeMetadata      `json:"metadata"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// RecipeMetadata represents extracted recipe metadata
type RecipeMetadata struct {
	CPUMin            int      `json:"cpuMin"`
	CPURecommended    int      `json:"cpuRecommended"`
	MemoryMin         int      `json:"memoryMin"`         // MB
	MemoryRecommended int      `json:"memoryRecommended"` // MB
	DiskMin           int      `json:"diskMin"`           // MB
	DiskRecommended   int      `json:"diskRecommended"`   // MB
	Parameters        []string `json:"parameters"`
	EstimatedDuration int      `json:"estimatedDuration"` // seconds
}

// RecipeParameter represents a parameter in a recipe
type RecipeParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // string, password, number, boolean, select
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Required    bool        `json:"required"`
	Validation  *string     `json:"validation,omitempty"` // regex pattern
	Options     []string    `json:"options,omitempty"`
	MinLength   *int        `json:"minLength,omitempty"`
	MaxLength   *int        `json:"maxLength,omitempty"`
	Minimum     *int        `json:"minimum,omitempty"`
	Maximum     *int        `json:"maximum,omitempty"`
}

// CreateRecipeRequest represents a request to create/upload a recipe
type CreateRecipeRequest struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Tags        []string        `json:"tags"`
	Content     json.RawMessage `json:"content"`
}

// UploadISORequest represents a request to upload an ISO
type UploadISORequest struct {
	Name         string  `json:"name"`
	Version      string  `json:"version"`
	OSType       string  `json:"osType"`
	Architecture string  `json:"architecture"`
	Checksum     string  `json:"checksum"`
	ChecksumType string  `json:"checksumType"`
	URL          *string `json:"url,omitempty"`
}

// CreateVMFromRecipeRequest represents a request to create a VM from a recipe
type CreateVMFromRecipeRequest struct {
	RecipeID string          `json:"recipeID"`
	Config   json.RawMessage `json:"config"`
	VMName   string          `json:"vmName,omitempty"`
	CPU      int             `json:"cpu,omitempty"`
	Memory   int             `json:"memory,omitempty"`  // MB
	Disk     int             `json:"disk,omitempty"`    // MB
	Network  *string         `json:"network,omitempty"` // VPC/Network ID
}

// CreateInstallationJobRequest represents a request to start OS installation from ISO
type CreateInstallationJobRequest struct {
	ISOID   string `json:"isoID"`
	VMID    string `json:"vmID"`
	Timeout int    `json:"timeout,omitempty"` // seconds, default 3600
}
