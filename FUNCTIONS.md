# FUNCTIONS: Workflow Diagrams with API Coverage

## Overview

This document visualizes the major workflows in Capper with Mermaid diagrams, showing which API endpoints are called (✅ implemented in frontend) and which are NOT called (❌ missing from frontend).

**Color Coding**:
- 🟢 **Green**: Fully implemented frontend UI
- 🟡 **Yellow**: Partially implemented
- 🔴 **Red**: Not implemented in frontend (backend API exists but no UI)
- 🔵 **Blue**: Information-only (logging, monitoring)

---

## 1. Instance Lifecycle Workflow

```mermaid
graph TD
    A["🟢 Create Instance<br/>POST /api/v1/instances"] --> B["🟢 Instance Created"]
    B --> C["🟢 Start Instance<br/>POST /api/v1/instances/{id}/start"]
    C --> D["🟢 Instance Running"]
    D --> E{"🟡 User Action?"}
    
    E -->|View| F["🟢 Get Instance<br/>GET /api/v1/instances/{id}"]
    E -->|Logs| G["🟡 View Logs<br/>GET /api/v1/instances/{id}/logs"]
    G -->|Real-time| G1["🔴 Stream Logs<br/>GET /api/v1/instances/{id}/logs?follow=true"]
    E -->|Console| G2["🔴 Terminal Access<br/>GET /api/v1/instances/{id}/terminal"]
    E -->|Modify| H["🟢 Update Instance<br/>PATCH /api/v1/instances/{id}"]
    E -->|Stop| I["🟢 Stop Instance<br/>POST /api/v1/instances/{id}/stop"]
    E -->|Restart| J["🟢 Restart Instance<br/>POST /api/v1/instances/{id}/restart"]
    E -->|Reboot| J1["🔴 Reboot Instance<br/>POST /api/v1/instances/{id}/reboot"]
    E -->|Protect| K["🔴 Enable Termination Lock<br/>POST /api/v1/instances/{id}/protect-termination"]
    
    I --> L["🟢 Instance Stopped"]
    L --> M{"Delete?"}
    J --> D
    J1 --> D
    K --> D
    
    M -->|No| N["🟢 Restart/Continue"]
    M -->|Yes| O["🟢 Delete Preflight<br/>POST /api/v1/{type}/{id}/delete-preflight"]
    N --> D
    O --> P["🟢 Delete Confirm<br/>POST /api/v1/{type}/{id}/delete-confirm"]
    P --> Q["🔵 Deletion Job Status<br/>GET /api/v1/deletion-jobs/{jobId}"]
    Q --> R["🟢 Instance Deleted"]
    
    style A fill:#90EE90
    style C fill:#90EE90
    style D fill:#90EE90
    style F fill:#90EE90
    style G fill:#FFD700
    style G1 fill:#FF6B6B
    style G2 fill:#FF6B6B
    style H fill:#90EE90
    style I fill:#90EE90
    style J fill:#90EE90
    style J1 fill:#FF6B6B
    style K fill:#FF6B6B
    style L fill:#90EE90
    style O fill:#90EE90
    style P fill:#90EE90
    style Q fill:#87CEEB
    style R fill:#90EE90
```

**Coverage**: 85% implemented  
**Missing Features (Red)**: Stream logs, terminal access, reboot, termination protection

---

## 2. Network Interface (ENI) Lifecycle

```mermaid
graph TD
    A["🔴 Create ENI<br/>POST /api/v1/network-interfaces"] --> B["🔴 ENI Created"]
    B --> C["🔴 View ENI Details<br/>GET /api/v1/network-interfaces/{eniId}"]
    C --> D{"ENI Management"}
    
    D -->|Add Private IP| E["🔴 Assign Private IP<br/>POST /api/v1/network-interfaces/{eniId}/private-ips"]
    D -->|Attach to Instance| F["🔴 Attach ENI<br/>POST /api/v1/network-interfaces/{eniId}/attach"]
    D -->|View All| G["🔴 List ENIs<br/>GET /api/v1/network-interfaces"]
    D -->|Delete| H["🔴 Delete ENI<br/>DELETE /api/v1/network-interfaces/{eniId}"]
    
    F --> I["🔴 ENI Attached to Instance"]
    I --> J["🔴 Detach ENI<br/>POST /api/v1/network-interfaces/{eniId}/detach"]
    J --> B
    
    E --> B
    G --> B
    H --> K["🔴 ENI Deleted"]
    
    style A fill:#FF6B6B
    style B fill:#FF6B6B
    style C fill:#FF6B6B
    style D fill:#FF6B6B
    style E fill:#FF6B6B
    style F fill:#FF6B6B
    style G fill:#FF6B6B
    style H fill:#FF6B6B
    style I fill:#FF6B6B
    style J fill:#FF6B6B
    style K fill:#FF6B6B
```

**Coverage**: 0% implemented  
**Critical Gap**: Entire ENI subsystem missing from frontend

---

## 3. VPC & Networking Setup Workflow

```mermaid
graph TD
    A["🟢 Create VPC<br/>POST /api/v1/vpcs"] --> B["🟢 VPC Created"]
    B --> C["🟢 Create Subnet<br/>POST /api/v1/vpcs/{vpc}/subnets"]
    C --> D["🟢 Subnet Created"]
    D --> E{"Configure Networking"}
    
    E -->|Security| F["🟢 Create Security Group<br/>POST /api/v1/security-groups"]
    F --> F1["🟢 Add SG Rules<br/>POST /api/v1/security-groups/{sgId}/rules"]
    
    E -->|Routing| G["🟢 Create Route Table<br/>POST /api/v1/vpcs/{vpc}/route-tables"]
    G --> G1["🟡 Associate Route Table<br/>POST /api/v1/subnets/{subnetId}/associate-route-table"]
    G1 --> G2["🟢 Add Routes<br/>POST /api/v1/route-tables/{rtbId}/routes"]
    
    E -->|Internet| H["🟢 Create IGW<br/>POST /api/v1/internet-gateways"]
    H --> H1["🟢 Add Route to IGW<br/>POST /api/v1/route-tables/{rtbId}/routes"]
    
    E -->|NAT| I["🟢 Create NAT Gateway<br/>POST /api/v1/nat-gateways"]
    I --> I1["🟢 Add Route to NAT<br/>POST /api/v1/route-tables/{rtbId}/routes"]
    
    E -->|Network ACLs| J["🟡 Create Network ACL<br/>POST /api/v1/network-acls"]
    J --> J1["🟡 Add ACL Rules<br/>POST /api/v1/network-acls/{aclId}/entries"]
    
    E -->|Advanced| K["🔴 Create VPC Peering<br/>POST /api/v1/vpc-peerings"]
    E -->|Advanced| L["🔴 Create VPC Endpoint<br/>POST /api/v1/vpc-endpoints"]
    E -->|Advanced| M["🔴 Manage ENIs<br/>ENI Operations"]
    
    F1 --> N["🟢 VPC Ready"]
    G2 --> N
    H1 --> N
    I1 --> N
    J1 --> N
    K --> N
    L --> N
    M --> N
    
    style A fill:#90EE90
    style B fill:#90EE90
    style C fill:#90EE90
    style D fill:#90EE90
    style F fill:#90EE90
    style F1 fill:#90EE90
    style G fill:#90EE90
    style G1 fill:#FFD700
    style G2 fill:#90EE90
    style H fill:#90EE90
    style H1 fill:#90EE90
    style I fill:#90EE90
    style I1 fill:#90EE90
    style J fill:#FFD700
    style J1 fill:#FFD700
    style K fill:#FF6B6B
    style L fill:#FF6B6B
    style M fill:#FF6B6B
    style N fill:#90EE90
```

**Coverage**: 75% implemented  
**Missing Features (Red)**: VPC peering, VPC endpoints, ENI management, some route table operations

---

## 4. Load Balancer Workflow

```mermaid
graph TD
    A["🟢 Create Load Balancer<br/>POST /api/v1/lb"] --> B["🟢 LB Created"]
    B --> C["🟢 Create Listeners<br/>POST /api/v1/lb/{name}/listeners"]
    C --> D["🟢 Listener Created"]
    D --> E{"Configure LB"}
    
    E -->|Certificates| F["🟢 Attach Certificate<br/>POST /api/v1/lb/{name}/listeners/{id}/certificates"]
    F --> F1["🟢 LB HTTPS Ready"]
    
    E -->|Targets| G["🟢 Create Target Group<br/>POST /api/v1/lb/{name}/target-groups"]
    G --> H["🟢 Add Targets<br/>POST /api/v1/lb/{name}/target-groups/{tgId}/targets"]
    H --> I["🟢 Targets Added"]
    
    E -->|Monitor| J["🟡 Get LB Details<br/>GET /api/v1/lb/{name}"]
    J --> K["🔵 Monitoring Data<br/>GET /api/v1/load-balancers/{id}/monitoring"]
    
    F1 --> L["🟢 LB Ready"]
    I --> L
    K --> L
    
    L --> M["🟢 LB Operations"]
    M -->|Update| N["🟡 Patch Listener<br/>PATCH /api/v1/lb/{name}/listeners/{id}"]
    M -->|Remove Target| O["🟢 Remove Target<br/>DELETE /api/v1/lb/{name}/target-groups/{tgId}/targets/{targetId}"]
    M -->|Delete| P["🟢 Delete LB<br/>DELETE /api/v1/lb/{name}"]
    
    N --> L
    O --> L
    P --> Q["🟢 LB Deleted"]
    
    style A fill:#90EE90
    style B fill:#90EE90
    style C fill:#90EE90
    style D fill:#90EE90
    style F fill:#90EE90
    style F1 fill:#90EE90
    style G fill:#90EE90
    style H fill:#90EE90
    style I fill:#90EE90
    style J fill:#FFD700
    style K fill:#87CEEB
    style L fill:#90EE90
    style M fill:#90EE90
    style N fill:#FFD700
    style O fill:#90EE90
    style P fill:#90EE90
    style Q fill:#90EE90
```

**Coverage**: 90% implemented  
**Minor Gaps**: Some advanced listener updates

---

## 5. Storage & Backup Workflow

```mermaid
graph TD
    A["🟢 Create Volume<br/>POST /api/v1/storage/volumes"] --> B["🟢 Volume Created"]
    B --> C["🟢 Attach Volume<br/>POST /api/v1/storage/volumes/{name}/attach"]
    C --> D["🟢 Volume Ready"]
    D --> E{"Storage Operations"}
    
    E -->|S3 Buckets| F["🟢 Create Bucket<br/>POST /api/v1/storage/buckets"]
    F --> F1["🟢 Upload Objects<br/>PUT /api/v1/storage/buckets/{bucket}/objects/{key}"]
    F1 --> F2["🔴 Manage Credentials<br/>POST /api/v1/s3/credentials"]
    F2 --> F3["🔴 Bucket Policy<br/>PUT /api/v1/s3/buckets/{bucket}/policy"]
    
    E -->|Backup| G["🟢 Create Backup<br/>POST /api/v1/backups"]
    G --> H["🟢 Backup Policies<br/>POST /api/v1/backup-policies"]
    H --> I["🟢 Restore Backup<br/>POST /api/v1/backups/{id}/restore"]
    
    E -->|CSD Storage| J["🔴 Create CSD Volume<br/>POST /api/v1/csd/volumes"]
    J --> J1["🔴 Create Snapshot<br/>POST /api/v1/csd/volumes/{vol}/snapshots"]
    J1 --> J2["🔴 Manage Leases<br/>POST /api/v1/csd/volumes/{vol}/leases/revoke"]
    
    E -->|Detach| K["🟢 Detach Volume<br/>POST /api/v1/storage/volumes/{name}/detach"]
    K --> L["🟢 Volume Detached"]
    
    F3 --> M["🟢 S3 Configured"]
    I --> M
    J2 --> M
    
    style A fill:#90EE90
    style B fill:#90EE90
    style C fill:#90EE90
    style D fill:#90EE90
    style E fill:#90EE90
    style F fill:#90EE90
    style F1 fill:#90EE90
    style F2 fill:#FF6B6B
    style F3 fill:#FF6B6B
    style G fill:#90EE90
    style H fill:#90EE90
    style I fill:#90EE90
    style J fill:#FF6B6B
    style J1 fill:#FF6B6B
    style J2 fill:#FF6B6B
    style K fill:#90EE90
    style L fill:#90EE90
    style M fill:#90EE90
```

**Coverage**: 70% implemented  
**Missing Features (Red)**: S3 credentials, S3 bucket policies, entire CSD subsystem

---

## 6. IAM & Access Control Workflow

```mermaid
graph TD
    A["🟢 Select Account<br/>Account Context"] --> B["🟢 Account Selected"]
    B --> C{"IAM Operations"}
    
    C -->|Users| D["🟢 List IAM Users<br/>GET /api/v1/accounts/{account}/iam/users"]
    D --> E["🟢 Create User<br/>POST /api/v1/accounts/{account}/iam/users"]
    E --> F["🟢 User Created"]
    F --> F1["🟡 Set Password<br/>POST /api/v1/users/{id}/password"]
    F1 --> F2["🟡 Grant Roles<br/>POST /api/v1/users/{id}/roles"]
    
    C -->|Groups| G["🟢 List Groups<br/>GET /api/v1/accounts/{account}/iam/groups"]
    G --> H["🟢 Create Group<br/>POST /api/v1/accounts/{account}/iam/groups"]
    H --> I["🟢 Group Created"]
    I --> I1["🟢 Add Members<br/>POST /api/v1/accounts/{account}/iam/groups/{id}/members"]
    
    C -->|Roles| J["🟢 List Roles<br/>GET /api/v1/accounts/{account}/iam/roles"]
    J --> K["🟢 Create Role<br/>POST /api/v1/accounts/{account}/iam/roles"]
    K --> L["🟢 Role Created"]
    
    C -->|Policies| M["🟢 List Policies<br/>GET /api/v1/accounts/{account}/iam/policies"]
    M --> N["🟢 Create Policy<br/>POST /api/v1/accounts/{account}/iam/policies"]
    N --> O["🟢 Policy Created"]
    O --> O1["🟢 Attach to Role<br/>POST /api/v1/accounts/{account}/iam/policies/{id}/attach"]
    
    C -->|Service Accounts| P["🟢 List Service Accounts<br/>GET /api/v1/accounts/{account}/iam/service-accounts"]
    P --> Q["🟢 Create SA<br/>POST /api/v1/accounts/{account}/iam/service-accounts"]
    Q --> R["🟢 SA Created"]
    R --> R1["🟢 Issue Token<br/>POST /api/v1/accounts/{account}/iam/service-accounts/{id}/tokens"]
    
    C -->|Audit| S["🔵 View Audit Log<br/>GET /api/v1/accounts/{account}/audit"]
    
    F2 --> T["🟢 IAM Configured"]
    I1 --> T
    O1 --> T
    R1 --> T
    S --> T
    
    style A fill:#90EE90
    style B fill:#90EE90
    style C fill:#90EE90
    style D fill:#90EE90
    style E fill:#90EE90
    style F fill:#90EE90
    style F1 fill:#FFD700
    style F2 fill:#FFD700
    style G fill:#90EE90
    style H fill:#90EE90
    style I fill:#90EE90
    style I1 fill:#90EE90
    style J fill:#90EE90
    style K fill:#90EE90
    style L fill:#90EE90
    style M fill:#90EE90
    style N fill:#90EE90
    style O fill:#90EE90
    style O1 fill:#90EE90
    style P fill:#90EE90
    style Q fill:#90EE90
    style R fill:#90EE90
    style R1 fill:#90EE90
    style S fill:#87CEEB
    style T fill:#90EE90
```

**Coverage**: 95% implemented  
**Minor Gaps**: Some RBAC operations

---

## 7. Monitoring & Observability Workflow

```mermaid
graph TD
    A["🔵 Monitor Resources<br/>Observability"] --> B["🟢 List Resources<br/>GET /api/v1/resources"]
    B --> C["🟢 Select Resource<br/>GET /api/v1/resources/{id}"]
    C --> D{"View Information"}
    
    D -->|Metrics| E["🟢 Get Metrics<br/>GET /api/v1/metrics/query"]
    E --> E1["🔵 Display Chart"]
    
    D -->|Events| F["🟢 List Events<br/>GET /api/v1/resource-events"]
    F --> F1["🔵 Timeline View"]
    
    D -->|Configuration| G["🟡 Get Resource Config<br/>GET /api/v1/resources/{id}/config"]
    G --> G1["🔴 Repair Drift<br/>POST /api/v1/resources/{id}/drift/repair"]
    
    D -->|Alerts| H["🟢 List Alerts<br/>GET /api/v1/alerts"]
    H --> I["🟢 Create Alert Rule<br/>POST /api/v1/alerts/rules"]
    I --> I1["🟢 Ack/Resolve<br/>POST /api/v1/alerts/{id}/ack"]
    
    E1 --> J["🟢 Monitoring Dashboard"]
    F1 --> J
    G1 --> J
    I1 --> J
    
    J --> K{"Alert Status?"}
    K -->|Healthy| L["🟢 All Good"]
    K -->|Issues| M["🔴 Alert on Issues<br/>Notifications Sent"]
    
    style A fill:#87CEEB
    style B fill:#90EE90
    style C fill:#90EE90
    style D fill:#90EE90
    style E fill:#90EE90
    style E1 fill:#87CEEB
    style F fill:#90EE90
    style F1 fill:#87CEEB
    style G fill:#FFD700
    style G1 fill:#FF6B6B
    style H fill:#90EE90
    style I fill:#90EE90
    style I1 fill:#90EE90
    style J fill:#90EE90
    style K fill:#90EE90
    style L fill:#90EE90
    style M fill:#FF6B6B
```

**Coverage**: 85% implemented  
**Missing Features (Red)**: Config drift repair, drift visualization

---

## 8. Deletion Workflow (Multi-Step)

```mermaid
graph TD
    A["User Initiates Delete<br/>Click Delete Button"] --> B["🟢 Get Preflight Info<br/>POST /{resourceType}/{id}/delete-preflight"]
    B --> C["🟢 Show Dependencies<br/>What will be deleted?"]
    C --> D["User Confirms?"]
    
    D -->|Cancel| E["🟢 Abort Deletion"]
    D -->|Confirm| F["User Types 'DELETE'<br/>Confirmation Dialog"]
    
    F --> G["🟢 Confirm Deletion<br/>POST /{resourceType}/{id}/delete-confirm"]
    G --> H["🔵 Async Job Started<br/>Returns jobId"]
    
    H --> I["🔵 Poll Job Status<br/>GET /deletion-jobs/{jobId}"]
    I --> J{"Job Status?"}
    
    J -->|Running| K["🔵 Show Progress<br/>% Complete"]
    K --> I
    
    J -->|Failed| L["🔴 Show Error<br/>Allow retry/manual cleanup"]
    J -->|Success| M["🟢 Resource Deleted<br/>Refresh UI"]
    
    E --> N["Back to Resource"]
    L --> N
    M --> N
    
    style A fill:#90EE90
    style B fill:#90EE90
    style C fill:#90EE90
    style D fill:#90EE90
    style E fill:#90EE90
    style F fill:#90EE90
    style G fill:#90EE90
    style H fill:#87CEEB
    style I fill:#87CEEB
    style J fill:#87CEEB
    style K fill:#87CEEB
    style L fill:#FF6B6B
    style M fill:#90EE90
    style N fill:#90EE90
```

**Coverage**: 95% implemented  
**Missing Features (Red)**: Error handling and recovery UI

---

## 9. VPC Creation & Deletion Workflow

```mermaid
graph TD
    A["🟢 Create VPC<br/>POST /api/v1/vpcs"] --> B["🟢 VPC Created"]
    B --> C["🟢 Add Subnets<br/>POST /api/v1/vpcs/{vpc}/subnets"]
    C --> D["🟢 Add Network Resources<br/>Security Groups, Route Tables, etc."]
    D --> E["🟢 VPC Production Ready"]
    
    E --> F["User Initiates Delete<br/>Click Delete VPC"]
    F --> G["🟢 Get Dependencies<br/>POST /api/v1/vpcs/{vpc}/delete-preflight"]
    G --> H["🔴 Show ALL Dependent Resources<br/>Subnets, ENIs, Instances, etc."]
    H --> I["User Confirms Cascade Delete?"]
    
    I -->|No| J["Keep VPC"]
    I -->|Yes| K["🟢 Confirm Delete<br/>POST /api/v1/vpcs/{vpc}/delete-confirm"]
    
    J --> E
    K --> L["🔵 Async Cascade Deletion<br/>Deletes all dependencies"]
    L --> M["🟢 VPC Deleted<br/>All sub-resources removed"]
    
    style A fill:#90EE90
    style B fill:#90EE90
    style C fill:#90EE90
    style D fill:#90EE90
    style E fill:#90EE90
    style F fill:#90EE90
    style G fill:#90EE90
    style H fill:#FF6B6B
    style I fill:#90EE90
    style J fill:#90EE90
    style K fill:#90EE90
    style L fill:#87CEEB
    style M fill:#90EE90
```

**Coverage**: 85% implemented  
**Gap Note**: Dependency visualization (Red) needed for better UX

---

## Coverage Summary

| Workflow | Coverage | Status | Missing |
|----------|----------|--------|---------|
| Instance Lifecycle | 85% | 🟢 Good | Reboot, terminal, stream logs, termination protection |
| ENI Management | 0% | 🔴 Critical | Entire subsystem |
| VPC & Networking | 75% | 🟡 Fair | Peering, endpoints, ENI management |
| Load Balancers | 90% | 🟢 Good | Advanced listener updates |
| Storage & Backup | 70% | 🟡 Fair | S3 creds, policies, CSD storage |
| IAM & Access | 95% | 🟢 Excellent | Minor RBAC operations |
| Monitoring | 85% | 🟢 Good | Drift repair, visualization |
| Deletion | 95% | 🟢 Excellent | Error recovery |
| VPC Create/Delete | 85% | 🟢 Good | Dependency visualization |

---

## Critical Missing Subsystems (🔴 Red)

### High Priority (Must Implement)
1. **ENI (Network Interface) Management** - 0% coverage
   - Complete subsystem missing from UI
   - 7 backend endpoints not called
   - Impact: Cannot manage network interfaces

2. **S3 Credentials & Policies** - 0% coverage
   - 5 backend endpoints not implemented in UI
   - Impact: Cannot manage S3 access from console

3. **Instance Reboot** - 0% coverage
   - 1 endpoint missing
   - Impact: Users must stop/start instead

4. **Terminal/Console Access** - 0% coverage
   - 1 endpoint missing
   - Impact: No SSH/console from browser

### Medium Priority (Should Implement)
1. **VPC Peering** - 0% coverage
2. **VPC Endpoints** - 0% coverage
3. **CSD Shared Storage** - 0% coverage
4. **Config Drift Repair** - 0% coverage

---

## Implementation Priority Queue

```mermaid
graph LR
    A["Phase 1<br/>Weeks 1-3"] --> B["Phase 2<br/>Weeks 4-6"]
    B --> C["Phase 3<br/>Weeks 7-9"]
    C --> D["Phase 4<br/>Weeks 10-12"]
    D --> E["Phase 5<br/>Weeks 13-16"]
    
    A --> A1["🔴 Instance Reboot<br/>🔴 Terminal<br/>🔴 ENI Full CRUD<br/>🟡 Log Streaming<br/>🟡 Public IPs"]
    
    B --> B1["🔴 VPC Peering<br/>🔴 VPC Endpoints<br/>🟡 DNS-VPC Assoc<br/>🟡 Subnet Mgmt"]
    
    C --> C1["🔴 S3 Credentials<br/>🔴 S3 Policies<br/>🟢 Backups Complete"]
    
    D --> D1["🔴 Placement Policies<br/>🟡 Autoscaling Complete<br/>🔵 Scheduler Viz"]
    
    E --> E1["🔴 CSD Storage<br/>🟡 Advanced Features<br/>🟢 Polish & Testing"]
    
    style A fill:#FFD700
    style B fill:#FFD700
    style C fill:#FFD700
    style D fill:#FFD700
    style E fill:#FFD700
```

---

## Legend

| Color | Meaning | Status |
|-------|---------|--------|
| 🟢 Green | Fully implemented in frontend | Production ready |
| 🟡 Yellow | Partially implemented | Partial coverage |
| 🔴 Red | NOT implemented in frontend | Backend API exists, UI missing |
| 🔵 Blue | Information/monitoring only | Reference data, no mutations |

---

**Document Version**: 1.0  
**Created**: 2026-07-01  
**Coverage Analysis**: Complete  
**Total Backend Endpoints**: 550+  
**Implemented in Frontend**: 300+ (55%)  
**Missing from Frontend**: 250+ (45%)
