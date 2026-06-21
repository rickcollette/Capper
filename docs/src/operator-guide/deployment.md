---
title: Deployment Guide
description: Deploy Capper to cloud.cappervm.com using deploy/deploy.sh
owner: engineering
status: stable
reviewed: 2026-06-21
---

# Deployment Guide

Capper deployments are fully automated via `deploy/deploy.sh`, which handles compilation, image building, packaging, and remote installation in a single command.

## Overview

The deployment process:
1. **Build** — Compiles Go binaries, builds CapperWeb, builds .cap images
2. **Package** — Creates AIO (All-In-One) tarball with all components
3. **Ship** — Copies tarball and setup script to remote host
4. **Install** — Runs setup on remote, provisions TLS cert, starts services
5. **Verify** — Health checks and endpoint validation

**Time:** ~10-15 minutes end-to-end  
**Requirements:** Docker (for image building), SSH access to remote host

---

## Prerequisites

### Local Machine
- Go 1.22+
- Docker (for building .cap images)
- SSH client
- SSH deploy key at `~/.ssh/deploy` (or custom path via SSH_KEY env)

### Remote Host
- Ubuntu 24.04 (supported; others may work)
- SSH access with sudo privilege
- Open ports: 80 (HTTP), 443 (HTTPS)
- ~10GB free disk space
- Internet access (for Let's Encrypt ACME)

---

## Quick Deploy

### 1. Simple Deployment

```bash
cd /path/to/Capper
deploy/deploy.sh
```

This deploys to `cloud.cappervm.com` with:
- Auto-bumped version
- Docker-built .cap images (alpine, ubuntu, rockylinux, alma)
- CapperWeb UI
- Self-signed Let's Encrypt certificate
- Google OAuth2 (if configured)

### 2. Custom Host

```bash
DEPLOY_HOST=my-server.example.com deploy/deploy.sh
```

### 3. Custom User & Key

```bash
DEPLOY_USER=ubuntu SSH_KEY=/path/to/key deploy/deploy.sh
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEPLOY_HOST` | cloud.cappervm.com | SSH host for deployment |
| `DEPLOY_USER` | megalith | SSH user |
| `SSH_KEY` | ~/.ssh/deploy | SSH private key path |
| `DOMAIN` | {DEPLOY_HOST} | TLS certificate domain |
| `ACME_EMAIL` | rcollet@gmail.com | Let's Encrypt contact email |
| `ACME_STAGING` | 0 | Use Let's Encrypt staging (1=yes, 0=prod) |
| `BACKEND` | capdb | Database backend (capdb or sqlite) |
| `SKIP_BUILD` | 0 | Skip local build, reuse existing AIO tarball |
| `SKIP_TESTS` | 0 | Skip build-time tests |
| `BUMP_VERSION` | 1 | Auto-bump patch version |
| `VERSION` | auto | Explicit version (overrides bump) |
| `OAUTH_ENV_FILE` | deploy/oauth2.env | OAuth2 credentials file |

### Full Custom Deployment

```bash
DEPLOY_HOST=vm.company.com \
DEPLOY_USER=capper \
SSH_KEY=/etc/ssh/keys/deploy \
DOMAIN=vm.company.com \
ACME_EMAIL=ops@company.com \
BACKEND=capdb \
deploy/deploy.sh
```

---

## What Gets Built

### Stage 1: Local Build (`scripts/build-aio.sh`)
- **Capper Control Plane** (`cmd/capper`) — REST API, control plane
- **Capper Agent** (`cmd/capper-agent`) — Node agent daemon
- **CapInit** (`cmd/capinit`) — Instance initialization
- **CapDB** (if `-tags capdb`) — Networked database backend
- **CapperWeb** — React + Vite UI
- **Container Images:**
  - `alpine.cap` (15 MB)
  - `ubuntu.cap` (36 MB)
  - `rockylinux.cap` (69 MB)
  - `alma.cap` (76 MB)

### Output
- `DIST/AIO/capper-aio-0.1.38-linux-amd64.tgz` (~217 MB with images)
- `DIST/AIO/capper-aio-0.1.38-linux-amd64.tgz.sha256` (checksums)

---

## What Gets Deployed

### Remote Installation (`deploy/remote-setup.sh`)

#### 1. System Setup
- Installs required packages (systemd, curl, etc.)
- Creates service user and directories
- Sets up SQLite database
- Provisions CapDB if selected

#### 2. Service Installation
- Installs binaries into `/opt/capper/`
- Creates symlinks for versioning
- Atomic version flip (no downtime)
- Installs systemd units

#### 3. Image Registration
- Uploads 4 .cap images via API
- Fallback to direct copy if API unavailable
- Creates image index in database

#### 4. TLS Certificate
- Installs certbot
- Obtains Let's Encrypt certificate
- Configures nginx reverse proxy
- Sets up auto-renewal

#### 5. OAuth2 (Optional)
- Installs oauth2-proxy if credentials provided
- Configures Google SSO
- Sets allowed domains

#### 6. Service Startup
- Starts capper-control (main API)
- Starts capper-agent (local node)
- Starts CapDB (if selected)
- Starts nginx
- Verifies health

---

## Monitoring Deployment

### View Output
```bash
# See full deployment output
deploy/deploy.sh 2>&1 | tee deploy.log

# Watch remote logs during deployment
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "sudo tail -f /var/log/syslog"
```

### Check Progress
```bash
# Health check
curl https://cloud.cappervm.com/api/v1/health

# Service status
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "sudo systemctl status capper-control"

# API endpoints
curl -s https://cloud.cappervm.com/api/v1/health | jq .
```

---

## Verification

After deployment completes, verify:

### 1. Services UP
```bash
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "capper aio status"
```

Expected output:
```
control plane: UP
capdb: UP (localhost:5432)
agent: UP
```

### 2. HTTPS Working
```bash
curl -I https://cloud.cappervm.com/
# HTTP 200 OK
```

### 3. Images Available
```bash
curl -H "Authorization: Bearer $TOKEN" \
  https://cloud.cappervm.com/api/v1/images | jq .
```

Expected: 4 images (alpine, ubuntu, rockylinux, alma)

### 4. Web UI Accessible
```
https://cloud.cappervm.com/
# Should show login page
```

---

## Troubleshooting

### Build Fails: "docker not found"
**Cause:** Docker required for building .cap images  
**Solution:**
```bash
# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Skip Docker images (fallback to direct copy)
# Set SKIP_IMAGE=1 in build-aio.sh
```

### Deployment Fails: "SSH key permission denied"
**Cause:** SSH key permissions too open (>600)  
**Solution:**
```bash
chmod 600 ~/.ssh/deploy
```

### TLS Certificate Fails
**Cause:** DNS not resolving or firewall blocking ACME  
**Solution:**
```bash
# Test DNS
nslookup cloud.cappervm.com

# Test HTTP access
curl -I http://cloud.cappervm.com/

# Check firewall rules
sudo iptables -L | grep -E "80|443"
```

### Images Not Available
**Cause:** Image upload failed during setup  
**Solution:**
```bash
# Check remote logs
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "sudo journalctl -u capper-control | grep image"

# Manually upload images
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "sudo ls -lh /var/lib/capper/images/"
```

### Service Won't Start
**Cause:** Port already in use or permission issues  
**Solution:**
```bash
# Check port usage
sudo lsof -i :8080  # API default
sudo lsof -i :443   # HTTPS

# Check service logs
sudo journalctl -u capper-control -n 50

# Check permissions
sudo ls -ld /var/lib/capper/
```

---

## Rolling Updates

### Upgrade to New Version

```bash
# Version auto-bumps (0.1.38 → 0.1.39)
deploy/deploy.sh

# The script handles:
# - Atomic binary switch (no downtime)
# - Database migrations
# - Config updates
# - Service restart
```

### Rollback

If new version has issues:
```bash
# Check available versions
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "ls -la /opt/capper/"

# Switch to previous version
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "sudo ln -sf /opt/capper/0.1.37 /opt/capper/current && \
   sudo systemctl restart capper-control"
```

---

## Configuration

### OAuth2 Setup

Create `deploy/oauth2.env`:
```bash
OAUTH2_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
OAUTH2_CLIENT_SECRET=your-google-client-secret
ALLOWED_DOMAINS=example.com,team@company.com
```

Then:
```bash
deploy/deploy.sh
```

### Custom Port

Edit `deploy/remote-setup.sh` or set environment:
```bash
# In remote-setup.sh, line ~180
API_ADDR=127.0.0.1:8686
```

---

## Data Persistence

### Database
- Located at: `/var/lib/capper/capper.db` (SQLite)
- Or: CapDB backend at `/var/lib/capper/capdb/`
- Survives upgrades and restarts

### Stored Data
- Instance metadata
- VPC and networking configuration
- Load balancer definitions
- User accounts and IAM policies
- Instance images
- Deletion job history

### Backup

```bash
# Backup database
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "sudo cp /var/lib/capper/capper.db /backup/capper-$(date +%s).db"

# Backup entire data directory
ssh -i ~/.ssh/deploy megalith@cloud.cappervm.com \
  "sudo tar czf /backup/capper-$(date +%Y%m%d).tar.gz /var/lib/capper/"
```

---

## Performance & Scaling

### Single-Node Deployment
- Suitable for: Development, testing, small production (10-100 instances)
- Database: SQLite (embedded)
- CPU: 4+ cores recommended
- RAM: 8+ GB recommended
- Storage: 50+ GB recommended

### Networked Deployment
- Use CapDB for connection pooling and remote access
- Suitable for: Production with multiple agents
- Database: CapDB (separate server)
- Control plane can be dedicated hardware

---

## Security Considerations

### SSH Access
- Deploy key should be restricted to deployment user
- Use SSH agent or SSH config to manage key access
- Consider IP-whitelisting for SSH

### TLS Certificates
- Let's Encrypt certificates auto-renew
- Certificate pinning not recommended (cert changes yearly)
- Use system CA bundle for cert validation

### Firewall Rules
```bash
# Allow SSH (deployment)
sudo ufw allow 22/tcp

# Allow HTTP (Let's Encrypt ACME)
sudo ufw allow 80/tcp

# Allow HTTPS (API and Web UI)
sudo ufw allow 443/tcp

# Restrict API to internal network (optional)
# sudo ufw allow from 10.0.0.0/8 to any port 8080
```

---

**Version:** 0.1.38 | **Last Updated:** 2026-06-21  
**Status:** Production-Ready | **Tested:** Ubuntu 24.04 LTS
