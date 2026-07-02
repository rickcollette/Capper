# PASS1: Backend API Endpoint Review

## Overview
Complete audit of all Capper REST API endpoints available for frontend consumption. API base path: `/api/v1/`

---

## 1. Organization & Account Management

### Orgs
- `GET /api/v1/orgs` - List all organizations
- `POST /api/v1/orgs` - Create new organization
- `GET /api/v1/orgs/{org}` - Get org details
- `PATCH /api/v1/orgs/{org}` - Update organization
- `DELETE /api/v1/orgs/{org}` - Delete organization

### Org Accounts
- `GET /api/v1/orgs/{org}/accounts` - List accounts in org
- `POST /api/v1/orgs/{org}/accounts` - Create account
- `GET /api/v1/orgs/{org}/accounts/{account}` - Get account details
- `PATCH /api/v1/orgs/{org}/accounts/{account}` - Update account
- `DELETE /api/v1/orgs/{org}/accounts/{account}` - Delete account
- `POST /api/v1/orgs/{org}/accounts/{account}/suspend` - Suspend account
- `POST /api/v1/orgs/{org}/accounts/{account}/reactivate` - Reactivate account

### Org Root Users
- `GET /api/v1/orgs/{org}/root-users` - List org root users
- `POST /api/v1/orgs/{org}/root-users` - Add root user to org
- `DELETE /api/v1/orgs/{org}/root-users/{userID}` - Remove root user

### Account Root Users
- `GET /api/v1/orgs/{org}/accounts/{account}/root-users` - List account root users
- `POST /api/v1/orgs/{org}/accounts/{account}/root-users` - Add root user to account
- `DELETE /api/v1/orgs/{org}/accounts/{account}/root-users/{userID}` - Remove account root user

### Guardrails
- `GET /api/v1/orgs/{org}/guardrails` - List org guardrails
- `POST /api/v1/orgs/{org}/guardrails` - Create guardrail
- `GET /api/v1/orgs/{org}/guardrails/{id}` - Get guardrail
- `DELETE /api/v1/orgs/{org}/guardrails/{id}` - Delete guardrail

---

## 2. Authentication & Sessions

- `POST /api/v1/auth/session` - Create session
- `DELETE /api/v1/auth/session` - Delete/logout session
- `GET /api/v1/auth/session` - Get session info
- `POST /api/v1/auth/login` - Local login
- `GET /api/v1/auth/google/callback` - Google OAuth callback

---

## 3. User Management (RBAC)

### User Lifecycle
- `GET /api/v1/users` - List RBAC users
- `POST /api/v1/users` - Create RBAC user
- `GET /api/v1/users/me` - Get current user identity
- `PATCH /api/v1/users/me` - Update own profile
- `POST /api/v1/users/me/password` - Change own password
- `POST /api/v1/users/{id}/password` - Set user password (admin)
- `POST /api/v1/users/{id}/approve` - Approve pending user
- `POST /api/v1/users/{id}/disable` - Disable user

### User Roles & Groups
- `POST /api/v1/users/{id}/roles` - Grant role to user
- `DELETE /api/v1/users/{id}/roles/{role}` - Revoke role from user

### IAM Groups
- `GET /api/v1/iam/groups` - List IAM groups
- `POST /api/v1/iam/groups` - Create group
- `POST /api/v1/iam/groups/{group}/members` - Add user to group
- `DELETE /api/v1/iam/groups/{group}/members/{user}` - Remove user from group

### IAM Roles & Policies
- `GET /api/v1/iam/roles` - List IAM roles
- `POST /api/v1/iam/roles` - Create IAM role
- `GET /api/v1/iam/policies` - List IAM policies
- `POST /api/v1/iam/policies` - Create IAM policy
- `POST /api/v1/iam/simulate` - Simulate IAM permission
- `GET /api/v1/iam/audit` - Get IAM audit log

### IAM Tokens
- `POST /api/v1/iam/tokens` - Issue IAM token
- `GET /api/v1/iam/tokens` - List tokens

### IAM Users (Legacy)
- `GET /api/v1/iam/users` - List IAM users
- `POST /api/v1/iam/users` - Create IAM user
- `DELETE /api/v1/iam/users/{name}` - Delete IAM user

### Account-Scoped IAM
- `GET /api/v1/accounts/{account}/iam/users` - List account IAM users
- `POST /api/v1/accounts/{account}/iam/users` - Create account IAM user
- `GET /api/v1/accounts/{account}/iam/users/{userId}` - Get account IAM user
- `PATCH /api/v1/accounts/{account}/iam/users/{userId}` - Update account IAM user
- `DELETE /api/v1/accounts/{account}/iam/users/{id}` - Delete account IAM user

### Account IAM Groups
- `GET /api/v1/accounts/{account}/iam/groups` - List account IAM groups
- `POST /api/v1/accounts/{account}/iam/groups` - Create account group
- `GET /api/v1/accounts/{account}/iam/groups/{groupId}` - Get account group
- `PATCH /api/v1/accounts/{account}/iam/groups/{groupId}` - Update account group
- `DELETE /api/v1/accounts/{account}/iam/groups/{id}` - Delete account group
- `POST /api/v1/accounts/{account}/iam/groups/{id}/members` - Add member to account group
- `DELETE /api/v1/accounts/{account}/iam/groups/{id}/members/{userID}` - Remove member from account group

### Account IAM Roles & Policies
- `GET /api/v1/accounts/{account}/iam/roles` - List account IAM roles
- `POST /api/v1/accounts/{account}/iam/roles` - Create account IAM role
- `GET /api/v1/accounts/{account}/iam/roles/{roleId}` - Get account IAM role
- `PATCH /api/v1/accounts/{account}/iam/roles/{roleId}` - Update account IAM role
- `DELETE /api/v1/accounts/{account}/iam/roles/{id}` - Delete account IAM role
- `POST /api/v1/accounts/{account}/iam/roles/{roleId}/assume` - Assume account role
- `GET /api/v1/accounts/{account}/iam/policies` - List account IAM policies
- `POST /api/v1/accounts/{account}/iam/policies` - Create account IAM policy
- `GET /api/v1/accounts/{account}/iam/policies/{id}` - Get account IAM policy
- `PUT /api/v1/accounts/{account}/iam/policies/{id}` - Update account IAM policy
- `DELETE /api/v1/accounts/{account}/iam/policies/{id}` - Delete account IAM policy
- `POST /api/v1/accounts/{account}/iam/policies/{id}/attach` - Attach policy to role
- `POST /api/v1/accounts/{account}/iam/policies/{id}/detach` - Detach policy from role
- `POST /api/v1/accounts/{account}/iam/simulate` - Simulate account IAM permission

### Account Service Accounts
- `GET /api/v1/accounts/{account}/iam/service-accounts` - List service accounts
- `POST /api/v1/accounts/{account}/iam/service-accounts` - Create service account
- `DELETE /api/v1/accounts/{account}/iam/service-accounts/{id}` - Delete service account
- `POST /api/v1/accounts/{account}/iam/service-accounts/{id}/tokens` - Issue service account token

### Cross-Account
- `POST /api/v1/iam/assume-role` - Assume role across accounts

---

## 4. Instances (Compute)

### Instance Lifecycle
- `GET /api/v1/instances` - List instances
- `POST /api/v1/instances` - Create instance
- `GET /api/v1/instances/{id}` - Get instance details
- `PATCH /api/v1/instances/{id}` - Update instance
- `DELETE /api/v1/instances/{id}` - Delete instance
- `POST /api/v1/instances/{id}/start` - Start instance
- `POST /api/v1/instances/{id}/stop` - Stop instance
- `POST /api/v1/instances/{id}/restart` - Restart instance
- `POST /api/v1/instances/{id}/reboot` - Reboot instance

### Instance Protection
- `POST /api/v1/instances/{id}/protect-termination` - Enable termination protection
- `DELETE /api/v1/instances/{id}/protect-termination` - Disable termination protection

### Instance Networking
- `POST /api/v1/instances/{id}/attach-network-interface` - Attach ENI to instance
- `POST /api/v1/instances/{id}/detach-network-interface` - Detach ENI from instance

### Instance Monitoring & Logs
- `GET /api/v1/instances/{id}/logs` - Get instance logs
- `GET /api/v1/instances/{id}/logs/stdout` - Get instance stdout
- `GET /api/v1/instances/{id}/logs/stderr` - Get instance stderr
- `GET /api/v1/instances/{id}/events` - Get instance events
- `GET /api/v1/instances/{id}/monitoring` - Get instance monitoring data

### Instance Terminal
- `GET /api/v1/instances/{id}/terminal` - Get terminal access

### Instance Metadata
- `GET /api/v1/instances/{id}/metadata` - Get instance metadata
- `PUT /api/v1/instances/{id}/metadata` - Set instance metadata
- `GET /api/v1/instances/{id}/metadata/{tab}` - Get metadata tab

### Instance Capacity
- `GET /api/v1/instance-disk-capacity` - Get disk capacity info

---

## 5. Images & Compute Types

### Images
- `GET /api/v1/images` - List images
- `POST /api/v1/images/import` - Import image
- `POST /api/v1/images/upload` - Upload image
- `GET /api/v1/images/{name}` - Get image details
- `DELETE /api/v1/images/{name}` - Delete image
- `POST /api/v1/images/{name}/scan` - Scan image for vulnerabilities
- `GET /api/v1/images/{name}/sbom` - Get image SBOM
- `POST /api/v1/images/{name}/sbom` - Update image SBOM
- `GET /api/v1/images/{name}/provenance` - Get image provenance
- `POST /api/v1/images/{name}/provenance` - Update image provenance
- `POST /api/v1/images/{name}/publish` - Publish image

### Capsule Types (Compute Types)
- `GET /api/v1/capsule-types` - List capsule types
- `POST /api/v1/capsule-types` - Create capsule type
- `GET /api/v1/capsule-types/{name}` - Get capsule type
- `GET /api/v1/capsule-types/{name}/audit` - Get capsule type audit
- `POST /api/v1/capsule-types/{name}/deprecate` - Deprecate capsule type
- `DELETE /api/v1/capsule-types/{name}` - Delete capsule type

### Instance Types (alias)
- `GET /api/v1/instance-types` - List instance types
- `GET /api/v1/instance-types/{name}` - Get instance type

---

## 6. Key Pairs & Launch Templates

### Key Pairs
- `GET /api/v1/key-pairs` - List key pairs
- `POST /api/v1/key-pairs` - Create key pair
- `GET /api/v1/key-pairs/{keyName}` - Get key pair
- `DELETE /api/v1/key-pairs/{keyName}` - Delete key pair

### Launch Templates
- `GET /api/v1/launch-templates` - List launch templates
- `POST /api/v1/launch-templates` - Create launch template
- `GET /api/v1/launch-templates/{templateId}` - Get launch template
- `GET /api/v1/launch-templates/{templateId}/versions` - List launch template versions
- `POST /api/v1/launch-templates/{templateId}/versions` - Create launch template version

---

## 7. Networking - VPCs & Subnets

### VPCs (Unified Model)
- `GET /api/v1/vpcs` - List VPCs
- `POST /api/v1/vpcs` - Create VPC
- `GET /api/v1/vpcs/{vpc}` - Get VPC details
- `PATCH /api/v1/vpcs/{vpc}` - Update VPC
- `DELETE /api/v1/vpcs/{vpc}` - Delete VPC
- `GET /api/v1/vpcs/{vpc}/summary` - Get VPC summary
- `GET /api/v1/vpcs/{vpc}/detail` - Get VPC detailed info
- `GET /api/v1/vpcs/{vpc}/dependencies` - Get VPC dependencies
- `POST /api/v1/vpcs/{vpc}/copy` - Copy VPC
- `POST /api/v1/vpcs/{vpc}/move` - Move VPC

### VPC Subnets
- `GET /api/v1/vpcs/{vpc}/subnets` - List VPC subnets
- `POST /api/v1/vpcs/{vpc}/subnets` - Create VPC subnet
- `GET /api/v1/subnets/{subnetId}` - Get subnet details
- `PATCH /api/v1/subnets/{subnetId}` - Update subnet
- `DELETE /api/v1/subnets/{subnetId}` - Delete subnet
- `GET /api/v1/subnets/{subnetId}/dependencies` - Get subnet dependencies
- `GET /api/v1/subnets/{subnetId}/available-ips` - Get available IPs in subnet
- `POST /api/v1/subnets/{subnetId}/associate-route-table` - Associate route table with subnet

### VPC Route Tables
- `GET /api/v1/vpcs/{vpc}/route-tables` - List VPC route tables
- `POST /api/v1/vpcs/{vpc}/route-tables` - Create route table
- `GET /api/v1/route-tables/{routeTableId}` - Get route table details
- `POST /api/v1/route-tables/{routeTableId}/routes` - Add route
- `DELETE /api/v1/route-tables/{routeTableId}/routes/{routeId}` - Delete route

### VPC Routes (Legacy Flat)
- `GET /api/v1/vpcs/{vpc}/routes` - List VPC routes
- `POST /api/v1/vpcs/{vpc}/routes` - Create route

---

## 8. Networking - Security & Access Control

### Security Groups
- `GET /api/v1/security-groups` - List security groups
- `POST /api/v1/security-groups` - Create security group
- `GET /api/v1/security-groups/{sgId}` - Get security group
- `DELETE /api/v1/security-groups/{sgId}` - Delete security group
- `POST /api/v1/security-groups/{sgId}/rules` - Add security group rule
- `DELETE /api/v1/security-group-rules/{ruleId}` - Delete security group rule

### Network ACLs
- `GET /api/v1/network-acls` - List network ACLs
- `POST /api/v1/network-acls` - Create network ACL
- `GET /api/v1/network-acls/{aclId}` - Get network ACL
- `DELETE /api/v1/network-acls/{aclId}` - Delete network ACL
- `POST /api/v1/network-acls/{aclId}/entries` - Add network ACL entry
- `DELETE /api/v1/network-acls/{aclId}/entries/{ruleNumber}` - Delete network ACL entry

### Firewalls
- `GET /api/v1/firewalls` - List firewalls
- `POST /api/v1/firewalls` - Create firewall
- `GET /api/v1/firewalls/{name}` - Get firewall
- `DELETE /api/v1/firewalls/{name}` - Delete firewall
- `POST /api/v1/firewalls/{name}/apply` - Apply firewall rules
- `GET /api/v1/firewalls/{name}/rules` - List firewall rules
- `POST /api/v1/firewalls/{name}/rules` - Create firewall rule
- `DELETE /api/v1/firewalls/{name}/rules/{id}` - Delete firewall rule

---

## 9. Networking - Gateways & Routing

### Internet Gateways
- `GET /api/v1/internet-gateways` - List internet gateways
- `POST /api/v1/internet-gateways` - Create internet gateway
- `DELETE /api/v1/internet-gateways/{igwId}` - Delete internet gateway

### NAT Gateways
- `GET /api/v1/nat-gateways` - List NAT gateways
- `POST /api/v1/nat-gateways` - Create NAT gateway
- `GET /api/v1/nat-gateways/{natId}` - Get NAT gateway
- `DELETE /api/v1/nat-gateways/{natId}` - Delete NAT gateway

### Public IPs
- `GET /api/v1/public-ips` - List public IPs
- `POST /api/v1/public-ips/allocate` - Allocate public IP
- `POST /api/v1/public-ips/{allocationId}/associate` - Associate public IP
- `POST /api/v1/public-ips/{associationId}/disassociate` - Disassociate public IP
- `DELETE /api/v1/public-ips/{allocationId}` - Release public IP

---

## 10. Networking - ENIs (Elastic Network Interfaces)

- `GET /api/v1/network-interfaces` - List ENIs
- `POST /api/v1/network-interfaces` - Create ENI
- `GET /api/v1/network-interfaces/{eniId}` - Get ENI details
- `DELETE /api/v1/network-interfaces/{eniId}` - Delete ENI
- `POST /api/v1/network-interfaces/{eniId}/attach` - Attach ENI
- `POST /api/v1/network-interfaces/{eniId}/detach` - Detach ENI
- `POST /api/v1/network-interfaces/{eniId}/private-ips` - Assign private IP to ENI

---

## 11. Networking - Advanced Features

### Reachability & Analysis
- `POST /api/v1/reachability/analyze` - Analyze reachability
- `GET /api/v1/networking/topology` - Get networking topology graph
- `GET /api/v1/networking/dashboard` - Get networking dashboard
- `GET /api/v1/networking/drift` - Get networking drift

### VPC Endpoints
- `GET /api/v1/vpc-endpoints` - List VPC endpoints
- `POST /api/v1/vpc-endpoints` - Create VPC endpoint

### VPC Peering
- `GET /api/v1/vpc-peerings` - List VPC peerings
- `POST /api/v1/vpc-peerings` - Create VPC peering

### Flow Logs
- `GET /api/v1/flow-logs` - List flow logs
- `POST /api/v1/flow-logs` - Create flow log

---

## 12. Load Balancers & Target Groups

### Load Balancers
- `GET /api/v1/lb` - List load balancers
- `POST /api/v1/lb` - Create load balancer
- `GET /api/v1/lb/{name}` - Get load balancer
- `DELETE /api/v1/lb/{name}` - Delete load balancer
- `POST /api/v1/lb/{name}/backends` - Add LB backend
- `DELETE /api/v1/lb/{name}/backends/{address}` - Remove LB backend

### LB Listeners
- `GET /api/v1/lb/{name}/listeners` - List LB listeners
- `POST /api/v1/lb/{name}/listeners` - Create LB listener
- `GET /api/v1/lb/{name}/listeners/{id}` - Get LB listener
- `PATCH /api/v1/lb/{name}/listeners/{id}` - Update LB listener
- `DELETE /api/v1/lb/{name}/listeners/{id}` - Delete LB listener
- `POST /api/v1/lb/{name}/listeners/{id}/certificates` - Attach listener certificate
- `DELETE /api/v1/lb/{name}/listeners/{id}/certificates` - Detach listener certificate

### Target Groups
- `GET /api/v1/target-groups` - List target groups
- `POST /api/v1/target-groups` - Create target group
- `GET /api/v1/lb/{name}/target-groups` - Get LB target groups
- `POST /api/v1/lb/{name}/target-groups` - Create LB target group
- `DELETE /api/v1/lb/{name}/target-groups/{tgId}` - Delete LB target group
- `GET /api/v1/lb/{name}/target-groups/{tgId}/targets` - List LB targets
- `POST /api/v1/lb/{name}/target-groups/{tgId}/targets` - Add LB target
- `DELETE /api/v1/lb/{name}/target-groups/{tgId}/targets/{targetId}` - Remove LB target

---

## 13. Storage

### Volumes
- `GET /api/v1/storage/volumes` - List volumes
- `POST /api/v1/storage/volumes` - Create volume
- `POST /api/v1/storage/volumes/{name}/attach` - Attach volume
- `POST /api/v1/storage/volumes/{name}/detach` - Detach volume
- `DELETE /api/v1/storage/volumes/{name}` - Delete volume

### S3 Buckets
- `GET /api/v1/storage/buckets` - List S3 buckets
- `POST /api/v1/storage/buckets` - Create S3 bucket
- `GET /api/v1/storage/buckets/{bucket}` - Get S3 bucket details
- `DELETE /api/v1/storage/buckets/{bucket}` - Delete S3 bucket

### S3 Objects
- `GET /api/v1/storage/buckets/{bucket}/objects` - List S3 objects
- `GET /api/v1/storage/buckets/{bucket}/objects/{key...}` - Get S3 object
- `PUT /api/v1/storage/buckets/{bucket}/objects/{key...}` - Put S3 object (raw)
- `POST /api/v1/storage/buckets/{bucket}/objects/{key...}` - Put S3 object (form)
- `DELETE /api/v1/storage/buckets/{bucket}/objects/{key...}` - Delete S3 object

### S3 Credentials
- `GET /api/v1/s3/credentials` - List S3 credentials
- `POST /api/v1/s3/credentials` - Create S3 credential
- `DELETE /api/v1/s3/credentials/{id}` - Delete S3 credential

### S3 Bucket Policies
- `GET /api/v1/s3/buckets/{bucket}/policy` - Get S3 bucket policy
- `PUT /api/v1/s3/buckets/{bucket}/policy` - Set S3 bucket policy
- `DELETE /api/v1/s3/buckets/{bucket}/policy` - Delete S3 bucket policy

---

## 14. DNS

### DNS Zones
- `GET /api/v1/dns/zones` - List DNS zones
- `POST /api/v1/dns/zones` - Create DNS zone
- `GET /api/v1/dns/zones/{zone}` - Get DNS zone
- `DELETE /api/v1/dns/zones/{zone}` - Delete DNS zone

### DNS Records
- `POST /api/v1/dns/zones/{zone}/records` - Create DNS record
- `DELETE /api/v1/dns/zones/{zone}/records/{id}` - Delete DNS record

### DNS VPC Associations
- `GET /api/v1/dns/zones/{zone}/vpc-associations` - List DNS zone VPC associations
- `POST /api/v1/dns/zones/{zone}/vpc-associations` - Associate DNS zone with VPC
- `DELETE /api/v1/dns/zones/{zone}/vpc-associations` - Disassociate DNS zone from VPC

### DNS Query
- `POST /api/v1/dns/query` - Perform DNS query

---

## 15. Certificates & TLS

### Certificates
- `GET /api/v1/certs` - List certificates
- `POST /api/v1/certs` - Create certificate
- `DELETE /api/v1/certs/{name}` - Delete certificate
- `GET /api/v1/certificates/{id}/monitoring` - Get certificate monitoring

---

## 16. Secrets & Encryption

### Secrets
- `GET /api/v1/secrets` - List secrets
- `POST /api/v1/secrets` - Create secret
- `GET /api/v1/secrets/{name}` - Get secret
- `DELETE /api/v1/secrets/{name}` - Delete secret

### KMS Keys
- `GET /api/v1/kms/keys` - List KMS keys
- `POST /api/v1/kms/keys` - Create KMS key
- `DELETE /api/v1/kms/keys/{name}` - Delete KMS key
- `POST /api/v1/kms/keys/{name}/rotate` - Rotate KMS key
- `POST /api/v1/kms/keys/{name}/encrypt` - Encrypt with KMS key
- `POST /api/v1/kms/keys/{name}/decrypt` - Decrypt with KMS key

---

## 17. Serverless Functions

### Functions
- `POST /api/v1/functions` - Create function
- `GET /api/v1/functions` - List functions
- `GET /api/v1/functions/{id}` - Get function
- `PATCH /api/v1/functions/{id}` - Update function
- `DELETE /api/v1/functions/{id}` - Delete function

### Function Versions
- `POST /api/v1/functions/{id}/versions` - Create function version
- `GET /api/v1/functions/{id}/versions` - List function versions

### Function Invocation
- `POST /api/v1/functions/{id}/invoke` - Invoke function
- `GET /api/v1/functions/{id}/invocations` - List function invocations

### Function Triggers
- `POST /api/v1/functions/{id}/triggers` - Create function trigger
- `GET /api/v1/functions/{id}/triggers` - List function triggers
- `DELETE /api/v1/functions/{id}/triggers/{triggerId}` - Delete function trigger

---

## 18. Model Context Protocol (MCP) Servers

- `POST /api/v1/mcp/servers` - Create MCP server
- `GET /api/v1/mcp/servers` - List MCP servers
- `GET /api/v1/mcp/servers/{id}` - Get MCP server
- `DELETE /api/v1/mcp/servers/{id}` - Delete MCP server
- `GET /api/v1/mcp/servers/{id}/tools` - List MCP tools
- `POST /api/v1/mcp/servers/{id}/tools/sync` - Sync MCP tools
- `POST /api/v1/mcp/servers/{id}/tools/{toolName}/invoke` - Invoke MCP tool
- `GET /api/v1/mcp/servers/{id}/invocations` - List MCP invocations
- `GET /api/v1/mcp/approvals` - List MCP approvals
- `POST /api/v1/mcp/approvals/{id}/approve` - Approve MCP action
- `POST /api/v1/mcp/approvals/{id}/deny` - Deny MCP action

---

## 19. Compute Groups & Autoscaling

### Compute Groups
- `GET /api/v1/groups` - List compute groups
- `POST /api/v1/groups` - Create compute group
- `GET /api/v1/groups/{name}` - Get compute group
- `DELETE /api/v1/groups/{name}` - Delete compute group
- `POST /api/v1/groups/{name}/scale` - Scale compute group
- `GET /api/v1/groups/{name}/instances` - List group instances
- `POST /api/v1/groups/{name}/reconcile` - Reconcile group

### Group Autoscaling
- `GET /api/v1/groups/{name}/autoscale` - Get group autoscale config
- `POST /api/v1/groups/{name}/autoscale/disable` - Disable group autoscaling
- `POST /api/v1/groups/{name}/autoscale/evaluate` - Evaluate autoscale
- `GET /api/v1/groups/{name}/autoscale/decisions` - List autoscale decisions

### Autoscale Policies
- `GET /api/v1/autoscale/policies` - List autoscale policies
- `POST /api/v1/autoscale/policies` - Create autoscale policy
- `GET /api/v1/autoscale/policies/{policy}` - Get autoscale policy
- `PATCH /api/v1/autoscale/policies/{policy}` - Update autoscale policy
- `DELETE /api/v1/autoscale/policies/{policy}` - Delete autoscale policy

---

## 20. Topology - Realms, Regions, Zones, Nodes

### Realms
- `GET /api/v1/realms` - List realms
- `POST /api/v1/realms` - Create realm
- `GET /api/v1/realms/{realm}` - Get realm
- `PATCH /api/v1/realms/{realm}` - Update realm
- `DELETE /api/v1/realms/{realm}` - Delete realm

### Regions
- `GET /api/v1/regions` - List regions
- `POST /api/v1/regions` - Create region
- `GET /api/v1/regions/{region}` - Get region
- `PATCH /api/v1/regions/{region}` - Update region
- `DELETE /api/v1/regions/{region}` - Delete region
- `POST /api/v1/regions/{region}/drain` - Drain region
- `POST /api/v1/regions/{region}/undrain` - Undrain region
- `POST /api/v1/regions/{region}/evacuate` - Evacuate region
- `POST /api/v1/regions/{region}/promote` - Promote region

### Zones
- `GET /api/v1/zones` - List zones
- `POST /api/v1/zones` - Create zone
- `GET /api/v1/zones/{zone}` - Get zone
- `PATCH /api/v1/zones/{zone}` - Update zone
- `DELETE /api/v1/zones/{zone}` - Delete zone
- `POST /api/v1/zones/{zone}/cordon` - Cordon zone
- `POST /api/v1/zones/{zone}/uncordon` - Uncordon zone
- `POST /api/v1/zones/{zone}/drain` - Drain zone
- `POST /api/v1/zones/{zone}/undrain` - Undrain zone
- `POST /api/v1/zones/{zone}/evacuate` - Evacuate zone

### Nodes
- `GET /api/v1/nodes` - List nodes
- `POST /api/v1/nodes` - Register node
- `POST /api/v1/nodes/join` - Node join
- `GET /api/v1/nodes/{node}` - Get node
- `PATCH /api/v1/nodes/{node}` - Update node
- `DELETE /api/v1/nodes/{node}` - Delete node
- `POST /api/v1/nodes/{node}/cordon` - Cordon node
- `POST /api/v1/nodes/{node}/uncordon` - Uncordon node
- `POST /api/v1/nodes/{node}/drain` - Drain node
- `POST /api/v1/nodes/{node}/undrain` - Undrain node
- `POST /api/v1/nodes/{node}/heartbeat` - Node heartbeat
- `POST /api/v1/nodes/{node}/inventory` - Node inventory
- `POST /api/v1/nodes/{node}/services` - Post node services
- `GET /api/v1/nodes/{node}/services` - List node services
- `POST /api/v1/nodes/{node}/approve` - Approve node
- `GET /api/v1/nodes/{node}/monitoring` - Get node monitoring

### Node Pools
- `GET /api/v1/node-pools` - List node pools
- `POST /api/v1/node-pools` - Create node pool
- `GET /api/v1/node-pools/{pool}` - Get node pool
- `PATCH /api/v1/node-pools/{pool}` - Update node pool
- `DELETE /api/v1/node-pools/{pool}` - Delete node pool
- `POST /api/v1/node-pools/{pool}/members` - Add node pool member
- `DELETE /api/v1/node-pools/{pool}/members/{nodeID}` - Remove node pool member
- `GET /api/v1/node-pools/{pool}/members` - List node pool members

### Service Nodes
- `GET /api/v1/service-nodes` - List service nodes
- `GET /api/v1/service-nodes/{role}` - Get service nodes by role

### Join Tokens
- `GET /api/v1/join-tokens` - List join tokens
- `POST /api/v1/join-tokens` - Create join token
- `DELETE /api/v1/join-tokens/{id}` - Delete join token

---

## 21. Placement & Scheduling

### Placement Policies
- `GET /api/v1/placement/policies` - List placement policies
- `POST /api/v1/placement/policies` - Create placement policy
- `GET /api/v1/placement/policies/{policy}` - Get placement policy
- `DELETE /api/v1/placement/policies/{policy}` - Delete placement policy

### Scheduler
- `POST /api/v1/scheduler/simulate` - Simulate scheduling
- `GET /api/v1/scheduler/capacity` - Get scheduler capacity
- `GET /api/v1/scheduler/placements` - Get scheduler placements

---

## 22. Monitoring & Observability

### Resource Monitoring
- `GET /api/v1/resources` - List resources
- `POST /api/v1/resources/sync` - Sync resources
- `GET /api/v1/resources/{id}` - Get resource
- `GET /api/v1/resources/{id}/config` - Get resource config
- `GET /api/v1/resources/{id}/events` - Get resource events
- `GET /api/v1/resources/{id}/metrics` - Get resource metrics
- `POST /api/v1/resources/{id}/drift/repair` - Repair resource drift

### Drift & Config
- `GET /api/v1/config/drift` - List config drift

### Metrics
- `POST /api/v1/metrics/ingest` - Ingest metrics
- `GET /api/v1/metrics/query` - Query metrics
- `POST /api/v1/metrics/custom` - Push custom metric

### Resource Events
- `POST /api/v1/resource-events` - Create resource event
- `GET /api/v1/resource-events` - List resource events

### Alerts & Rules
- `GET /api/v1/alerts` - List alerts
- `GET /api/v1/alerts/rules` - List alert rules
- `POST /api/v1/alerts/rules` - Create alert rule
- `PATCH /api/v1/alerts/rules/{id}` - Update alert rule
- `DELETE /api/v1/alerts/rules/{id}` - Delete alert rule
- `POST /api/v1/alerts/{id}/ack` - Acknowledge alert
- `POST /api/v1/alerts/{id}/resolve` - Resolve alert

### Health Checks
- `GET /api/v1/health-checks` - List health checks
- `GET /api/v1/health-checks/{instanceId}` - Get health check

### Service Health & Migrations
- `GET /api/v1/topology/health` - List service health
- `POST /api/v1/topology/health` - Upsert service health
- `GET /api/v1/migrations` - List migration plans
- `POST /api/v1/migrations` - Create migration plan
- `GET /api/v1/migrations/{plan}` - Get migration plan

---

## 23. Cap Init (User Data & Templating)

- `GET /api/v1/capinit/status` - Get capinit status
- `GET /api/v1/capinit/templates` - List capinit templates
- `POST /api/v1/capinit/templates` - Create capinit template
- `GET /api/v1/capinit/templates/{id}` - Get capinit template
- `PUT /api/v1/capinit/templates/{id}` - Update capinit template
- `DELETE /api/v1/capinit/templates/{id}` - Delete capinit template
- `POST /api/v1/capinit/render` - Render capinit template

---

## 24. Factory (Image Building & Management)

- `GET /api/v1/factory/status` - Get factory status
- `GET /api/v1/factory/jobs` - List factory jobs
- `GET /api/v1/factory/jobs/{id}` - Get factory job
- `GET /api/v1/factory/images` - List factory images
- `GET /api/v1/factory/sync/status` - Get factory sync status
- `POST /api/v1/factory/images/{id}/push` - Push factory image
- `POST /api/v1/factory/images/{id}/rescan` - Rescan factory image

---

## 25. Marketplace

- `GET /api/v1/marketplace/images` - List marketplace images
- `GET /api/v1/marketplace/images/{id}` - Get marketplace image
- `GET /api/v1/marketplace/images/{id}/scans` - Get marketplace image scans
- `POST /api/v1/marketplace/images/{id}/install` - Install marketplace image
- `POST /api/v1/marketplace/images/{id}/approve` - Approve marketplace image
- `POST /api/v1/marketplace/images/{id}/reject` - Reject marketplace image
- `POST /api/v1/marketplace/images/{id}/quarantine` - Quarantine marketplace image

---

## 26. Backups & Disaster Recovery

### Backups
- `GET /api/v1/backups` - List backups
- `POST /api/v1/backups` - Create backup
- `POST /api/v1/backups/{id}/restore` - Restore backup

### Backup Policies
- `GET /api/v1/backup-policies` - List backup policies
- `POST /api/v1/backup-policies` - Create backup policy
- `DELETE /api/v1/backup-policies/{name}` - Delete backup policy

### Stacks
- `GET /api/v1/stacks` - List stacks
- `POST /api/v1/stacks` - Create stack
- `GET /api/v1/stacks/{name}` - Get stack
- `DELETE /api/v1/stacks/{name}` - Delete stack
- `POST /api/v1/stacks/{name}/diff` - Stack diff

---

## 27. Databases

- `GET /api/v1/databases` - List databases
- `POST /api/v1/databases` - Create database
- `GET /api/v1/databases/{name}` - Get database
- `DELETE /api/v1/databases/{name}` - Delete database

---

## 28. AI & Agents

- `GET /api/v1/ai/agents` - List AI agents
- `POST /api/v1/ai/agents` - Create AI agent
- `GET /api/v1/ai/sessions` - List AI sessions
- `POST /api/v1/ai/sessions` - Create AI session

---

## 29. Governance & Compliance

### Governance Policies
- `GET /api/v1/governance/policies` - List governance policies
- `POST /api/v1/governance/policies` - Create governance policy
- `POST /api/v1/governance/evaluate` - Evaluate governance

### Posture Management
- `GET /api/v1/posture/findings` - List posture findings
- `POST /api/v1/posture/scan` - Perform posture scan

### Quotas
- `GET /api/v1/quotas` - List quotas
- `POST /api/v1/quotas` - Set quota

---

## 30. Ingress & Queues

### Ingress
- `GET /api/v1/ingress` - List ingress
- `POST /api/v1/ingress` - Create ingress
- `DELETE /api/v1/ingress/{name}` - Delete ingress

### Queues
- `GET /api/v1/queues` - List queues
- `POST /api/v1/queues` - Create queue
- `DELETE /api/v1/queues/{name}` - Delete queue
- `POST /api/v1/queues/{name}/publish` - Publish to queue
- `POST /api/v1/queues/{name}/consume` - Consume from queue

---

## 31. Shared Storage (CSD)

- `GET /api/v1/csd/volumes` - List CSD volumes
- `POST /api/v1/csd/volumes` - Create CSD volume
- `GET /api/v1/csd/volumes/{vol}` - Get CSD volume
- `DELETE /api/v1/csd/volumes/{vol}` - Delete CSD volume
- `POST /api/v1/csd/volumes/{vol}/attach` - Attach CSD volume
- `POST /api/v1/csd/volumes/{vol}/detach` - Detach CSD volume
- `GET /api/v1/csd/volumes/{vol}/attachments` - List CSD attachments
- `GET /api/v1/csd/volumes/{vol}/snapshots` - List CSD snapshots
- `POST /api/v1/csd/volumes/{vol}/snapshots` - Create CSD snapshot
- `GET /api/v1/csd/volumes/{vol}/leases` - List CSD leases
- `POST /api/v1/csd/volumes/{vol}/leases/revoke` - Revoke CSD leases
- `GET /api/v1/csd/volumes/{vol}/replicas` - List CSD replicas
- `POST /api/v1/csd/volumes/{vol}/repair` - Repair CSD volume

---

## 32. VPC Mobility

- `POST /api/v1/vpcs/{vpc}/mobility/plans` - Create mobility plan
- `GET /api/v1/vpcs/{vpc}/mobility/plans` - List mobility plans
- `GET /api/v1/vpcs/{vpc}/mobility/plans/{plan}` - Get mobility plan
- `POST /api/v1/vpcs/{vpc}/mobility/plans/{plan}/approve` - Approve mobility plan
- `POST /api/v1/vpcs/{vpc}/mobility/plans/{plan}/execute` - Execute mobility plan
- `POST /api/v1/vpcs/{vpc}/mobility/plans/{plan}/cancel` - Cancel mobility plan
- `GET /api/v1/vpcs/{vpc}/mobility/plans/{plan}/dry-run` - Dry-run mobility plan
- `GET /api/v1/vpcs/{vpc}/mobility/jobs` - List mobility jobs
- `GET /api/v1/vpcs/{vpc}/mobility/jobs/{job}` - Get mobility job
- `POST /api/v1/vpcs/{vpc}/mobility/jobs/{job}/cutover` - Cutover mobility job
- `POST /api/v1/vpcs/{vpc}/mobility/jobs/{job}/rollback` - Rollback mobility job
- `POST /api/v1/vpcs/{vpc}/mobility/jobs/{job}/cancel` - Cancel mobility job
- `GET /api/v1/vpcs/{vpc}/mobility/jobs/{job}/steps` - List mobility job steps
- `GET /api/v1/vpcs/{vpc}/mobility/jobs/{job}/mappings` - List mobility job mappings

---

## 33. GPU Management

- `GET /api/v1/gpu` - List GPUs
- `POST /api/v1/gpu` - Add GPU
- `DELETE /api/v1/gpu/{id}` - Delete GPU
- `POST /api/v1/gpu/{id}/release` - Release GPU
- `POST /api/v1/gpu/{id}/assign` - Assign GPU

---

## 34. IP Management (IPAM)

### IP Pools
- `POST /api/v1/ip-pools` - Create IP pool
- `GET /api/v1/ip-pools` - List IP pools
- `GET /api/v1/ip-pools/{id}` - Get IP pool
- `DELETE /api/v1/ip-pools/{id}` - Delete IP pool

### IPs
- `POST /api/v1/ips/reserve` - Reserve IP
- `GET /api/v1/ips` - List IPs
- `GET /api/v1/ips/{id}` - Get IP
- `POST /api/v1/ips/{id}/release` - Release IP
- `POST /api/v1/ips/{id}/attach` - Attach IP
- `POST /api/v1/ips/{id}/detach` - Detach IP

### Admin IP Exclusions
- `GET /api/v1/admin/ip-exclusions` - List IP exclusions
- `POST /api/v1/admin/ip-exclusions` - Create IP exclusion
- `DELETE /api/v1/admin/ip-exclusions/{id}` - Delete IP exclusion

---

## 35. Admin Functions - Host Management

### Host Limits
- `GET /api/v1/admin/limits/host` - Get host limits
- `PUT /api/v1/admin/limits/host` - Set host limits

### Host Storage
- `GET /api/v1/admin/disks` - List disks
- `GET /api/v1/admin/storage-pools` - List storage pools
- `POST /api/v1/admin/storage-pools` - Create storage pool
- `DELETE /api/v1/admin/storage-pools/{id}` - Delete storage pool
- `GET /api/v1/admin/storage-pools/{id}/allocations` - List storage allocations
- `POST /api/v1/admin/storage-pools/{id}/allocations` - Create storage allocation
- `DELETE /api/v1/admin/storage-allocations/{id}` - Delete storage allocation
- `GET /api/v1/admin/storage/settings` - Get storage settings
- `PUT /api/v1/admin/storage/settings` - Set storage settings

### Host Security
- `GET /api/v1/admin/hostsec/nodes` - List hostsec nodes

---

## 36. Admin Functions - Fail2Ban

- `GET /api/v1/admin/fail2ban/status` - Get fail2ban status
- `POST /api/v1/admin/fail2ban/ban` - Ban IP with fail2ban
- `POST /api/v1/admin/fail2ban/unban` - Unban IP with fail2ban
- `POST /api/v1/admin/fail2ban/unban-all` - Unban all with fail2ban
- `POST /api/v1/admin/fail2ban/flush` - Flush fail2ban
- `POST /api/v1/admin/fail2ban/reload` - Reload fail2ban
- `GET /api/v1/admin/fail2ban/blocklist` - Get fail2ban blocklist
- `POST /api/v1/admin/fail2ban/blocklist` - Add to fail2ban blocklist
- `DELETE /api/v1/admin/fail2ban/blocklist/{id}` - Remove from fail2ban blocklist
- `GET /api/v1/admin/fail2ban/allowlist` - Get fail2ban allowlist
- `PUT /api/v1/admin/fail2ban/allowlist` - Set fail2ban allowlist

---

## 37. Admin Functions - UFW Firewall

- `GET /api/v1/admin/ufw/status` - Get UFW status
- `POST /api/v1/admin/ufw/rules` - Add UFW rule
- `DELETE /api/v1/admin/ufw/rules/{num}` - Delete UFW rule
- `GET /api/v1/admin/ufw/defaults` - Get UFW defaults
- `PUT /api/v1/admin/ufw/defaults` - Set UFW defaults
- `POST /api/v1/admin/ufw/enable` - Enable UFW
- `POST /api/v1/admin/ufw/disable` - Disable UFW

---

## 38. Deletion Flow

- `POST /api/v1/{resourceType}/{resourceId}/delete-preflight` - Get deletion preflight info
- `POST /api/v1/{resourceType}/{resourceId}/delete-confirm` - Confirm deletion
- `GET /api/v1/deletion-jobs/{jobId}` - Get deletion job status

---

## 39. System & Utility Endpoints

- `GET /api/v1/health` - Health check
- `GET /api/v1/version` - Get API version
- `GET /api/v1/openapi.json` - Get OpenAPI spec
- `GET /api/v1/daemon/status` - Get daemon status
- `GET /api/v1/db/stats` - Get database stats
- `GET /api/v1/events` - Get system events
- `GET /api/v1/search` - Global search with filters
- `GET /api/v1/accounts/{account}/audit` - Get account audit log

---

## API Patterns & Notes

1. **Authentication**: Most endpoints require session or bearer token
2. **Base URL**: `/api/v1/`
3. **Response Format**: JSON with standard response envelope
4. **Error Handling**: HTTP status codes + error messages
5. **Pagination**: Query params (limit, offset) on list endpoints
6. **Filtering**: Query params on GET endpoints
7. **Path Parameters**: Curly braces denote path parameters
8. **Deletion Confirmation**: Two-step deletion flow (preflight → confirm)
9. **Async Operations**: Many operations return job IDs for status polling
10. **Account Scoping**: Many resources support cross-account operations

---

**Total Endpoints: ~550+**
**Last Updated: 2026-07-01**
