# PASS3: Gap Analysis - Backend vs Frontend

## Executive Summary

**Total Backend Endpoints**: 550+  
**Total Frontend API Calls**: 300+  
**Gap**: ~250 endpoints (45% of backend API is not being called by frontend)

This document identifies all gaps between what the backend provides and what the frontend actually uses.

---

## 1. Critical Missing Frontend Implementations

These are important backend features that have NO frontend UI yet:

### 1.1 Instance Management Gaps

#### Completed ✅:
- **Instance Reboot** - `POST /api/v1/instances/{id}/reboot` (IMPLEMENTED - INSTANCE-001)
  - ✅ Backend endpoint verified: Full reboot operation
  - ✅ Frontend: Reboot button added to InstanceDetail.tsx 
  - ✅ API hook: reboot mutation in useInstanceActions()
  - ✅ Impact: Users can now perform clean reboot operations

- **Instance Termination Protection** - `POST /api/v1/instances/{id}/protect-termination` / `DELETE /api/v1/instances/{id}/protect-termination` (IMPLEMENTED - INSTANCE-002)
  - ✅ Backend: Lock/unlock termination endpoints verified
  - ✅ Frontend: Toggle switch added to Overview tab with color-coded status
  - ✅ Status: Shows Protected/Unprotected state with smooth animation
  - ✅ Impact: Users can now safely protect critical instances from accidental deletion

- **Instance Terminal Access** - `GET /api/v1/instances/{id}/terminal` (ALREADY IMPLEMENTED - INSTANCE-003)
  - ✅ Backend: Terminal WebSocket endpoint verified
  - ✅ Frontend: Console tab with xterm.js integration complete
  - ✅ Component: InstanceTerminal.tsx with resize handling & Ctrl+R reconnect
  - ✅ Impact: Full terminal access from web UI with proper lifecycle management

- **Instance Logs as Stream** - `GET /api/v1/instances/{id}/logs/{stream}?follow=true` (ALREADY IMPLEMENTED - INSTANCE-004)
  - ✅ Backend: Log streaming endpoint with follow parameter verified
  - ✅ Frontend: useLogFollow hook with ReadableStream implementation
  - ✅ UI: Follow toggle button in Logs tab with error display
  - ✅ Impact: Real-time log streaming fully functional with proper error handling

#### Not Implemented:

- **Instance Attach/Detach Network Interfaces** - `POST /api/v1/instances/{id}/attach-network-interface`, `POST /api/v1/instances/{id}/detach-network-interface` (IMPLEMENTED - INSTANCE-005)
  - ✅ Backend: Attach/detach endpoints verified
  - ✅ Frontend: Mutations added to useInstanceActions
  - ✅ UI: Networking tab with link to ENI management
  - ✅ Impact: Users can now manage ENI attachments dynamically

---

### 1.2 Networking Gaps

#### Completed ✅:
- **Public IP Management** - COMPLETE (NETWORK-002)
  - ✅ Backend endpoints verified (5 operations): List, Allocate, Associate, Disassociate, Release
  - ✅ Frontend: PublicIPs.tsx listing page with filters
  - ✅ API: publicip.ts with full mutation hooks
  - ✅ Dialogs: AssociatePublicIPDialog for resource selection
  - ✅ Features: List with status filter, allocate new, associate/disassociate, release
  - ✅ Impact: Users can now fully manage public IP lifecycle

- **Elastic Network Interfaces (ENIs)** - COMPLETE SUBSYSTEM IMPLEMENTED (NETWORK-001)
  - ✅ Backend endpoints verified (7 operations): List, Create, Get, Delete, Attach, Detach, Manage IPs
  - ✅ Frontend: Complete ENI management pages created
  - ✅ Components: ENIs.tsx (listing), ENIDetail.tsx (detail), CreateENIDialog, AttachENIDialog
  - ✅ API: eni.ts with useNetworkInterfaces, useCreateNetworkInterface, useAttachNetworkInterface, etc.
  - ✅ Features: List with filters, create, view details, attach/detach, manage private IPs
  - ✅ Impact: Complete network interface lifecycle management now available

#### Not Implemented:

- **VPC Peering** - NOT IMPLEMENTED
  - Backend endpoints:
    - `GET /api/v1/vpc-peerings` - List VPC peerings
    - `POST /api/v1/vpc-peerings` - Create VPC peering
  - Frontend missing: NO peering UI
  - Impact: Cannot establish VPC-to-VPC connections through UI

- **VPC Endpoints** - PARTIALLY IMPLEMENTED
  - Backend endpoints:
    - `GET /api/v1/vpc-endpoints` - List VPC endpoints
    - `POST /api/v1/vpc-endpoints` - Create VPC endpoint
  - Frontend missing: No dedicated UI for endpoint creation/management
  - Impact: Cannot create service endpoints through UI

- **Subnet Route Table Association** - LIKELY MISSING
  - Backend: `POST /api/v1/subnets/{subnetId}/associate-route-table`
  - Frontend: Route table management minimal
  - Impact: Subnets may not be properly associated with route tables

- **Subnet Get/Patch Operations**
  - Backend: `GET /api/v1/subnets/{subnetId}`, `PATCH /api/v1/subnets/{subnetId}`
  - Frontend: Operations may not be implemented
  - Impact: Cannot view/edit subnet properties

- **DNS Zone VPC Associations** - NOT IMPLEMENTED
  - Backend endpoints:
    - `GET /api/v1/dns/zones/{zone}/vpc-associations`
    - `POST /api/v1/dns/zones/{zone}/vpc-associations`
    - `DELETE /api/v1/dns/zones/{zone}/vpc-associations`
  - Frontend missing: No DNS-VPC binding UI
  - Impact: Cannot associate DNS zones with VPCs

---

### 1.3 Storage Gaps

#### Not Implemented:
- **CSD (Collaborative Shared Storage) Volumes** - ENTIRE SUBSYSTEM MISSING
  - 15+ backend endpoints for shared volumes (not called by frontend)
  - Includes: volume creation, attachment, snapshots, leases, replicas, repair
  - Impact: Cannot use shared storage features through UI

- **S3 Credentials Management**
  - Backend: `GET /api/v1/s3/credentials`, `POST /api/v1/s3/credentials`, `DELETE /api/v1/s3/credentials/{id}`
  - Frontend missing: No S3 credentials UI
  - Impact: Cannot manage S3 access keys from console

- **S3 Bucket Policies**
  - Backend: `GET /api/v1/s3/buckets/{bucket}/policy`, `PUT /api/v1/s3/buckets/{bucket}/policy`, `DELETE /api/v1/s3/buckets/{bucket}/policy`
  - Frontend missing: No bucket policy editor
  - Impact: Cannot configure S3 permissions from UI

---

### 1.4 Compute & Scheduling Gaps

#### Not Implemented:
- **Placement Policies** - NOT IMPLEMENTED
  - Backend endpoints: 3+ endpoints for placement policy management
  - Frontend missing: No placement policy UI
  - Impact: Cannot define custom instance placement rules

- **Scheduler Operations** - NOT IMPLEMENTED
  - Backend endpoints:
    - `POST /api/v1/scheduler/simulate` - Simulate scheduling
    - `GET /api/v1/scheduler/capacity` - Get scheduler capacity
    - `GET /api/v1/scheduler/placements` - Get scheduler placements
  - Frontend missing: No scheduler UI
  - Impact: Cannot view/simulate scheduling decisions

- **Autoscale Policies** - PARTIALLY MISSING
  - Backend has full CRUD for autoscale policies
  - Frontend may not have full policy editor
  - Impact: Autoscaling configuration limited

- **Compute Group Autoscaling**
  - Backend: `GET /api/v1/groups/{name}/autoscale`, `POST /api/v1/groups/{name}/autoscale/disable`, `POST /api/v1/groups/{name}/autoscale/evaluate`
  - Frontend: May have limited autoscale configuration
  - Impact: Users cannot fully configure group autoscaling

---

### 1.5 Admin & Host Security Gaps

#### Not Implemented:
- **Host Security Nodes** - NOT CALLED
  - Backend: `GET /api/v1/admin/hostsec/nodes` - Get hostsec node info
  - Frontend missing: No hostsec node management UI
  - Impact: Cannot manage host security nodes from UI

---

### 1.6 Other Missing Features

#### Cap Init (User Data)
- **Cap Init Status** - `GET /api/v1/capinit/status` (backend available, check if called)
- **Cap Init Rendering** - `POST /api/v1/capinit/render` (variable substitution may not have UI)

#### Resource Monitoring
- **Resource Config** - `GET /api/v1/resources/{id}/config` (may not display in UI)
- **Config Drift Repair** - `POST /api/v1/resources/{id}/drift/repair` (backend has it, UI may not show it)
- **Metrics Ingest** - `POST /api/v1/metrics/ingest` (custom metric push may not have UI)

#### Secrets
- **Get Secret Value** - `GET /api/v1/secrets/{name}` (may not show secret content in UI)

#### Nodes & Topology
- **Node Inventory** - `POST /api/v1/nodes/{node}/inventory` (backend accepts inventory, UI may not send it)
- **Node Services** - `POST /api/v1/nodes/{node}/services` (setting node services may not have UI)
- **Node Join** - `POST /api/v1/nodes/join` (cluster join may not be in UI)

#### VPC Mobility
- **Dry Run Migration** - `GET /api/v1/vpcs/{vpc}/mobility/plans/{plan}/dry-run` (may not be called)
- **Job Cutover** - `POST /api/v1/vpcs/{vpc}/mobility/jobs/{job}/cutover` (may not have cutover UI)

#### GPU Management
- **GPU Assignment Details** - `POST /api/v1/gpu/{id}/assign` (may not have full UI)
- **GPU Release** - `POST /api/v1/gpu/{id}/release` (may not have UI)

---

## 2. Partially Implemented Features

These features exist in both backend and frontend, but may have incomplete implementations:

### 2.1 Incomplete Implementations

- **DNS Records**: Frontend has zone management but DNS record creation/deletion might be partial
- **Network ACLs**: Frontend may support listing but not full rule management
- **Firewalls**: Rules management might be incomplete
- **Certificates**: ACME account management might be partial
- **Autoscaling**: Policy management may be incomplete  
- **Migrations**: Migration plan management might lack full UI
- **Serverless Functions**: Function version management may be incomplete
- **MCP Servers**: Tool invocation UI may not be implemented

---

## 3. Backend-Only Endpoints (Not Needed by Frontend)

These endpoints exist in backend but likely shouldn't be called by frontend (IMDS, internal, etc.):

- `/capper/v1/...` - CapInit metadata endpoints (IMDS-compatible, internal)
- `/latest/meta-data/...` - EC2-compatible metadata (internal to instances)
- `/latest/user-data` - User data endpoint (internal)
- `/auth/google/callback` - OAuth callback (browser redirect, not API call)

---

## 4. Summary Table: Gap Categories

| Category | Backend Endpoints | Frontend Calls | Gap | Priority |
|----------|-------------------|----------------|-----|----------|
| Instance Management | 20+ | 15 | 5 | **HIGH** |
| Networking - Core | 50+ | 35 | 15 | **HIGH** |
| Networking - Advanced | 25+ | 10 | 15 | **MEDIUM** |
| Storage - General | 15+ | 8 | 7 | **HIGH** |
| Storage - CSD | 15+ | 0 | 15 | **MEDIUM** |
| Storage - S3 | 10+ | 2 | 8 | **MEDIUM** |
| Compute & Scheduling | 15+ | 5 | 10 | **MEDIUM** |
| Admin Functions | 35+ | 20 | 15 | **MEDIUM** |
| Monitoring | 25+ | 20 | 5 | **LOW** |
| Serverless | 25+ | 15 | 10 | **MEDIUM** |
| Certificates | 15+ | 10 | 5 | **LOW** |
| IAM/Auth | 70+ | 60 | 10 | **LOW** |
| **TOTAL** | **550+** | **300+** | **250+** | - |

---

## 5. Critical Gaps by Priority

### 🔴 HIGH PRIORITY (Must Implement)

1. **Instance Terminal Access** - Users need console access
2. **ENI Management** - Network interface lifecycle management
3. **Instance Reboot** - Alternative to stop/start
4. **Termination Protection** - Safety for critical instances
5. **Instance Attach/Detach ENI** - Dynamic networking
6. **S3 Credentials** - S3 key management
7. **DNS VPC Associations** - DNS zone binding

### 🟡 MEDIUM PRIORITY (Should Implement)

1. **CSD Shared Storage** - Collaborative storage features
2. **Public IP Management** - Independent IP allocation
3. **VPC Peering** - VPC-to-VPC connectivity
4. **Placement Policies** - Custom placement rules
5. **Scheduler UI** - View scheduling decisions
6. **S3 Bucket Policies** - Permission management
7. **Complete Autoscaling** - Full policy editor
8. **Log Streaming** - Real-time log follow

### 🟢 LOW PRIORITY (Nice to Have)

1. **Scheduler Capacity View** - Information-only
2. **Config Drift Visualization** - Information-only
3. **Advanced Monitoring** - Information-only
4. **Host Security Nodes** - Admin-only
5. **Cap Init Status** - Information-only

---

## 6. Gap Details by Feature Area

### 6.1 Instance Management - Missing 5 Endpoints

**High Impact**:
- `POST /api/v1/instances/{id}/reboot` - Reboot without stopping
- `POST /api/v1/instances/{id}/protect-termination` - Set termination lock
- `DELETE /api/v1/instances/{id}/protect-termination` - Unset termination lock
- `GET /api/v1/instances/{id}/terminal` - Terminal/console access

**Medium Impact**:
- `POST /api/v1/instances/{id}/attach-network-interface` - Attach ENI
- `POST /api/v1/instances/{id}/detach-network-interface` - Detach ENI

**Low Impact**:
- Streaming logs with `?follow=true` parameter not utilized

---

### 6.2 Networking - Missing 15 Endpoints

**ENI Management (Complete Gap)**:
- `GET /api/v1/network-interfaces` - List ENIs
- `POST /api/v1/network-interfaces` - Create ENI
- `GET /api/v1/network-interfaces/{eniId}` - Get ENI
- `DELETE /api/v1/network-interfaces/{eniId}` - Delete ENI
- `POST /api/v1/network-interfaces/{eniId}/attach` - Attach ENI
- `POST /api/v1/network-interfaces/{eniId}/detach` - Detach ENI
- `POST /api/v1/network-interfaces/{eniId}/private-ips` - Assign private IPs

**Public IP Management**:
- `GET /api/v1/public-ips` - List public IPs
- `POST /api/v1/public-ips/allocate` - Allocate IP
- `POST /api/v1/public-ips/{allocationId}/associate` - Associate IP
- `POST /api/v1/public-ips/{associationId}/disassociate` - Disassociate IP
- `DELETE /api/v1/public-ips/{allocationId}` - Release IP

**VPC Peering**:
- `GET /api/v1/vpc-peerings` - List peerings
- `POST /api/v1/vpc-peerings` - Create peering

**DNS-VPC Association**:
- `GET /api/v1/dns/zones/{zone}/vpc-associations` - List associations
- `POST /api/v1/dns/zones/{zone}/vpc-associations` - Create association
- `DELETE /api/v1/dns/zones/{zone}/vpc-associations` - Delete association

**VPC Endpoints**:
- `POST /api/v1/vpc-endpoints` - Create endpoint (GET exists, POST might be missing)

---

### 6.3 Storage - Missing 23 Endpoints

**CSD Shared Storage (Completely Unimplemented)**:
- 15 endpoints for volume management, snapshots, leases, replicas

**S3 Credentials**:
- `GET /api/v1/s3/credentials` - List credentials
- `POST /api/v1/s3/credentials` - Create credential
- `DELETE /api/v1/s3/credentials/{id}` - Delete credential

**S3 Bucket Policies**:
- `GET /api/v1/s3/buckets/{bucket}/policy` - Get policy
- `PUT /api/v1/s3/buckets/{bucket}/policy` - Set policy
- `DELETE /api/v1/s3/buckets/{bucket}/policy` - Delete policy

**Storage Snapshots**:
- Backend has snapshots, frontend may not expose them

---

### 6.4 Compute - Missing 10 Endpoints

**Placement Policies**:
- `GET /api/v1/placement/policies` - List policies
- `POST /api/v1/placement/policies` - Create policy
- `GET /api/v1/placement/policies/{policy}` - Get policy
- `DELETE /api/v1/placement/policies/{policy}` - Delete policy

**Scheduler**:
- `POST /api/v1/scheduler/simulate` - Simulate scheduling
- `GET /api/v1/scheduler/capacity` - Get capacity info
- `GET /api/v1/scheduler/placements` - Get placements

**Autoscaling**:
- Policy CRUD may be incomplete
- Group autoscaling configuration may be incomplete

---

## 7. Recommendations

### Phase 1: Critical Gaps (2-3 weeks)
1. Add Instance Reboot button
2. Add Termination Protection toggle
3. Build ENI management page
4. Build Terminal/Console access
5. Add instance attach/detach ENI operations

### Phase 2: Important Gaps (3-4 weeks)
1. Add S3 credentials management
2. Add S3 bucket policy editor
3. Build VPC peering management
4. Complete DNS zone VPC associations
5. Add public IP management

### Phase 3: Nice-to-Have (4+ weeks)
1. Implement CSD shared storage UI
2. Implement placement policies UI
3. Implement scheduler visualization
4. Add log streaming
5. Complete autoscaling configuration

---

**Analysis Date**: 2026-07-01  
**Endpoint Gap**: 45% of backend API not exposed to frontend
