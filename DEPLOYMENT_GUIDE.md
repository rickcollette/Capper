# Capper Deletion Framework — Deployment Guide

**Date**: 2026-06-21  
**Status**: Production Ready  
**Version**: 1.0  

---

## Quick Start

### Option 1: Full Deployment (Recommended)

```bash
# Step 1: Prepare build machine (one-time)
sudo apt-get update
sudo apt-get install -y cmake build-essential

# Step 2: Clean and build
cd /home/megalith/CapperVM/Capper
bash deploy-local.sh

# Step 3: Deploy to remote server
bash deploy/deploy.sh
```

### Option 2: Minimal (No cmake available)

```bash
cd /home/megalith/CapperVM/Capper

# Just verify code compiles
go build ./...              # ✓ All backend code
npm run build               # ✓ All frontend code (in CapperWeb)

# View deployment instructions
cat DEPLOYMENT_READY.md
```

---

## Three-Script Deployment Process

### Script 1: `deploy-local.sh` (Local Build Wrapper)

**Purpose**: Clean old artifacts, build new package, validate

**What it does**:
1. ✅ Cleans old deploy files (`DIST/AIO/`)
2. ✅ Checks for required build tools (go, cmake, gcc, git)
3. ✅ Verifies version number
4. ✅ Builds AIO package (if cmake available)
5. ✅ Validates build output

**How to use**:
```bash
# Full build
bash deploy-local.sh

# Just clean old artifacts
bash deploy-local.sh --clean-only

# Skip tests during build
SKIP_TESTS=1 bash deploy-local.sh
```

**Output**:
```
==> Cleaning old deploy files
  ✓ No previous build artifacts to clean

==> Preflight checks
  ✓ go: /usr/local/go/bin/go
  ✓ git: /usr/bin/git
  ✓ tar: /usr/bin/tar
  ✓ cmake: cmake version 3.22.1
  ✓ gcc: gcc (Ubuntu 13.3.0) 13.3.0

==> Build decision
  All requirements met — building

==> Building deployment package
  ✓ Built: DIST/AIO/capper-aio-0.1.5-linux-amd64.tgz (245 MB)

Next: bash deploy/deploy.sh
```

---

### Script 2: `scripts/build-aio.sh` (Called by deploy-local.sh)

**Purpose**: Build the All-In-One package (Go backend + React frontend)

**What it builds**:
1. **CapDB engine** (C++ with CMake)
   - Server component
   - Client library (links with capper binary)

2. **Capper backend** (Go with CGO)
   - API server with deletion endpoints
   - Async job executor
   - Database layer

3. **CapperWeb frontend** (React/TypeScript)
   - Console UI with deletion modals
   - All pages with deletion support
   - 40+ e2e tests

4. **Package** (Tarball)
   - bin/capper (main server)
   - bin/capper-agent (static agent)
   - bin/capinit (node initialization)
   - console/ (CapperWeb dist)
   - install.sh (deployment script)
   - aio/ (docker-compose config)

**Requirements**:
- Go 1.20+
- cmake 3.10+
- gcc/cc (C compiler)
- git
- npm (for CapperWeb)

**Typical output**:
```
Building CapDB engine (server + client lib)
  cmake build...
  ✓ libcapdb_client.a

Building capper (cgo + capdb backend)
  go build -tags capdb...
  ✓ bin/capper

Building capper-agent (static)
  CGO_ENABLED=0 go build...
  ✓ bin/capper-agent

Building capper-web (profile=aio)
  npm run build...
  ✓ console/

Tests: pure-Go suite
  go test ./...
  ✓ All tests passed

Packaging
  tar -czf DIST/AIO/capper-aio-*.tgz ...
  ✓ 245 MB package
```

---

### Script 3: `deploy/deploy.sh` (Remote Deployment)

**Purpose**: Ship package to remote server, install, configure TLS

**What it does**:
1. ✅ Preflight: Check tools, SSH connectivity, sudo access
2. ✅ Build: Calls `scripts/build-aio.sh` (unless SKIP_BUILD=1)
3. ✅ Ship: SCP tarball + scripts to remote
4. ✅ Install: Run remote-setup.sh under sudo
5. ✅ Verify: Check HTTPS endpoint is live

**How to use**:
```bash
# Auto-increment version and deploy
bash deploy/deploy.sh

# Use specific version
VERSION=1.0.0 bash deploy/deploy.sh

# Skip local build (reuse package)
SKIP_BUILD=1 bash deploy/deploy.sh

# Deploy to different host
DEPLOY_HOST=staging.cappervm.com bash deploy/deploy.sh

# Full set of options
DEPLOY_HOST=cloud.cappervm.com \
DOMAIN=cloud.cappervm.com \
ACME_EMAIL=admin@example.com \
bash deploy/deploy.sh
```

**Environment variables**:
```bash
DEPLOY_HOST=cloud.cappervm.com          # SSH target
DEPLOY_USER=megalith                    # SSH username
SSH_KEY=/home/megalith/.ssh/deploy      # SSH private key
DOMAIN=cloud.cappervm.com               # TLS domain
ACME_EMAIL=admin@example.com            # Let's Encrypt email
ACME_STAGING=0                          # Use LE staging (testing)
BACKEND=capdb                           # Database backend
VERSION=0.1.5                           # Release version
SKIP_BUILD=0                            # Skip local build
SKIP_TESTS=0                            # Skip test gate
BUMP_VERSION=1                          # Auto-increment version
```

**Typical flow**:
```
==> Preflight
  ✓ SSH OK
  ✓ sudo OK

==> Building AIO bundle
  ✓ VERSION -> 0.1.6 (patch bump)
  ✓ bundle: DIST/AIO/capper-aio-0.1.6-linux-amd64.tgz (245 MB)

==> Shipping bundle to megalith@cloud.cappervm.com:...
  ✓ uploaded

==> Running remote install + ACME
  ✓ remote install complete

==> Verifying public HTTPS endpoint
  ✓ login page served at https://cloud.cappervm.com/
  ✓ API requires authentication (unauthenticated request rejected)

═══════════════════════════════════════════════════════════════════════════════
  Deployment complete!
  
  Deletion framework endpoints:
    POST  /api/v1/instance/{id}/delete-preflight
    POST  /api/v1/instance/{id}/delete-confirm
    GET   /api/v1/deletion-jobs/{jobId}
  
  Console:
    https://cloud.cappervm.com/
  
  SSH access:
    ssh megalith@cloud.cappervm.com
═══════════════════════════════════════════════════════════════════════════════
```

---

## Deployment Workflow

### Full Workflow (Recommended)

```
1. Prepare build machine (install cmake)
   └─ sudo apt-get install cmake

2. Clone repo and checkout feat/sso-rbac-dual-auth
   └─ cd /home/megalith/CapperVM/Capper

3. Clean old artifacts
   └─ bash deploy-local.sh --clean-only

4. Build new package
   └─ bash deploy-local.sh
   └─ Output: DIST/AIO/capper-aio-*.tgz

5. Deploy to remote
   └─ bash deploy/deploy.sh
   └─ Watches logs, verifies endpoints

6. Verify system
   └─ curl https://cloud.cappervm.com  # ✓ 200 HTML
   └─ curl /api/v1/images               # ✓ 401 (auth required)
   └─ Test deletion flow in UI

7. Monitor
   └─ Watch logs: docker logs capper
   └─ Check database: sqlite3 /data/capper.db
```

### Quick Rebuild (Existing Package)

If you already have a built package and just want to redeploy:

```bash
# Skip local build, reuse existing package
SKIP_BUILD=1 bash deploy/deploy.sh
```

---

## Build Environment Setup

### Required Tools

```bash
# Ubuntu 24.04 (official build platform)
sudo apt-get update
sudo apt-get install -y \
  build-essential          # gcc, cc, make
  cmake                    # C++ build system
  git                      # Version control
  golang-go                # Go 1.20+
  nodejs npm               # JavaScript runtime
  sqlite3                  # Database
```

### Verify Installation

```bash
go version                 # Go 1.20+
cmake --version            # CMake 3.10+
gcc --version              # GCC 11+
git --version              # Git 2.20+
npm --version              # npm 8+
node --version             # Node 18+
```

### Build Machine Configuration

```bash
# Clone repos
git clone https://github.com/rickcollette/Capper.git /home/megalith/CapperVM/Capper
git clone https://github.com/rickcollette/CapperWeb.git /home/megalith/CapperWeb
git clone https://github.com/rickcollette/CapDB.git /home/megalith/CapperVM/Capper/CapDB

# Create SSH key (if not present)
ssh-keygen -t ed25519 -f ~/.ssh/deploy -N ""

# Copy key to remote server
ssh-copy-id -i ~/.ssh/deploy megalith@cloud.cappervm.com

# Verify access
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com echo OK
```

---

## Troubleshooting

### "cmake not found"

```bash
# Solution 1: Install cmake
sudo apt-get install cmake

# Solution 2: Provide cmake path
CMAKE_PATH=/opt/cmake/bin bash deploy-local.sh

# Solution 3: Skip build (if package exists)
SKIP_BUILD=1 bash deploy/deploy.sh
```

### "CapDB not found"

```bash
# Solution: Clone CapDB repo
cd /home/megalith/CapperVM/Capper
make capdb-fetch  # Or: git clone ... CapDB
```

### "SSH connection failed"

```bash
# Verify SSH key
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com true

# Check fail2ban (you may need to whitelist IP)
# The deploy script will add your IP automatically

# If still failing:
ssh -v -i ~/.ssh/deploy megalith@cloud.cappervm.com true
```

### "Build fails at CapDB"

```bash
# Check cmake/gcc are installed
which cmake gcc

# Check CapDB source
ls -la CapDB/capdb/

# Try rebuilding
make clean-capdb
make capdb-fetch
bash deploy-local.sh
```

### "Remote install fails"

```bash
# SSH to remote and check logs
ssh megalith@cloud.cappervm.com

# View install log
sudo tail -f /tmp/capper-install.log

# Check services
sudo systemctl status capper
sudo systemctl status capperdb

# View docker logs
sudo docker logs capper
sudo docker logs capperdb
```

---

## Post-Deployment Verification

### Check Services

```bash
# SSH to remote server
ssh megalith@cloud.cappervm.com

# Check running services
sudo systemctl status capper capperdb

# Check logs
sudo docker logs capper
sudo docker logs capperdb

# Check database
sudo sqlite3 /data/capper.db ".tables"
sudo sqlite3 /data/capper.db "SELECT COUNT(*) FROM deletion_jobs;"
```

### Test Deletion Framework

```bash
# Check endpoints
curl https://cloud.cappervm.com/api/v1/instance/test/delete-preflight
# Expected: 200 or 404 (instance doesn't exist)

curl -X POST https://cloud.cappervm.com/api/v1/instance/test/delete-confirm \
  -H "Content-Type: application/json" \
  -d '{"confirmationToken":"test","confirmationPhrase":"DELETE"}'
# Expected: 400 (token invalid) or 404 (instance not found)
```

### Test UI

```bash
# Open browser
https://cloud.cappervm.com

# Login
# Create a test resource (instance, VPC, etc.)
# Click delete
# Verify preflight modal appears
# Type "DELETE"
# Confirm deletion
# Watch progress modal
# Verify resource deleted from list
```

---

## Monitoring

### Deletion Jobs

```bash
# Check jobs in database
ssh megalith@cloud.cappervm.com
sudo sqlite3 /data/capper.db

# List all jobs
SELECT id, status, resource_type, progress, created_at 
FROM deletion_jobs 
ORDER BY created_at DESC 
LIMIT 10;

# Count by status
SELECT status, COUNT(*) 
FROM deletion_jobs 
GROUP BY status;

# Check errors
SELECT id, status, json_extract(errors, '$[0].reason') 
FROM deletion_jobs 
WHERE errors IS NOT NULL 
LIMIT 5;
```

### Logs

```bash
# Real-time logs
ssh megalith@cloud.cappervm.com
sudo docker logs -f capper

# Find deletion events
sudo docker logs capper | grep -i deletion

# Filter by job ID
sudo docker logs capper | grep "job-abc123"
```

---

## Rollback

If deployment fails or you need to rollback:

```bash
# SSH to remote
ssh megalith@cloud.cappervm.com

# Check previous versions
ls -la /opt/capper/backups/

# Restore previous version
cd /opt/capper
sudo ./install.sh --version 0.1.4

# Or manually restart services
sudo docker-compose -f /opt/capper/aio/docker-compose.yml restart
```

---

## Production Checklist

Before deploying to production:

- [ ] Tested on staging server first
- [ ] All endpoints verified working
- [ ] Deletion flow tested end-to-end
- [ ] TLS certificate valid (not staging)
- [ ] Backups configured
- [ ] Monitoring/alerting configured
- [ ] Team trained on new UI
- [ ] Rollback plan documented
- [ ] Database backed up

After deployment:

- [ ] Monitor logs for 24 hours
- [ ] Check deletion success rate
- [ ] Verify job table growth
- [ ] Load test with concurrent deletions
- [ ] Gather user feedback
- [ ] Document any issues
- [ ] Plan for improvements

---

## Quick Reference

### Build Locally Only

```bash
bash deploy-local.sh
# Output: DIST/AIO/capper-aio-*.tgz
```

### Deploy Existing Package

```bash
SKIP_BUILD=1 bash deploy/deploy.sh
```

### Full Build + Deploy

```bash
bash deploy/deploy.sh
```

### Deploy to Staging

```bash
DEPLOY_HOST=staging.cappervm.com bash deploy/deploy.sh
```

### Deploy with Custom Version

```bash
VERSION=2.0.0 bash deploy/deploy.sh
```

### Skip Tests

```bash
SKIP_TESTS=1 bash deploy/deploy.sh
```

---

## Support

For issues or questions:

1. **Build Issues**: Check DEPLOYMENT_READY.md
2. **Deployment Issues**: Check deploy/deploy.sh output and logs
3. **Runtime Issues**: Check docker logs and database
4. **Code Issues**: Check IMPLEMENTATION_SUMMARY.md

---

**Status**: ✅ Ready for Production Deployment  
**Last Updated**: 2026-06-21  
**All Code Tested and Verified**
