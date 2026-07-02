# Capper Workflow Diagrams

Visual representations of Capper workflows showing API endpoint coverage and implementation status.

## Files

| # | Diagram | Coverage | Status | Key Gap |
|---|---------|----------|--------|---------|
| 1 | [Instance Lifecycle](1-instance-lifecycle.md) | 85% | 🟢 Good | Reboot, terminal, streaming logs, termination protection |
| 2 | [ENI Lifecycle](2-eni-lifecycle.md) | 0% | 🔴 Critical | Entire ENI subsystem missing |
| 3 | [VPC & Networking](3-vpc-networking.md) | 75% | 🟡 Fair | VPC peering, endpoints, ENI management |
| 4 | [Load Balancer](4-load-balancer.md) | 90% | 🟢 Good | Some advanced listener updates |
| 5 | [Storage & Backup](5-storage-backup.md) | 70% | 🟡 Fair | S3 credentials, policies, CSD storage |
| 6 | [IAM & Access](6-iam-access.md) | 95% | 🟢 Excellent | Minor RBAC operations |
| 7 | [Monitoring](7-monitoring.md) | 85% | 🟢 Good | Config drift repair, visualization |
| 8 | [Deletion Workflow](8-deletion-workflow.md) | 95% | 🟢 Excellent | Error recovery UI |
| 9 | [VPC Create/Delete](9-vpc-create-delete.md) | 85% | 🟢 Good | Dependency visualization |

## Color Legend

- 🟢 **Green**: Fully implemented in frontend
- 🟡 **Yellow**: Partially implemented
- 🔴 **Red**: NOT implemented in frontend (backend API exists, UI missing)
- 🔵 **Blue**: Information/monitoring only (no mutations)

## How to Use

1. **View Individual Workflows**: Click on any diagram file to see a specific workflow
2. **Reference Implementation Status**: Green endpoints are safe to use; red indicates missing frontend UI
3. **Plan Features**: Use as reference when planning frontend feature additions (see ../project/PLAN.md)
4. **Identify Gaps**: Red items in ../project/PASS3.md correspond to red nodes in these diagrams

## Overall Coverage

- **Total Backend Endpoints**: 550+
- **Implemented in Frontend**: 300+ (55%)
- **Missing from Frontend**: 250+ (45%)

## References

- See [PASS1.md](../project/PASS1.md) for complete backend API reference
- See [PASS2.md](../project/PASS2.md) for complete frontend API calls
- See [PASS3.md](../project/PASS3.md) for detailed gap analysis
- See [PLAN.md](../project/PLAN.md) for implementation roadmap
- See [FUNCTIONS.md](../project/FUNCTIONS.md) for all diagrams in one file

---

**Last Updated**: 2026-07-01
