---
title: Resource Deletion Framework
description: Safe, comprehensive deletion of cloud resources with confirmation gates and cascading support
owner: engineering
status: stable
reviewed: 2026-06-21
---

# Resource Deletion Framework

Capper provides a safe, auditable framework for deleting cloud resources. The deletion process follows a **3-phase model** that prevents accidental deletions while supporting cascading deletion of dependent resources.

## Three Phases of Deletion

### Phase 1: Preflight — Discover What Will Be Deleted

Before any deletion occurs, clients can query what would be deleted without making any changes.

**Endpoint:** `POST /api/v1/{resourceType}/{resourceId}/delete-preflight`

**Request:**
```json
POST /api/v1/vpcs/vpc_12345/delete-preflight
```

**Response:**
```json
{
  "data": {
    "resourceType": "vpc",
    "resourceId": "vpc_12345",
    "confirmationToken": "token_abc123...",
    "requiresConfirmation": true,
    "deleteOrder": [
      "subnets",
      "network-acls",
      "route-tables",
      "internet-gateways",
      "vpc_12345 (vpc)"
    ],
    "message": "Use the confirmationToken to proceed with deletion. Confirmation requires typing DELETE in uppercase."
  }
}
```

The preflight phase shows:
- What resources will be deleted
- The order of deletion (dependencies first, parent last)
- A one-time confirmation token
- Instructions for the next phase

### Phase 2: Confirm — Validate User Intent

The user must explicitly confirm they want to delete by:
1. Providing the confirmation token from phase 1
2. Typing the exact phrase **"DELETE"** in uppercase

This prevents accidental deletions and ensures the user understands what's being deleted.

**Endpoint:** `POST /api/v1/{resourceType}/{resourceId}/delete-confirm`

**Request:**
```json
{
  "confirmationToken": "token_abc123...",
  "confirmationPhrase": "DELETE"
}
```

**Response:**
```json
{
  "data": {
    "jobId": "del-18bb30761e243e12",
    "status": "queued",
    "pollUrl": "/api/v1/deletion-jobs/del-18bb30761e243e12"
  }
}
```

The confirmation response includes a **deletion job ID** for polling progress.

### Phase 3: Progress — Async Execution with Real-Time Tracking

Deletion is asynchronous and can be monitored via polling.

**Endpoint:** `GET /api/v1/deletion-jobs/{jobId}`

**Response:**
```json
{
  "data": {
    "id": "del-18bb30761e243e12",
    "status": "running",
    "resourceType": "vpc",
    "resourceId": "vpc_12345",
    "progress": 45,
    "completedSteps": [
      "validate",
      "delete-instances"
    ],
    "currentStep": "delete-load-balancers",
    "remainingSteps": [
      "delete-vpc"
    ],
    "errors": []
  }
}
```

During progress polling, clients can see:
- Current progress percentage (0-100%)
- Completed steps
- Current step being executed
- Remaining steps
- Any errors that occurred

If an error occurs:
```json
{
  "data": {
    "id": "del-18bb30761e243e12",
    "status": "failed",
    "progress": 60,
    "completedSteps": ["validate", "delete-instances"],
    "currentStep": "delete-load-balancers",
    "errors": [
      {
        "step": "delete-load-balancers",
        "resource": "load-balancer",
        "resourceId": "lb_789",
        "reason": "Load balancer still has active connections",
        "recoverable": true,
        "recovery": "Wait for active connections to close, then retry deletion"
      }
    ]
  }
}
```

---

## Cascading Deletion

When you delete a resource, all dependent resources are automatically deleted in the correct order:

### VPC Deletion Cascade

Deleting a VPC removes everything in this order:
1. All instances in the VPC (stops, detaches ENIs, removes)
2. All load balancers attached to the VPC (releases VIPs)
3. All subnets in the VPC
4. All route tables in the VPC
5. All network ACLs
6. Internet gateways
7. The VPC itself

**Example:** Delete a VPC with 3 instances and 1 load balancer
```
VPC deletion flow:
├─ Step 1: Delete instances (30%)
│  ├─ Stop instance-1 (10%)
│  ├─ Stop instance-2 (20%)
│  └─ Stop instance-3 (30%)
├─ Step 2: Delete load balancers (60%)
│  └─ Release VIP and delete lb-1 (60%)
├─ Step 3: Delete VPC infrastructure (90%)
│  ├─ Delete subnets
│  ├─ Delete route tables
│  └─ Delete network ACLs
└─ Step 4: Delete VPC (100%)
   └─ VPC removed from topology and vpc stores
```

### Instance Deletion

Deleting an instance:
1. Validates instance exists
2. Stops the instance gracefully (5s timeout)
3. Detaches all network interfaces
4. Removes compute resources
5. Releases IP addresses
6. Records billing event

### Load Balancer Deletion

Deleting a load balancer:
1. Validates load balancer exists
2. Removes from target groups
3. Releases routable IP (VIP)
4. Deletes load balancer record
5. Records billing event

---

## API Integration Examples

### Python Example

```python
import requests
import time

BASE_URL = "https://cloud.cappervm.com/api/v1"
TOKEN = "your-bearer-token"
headers = {"Authorization": f"Bearer {TOKEN}"}

# Step 1: Preflight
response = requests.post(
    f"{BASE_URL}/vpcs/vpc_12345/delete-preflight",
    headers=headers
)
data = response.json()["data"]
token = data["confirmationToken"]
print(f"Will delete: {data['deleteOrder']}")

# Step 2: Confirm
response = requests.post(
    f"{BASE_URL}/vpcs/vpc_12345/delete-confirm",
    json={
        "confirmationToken": token,
        "confirmationPhrase": "DELETE"
    },
    headers=headers
)
job_id = response.json()["data"]["jobId"]
print(f"Deletion job: {job_id}")

# Step 3: Poll progress
while True:
    response = requests.get(
        f"{BASE_URL}/deletion-jobs/{job_id}",
        headers=headers
    )
    job = response.json()["data"]
    print(f"Progress: {job['progress']}% - {job['currentStep']}")
    
    if job["status"] in ["completed", "failed"]:
        print(f"Status: {job['status']}")
        if job.get("errors"):
            for err in job["errors"]:
                print(f"  Error: {err['reason']}")
                print(f"  Recovery: {err['recovery']}")
        break
    
    time.sleep(2)
```

### cURL Example

```bash
# Step 1: Preflight
TOKEN="your-bearer-token"
curl -X POST https://cloud.cappervm.com/api/v1/vpcs/vpc_12345/delete-preflight \
  -H "Authorization: Bearer $TOKEN"

# Step 2: Confirm (requires confirmation token from step 1)
curl -X POST https://cloud.cappervm.com/api/v1/vpcs/vpc_12345/delete-confirm \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"confirmationToken":"...", "confirmationPhrase":"DELETE"}'

# Step 3: Poll (get jobId from step 2)
curl -X GET https://cloud.cappervm.com/api/v1/deletion-jobs/del-18bb30761e243e12 \
  -H "Authorization: Bearer $TOKEN"
```

---

## Web UI Deletion Flow

The CapperWeb interface provides a guided deletion experience:

1. **Click Delete Button** — Opens preflight modal showing what will be deleted
2. **Type Confirmation** — User must type "DELETE" in uppercase input field
3. **Click Confirm** — Submits confirmation and shows progress modal
4. **Monitor Progress** — Real-time progress bar with step names
5. **Completion** — Shows success or displays errors with recovery suggestions

---

## Error Handling

Some deletions may fail with recoverable errors. The error response includes:

- **reason** — Why deletion failed (e.g., "Load balancer still has active connections")
- **recoverable** — Whether the error can be fixed and deletion retried
- **recovery** — Specific action to resolve the error

### Common Errors and Recovery

| Error | Cause | Recovery |
|-------|-------|----------|
| Load balancer still has active connections | Active client connections | Wait for connections to close, then retry |
| Cannot release routable IP | IP in use by other resource | Check dependencies, wait, then retry |
| Instance still running | Stop operation timed out | Manually stop instance, then retry |
| Disk not writable | Filesystem issue | Check disk space and permissions |

---

## Best Practices

### Before Deletion

1. **Use Preflight** — Always call preflight first to see what will be deleted
2. **Review Dependencies** — Understand the cascade order
3. **Backup Data** — Ensure you've backed up any important data
4. **Notify Team** — If deleting shared resources, inform your team

### During Deletion

1. **Monitor Progress** — Poll deletion status regularly (2-5 second intervals)
2. **Watch for Errors** — Check error responses and follow recovery suggestions
3. **Don't Retrigger** — Don't start another deletion while one is in progress

### After Deletion

1. **Verify Removal** — Confirm resource no longer appears in list
2. **Check Dependencies** — Ensure dependent resources were also removed
3. **Review Billing** — Verify charges were recorded

---

## Implementation Details

### Database Persistence

Deletion jobs are persisted in SQLite with:
- Job ID and status tracking
- Step-by-step progress tracking
- Error logging with recovery suggestions
- RFC3339 timestamps for auditing
- Automatic cleanup after 7 days

### Async Execution

Deletions run asynchronously in goroutines with:
- Panic recovery to prevent server crashes
- Atomic operations for ACID guarantees
- Proper ordering (dependencies first, parent last)
- Resource cleanup verification

### TOCTOU Protection

The framework prevents Time-of-Check-Time-of-Use race conditions:
- Preflight validates resource hasn't changed
- Confirmation token ties deletion to specific preflight result
- Deletion re-validates before each step

---

## Troubleshooting

### Deletion Stuck in "running" State

**Cause:** Server crashed or job was orphaned  
**Solution:** 
1. Check server logs: `journalctl -u capper-control`
2. Wait 10 minutes for job to timeout
3. Manually check if resource was actually deleted
4. If not deleted, retry the deletion

### "Confirmation token is required" Error

**Cause:** Missing token from preflight response  
**Solution:** 
1. Run preflight again
2. Copy the `confirmationToken` from the response
3. Use it in the confirm request

### "Subnet is not in vpc" Error

**Cause:** VPC ID/slug mismatch in form submission  
**Solution:**
1. This should not occur in 0.1.36+
2. Refresh browser and retry
3. Check browser console for full error message

---

**Version:** 0.1.38 | **Last Updated:** 2026-06-21
