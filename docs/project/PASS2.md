# PASS2: Frontend API Call Analysis

## Overview
Complete audit of all API calls made by the CapperWeb frontend to the Capper backend. Analysis covers ~300+ frontend API calls across all features.

**Frontend Location**: `/home/megalith/CapperVM/CapperWeb/src/`  
**API Client Base**: `/api/v1/`  
**HTTP Client**: Native `fetch()` API with custom `apiFetch()` wrapper  
**State Management**: React Query (TanStack Query)  
**Framework**: React with TypeScript  

---

## 1. Authentication & Sessions

### File: `api/auth.ts`

| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/auth/session` | POST | Create new session | `POST /api/v1/auth/session` - Create user session |
| `/auth/session` | DELETE | Logout/destroy session | `DELETE /api/v1/auth/session` - Logout |
| `/auth/session` | GET | Get current session info | `GET /api/v1/auth/session` - Check auth status |
| `/auth/login` | POST | Local login | `POST /api/v1/auth/login` - {"email":"user@example.com","password":"pass"}` |

**Frontend Usage**:
- Login page uses `/auth/login` with credentials
- Session management in authentication context
- Automatic session validation on app load

---

## 2. Instance Management

### File: `api/instances.ts`

| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/instances` | GET | List all instances | `GET /api/v1/instances?limit=50&offset=0` |
| `/instances` | POST | Create new instance | `POST /api/v1/instances` - {"name":"web-1","type":"t2.medium","image":"ubuntu-20.04"}` |
| `/instances/{id}` | GET | Get instance details | `GET /api/v1/instances/i-12345` |
| `/instances/{id}` | PATCH | Update instance | `PATCH /api/v1/instances/i-12345` - {"labels":{"env":"prod"}}` |
| `/instances/{id}` | DELETE | Delete instance | `DELETE /api/v1/instances/i-12345` |
| `/instances/{id}/start` | POST | Start instance | `POST /api/v1/instances/i-12345/start` |
| `/instances/{id}/stop` | POST | Stop instance | `POST /api/v1/instances/i-12345/stop` |
| `/instances/{id}/restart` | POST | Restart instance | `POST /api/v1/instances/i-12345/restart` |
| `/instances/{id}/reboot` | POST | Reboot instance | `POST /api/v1/instances/i-12345/reboot` |
| `/instances/{id}/logs` | GET | Get startup/error logs | `GET /api/v1/instances/i-12345/logs` |
| `/instances/{id}/logs/stdout` | GET | Get stdout logs | `GET /api/v1/instances/i-12345/logs/stdout` |
| `/instances/{id}/logs/stderr` | GET | Get stderr logs | `GET /api/v1/instances/i-12345/logs/stderr` |
| `/instances/{id}/logs/{stream}?follow=true` | GET | Stream logs (follow mode) | `GET /api/v1/instances/i-12345/logs/stdout?follow=true` - WebSocket |
| `/instances/{id}/events` | GET | Get instance events | `GET /api/v1/instances/i-12345/events` |
| `/instances/{id}/metadata` | GET | Get instance metadata | `GET /api/v1/instances/i-12345/metadata` |
| `/instance-disk-capacity` | GET | Get disk capacity info | `GET /api/v1/instance-disk-capacity` |
| `/instances/{id}/terminal` | GET | Get terminal access | `GET /api/v1/instances/i-12345/terminal` - Returns WebSocket URL |

**Frontend Usage**:
- Instance list page polls `/instances?limit=50`
- Instance detail page shows logs via `/instances/{id}/logs`
- Create instance modal uses POST `/instances`
- Terminal access initiates WebSocket connection

---

## 3. VPC & Networking

### File: `api/vpcnet.ts`

#### VPCs
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/vpcs` | GET | List all VPCs | `GET /api/v1/vpcs?limit=50` |
| `/vpcs` | POST | Create VPC | `POST /api/v1/vpcs` - {"name":"prod-vpc","cidr":"10.0.0.0/16"}` |
| `/vpcs/{vpc}` | GET | Get VPC details | `GET /api/v1/vpcs/vpc-123` |
| `/vpcs/{vpc}/detail` | GET | Get detailed VPC info | `GET /api/v1/vpcs/vpc-123/detail` |
| `/vpcs/{vpc}/summary` | GET | Get VPC summary | `GET /api/v1/vpcs/vpc-123/summary` |
| `/vpcs/{vpc}/dependencies` | GET | Get VPC dependencies | `GET /api/v1/vpcs/vpc-123/dependencies` |
| `/vpcs/{vpc}` | PATCH | Update VPC | `PATCH /api/v1/vpcs/vpc-123` - {"name":"prod-vpc-v2"}` |
| `/vpcs/{vpc}` | DELETE | Delete VPC (preflight flow) | `POST /api/v1/vpc-123/delete-preflight` then `POST /api/v1/vpc-123/delete-confirm` |
| `/vpcs/{vpc}/copy` | POST | Copy VPC | `POST /api/v1/vpcs/vpc-123/copy` - {"newName":"prod-vpc-backup"}` |
| `/vpcs/{vpc}/move` | POST | Move VPC | `POST /api/v1/vpcs/vpc-123/move` - {"targetRegion":"us-west-2"}` |

#### Subnets
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/vpcs/{vpc}/subnets` | GET | List VPC subnets | `GET /api/v1/vpcs/vpc-123/subnets` |
| `/vpcs/{vpc}/subnets` | POST | Create subnet | `POST /api/v1/vpcs/vpc-123/subnets` - {"cidr":"10.0.1.0/24","az":"us-east-1a"}` |
| `/vpcs/{vpc}/subnets?purpose={purpose}` | GET | List subnets by purpose | `GET /api/v1/vpcs/vpc-123/subnets?purpose=public` |
| `/subnets/{subnetId}` | GET | Get subnet details | `GET /api/v1/subnets/subnet-456` |
| `/subnets/{subnetId}` | PATCH | Update subnet | `PATCH /api/v1/subnets/subnet-456` - {"tags":{"name":"public-1a"}}` |
| `/subnets/{subnetId}` | DELETE | Delete subnet | `DELETE /api/v1/subnets/subnet-456` |
| `/subnets/{subnetId}/dependencies` | GET | Get subnet dependencies | `GET /api/v1/subnets/subnet-456/dependencies` |
| `/subnets/{subnetId}/available-ips` | GET | Get available IPs in subnet | `GET /api/v1/subnets/subnet-456/available-ips` |

#### Route Tables & Routes
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/vpcs/{vpc}/route-tables` | GET | List route tables | `GET /api/v1/vpcs/vpc-123/route-tables` |
| `/vpcs/{vpc}/route-tables` | POST | Create route table | `POST /api/v1/vpcs/vpc-123/route-tables` - {"tags":{"name":"private-routes"}}` |
| `/vpcs/{vpc}/routes` | GET | List VPC routes (legacy) | `GET /api/v1/vpcs/vpc-123/routes` |
| `/vpcs/{vpc}/routes` | POST | Create route (legacy) | `POST /api/v1/vpcs/vpc-123/routes` - {"destination":"0.0.0.0/0","target":"nat-123"}` |
| `/route-tables/{routeTableId}` | GET | Get route table details | `GET /api/v1/route-tables/rtb-789` |
| `/route-tables/{routeTableId}/routes` | POST | Add route | `POST /api/v1/route-tables/rtb-789/routes` - {"destination":"192.168.0.0/16","target":"peer-id"}` |
| `/route-tables/{routeTableId}/routes/{routeId}` | DELETE | Delete route | `DELETE /api/v1/route-tables/rtb-789/routes/route-id` |

#### Security Groups
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/security-groups` | GET | List security groups | `GET /api/v1/security-groups?vpcId=vpc-123` |
| `/security-groups` | POST | Create security group | `POST /api/v1/security-groups` - {"name":"web-sg","vpcId":"vpc-123"}` |
| `/security-groups/{sgId}` | GET | Get security group | `GET /api/v1/security-groups/sg-123` |
| `/security-groups/{sgId}` | DELETE | Delete security group | `DELETE /api/v1/security-groups/sg-123` |
| `/security-groups/{sgId}/rules` | POST | Add SG rule | `POST /api/v1/security-groups/sg-123/rules` - {"protocol":"tcp","port":443,"source":"0.0.0.0/0"}` |
| `/security-group-rules/{ruleId}` | DELETE | Delete SG rule | `DELETE /api/v1/security-group-rules/sgr-456` |

#### Network ACLs
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/network-acls` | GET | List network ACLs | `GET /api/v1/network-acls?vpcId=vpc-123` |
| `/network-acls` | POST | Create network ACL | `POST /api/v1/network-acls` - {"vpcId":"vpc-123","name":"private-acl"}` |
| `/network-acls/{aclId}` | GET | Get network ACL | `GET /api/v1/network-acls/acl-123` |
| `/network-acls/{aclId}` | DELETE | Delete network ACL | `DELETE /api/v1/network-acls/acl-123` |
| `/network-acls/{aclId}/entries` | POST | Add ACL entry | `POST /api/v1/network-acls/acl-123/entries` - {"ruleNum":100,"protocol":"tcp","port":22}` |
| `/network-acls/{aclId}/entries/{ruleNumber}` | DELETE | Delete ACL entry | `DELETE /api/v1/network-acls/acl-123/entries/100` |

#### Gateways
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/internet-gateways` | GET | List internet gateways | `GET /api/v1/internet-gateways?vpcId=vpc-123` |
| `/internet-gateways` | POST | Create internet gateway | `POST /api/v1/internet-gateways` - {"vpcId":"vpc-123","name":"igw-prod"}` |
| `/internet-gateways/{igwId}` | DELETE | Delete internet gateway | `DELETE /api/v1/internet-gateways/igw-123` |
| `/nat-gateways` | GET | List NAT gateways | `GET /api/v1/nat-gateways?vpcId=vpc-123` |
| `/nat-gateways` | POST | Create NAT gateway | `POST /api/v1/nat-gateways` - {"subnetId":"subnet-456","allocationId":"eip-123"}` |
| `/nat-gateways/{natId}` | GET | Get NAT gateway | `GET /api/v1/nat-gateways/nat-123` |
| `/nat-gateways/{natId}` | DELETE | Delete NAT gateway | `DELETE /api/v1/nat-gateways/nat-123` |

#### Advanced Networking
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/networking/topology` | GET | Get network topology graph | `GET /api/v1/networking/topology` - Returns graph JSON |
| `/networking/dashboard` | GET | Get networking dashboard | `GET /api/v1/networking/dashboard` - Stats, utilization |
| `/reachability/analyze` | POST | Analyze network reachability | `POST /api/v1/reachability/analyze` - {"source":"i-123","target":"i-456"}` |
| `/flow-logs` | GET | List flow logs | `GET /api/v1/flow-logs?resourceId=vpc-123` |
| `/flow-logs` | POST | Create flow log | `POST /api/v1/flow-logs` - {"resourceId":"vpc-123","trafficType":"all"}` |
| `/vpc-endpoints` | GET | List VPC endpoints | `GET /api/v1/vpc-endpoints?vpcId=vpc-123` |

**Frontend Usage**:
- VPC list page calls `GET /vpcs` on load
- VPC detail page fetches detailed info via `/vpcs/{id}/detail`
- Subnet creation modal uses `POST /vpcs/{vpc}/subnets`
- Security group rule editor uses `/security-groups/{sgId}/rules` endpoints
- Topology visualization fetches graph via `/networking/topology`
- Network flow logs displayed on Monitoring page

---

## 4. Organizations & Account Management

### File: `api/orgs.ts`

| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/orgs` | GET | List organizations | `GET /api/v1/orgs` |
| `/orgs` | POST | Create organization | `POST /api/v1/orgs` - {"name":"acme-corp"}` |
| `/orgs/{org}` | GET | Get org details | `GET /api/v1/orgs/acme-corp` |
| `/orgs/{org}` | PATCH | Update organization | `PATCH /api/v1/orgs/acme-corp` - {"displayName":"ACME Inc"}` |
| `/orgs/{org}` | DELETE | Delete organization | `DELETE /api/v1/orgs/acme-corp` |
| `/orgs/{org}/accounts` | GET | List org accounts | `GET /api/v1/orgs/acme-corp/accounts` |
| `/orgs/{org}/accounts` | POST | Create account | `POST /api/v1/orgs/acme-corp/accounts` - {"name":"prod"}` |
| `/orgs/{org}/accounts/{account}` | GET | Get account details | `GET /api/v1/orgs/acme-corp/accounts/prod` |
| `/orgs/{org}/accounts/{account}` | PATCH | Update account | `PATCH /api/v1/orgs/acme-corp/accounts/prod` - {"status":"active"}` |
| `/orgs/{org}/accounts/{account}` | DELETE | Delete account | `DELETE /api/v1/orgs/acme-corp/accounts/prod` |
| `/orgs/{org}/accounts/{account}/suspend` | POST | Suspend account | `POST /api/v1/orgs/acme-corp/accounts/prod/suspend` |
| `/orgs/{org}/accounts/{account}/reactivate` | POST | Reactivate account | `POST /api/v1/orgs/acme-corp/accounts/prod/reactivate` |

**Frontend Usage**:
- Org switcher dropdown calls `GET /orgs`
- Org settings page displays via `/orgs/{org}`
- Account selector in header uses `/orgs/{org}/accounts`

---

## 5. IAM & Access Control

### File: `api/iam.ts`, `api/access.ts`

#### Account-Scoped IAM (Primary)
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/accounts/{account}/iam/users` | GET | List account IAM users | `GET /api/v1/accounts/prod/iam/users` |
| `/accounts/{account}/iam/users` | POST | Create account IAM user | `POST /api/v1/accounts/prod/iam/users` - {"name":"app-user","accessKeys":1}` |
| `/accounts/{account}/iam/users/{userId}` | GET | Get account IAM user | `GET /api/v1/accounts/prod/iam/users/user-123` |
| `/accounts/{account}/iam/users/{userId}` | PATCH | Update account IAM user | `PATCH /api/v1/accounts/prod/iam/users/user-123` - {"status":"active"}` |
| `/accounts/{account}/iam/users/{userId}` | DELETE | Delete account IAM user | `DELETE /api/v1/accounts/prod/iam/users/user-123` |
| `/accounts/{account}/iam/groups` | GET | List account IAM groups | `GET /api/v1/accounts/prod/iam/groups` |
| `/accounts/{account}/iam/groups` | POST | Create account group | `POST /api/v1/accounts/prod/iam/groups` - {"name":"admins"}` |
| `/accounts/{account}/iam/groups/{groupId}` | GET | Get account group | `GET /api/v1/accounts/prod/iam/groups/group-456` |
| `/accounts/{account}/iam/groups/{groupId}` | PATCH | Update account group | `PATCH /api/v1/accounts/prod/iam/groups/group-456` |
| `/accounts/{account}/iam/groups/{groupId}` | DELETE | Delete account group | `DELETE /api/v1/accounts/prod/iam/groups/group-456` |
| `/accounts/{account}/iam/groups/{groupId}/members` | POST | Add member to group | `POST /api/v1/accounts/prod/iam/groups/group-456/members` - {"userId":"user-123"}` |
| `/accounts/{account}/iam/groups/{groupId}/members/{userID}` | DELETE | Remove member from group | `DELETE /api/v1/accounts/prod/iam/groups/group-456/members/user-123` |
| `/accounts/{account}/iam/roles` | GET | List account roles | `GET /api/v1/accounts/prod/iam/roles` |
| `/accounts/{account}/iam/roles` | POST | Create account role | `POST /api/v1/accounts/prod/iam/roles` - {"name":"ec2-admin","permissions":["ec2:*"]}` |
| `/accounts/{account}/iam/roles/{roleId}` | GET | Get account role | `GET /api/v1/accounts/prod/iam/roles/role-789` |
| `/accounts/{account}/iam/roles/{roleId}` | PATCH | Update account role | `PATCH /api/v1/accounts/prod/iam/roles/role-789` |
| `/accounts/{account}/iam/roles/{roleId}` | DELETE | Delete account role | `DELETE /api/v1/accounts/prod/iam/roles/role-789` |
| `/accounts/{account}/iam/roles/{roleId}/assume` | POST | Assume role | `POST /api/v1/accounts/prod/iam/roles/role-789/assume` |
| `/accounts/{account}/iam/service-accounts` | GET | List service accounts | `GET /api/v1/accounts/prod/iam/service-accounts` |
| `/accounts/{account}/iam/service-accounts` | POST | Create service account | `POST /api/v1/accounts/prod/iam/service-accounts` - {"name":"ci-cd","permissions":["ec2:describe"]}` |
| `/accounts/{account}/iam/service-accounts/{id}` | DELETE | Delete service account | `DELETE /api/v1/accounts/prod/iam/service-accounts/sa-111` |
| `/accounts/{account}/iam/service-accounts/{id}/tokens` | POST | Issue service account token | `POST /api/v1/accounts/prod/iam/service-accounts/sa-111/tokens` |
| `/accounts/{account}/iam/policies` | GET | List account policies | `GET /api/v1/accounts/prod/iam/policies` |
| `/accounts/{account}/iam/policies` | POST | Create account policy | `POST /api/v1/accounts/prod/iam/policies` - {"name":"ec2-policy","statements":[...]}` |
| `/accounts/{account}/iam/policies/{id}` | GET | Get account policy | `GET /api/v1/accounts/prod/iam/policies/policy-222` |
| `/accounts/{account}/iam/policies/{id}` | PUT | Update account policy | `PUT /api/v1/accounts/prod/iam/policies/policy-222` |
| `/accounts/{account}/iam/policies/{id}` | DELETE | Delete account policy | `DELETE /api/v1/accounts/prod/iam/policies/policy-222` |
| `/accounts/{account}/iam/policies/{id}/attach` | POST | Attach policy to role | `POST /api/v1/accounts/prod/iam/policies/policy-222/attach` - {"principalId":"role-789"}` |
| `/accounts/{account}/iam/policies/{id}/detach` | POST | Detach policy from role | `POST /api/v1/accounts/prod/iam/policies/policy-222/detach` |
| `/accounts/{account}/iam/simulate` | POST | Simulate IAM permission | `POST /api/v1/accounts/prod/iam/simulate` - {"action":"ec2:RunInstances"}` |
| `/accounts/{account}/audit` | GET | List account audit events | `GET /api/v1/accounts/prod/audit?limit=100` |

#### RBAC Users
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/users` | GET | List RBAC users | `GET /api/v1/users` |
| `/users` | POST | Create RBAC user | `POST /api/v1/users` - {"email":"admin@example.com"}` |
| `/users/me` | GET | Get current user | `GET /api/v1/users/me` |
| `/users/me` | PATCH | Update own profile | `PATCH /api/v1/users/me` - {"displayName":"John Doe"}` |
| `/users/me/password` | POST | Change own password | `POST /api/v1/users/me/password` - {"current":"old","new":"new"}` |
| `/users/{id}/password` | POST | Set user password (admin) | `POST /api/v1/users/admin-123/password` - {"password":"newpass"}` |
| `/users/{id}/approve` | POST | Approve pending user | `POST /api/v1/users/user-pending/approve` |
| `/users/{id}/disable` | POST | Disable user | `POST /api/v1/users/user-123/disable` |
| `/users/{id}/roles` | POST | Grant role to user | `POST /api/v1/users/user-123/roles` - {"role":"admin"}` |
| `/users/{id}/roles/{role}` | DELETE | Revoke role from user | `DELETE /api/v1/users/user-123/roles/admin` |
| `/iam/assume-role` | POST | Assume global role (CRN-based) | `POST /api/v1/iam/assume-role` - {"roleArn":"crn:..."}` |

**Frontend Usage**:
- IAM management page uses account-scoped endpoints
- User list calls `GET /accounts/{account}/iam/users`
- Policy editor uses `/accounts/{account}/iam/policies`
- Role assignment uses `/users/{id}/roles` endpoint
- Permission simulation via `/accounts/{account}/iam/simulate`

---

## 6. Certificates & TLS

### File: `api/certificates.ts`

| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/certs` | GET | List certificates | `GET /api/v1/certs` |
| `/certs/{id}` | GET | Get certificate | `GET /api/v1/certs/cert-123` |
| `/certificates` | GET | List certificates (v2) | `GET /api/v1/certificates` |
| `/certificates` | POST | Create certificate | `POST /api/v1/certificates` - {"name":"example.com","type":"self-signed"}` |
| `/certificates/{id}` | GET | Get certificate (v2) | `GET /api/v1/certificates/cert-123` |
| `/certificates/{id}` | DELETE | Delete certificate | `DELETE /api/v1/certificates/cert-123` |
| `/certificates/{id}/renew` | POST | Renew certificate | `POST /api/v1/certificates/cert-123/renew` |
| `/certificates/{id}/reissue` | POST | Reissue certificate | `POST /api/v1/certificates/cert-123/reissue` |
| `/certificates/{id}/revoke` | POST | Revoke certificate | `POST /api/v1/certificates/cert-123/revoke` |
| `/certificates/{id}/bindings` | GET | List certificate bindings | `GET /api/v1/certificates/cert-123/bindings` |
| `/certificates/{id}/bindings` | POST | Create certificate binding | `POST /api/v1/certificates/cert-123/bindings` - {"resourceId":"lb-456"}` |
| `/certificates/{certId}/bindings/{bindingId}` | DELETE | Delete certificate binding | `DELETE /api/v1/certificates/cert-123/bindings/bind-789` |
| `/certificates/acme-accounts` | GET | List ACME accounts | `GET /api/v1/certificates/acme-accounts` |
| `/certificates/acme-accounts` | POST | Create ACME account | `POST /api/v1/certificates/acme-accounts` - {"email":"admin@example.com"}` |
| `/certificates/acme-accounts/{id}` | DELETE | Delete ACME account | `DELETE /api/v1/certificates/acme-accounts/acme-123` |

**Frontend Usage**:
- Certificates page fetches list via `GET /certificates`
- Certificate details shown via GET endpoint
- ACME account management for Let's Encrypt integration
- Bindings show where certificates are used

---

## 7. Resource Monitoring & Observability

### File: `api/resourcemon.ts`, `api/resources.ts`

#### Resources & Config
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/resources` | GET | List resources (with filters) | `GET /api/v1/resources?type=instance&label=env=prod` |
| `/resources/{id}` | GET | Get resource details | `GET /api/v1/resources/res-123` |
| `/resources/{id}/config` | GET | Get resource config | `GET /api/v1/resources/res-123/config` |
| `/resources/{id}/events` | GET | Get resource events | `GET /api/v1/resources/res-123/events?limit=50` |
| `/resources/{id}/metrics` | GET | Get resource metrics | `GET /api/v1/resources/res-123/metrics?period=1h` |
| `/resources/sync` | POST | Sync resources | `POST /api/v1/resources/sync` |
| `/resources/{resourceId}/drift/repair` | POST | Repair config drift | `POST /api/v1/resources/res-123/drift/repair` |
| `/config/drift` | GET | List all drifts | `GET /api/v1/config/drift?limit=100` |

#### Metrics
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/metrics/ingest` | POST | Ingest custom metrics | `POST /api/v1/metrics/ingest` - {"metric":"cpu_usage","value":75}` |
| `/metrics/query` | GET | Query metrics | `GET /api/v1/metrics/query?metric=cpu_usage&start=1h-ago` |
| `/metrics/custom` | POST | Push custom metric | `POST /api/v1/metrics/custom` - {"name":"deploy_duration","value":120}` |

#### Events
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/resource-events` | GET | List resource events | `GET /api/v1/resource-events?resourceId=res-123` |
| `/resource-events` | POST | Create resource event | `POST /api/v1/resource-events` - {"resourceId":"res-123","type":"deployment"}` |

#### Alerts & Rules
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/alerts` | GET | List alerts | `GET /api/v1/alerts?status=firing` |
| `/alerts/rules` | GET | List alert rules | `GET /api/v1/alerts/rules` |
| `/alerts/rules` | POST | Create alert rule | `POST /api/v1/alerts/rules` - {"name":"high-cpu","condition":"cpu>80"}` |
| `/alerts/rules/{id}` | PATCH | Update alert rule | `PATCH /api/v1/alerts/rules/rule-123` |
| `/alerts/rules/{id}` | DELETE | Delete alert rule | `DELETE /api/v1/alerts/rules/rule-123` |
| `/alerts/{id}/ack` | POST | Acknowledge alert | `POST /api/v1/alerts/alert-456/ack` |
| `/alerts/{id}/resolve` | POST | Resolve alert | `POST /api/v1/alerts/alert-456/resolve` |

#### Service Monitoring
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/{service}/{id}/monitoring` | GET | Get monitoring data | `GET /api/v1/instances/i-123/monitoring` |

**Frontend Usage**:
- Resources page fetches via `GET /resources` with filters
- Monitoring dashboard fetches metrics via `/metrics/query`
- Alerts page fetches via `GET /alerts` and `GET /alerts/rules`
- Resource detail page shows config/events/metrics tabs

---

## 8. Images & Compute Types

### File: `api/images.ts`

| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/images` | GET | List images | `GET /api/v1/images?filter=custom` |
| `/images/{name}` | GET | Get image details | `GET /api/v1/images/ubuntu-20.04` |
| `/images/{name}` | DELETE | Delete image | `DELETE /api/v1/images/custom-image-v1` |
| `/images/{name}/scan` | POST | Scan image vulnerabilities | `POST /api/v1/images/custom-image-v1/scan` |
| `/images/{name}/sbom` | GET | Get image SBOM | `GET /api/v1/images/custom-image-v1/sbom` |
| `/images/{name}/sbom` | POST | Update image SBOM | `POST /api/v1/images/custom-image-v1/sbom` |
| `/images/{name}/provenance` | GET | Get image provenance | `GET /api/v1/images/custom-image-v1/provenance` |
| `/images/{name}/provenance` | POST | Update image provenance | `POST /api/v1/images/custom-image-v1/provenance` |
| `/images/import` | POST | Import image | `POST /api/v1/images/import` - {"source":"s3://bucket/image.ami"}` |
| `/images/upload` | POST | Upload image | `POST /api/v1/images/upload` - multipart/form-data |
| `/images/{name}/publish` | POST | Publish image | `POST /api/v1/images/custom-image-v1/publish` |

#### Capsule Types
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/capsule-types` | GET | List capsule types | `GET /api/v1/capsule-types` |
| `/capsule-types` | POST | Create capsule type | `POST /api/v1/capsule-types` - {"name":"custom-t3"}` |
| `/capsule-types/{name}` | GET | Get capsule type | `GET /api/v1/capsule-types/t2.medium` |
| `/capsule-types/{name}/audit` | GET | Get capsule type audit | `GET /api/v1/capsule-types/t2.medium/audit` |
| `/capsule-types/{name}/deprecate` | POST | Deprecate capsule type | `POST /api/v1/capsule-types/t2.micro/deprecate` |
| `/capsule-types/{name}` | DELETE | Delete capsule type | `DELETE /api/v1/capsule-types/custom-t3` |
| `/instance-types` | GET | List instance types (alias) | `GET /api/v1/instance-types` |
| `/instance-types/{name}` | GET | Get instance type (alias) | `GET /api/v1/instance-types/t2.medium` |

**Frontend Usage**:
- Image management page fetches via `GET /images`
- Image upload modal uses `POST /images/upload`
- Compute type selector uses `/instance-types` or `/capsule-types`
- Vulnerability scanner uses `POST /images/{name}/scan`

---

## 9. Key Management & Secrets

### File: `api/topology.ts`

#### KMS Keys
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/kms/keys` | GET | List KMS keys | `GET /api/v1/kms/keys` |
| `/kms/keys` | POST | Create KMS key | `POST /api/v1/kms/keys` - {"name":"prod-key","algorithm":"aes256"}` |
| `/kms/keys/{name}` | DELETE | Delete KMS key | `DELETE /api/v1/kms/keys/prod-key` |
| `/kms/keys/{name}/rotate` | POST | Rotate KMS key | `POST /api/v1/kms/keys/prod-key/rotate` |
| `/kms/keys/{name}/encrypt` | POST | Encrypt with KMS key | `POST /api/v1/kms/keys/prod-key/encrypt` - {"plaintext":"secret"}` |
| `/kms/keys/{name}/decrypt` | POST | Decrypt with KMS key | `POST /api/v1/kms/keys/prod-key/decrypt` - {"ciphertext":"encrypted"}` |

### File: `api/secrets.ts`

#### Secrets Management
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/secrets` | GET | List secrets | `GET /api/v1/secrets` |
| `/secrets` | POST | Create secret | `POST /api/v1/secrets` - {"name":"db-password","value":"p@ssw0rd"}` |
| `/secrets/{name}` | GET | Get secret | `GET /api/v1/secrets/db-password` |
| `/secrets/{name}` | DELETE | Delete secret | `DELETE /api/v1/secrets/db-password` |

**Frontend Usage**:
- Secrets manager page uses `/secrets` endpoints
- KMS key management uses `/kms/keys` endpoints
- Encryption/decryption operations use `POST /kms/keys/{name}/encrypt/decrypt`

---

## 10. Admin Functions

### File: `api/admin.ts`, `api/hostsec.ts`

#### Storage Administration
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/admin/disks` | GET | List disks | `GET /api/v1/admin/disks` |
| `/admin/storage-pools` | GET | List storage pools | `GET /api/v1/admin/storage-pools` |
| `/admin/storage-pools` | POST | Create storage pool | `POST /api/v1/admin/storage-pools` - {"name":"nvme-pool","type":"nvme"}` |
| `/admin/storage-pools/{id}` | DELETE | Delete storage pool | `DELETE /api/v1/admin/storage-pools/pool-123` |
| `/admin/storage-pools/{id}/allocations` | GET | List storage allocations | `GET /api/v1/admin/storage-pools/pool-123/allocations` |
| `/admin/storage-pools/{id}/allocations` | POST | Create allocation | `POST /api/v1/admin/storage-pools/pool-123/allocations` - {"size":"100G"}` |
| `/admin/storage-allocations/{id}` | DELETE | Delete allocation | `DELETE /api/v1/admin/storage-allocations/alloc-456` |
| `/admin/storage/settings` | GET | Get storage settings | `GET /api/v1/admin/storage/settings` |
| `/admin/storage/settings` | PUT | Update storage settings | `PUT /api/v1/admin/storage/settings` - {"compression":"zstd"}` |

#### Host Limits
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/admin/limits/host` | GET | Get host limits | `GET /api/v1/admin/limits/host` |
| `/admin/limits/host` | PUT | Set host limits | `PUT /api/v1/admin/limits/host` - {"maxInstances":100}` |

#### IP Exclusions
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/admin/ip-exclusions` | GET | List IP exclusions | `GET /api/v1/admin/ip-exclusions` |
| `/admin/ip-exclusions` | POST | Create IP exclusion | `POST /api/v1/admin/ip-exclusions` - {"ip":"192.168.1.1","reason":"reserved"}` |
| `/admin/ip-exclusions/{id}` | DELETE | Delete IP exclusion | `DELETE /api/v1/admin/ip-exclusions/ex-123` |

#### Fail2Ban
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/admin/fail2ban/status` | GET | Get fail2ban status | `GET /api/v1/admin/fail2ban/status` |
| `/admin/fail2ban/ban` | POST | Ban IP | `POST /api/v1/admin/fail2ban/ban` - {"ip":"1.2.3.4"}` |
| `/admin/fail2ban/unban` | POST | Unban IP | `POST /api/v1/admin/fail2ban/unban` - {"ip":"1.2.3.4"}` |
| `/admin/fail2ban/unban-all` | POST | Unban all | `POST /api/v1/admin/fail2ban/unban-all` |
| `/admin/fail2ban/flush` | POST | Flush fail2ban | `POST /api/v1/admin/fail2ban/flush` |
| `/admin/fail2ban/reload` | POST | Reload fail2ban | `POST /api/v1/admin/fail2ban/reload` |
| `/admin/fail2ban/blocklist` | GET | Get blocklist | `GET /api/v1/admin/fail2ban/blocklist` |
| `/admin/fail2ban/blocklist` | POST | Add to blocklist | `POST /api/v1/admin/fail2ban/blocklist` - {"ip":"1.2.3.4"}` |
| `/admin/fail2ban/blocklist/{id}` | DELETE | Remove from blocklist | `DELETE /api/v1/admin/fail2ban/blocklist/block-123` |
| `/admin/fail2ban/allowlist` | GET | Get allowlist | `GET /api/v1/admin/fail2ban/allowlist` |
| `/admin/fail2ban/allowlist` | PUT | Set allowlist | `PUT /api/v1/admin/fail2ban/allowlist` - {"ips":["10.0.0.0/8"]}` |

#### UFW Firewall
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/admin/ufw/status` | GET | Get UFW status | `GET /api/v1/admin/ufw/status` |
| `/admin/ufw/rules` | POST | Add UFW rule | `POST /api/v1/admin/ufw/rules` - {"action":"allow","port":22}` |
| `/admin/ufw/rules/{num}` | DELETE | Delete UFW rule | `DELETE /api/v1/admin/ufw/rules/5` |
| `/admin/ufw/defaults` | GET | Get UFW defaults | `GET /api/v1/admin/ufw/defaults` |
| `/admin/ufw/defaults` | PUT | Set UFW defaults | `PUT /api/v1/admin/ufw/defaults` - {"inbound":"deny","outbound":"allow"}` |
| `/admin/ufw/enable` | POST | Enable UFW | `POST /api/v1/admin/ufw/enable` |
| `/admin/ufw/disable` | POST | Disable UFW | `POST /api/v1/admin/ufw/disable` |

#### Host Security
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/admin/hostsec/nodes` | GET | Get host security nodes | `GET /api/v1/admin/hostsec/nodes` |

**Frontend Usage**:
- Admin dashboard fetches storage/host info via admin endpoints
- Fail2ban management uses ban/unban/blocklist endpoints
- UFW firewall rules managed via `/admin/ufw/rules`

---

## 11. Serverless & Functions

### File: `api/serverless.ts`

#### Functions
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/functions` | GET | List functions | `GET /api/v1/functions` |
| `/functions` | POST | Create function | `POST /api/v1/functions` - {"name":"api-handler","runtime":"python3.9","code":"..."}` |
| `/functions/{id}` | GET | Get function | `GET /api/v1/functions/func-123` |
| `/functions/{id}` | PATCH | Update function | `PATCH /api/v1/functions/func-123` - {"memory":512}` |
| `/functions/{id}` | DELETE | Delete function | `DELETE /api/v1/functions/func-123` |
| `/functions/{id}/versions` | GET | List versions | `GET /api/v1/functions/func-123/versions` |
| `/functions/{id}/versions` | POST | Create version | `POST /api/v1/functions/func-123/versions` - {"code":"..."}` |
| `/functions/{id}/invoke` | POST | Invoke function | `POST /api/v1/functions/func-123/invoke` - {"payload":{}}` |
| `/functions/{id}/triggers` | GET | List triggers | `GET /api/v1/functions/func-123/triggers` |
| `/functions/{id}/triggers` | POST | Create trigger | `POST /api/v1/functions/func-123/triggers` - {"type":"s3","bucket":"mybucket"}` |
| `/functions/{id}/triggers/{triggerId}` | DELETE | Delete trigger | `DELETE /api/v1/functions/func-123/triggers/trigger-456` |
| `/functions/{id}/invocations` | GET | List invocations | `GET /api/v1/functions/func-123/invocations?limit=50` |

#### MCP Servers
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/mcp/servers` | GET | List MCP servers | `GET /api/v1/mcp/servers` |
| `/mcp/servers` | POST | Create MCP server | `POST /api/v1/mcp/servers` - {"name":"web-scraper","url":"http://..."}` |
| `/mcp/servers/{id}` | GET | Get MCP server | `GET /api/v1/mcp/servers/mcp-123` |
| `/mcp/servers/{id}` | DELETE | Delete MCP server | `DELETE /api/v1/mcp/servers/mcp-123` |
| `/mcp/servers/{id}/tools` | GET | List MCP tools | `GET /api/v1/mcp/servers/mcp-123/tools` |
| `/mcp/servers/{id}/tools/sync` | POST | Sync MCP tools | `POST /api/v1/mcp/servers/mcp-123/tools/sync` |
| `/mcp/servers/{id}/tools/{toolName}/invoke` | POST | Invoke MCP tool | `POST /api/v1/mcp/servers/mcp-123/tools/search/invoke` - {"query":"..."}` |
| `/mcp/servers/{id}/invocations` | GET | List MCP invocations | `GET /api/v1/mcp/servers/mcp-123/invocations` |
| `/mcp/approvals` | GET | List MCP approvals | `GET /api/v1/mcp/approvals` |
| `/mcp/approvals/{id}/approve` | POST | Approve MCP action | `POST /api/v1/mcp/approvals/approval-789/approve` |
| `/mcp/approvals/{id}/deny` | POST | Deny MCP action | `POST /api/v1/mcp/approvals/approval-789/deny` |

**Frontend Usage**:
- Functions page lists via `GET /functions`
- Function editor uses `PATCH /functions/{id}`
- Function invocation via `POST /functions/{id}/invoke`
- MCP server management uses `/mcp/servers` endpoints
- Tool invocation uses `/mcp/servers/{id}/tools/{toolName}/invoke`

---

## 12. Storage & Backups

### File: `api/extras.ts`

#### S3 Buckets & Objects
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/storage/buckets` | GET | List buckets | `GET /api/v1/storage/buckets` |
| `/storage/buckets` | POST | Create bucket | `POST /api/v1/storage/buckets` - {"name":"my-bucket"}` |
| `/storage/buckets/{bucket}` | GET | Get bucket details | `GET /api/v1/storage/buckets/my-bucket` |
| `/storage/buckets/{bucket}` | DELETE | Delete bucket | `DELETE /api/v1/storage/buckets/my-bucket` |
| `/storage/buckets/{bucket}/objects` | GET | List objects | `GET /api/v1/storage/buckets/my-bucket/objects?prefix=logs/` |
| `/storage/buckets/{bucket}/objects/{key...}` | GET | Get object | `GET /api/v1/storage/buckets/my-bucket/objects/file.txt` |
| `/storage/buckets/{bucket}/objects/{key...}` | PUT | Put object (raw) | `PUT /api/v1/storage/buckets/my-bucket/objects/file.txt` |
| `/storage/buckets/{bucket}/objects/{key...}` | POST | Put object (form) | `POST /api/v1/storage/buckets/my-bucket/objects/file.txt` |
| `/storage/buckets/{bucket}/objects/{key...}` | DELETE | Delete object | `DELETE /api/v1/storage/buckets/my-bucket/objects/file.txt` |

#### Volumes
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/storage/volumes` | GET | List volumes | `GET /api/v1/storage/volumes` |
| `/storage/volumes` | POST | Create volume | `POST /api/v1/storage/volumes` - {"name":"data-vol","size":"100G"}` |
| `/storage/volumes/{name}/attach` | POST | Attach volume | `POST /api/v1/storage/volumes/data-vol/attach` - {"instanceId":"i-123"}` |
| `/storage/volumes/{name}/detach` | POST | Detach volume | `POST /api/v1/storage/volumes/data-vol/detach` |
| `/storage/volumes/{name}` | DELETE | Delete volume | `DELETE /api/v1/storage/volumes/data-vol` |

#### Backups
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/backups` | GET | List backups | `GET /api/v1/backups` |
| `/backups` | POST | Create backup | `POST /api/v1/backups` - {"resourceId":"i-123","name":"backup-1"}` |
| `/backups/{id}/restore` | POST | Restore backup | `POST /api/v1/backups/backup-456/restore` |
| `/backup-policies` | GET | List backup policies | `GET /api/v1/backup-policies` |
| `/backup-policies` | POST | Create backup policy | `POST /api/v1/backup-policies` - {"schedule":"daily","retention":7}` |
| `/backup-policies/{name}` | DELETE | Delete backup policy | `DELETE /api/v1/backup-policies/daily-policy` |

#### Databases
| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/databases` | GET | List databases | `GET /api/v1/databases` |
| `/databases` | POST | Create database | `POST /api/v1/databases` - {"name":"mydb","engine":"postgres"}` |
| `/databases/{name}` | GET | Get database | `GET /api/v1/databases/mydb` |
| `/databases/{name}` | DELETE | Delete database | `DELETE /api/v1/databases/mydb` |

**Frontend Usage**:
- S3 browser page fetches buckets via `GET /storage/buckets`
- Object listing uses `/storage/buckets/{bucket}/objects`
- Backup management uses `/backups` and `/backup-policies` endpoints
- Database console uses `/databases` endpoints

---

## 13. Load Balancers

### File: `api/extras.ts`

| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/lb` | GET | List load balancers | `GET /api/v1/lb` |
| `/lb` | POST | Create LB | `POST /api/v1/lb` - {"name":"api-lb","type":"application"}` |
| `/lb/{name}` | GET | Get LB details | `GET /api/v1/lb/api-lb` |
| `/lb/{name}` | DELETE | Delete LB | `DELETE /api/v1/lb/api-lb` |
| `/lb/{name}/listeners` | GET | List LB listeners | `GET /api/v1/lb/api-lb/listeners` |
| `/lb/{name}/listeners` | POST | Create listener | `POST /api/v1/lb/api-lb/listeners` - {"port":443,"protocol":"https"}` |
| `/lb/{name}/listeners/{id}` | GET | Get listener | `GET /api/v1/lb/api-lb/listeners/listener-789` |
| `/lb/{name}/listeners/{id}` | PATCH | Update listener | `PATCH /api/v1/lb/api-lb/listeners/listener-789` |
| `/lb/{name}/listeners/{id}` | DELETE | Delete listener | `DELETE /api/v1/lb/api-lb/listeners/listener-789` |
| `/lb/{name}/listeners/{id}/certificates` | POST | Attach certificate | `POST /api/v1/lb/api-lb/listeners/listener-789/certificates` - {"certId":"cert-123"}` |
| `/lb/{name}/listeners/{id}/certificates` | DELETE | Detach certificate | `DELETE /api/v1/lb/api-lb/listeners/listener-789/certificates` |
| `/lb/{name}/target-groups` | GET | List target groups | `GET /api/v1/lb/api-lb/target-groups` |
| `/lb/{name}/target-groups` | POST | Create target group | `POST /api/v1/lb/api-lb/target-groups` - {"name":"web-tg","port":80}` |
| `/lb/{name}/target-groups/{tgId}` | DELETE | Delete target group | `DELETE /api/v1/lb/api-lb/target-groups/tg-111` |
| `/lb/{name}/target-groups/{tgId}/targets` | GET | List targets | `GET /api/v1/lb/api-lb/target-groups/tg-111/targets` |
| `/lb/{name}/target-groups/{tgId}/targets` | POST | Add target | `POST /api/v1/lb/api-lb/target-groups/tg-111/targets` - {"instanceId":"i-123"}` |
| `/lb/{name}/target-groups/{tgId}/targets/{targetId}` | DELETE | Remove target | `DELETE /api/v1/lb/api-lb/target-groups/tg-111/targets/target-222` |
| `/target-groups` | GET | List target groups (global) | `GET /api/v1/target-groups` |
| `/target-groups` | POST | Create target group | `POST /api/v1/target-groups` - {"name":"tg-global"}` |

**Frontend Usage**:
- Load Balancers page fetches via `GET /lb`
- LB detail page shows listeners and target groups
- Listener configuration uses PATCH endpoint
- Target management via `/lb/{name}/target-groups/{tgId}/targets`

---

## 14. Deletion Flow

### File: `capperweb-components/deletion-api-client.ts`

| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/{resourceType}/{resourceId}/delete-preflight` | POST | Get deletion plan | `POST /api/v1/instance/i-123/delete-preflight` |
| `/{resourceType}/{resourceId}/delete-confirm` | POST | Confirm deletion | `POST /api/v1/instance/i-123/delete-confirm` - {"confirmationToken":"token","confirmationPhrase":"DELETE"}` |
| `/deletion-jobs/{jobId}` | GET | Poll deletion status | `GET /api/v1/deletion-jobs/job-456` |

**Frontend Usage**:
- Delete buttons trigger 3-phase flow:
  1. `DELETE` action calls POST preflight
  2. User confirms with "DELETE" phrase
  3. POST confirm initiates async job
  4. Frontend polls `/deletion-jobs/{jobId}` until complete

---

## 15. System & Utility Endpoints

### File: Various

| Endpoint | Method | Purpose | Example Call |
|----------|--------|---------|--------------|
| `/events?limit={limit}` | GET | Get recent events | `GET /api/v1/events?limit=50` |
| `/health` | GET | Health check | `GET /api/v1/health` |
| `/version` | GET | Get API version | `GET /api/v1/version` |
| `/daemon/status` | GET | Get daemon status | `GET /api/v1/daemon/status` |
| `/instance-disk-capacity` | GET | Get disk capacity | `GET /api/v1/instance-disk-capacity` |

---

## Summary Statistics

**Total Frontend API Calls: 300+**

**Breakdown by Category:**
- VPC & Networking: 80+ endpoints
- IAM & Access Control: 70+ endpoints
- Instances & Compute: 40+ endpoints
- Storage & Backups: 35+ endpoints
- Admin Functions: 30+ endpoints
- Serverless & Functions: 25+ endpoints
- Monitoring & Observability: 25+ endpoints
- Load Balancers: 20+ endpoints
- Images & Compute Types: 20+ endpoints
- Org & Account Management: 20+ endpoints
- Certificates: 15+ endpoints
- Secrets & Encryption: 10+ endpoints
- System Utilities: 5+ endpoints

**Key Frontend Characteristics:**
- Base URL: `/api/v1` (configurable via `VITE_CAPPER_API_URL`)
- HTTP Client: Native `fetch()` with custom wrapper
- Authentication: CSRF tokens + X-Capper-* headers
- State Management: React Query (TanStack Query)
- Framework: React + TypeScript
- API Modules: 20 separate client files

**Deletion Framework:**
- Two-step confirmation flow
- Async job tracking via polling
- Supports cascading deletions

---

**Last Updated: 2026-07-01**
