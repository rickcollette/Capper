---
title: "Capper Documentation"
description: "Documentation for the Capper private cloud platform."
owner: "docs"
status: "stable"
reviewed: "2026-06-21"
outputs:
  - markdown
  - web
  - pdf
---

# Capper Documentation

Capper is a Go-based, self-hosted, multi-tenant private cloud platform — a complete control plane and runtime stack you own and operate in a single binary.

**Current Version:** 0.1.38 | **Status:** Production-Ready

## What is Capper?

Capper provides:

- **Compute** — Capsule instances with bwrap/chroot/crun/runc isolation, image management, instance templates
- **Networking** — VPCs with dual-store architecture, subnets, security groups, firewalls, load balancers, DNS, public IPAM
- **Storage** — S3-compatible object storage, block volumes, distributed filesystem, snapshots, backups
- **Resource Deletion** — 3-phase framework with cascading deletion, progress tracking, and error recovery
- **IAM** — Multi-tenancy (orgs → accounts → projects), RBAC, OAuth2/SSO, token management, policy enforcement
- **Databases** — Managed database provisioning with automatic backups and replication
- **Certificates** — ACME/Let's Encrypt integration, renewal automation, internal CA
- **Observability** — Resource inventory, config drift detection, event logging, audit trails, metrics
- **Serverless** — Lambda-style functions with triggers, MCP server management
- **VPC Mobility** — Workload migration across realms/regions with full validation and rollback

## Recent Improvements (v0.1.34-0.1.38)

✅ **Deletion Framework** — Comprehensive 3-phase deletion with cascading support and progress tracking  
✅ **Dual-Store VPC Pattern** — Optimized architecture with canonical storage isolation  
✅ **Image Deployment** — Automated .cap image building and deployment in deploy/deploy.sh  
✅ **UI Fixes** — React Query cache management, VPC ID validation, metadata endpoint robustness  
✅ **Dead Code Cleanup** — Removed orphaned table definitions from vpc package  

## Quick Navigation

### For Operators
- [Deployment Guide](operator-guide/deployment.md) — Deploy Capper to cloud.cappervm.com
- [All-in-One Node](operator-guide/aio-node.md) — Single-node setup with CapDB
- [Upgrades](operator-guide/upgrades.md) — Seamless version upgrades with rollback
- [Routable IPs](operator-guide/routable-ips.md) — Public IP management and allocation
- [Certificates](operator-guide/certificates.md) — TLS certificate lifecycle
- [Multi-tenancy](operator-guide/multi-tenancy.md) — Organizations, accounts, projects, IAM

### For Users
- [Getting Started](getting-started/overview.md) — First steps with Capper
- [Instance Management](user-guide/instances.md) — Launch, manage, delete capsules
- [Networking](user-guide/networking.md) — VPC creation, subnets, security groups
- [Resource Deletion](user-guide/deletion-framework.md) — Safe deletion with confirmations
- [CLI Guide](reference/cli/capper.md) — Command-line interface reference

### For Developers
- [Developer Guide](developer-guide/index.md) — Architecture, build, testing
- [API Reference](reference/api/overview.md) — REST API endpoints
- [Storage Architecture](architecture/storage.md) — SQLite and CapDB backends
- [Repository Layout](developer-guide/repository-layout.md) — Code organization
- [SDK Guide](developer-guide/sdk.md) — Go SDK for programmatic access

### Architecture
- [System Overview](architecture/system-overview.md) — Control plane and subsystems
- [VPC Architecture](architecture/vpc.md) — Dual-store pattern and networking design
- [Deletion Framework](architecture/deletion.md) — 3-phase deletion with cascading support
