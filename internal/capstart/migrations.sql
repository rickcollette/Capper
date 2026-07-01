-- CapStart Database Migrations
-- Created: 2026-07-01

-- Migration 001: Create recipes table
-- Stores CapStart recipe metadata and definitions
CREATE TABLE IF NOT EXISTS recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(100),
    author VARCHAR(255) NOT NULL,
    tags TEXT[] DEFAULT ARRAY[]::TEXT[],
    schema JSONB, -- Recipe parameter schema
    content JSONB, -- Full recipe definition
    checksum VARCHAR(64), -- SHA256 of content
    is_builtin BOOLEAN DEFAULT FALSE,
    is_community BOOLEAN DEFAULT FALSE,
    author_id UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(name, version)
);

CREATE INDEX IF NOT EXISTS idx_recipes_name ON recipes(name);
CREATE INDEX IF NOT EXISTS idx_recipes_author ON recipes(author);
CREATE INDEX IF NOT EXISTS idx_recipes_is_builtin ON recipes(is_builtin);
CREATE INDEX IF NOT EXISTS idx_recipes_tags ON recipes USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_recipes_created_at ON recipes(created_at DESC);

-- Migration 002: Create recipe_executions table
-- Tracks individual recipe executions for VM provisioning
CREATE TABLE IF NOT EXISTS recipe_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    vm_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'pending', -- pending, running, success, failed, cancelled
    config JSONB, -- User-provided configuration merged with recipe defaults
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    logs TEXT, -- Execution logs
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_by UUID -- User who initiated execution
);

CREATE INDEX IF NOT EXISTS idx_recipe_executions_recipe_id ON recipe_executions(recipe_id);
CREATE INDEX IF NOT EXISTS idx_recipe_executions_vm_id ON recipe_executions(vm_id);
CREATE INDEX IF NOT EXISTS idx_recipe_executions_status ON recipe_executions(status);
CREATE INDEX IF NOT EXISTS idx_recipe_executions_created_at ON recipe_executions(created_at DESC);

-- Migration 003: Create isos table
-- Stores uploaded OS installation ISOs
CREATE TABLE IF NOT EXISTS isos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50),
    os_type VARCHAR(100), -- linux, windows, bsd, etc.
    architecture VARCHAR(50), -- x86_64, arm64, etc.
    file_size BIGINT,
    checksum VARCHAR(64),
    checksum_type VARCHAR(20) DEFAULT 'sha256', -- md5, sha256, etc.
    storage_path VARCHAR(1024), -- Path to ISO file in S3 or local storage
    is_verified BOOLEAN DEFAULT FALSE,
    url VARCHAR(1024), -- URL for remote ISOs
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by UUID
);

CREATE INDEX IF NOT EXISTS idx_isos_name ON isos(name);
CREATE INDEX IF NOT EXISTS idx_isos_os_type ON isos(os_type);
CREATE INDEX IF NOT EXISTS idx_isos_is_verified ON isos(is_verified);
CREATE INDEX IF NOT EXISTS idx_isos_created_at ON isos(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_isos_deleted_at ON isos(deleted_at);

-- Migration 004: Create installation_jobs table
-- Tracks OS installation from ISO
CREATE TABLE IF NOT EXISTS installation_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    iso_id UUID NOT NULL REFERENCES isos(id) ON DELETE CASCADE,
    vm_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'pending', -- pending, booted, running, installing, success, failed, cancelled
    booted_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    installer_logs TEXT, -- Captured from installation console
    timeout INTEGER DEFAULT 3600, -- Timeout in seconds
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_by UUID
);

CREATE INDEX IF NOT EXISTS idx_installation_jobs_iso_id ON installation_jobs(iso_id);
CREATE INDEX IF NOT EXISTS idx_installation_jobs_vm_id ON installation_jobs(vm_id);
CREATE INDEX IF NOT EXISTS idx_installation_jobs_status ON installation_jobs(status);
CREATE INDEX IF NOT EXISTS idx_installation_jobs_created_at ON installation_jobs(created_at DESC);

-- Migration 005: Alter instances table to add recipe references
-- Links instances to recipes and installations
ALTER TABLE instances ADD COLUMN IF NOT EXISTS recipe_id UUID REFERENCES recipes(id) ON DELETE SET NULL;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS installation_job_id UUID REFERENCES installation_jobs(id) ON DELETE SET NULL;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS iso_id UUID REFERENCES isos(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_instances_recipe_id ON instances(recipe_id);
CREATE INDEX IF NOT EXISTS idx_instances_installation_job_id ON instances(installation_job_id);
CREATE INDEX IF NOT EXISTS idx_instances_iso_id ON instances(iso_id);

-- Migration 006: Create recipe_versions table
-- Store version history of recipes
CREATE TABLE IF NOT EXISTS recipe_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    content JSONB NOT NULL,
    changelog TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    UNIQUE(recipe_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_recipe_versions_recipe_id ON recipe_versions(recipe_id);
CREATE INDEX IF NOT EXISTS idx_recipe_versions_created_at ON recipe_versions(created_at DESC);

-- Migration 007: Create recipe_templates table
-- Store recipe templates for common scenarios
CREATE TABLE IF NOT EXISTS recipe_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    base_recipe_id UUID REFERENCES recipes(id) ON DELETE SET NULL,
    template_schema JSONB, -- Variables and their defaults
    output_recipe JSONB, -- Generated recipe when instantiated
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_by UUID
);

CREATE INDEX IF NOT EXISTS idx_recipe_templates_name ON recipe_templates(name);

-- Migration 008: Update recipes trigger for updated_at
CREATE OR REPLACE FUNCTION update_recipes_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_recipes_updated_at_trigger ON recipes;
CREATE TRIGGER update_recipes_updated_at_trigger
BEFORE UPDATE ON recipes
FOR EACH ROW
EXECUTE FUNCTION update_recipes_updated_at();

-- Triggers for other tables...
CREATE OR REPLACE FUNCTION update_isos_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_isos_updated_at_trigger ON isos;
CREATE TRIGGER update_isos_updated_at_trigger
BEFORE UPDATE ON isos
FOR EACH ROW
EXECUTE FUNCTION update_isos_updated_at();

CREATE OR REPLACE FUNCTION update_installation_jobs_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_installation_jobs_updated_at_trigger ON installation_jobs;
CREATE TRIGGER update_installation_jobs_updated_at_trigger
BEFORE UPDATE ON installation_jobs
FOR EACH ROW
EXECUTE FUNCTION update_installation_jobs_updated_at();

-- Commit successful
-- Run this with: psql -d capper -f migrations.sql
