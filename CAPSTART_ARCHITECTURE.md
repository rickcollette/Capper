# CapStart Integration Architecture

## Overview

CapStart integration transforms CapperVM into a recipe-driven infrastructure platform, similar to LXE templates in ProxMox. This document defines the technical architecture for seamlessly integrating CapStart recipes into CapperVM's VM provisioning pipeline.

---

## Design Principles

1. **Minimal Coupling**: CapStart remains independent; CapperVM interfaces via well-defined APIs
2. **Composability**: Recipes can be combined with existing CapperVM workflows
3. **Async-First**: Long operations (ISO uploads, VM creation) run asynchronously with WebSocket updates
4. **Auditability**: All recipe executions are logged and traceable
5. **Safety**: Recipes are validated before execution; sandboxing prevents malicious code

---

## System Architecture

### High-Level Flow

```
┌─────────────────────────────────────────────────────────────┐
│                     CapperVM Frontend                        │
│  (React + React Query)                                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Recipe Browser → Recipe Details → VM Creation Wizard      │
│  ISO Upload UI → Installation Progress                     │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│                    CapperVM REST API                         │
│  (Go handlers)                                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  /api/v1/capstart/recipes/*        → Recipe management      │
│  /api/v1/capstart/isos/*           → ISO management         │
│  /api/v1/capstart/install/*        → Installation tracking  │
│  /api/v1/instances/* (extended)    → VM creation from recipe
└─────────────────────────────────────────────────────────────┘
                           ↓
                    ┌──────┴──────┐
                    ↓             ↓
            ┌────────────┐  ┌───────────────┐
            │  Database  │  │  CapStart Lib │
            │(PostgreSQL)│  │  (Go Package) │
            └────────────┘  └───────────────┘
                    ↓             ↓
                    └──────┬──────┘
                           ↓
            ┌──────────────────────────────┐
            │   Hypervisor / VM Provisioning│
            │   (Existing CapperVM logic)   │
            └──────────────────────────────┘
```

---

## Core Components

### 1. Recipe Storage Layer

**Purpose**: Persist and manage recipe metadata and versions

**Storage Options**:
- **Database (Primary)**: PostgreSQL recipes table
  - Recipe metadata (name, version, description)
  - Recipe content (JSON schema + config templates)
  - Version history and tags
  - Author/timestamp information
  
- **File Storage (Secondary)**: S3 or local filesystem
  - Recipe definition files
  - Large attachments (scripts, ISO lists)
  - Built-in recipe library bundles

**Schema**:
```sql
recipes (
  id UUID PRIMARY KEY,
  name VARCHAR NOT NULL,
  version VARCHAR NOT NULL,
  category VARCHAR,
  description TEXT,
  schema JSONB,  -- Recipe parameter schema
  content JSONB, -- Recipe definition
  author_id UUID,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  is_builtin BOOLEAN,
  is_community BOOLEAN,
  tags TEXT[],
  checksum VARCHAR,
  UNIQUE(name, version)
)

recipe_executions (
  id UUID PRIMARY KEY,
  recipe_id UUID REFERENCES recipes,
  vm_id UUID,
  status VARCHAR, -- pending, running, success, failed
  config JSONB,   -- Runtime configuration
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  error_message TEXT,
  logs TEXT,      -- Execution logs
  metadata JSONB  -- Custom metadata
)
```

### 2. Recipe Validator

**Purpose**: Ensure recipes meet requirements before execution

**Validations**:
- Schema compliance (required fields, types)
- Dependency resolution (required CapperVM features)
- Resource requirements (CPU, RAM, disk)
- Network configuration validity
- Script safety (no obvious malicious patterns)

**Output**:
- Valid/Invalid status
- List of validation errors
- List of warnings
- Resource requirements metadata

### 3. Recipe Executor

**Purpose**: Execute recipes to create and configure VMs

**Pipeline**:
1. Parse recipe definition
2. Merge user configuration with recipe defaults
3. Validate merged configuration
4. Create base VM (via existing CapperVM instance API)
5. Execute installation hooks (cloud-init, scripts)
6. Execute post-install hooks (software installation, configuration)
7. Monitor progress and report status
8. Clean up on failure

**Supported Hook Types**:
- `pre_provisioning`: Before VM creation
- `post_provisioning`: After VM boots
- `post_install`: After OS installation (for ISO-based VMs)
- `post_configuration`: Final configuration steps

### 4. ISO Management

**Purpose**: Handle OS installation ISOs

**Operations**:
- Upload and verify ISO integrity (checksums)
- Store in S3/local filesystem with metadata
- Track used/unused ISOs for cleanup
- Support URL-based ISO downloads (no upload needed)

**Metadata per ISO**:
- Name and version
- OS type (Linux, Windows, etc.)
- Supported architectures
- Default installation settings
- Kickstart/Preseed templates

### 5. Installation Tracker

**Purpose**: Monitor and track VM installations

**Features**:
- Real-time progress updates via WebSocket
- Log streaming from installation process
- Automatic retry on failure
- Timeout handling
- Fallback to manual installation if needed

---

## API Layer Design

### Recipe Management

```
GET    /api/v1/capstart/recipes
POST   /api/v1/capstart/recipes
GET    /api/v1/capstart/recipes/{id}
PUT    /api/v1/capstart/recipes/{id}
DELETE /api/v1/capstart/recipes/{id}
```

### Recipe Execution

```
POST   /api/v1/capstart/recipes/{id}/create-vm
GET    /api/v1/capstart/recipes/{id}/validate
POST   /api/v1/capstart/recipes/{id}/test
```

### Built-in Recipes

```
GET    /api/v1/capstart/recipes/builtin
GET    /api/v1/capstart/recipes/builtin/{name}
```

### ISO Management

```
GET    /api/v1/capstart/isos
POST   /api/v1/capstart/isos
DELETE /api/v1/capstart/isos/{id}
POST   /api/v1/capstart/isos/{id}/verify
```

### Installation Tracking

```
POST   /api/v1/capstart/install
GET    /api/v1/capstart/install/{jobId}
POST   /api/v1/capstart/install/{jobId}/cancel
WS     /api/v1/capstart/install/{jobId}/logs
```

---

## Data Flow Diagram

### Recipe-Based VM Creation

```
1. User selects recipe
   ↓
2. API: GET /recipes/{id}
   ↓
3. Frontend displays recipe details + configuration form
   ↓
4. User configures parameters
   ↓
5. API: POST /recipes/{id}/create-vm
   ├─ Validate recipe
   ├─ Merge user config with recipe defaults
   ├─ Create VM instance
   ├─ Execute recipe hooks
   └─ Return job ID
   ↓
6. Frontend polls: GET /install/{jobId}
   ↓
7. WebSocket: Stream logs in real-time
   ↓
8. VM ready / Error handling
```

### ISO-Based OS Installation

```
1. User uploads ISO
   ↓
2. API: POST /isos with file upload
   ├─ Verify integrity
   ├─ Store in S3/filesystem
   └─ Return ISO ID
   ↓
3. User creates VM from ISO
   ↓
4. API: POST /install with ISO ID
   ├─ Create VM with ISO as boot source
   ├─ Mount ISO in VM
   └─ Boot VM into installer
   ↓
5. WebSocket: Stream installation console
   ↓
6. User configures installation (keyboard, disk, etc.)
   ↓
7. Installation completes
   ├─ Eject ISO
   ├─ Reboot into installed OS
   └─ Mark VM ready
```

---

## Database Integration

### New Tables

```
recipes
recipe_executions
isos
installation_jobs
```

### Modified Tables

```
instances
  - Added: recipe_id (FOREIGN KEY)
  - Added: installation_job_id (FOREIGN KEY)
  - Added: iso_id (FOREIGN KEY)
```

### Migrations

- M001: Create recipes table
- M002: Create recipe_executions table
- M003: Create isos table
- M004: Create installation_jobs table
- M005: Alter instances table with recipe references

---

## Async Job Management

### Job Queue Strategy

**Option A**: Database-driven with polling
- Store jobs in database
- API polls for status
- Simple, no external dependencies
- Slight latency (polling interval)

**Option B**: Message queue-based (RabbitMQ/Redis)
- Better for high volume
- Requires additional infrastructure
- Lower latency

**Recommendation**: Start with Option A, migrate to Option B if needed

### Job Lifecycle

```
PENDING → RUNNING → SUCCESS/FAILED/CANCELLED

Polling frequency: 1 second (configurable)
Timeout: 1 hour (configurable per recipe)
Automatic retry: 3 attempts with exponential backoff
```

---

## Error Handling & Recovery

### Recipe Validation Failures
- Return detailed error list
- Suggest corrections
- Allow user to adjust configuration

### Recipe Execution Failures
- Log full error context
- Attempt automatic recovery (retry)
- Fall back to manual intervention
- Provide rollback option

### ISO Installation Failures
- Offer retry with logging enabled
- Allow manual takeover (direct console access)
- Support resume from checkpoint (if available)

---

## Security Considerations

### Recipe Validation & Sandboxing
- Validate recipe syntax before execution
- Sandboxed execution environment
- No direct filesystem/network access except for provisioning
- Script content review for community recipes

### Secret Management
- Store recipe secrets in encrypted database field
- Support secret injection via environment variables
- Never log secrets
- Rotation support

### Authorization & Auditing
- Recipes tagged by origin (built-in, community, custom)
- Execution audit trail
- User attribution for all operations
- Admin approval workflow for community recipes

---

## Performance Optimization

### Caching
- Cache recipe metadata (5 min TTL)
- Cache built-in recipe library (1 hour TTL)
- Cache validation results (until recipe changes)

### Async Operations
- All long operations run async with job tracking
- WebSocket for real-time updates (reduces polling)
- Background workers for recipe parsing and validation

### Resource Limits
- Max recipe size: 10MB
- Max concurrent installations: 10 per hypervisor
- Max recipe execution time: 2 hours
- ISO file size: Varies by hypervisor (typically 5-20GB)

---

## Extensibility

### Custom Recipe Hooks
Allow recipes to define custom hooks for application-specific setup
- Hook execution order
- Hook dependencies
- Hook timeout handling

### Community Recipe Repository
- Recipe sharing platform
- Marketplace discovery
- Version management
- Dependency tracking
- Rating/review system

### Recipe Templates
- Base recipe templates for common scenarios
- Inheritance and composition
- Variable interpolation
- Conditional sections

---

## Migration Strategy

### Phase 1: Core Foundation (Done)
- Database schema
- Basic API endpoints
- Recipe validator
- Simple executor

### Phase 2: Built-in Recipes (Week 3-4)
- PiHole, *arr suite, Minecraft, etc.
- Recipe library structure
- Template system

### Phase 3: Frontend UI (Week 5-6)
- Recipe browser
- Configuration forms
- Upload interfaces

### Phase 4: Advanced Features (Week 7-10)
- Custom recipes
- Community features
- Automation

### Phase 5: Polish & Launch (Week 11-12)
- Testing
- Documentation
- Performance optimization

---

## Success Metrics

- Recipe deployment <10 clicks
- VM creation success rate >95%
- Installation progress accuracy >99%
- Clear error messages for failures
- Documentation complete and clear
- Community engagement positive

---

**Document Version**: 1.0  
**Created**: 2026-07-01  
**Status**: Architecture Review Ready
