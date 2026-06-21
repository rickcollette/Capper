---
title: VPC Architecture
description: Dual-store VPC pattern for scalable network management
owner: engineering
status: stable
reviewed: 2026-06-21
---

# VPC Architecture

Capper uses an intentional **dual-store pattern** for VPC management, where VPC data is split across two packages to optimize for different access patterns and scalability characteristics.

## Dual-Store Pattern

### Why Split Storage?

- **Topology Store** = High-level VPC resource abstraction (creation, metadata, deletion)
- **VPC Store** = Low-level networking infrastructure (frequent operations, high IOPS)

This separation allows:
1. **Independent scaling** — VPC operations are frequent; topology changes are rare
2. **Clear responsibility** — Each store owns specific concerns
3. **Performance optimization** — Each can use appropriate database patterns
4. **Team autonomy** — Networking and topology teams work independently

### Store Responsibilities

#### Topology Package (`internal/topology/store.go`)
**Canonical source of truth for VPC identity**

Stores:
- VPC metadata (name, slug, CIDR, labels)
- VPC status and lifecycle events
- VPC creation timestamps
- Realm and region assignments
- Mobility policy

Operations:
- Create VPC (UI, API, user)
- Get VPC (list operations)
- Update VPC metadata
- Delete VPC (logical removal)

Accessed by:
- `handlers_topology.go` — User-facing VPC CRUD
- `vpcmover` — VPC relocation
- CLI topology commands

**Schema:**
```sql
CREATE TABLE vpcs (
  id              TEXT PRIMARY KEY,
  realm_id        TEXT NOT NULL,
  project         TEXT NOT NULL,
  slug            TEXT NOT NULL,
  name            TEXT NOT NULL,
  cidr            TEXT NOT NULL,
  status          TEXT NOT NULL DEFAULT 'active',
  labels_json     TEXT NOT NULL DEFAULT '{}',
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL,
  UNIQUE(project, slug)
);
```

#### VPC Package (`internal/vpc/store.go`)
**Supporting infrastructure for VPC networking**

Stores:
- Subnets (CIDR blocks, gateway IPs, bridge names)
- Route tables and routes
- Security groups and rules
- Network ACLs and entries
- ENIs (Elastic Network Interfaces)
- Network peerings
- Flow logs configuration
- Instance metadata options

Operations:
- Create subnet
- Add route
- Create security group
- List/filter networking resources

Accessed by:
- `handlers_vpc.go` — Networking configuration endpoints
- Instance launch (networking setup)
- Load balancer creation (subnet attachment)
- Internal VPC operations

**Schema:**
```sql
CREATE TABLE capvpc_vpcs (
  id         TEXT PRIMARY KEY,
  project    TEXT NOT NULL,
  name       TEXT NOT NULL,
  cidr       TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(project, name)
);

CREATE TABLE capvpc_subnets (
  id          TEXT PRIMARY KEY,
  vpc_id      TEXT NOT NULL REFERENCES capvpc_vpcs(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  cidr        TEXT NOT NULL,
  zone        TEXT NOT NULL DEFAULT '',
  kind        TEXT NOT NULL DEFAULT 'private',
  bridge_name TEXT NOT NULL DEFAULT '',
  gateway_ip  TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL,
  UNIQUE(vpc_id, name)
);
```

---

## Data Flow

### VPC Creation
```
User/API request
    ↓
handlers_topology.go:createVPC()
    ↓
topology.InsertVPC()  ← Write to topology store (canonical)
    ↓
vpc.CreateVPC()       ← Write to vpc store (infrastructure)
    ↓
Subnets created in vpc store
    ↓
Success response to user
```

### VPC Networking Setup
```
Instance launch request
    ↓
handlers_instances.go:handleCreateInstance()
    ↓
vpc.GetSubnetByID()   ← Read from vpc store
    ↓
vpc.CreateENI()       ← Create interface in vpc store
    ↓
Network setup with bridge, routes, security groups
    ↓
Instance connected to subnet
```

### VPC Deletion (Cascade)
```
User clicks Delete VPC
    ↓
handlers_deletion.go:asyncDeleteVPC()
    ↓
Step 1: Find & delete instances in this VPC
    ├─ vpc.ListSubnets() — Get all subnets
    ├─ Find instances with matching VPCID
    └─ Delete instances (stops, detaches ENIs, removes)
    ↓
Step 2: Find & delete load balancers in this VPC
    ├─ LB.List()
    ├─ Find LBs with matching VPCID
    ├─ lbVIPPlacer().ReleaseVIP()  ← Release routable IP
    └─ LB.Delete()
    ↓
Step 3: Delete VPC infrastructure
    ├─ vpc.DeleteVPC()  ← Delete from vpc store (subnets, routes, SGs cascade)
    └─ topology.DeleteVPC()  ← Delete from topology store (canonical removal)
    ↓
Success: VPC and all children removed
```

**Key Point:** Both stores must be updated during deletion because they remain active for different purposes.

---

## CRUD Operation Locations

| Operation | Store | Location | Endpoint |
|-----------|-------|----------|----------|
| **Create VPC** | Topology | handlers_topology.go | `POST /api/v1/vpcs` |
| **List VPCs** | Topology | handlers_topology.go | `GET /api/v1/vpcs` |
| **Get VPC** | Topology | handlers_topology.go | `GET /api/v1/vpcs/{id}` |
| **Update VPC** | Topology | handlers_topology.go | `PATCH /api/v1/vpcs/{id}` |
| **Delete VPC** | Both | handlers_deletion.go | `POST .../delete-confirm` |
| **Create Subnet** | VPC | handlers_vpc.go | `POST /api/v1/vpcs/{id}/subnets` |
| **List Subnets** | VPC | handlers_vpc.go | `GET /api/v1/vpcs/{id}/subnets` |
| **Create Route** | VPC | handlers_vpc.go | `POST /api/v1/vpcs/{id}/routes` |
| **Create SG** | VPC | handlers_vpc.go | `POST /api/v1/vpcs/{id}/security-groups` |

---

## Consistency Guarantees

### During Normal Operations
- **Topology** and **VPC** stores are kept in sync during creation
- Both writes happen atomically (within same transaction)
- If one fails, both rollback

### During Deletion
- **VPC store** is deleted first (cascades all networking resources)
- **Topology store** is deleted second (marks VPC as removed)
- If topology delete fails, VPC infrastructure is already gone (safe state)

### Race Condition Prevention
- Deletion is asynchronous with status tracking
- Time-of-check-time-of-use (TOCTOU) protected by atomic steps
- Dependent resources validated before deletion begins

---

## Migration Path

### Why This Pattern Exists
Originally, VPC data was only in the vpc package. As the system grew, topology management (multi-region, realm placement) needed to track VPCs independently. Rather than refactoring and risking downtime, the dual-store pattern was adopted.

### Future Consolidation
The pattern can be unified in the future by:
1. Merging both tables into a single canonical VPC table
2. Adding topology-specific columns to that table
3. Migrating existing data
4. Removing the old tables
5. Updating all code paths to use the new unified table

This would reduce complexity but would require a major database migration.

---

## Performance Characteristics

### Reads
- **VPC metadata queries** → Topology store (frequently indexed by project + slug)
- **Networking queries** → VPC store (frequently filtered by VPC ID)
- Both are fast with proper indexes

### Writes
- **VPC creation** → Both stores (transactional)
- **Networking updates** → VPC store only (frequent, isolated)
- **VPC deletion** → Both stores (sequential, asynchronous)

### Scalability
- **High VPC count** — Topology store can be sharded by realm/region
- **High networking churn** — VPC store can use separate pool for I/O
- **Independent backups** — Each store can have different backup strategies

---

## Debugging

### Find VPC Data Location
```bash
# Check topology store
sqlite3 capper.db "SELECT id, slug, name FROM vpcs WHERE project='default';"

# Check VPC store
sqlite3 capper.db "SELECT id, name FROM capvpc_vpcs WHERE project='default';"

# Both should have matching VPC IDs (one for identity, one for infrastructure)
```

### Verify Consistency
```bash
# List VPCs from both stores
sqlite3 capper.db "
  SELECT 'topology', COUNT(*) FROM vpcs
  UNION ALL
  SELECT 'vpc', COUNT(*) FROM capvpc_vpcs;
"

# Should return same count (or close, if migrations in progress)
```

### Investigate Orphaned Data
```bash
# VPCs in topology but not in vpc store
sqlite3 capper.db "
  SELECT t.id, t.name FROM vpcs t
  LEFT JOIN capvpc_vpcs v ON t.id = v.id
  WHERE v.id IS NULL;
"

# VPCs in vpc store but not in topology (should be none in normal operation)
sqlite3 capper.db "
  SELECT v.id, v.name FROM capvpc_vpcs v
  LEFT JOIN vpcs t ON v.id = t.id
  WHERE t.id IS NULL;
"
```

---

## Related Documentation

- [Deletion Framework](../user-guide/deletion-framework.md) — How cascading deletion uses both stores
- [ARCHITECTURE.md](../../ARCHITECTURE.md) — High-level architecture overview
- [Storage Architecture](./storage.md) — SQLite and CapDB backends

---

**Version:** 0.1.38 | **Last Updated:** 2026-06-21  
**Status:** Stable | **Pattern:** Intentional Dual-Store for Performance
