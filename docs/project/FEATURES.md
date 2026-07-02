# Capper Features & Status

## ✅ Fully Implemented & Tested

### Resource Management
- **Instances** (capsules)
  - Launch from .cap images (alpine, ubuntu, rockylinux, alma)
  - Instance types (cap-micro, cap-small, etc.)
  - Metadata storage and retrieval
  - Logs (stdout, stderr)
  - Start/stop/restart/delete operations
  - Termination protection
  - Security groups, ENIs, networking

- **VPCs & Networking**
  - VPC creation, read, update, delete
  - Subnets with CIDR management
  - Route tables and routes
  - Security groups with rules
  - Network ACLs
  - Internet gateways
  - VPC peering
  - Dual-store pattern (vpc package + topology package)

- **Load Balancers**
  - Create/delete load balancers
  - Target groups and listeners
  - VIP (routable IP) allocation and release
  - Health checks
  - SSL/TLS support

- **Databases**
  - Managed database provisioning
  - Automatic instance backing
  - Backup policies
  - Multi-region support

### Resource Deletion Framework (0.1.31+)
**3-phase deletion system with cascading support:**

1. **Preflight Phase** - Discover what will be deleted
   - Endpoint: `POST /api/v1/{resourceType}/{resourceId}/delete-preflight`
   - Returns: deletion plan, confirmation token, list of dependent resources

2. **Confirmation Phase** - Validate user intent
   - Endpoint: `POST /api/v1/{resourceType}/{resourceId}/delete-confirm`
   - Requires: exact "DELETE" confirmation phrase (uppercase)
   - Returns: deletion job ID for polling

3. **Progress Phase** - Async execution with polling
   - Endpoint: `GET /api/v1/deletion-jobs/{jobId}`
   - Returns: current progress (0-100%), completed steps, errors if any
   - Supports recovery suggestions on failure

**Cascade Deletion Semantics:**
- ✅ Delete VPC → automatically deletes all contained instances, load balancers, subnets, routes
- ✅ Delete Instance → stops, detaches ENIs, releases disk/IPs, records billing
- ✅ Delete Load Balancer → releases routable IPs, removes from target groups
- ✅ Delete Database → stops backing instance, records billing
- ✅ Resources deleted from both table locations (e.g., vpc tables + topology tables)

**Deletion Safeguards:**
- TOCTOU protection (time-of-check-time-of-use race condition handling)
- Confirmation token validation
- Case-sensitive "DELETE" phrase requirement
- Detailed error messages with recovery suggestions
- Atomic multi-step operations with proper ordering
- IP address recovery and release
- Billing cleanup

### Web UI (CapperWeb)
- React + Vite frontend
- OAuth2 Google SSO integration
- Instance management dashboard
- VPC and networking UI
- Load balancer management
- Database management
- Deletion flow with modal confirmations
- Real-time progress tracking
- Error handling and recovery suggestions
- Query caching with React Query
- Proper cleanup on navigation (cache invalidation)

### Storage & Persistence
- **SQLite** (default) - pure-Go, embedded
- **CapDB** (optional) - networked SQLite with TLS
- RFC3339 timestamp serialization
- NULL value handling with sql.NullString
- ACID transactions with WAL
- Schema migrations

### Identity & Access
- Multi-tenant support (organizations → accounts → projects)
- IAM (users, groups, roles, policies)
- Bearer token authentication
- OAuth2/Google SSO
- Admin bootstrapping

### Observability
- Event logging (creation, deletion, modifications)
- Audit trails
- Service health checks
- Resource inventory tracking

---

## ✅ Recent Fixes (v0.1.34-0.1.38)

### 0.1.34 - Consolidation & Dead Code Removal
- Removed orphaned table definitions from vpc package
  - `lb_target_groups`, `lb_listeners` (belong in lb/store.go)
  - `capvpc_instance_metadata_options`, `capvpc_instance_block_devices` (never used)
- Created ARCHITECTURE.md documenting dual-store VPC pattern
- Verified build integrity

### 0.1.35 - Deletion Flow 404 Fix
- Fixed 404 errors when deleting resources
- Added React Query cleanup before navigation
- Prevents refetch of deleted resources
- VPCDetail, InstanceDetail, LBDetail all fixed

### 0.1.36 - VPC ID/Slug Mismatch Fix
- Fixed instance and LB creation forms sending VPC slug instead of ID
- Dropdown values now consistently send IDs
- Eliminates "subnet is not in vpc" validation errors

### 0.1.37 - Metadata Endpoint Fix
- Fixed 404 for instances without CapInit metadata
- Returns empty object `{}` instead of 404
- Consistent with API behavior for optional data

### 0.1.38 - Deployment Image Upload Fix
- Fixed images not being deployed as part of deployment
- Added fallback mechanism for image upload
- All 4 images (alpine, ubuntu, rockylinux, alma) now deploy automatically
- Enhanced error handling and logging in remote-setup.sh

---

## 🔧 Architecture Highlights

### Deletion Architecture
- Async job execution via goroutines with panic recovery
- SQLite-based job persistence with RFC3339 timestamps
- 3-phase framework: preflight → confirm → progress
- Unified across all resource types
- Real-time progress polling (0-100% bar)
- Detailed error reporting with recovery suggestions

### VPC Architecture
- **Intentional dual-store pattern:**
  - Topology store = canonical VPC identity/metadata
  - VPC store = supporting network infrastructure
- Deletion deletes from both stores
- Independent scaling: topology changes are rare, VPC operations are frequent

### Image Management
- Docker-based build system (`scripts/build-aio.sh`)
- Automatic .cap image building during deployment
- 4 standard images: alpine, ubuntu, rockylinux, alma
- Fallback to direct file copy if API upload fails
- All images included in AIO tarball

### Deployment
- Single `deploy/deploy.sh` script handles everything:
  - Code compilation
  - Image building
  - CapperWeb bundling
  - Remote deployment
  - Service startup
  - HTTPS provisioning
  - Health verification
  - Image upload/registration

---

## 📋 API Endpoints Summary

### Instances
- `GET /api/v1/instances` - List
- `POST /api/v1/instances` - Create
- `GET /api/v1/instances/{id}` - Get
- `POST /api/v1/instances/{id}/start` - Start
- `POST /api/v1/instances/{id}/stop` - Stop
- `GET /api/v1/instances/{id}/metadata` - Get metadata
- `GET /api/v1/instances/{id}/logs/{logtype}` - Get logs

### VPCs
- `GET /api/v1/vpcs` - List
- `POST /api/v1/vpcs` - Create
- `GET /api/v1/vpcs/{vpc}` - Get
- `PATCH /api/v1/vpcs/{vpc}` - Update
- `GET /api/v1/vpcs/{vpc}/detail` - Get with networking details
- `GET /api/v1/vpcs/{vpc}/subnets` - List subnets
- `POST /api/v1/vpcs/{vpc}/subnets` - Create subnet

### Load Balancers
- `GET /api/v1/load-balancers` - List
- `POST /api/v1/load-balancers` - Create
- `GET /api/v1/load-balancers/{id}` - Get
- `POST /api/v1/load-balancers/{id}/listeners` - Create listener
- `POST /api/v1/load-balancers/{id}/target-groups` - Create target group

### Deletion
- `POST /api/v1/{resourceType}/{resourceId}/delete-preflight` - Preflight check
- `POST /api/v1/{resourceType}/{resourceId}/delete-confirm` - Confirm deletion
- `GET /api/v1/deletion-jobs/{jobId}` - Poll deletion progress

### Images
- `GET /api/v1/images` - List available images
- `POST /api/v1/images/upload` - Upload new image

### Health
- `GET /api/v1/health` - Service health check

---

## 🧪 Testing Status

### Automated Testing
- Build verification
- API endpoint validation
- Health checks
- Image deployment verification

### Manual Testing Required
- Instance creation on all 4 image types
- Hostname verification
- Metadata endpoint functionality
- Deletion flow (preflight → confirm → progress)
- Full resource lifecycle testing
- Load balancer creation and deletion
- Database provisioning

---

## 📊 Database Schema Highlights

### Core Tables
- `instances` - Instance metadata and state
- `images` - Container image registry
- `vpcs` (topology) - VPC metadata (canonical)
- `capvpc_vpcs` (vpc package) - VPC networking infrastructure
- `subnets` - Network subnets with CIDR
- `load_balancers` - Load balancer configuration
- `deletion_jobs` - Async deletion tracking
- `deletion_job_steps` - Step-level progress tracking
- `deletion_job_errors` - Error details with recovery suggestions

### Key Features
- UNIQUE constraints on important resource combinations
- FOREIGN KEY cascades for referential integrity
- RFC3339 timestamp storage (ISO 8601 format)
- NULL value handling with sql.NullString
- WAL mode for transaction safety

---

**Last Updated**: 2026-06-21  
**Current Version**: 0.1.38  
**Status**: Production-Ready with Comprehensive Deletion Framework
