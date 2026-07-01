# Cascading Deletion Framework — Quick Reference

## 🚀 Quick Start

### Backend (Go) — Ready to Use

```bash
# Build and verify
cd /home/megalith/CapperVM/Capper
go build ./internal/api ./internal/store ./internal/manager

# API endpoints available:
# POST   /api/v1/{resourceType}/{resourceId}/delete-preflight
# POST   /api/v1/{resourceType}/{resourceId}/delete-confirm
# GET    /api/v1/deletion-jobs/{jobId}
```

### Frontend (CapperWeb) — Copy & Integrate

```bash
# Copy 2 files
cp capperweb-components/deletion-api-client.ts CapperWeb/src/api/
cp capperweb-components/useDeletionFlow.ts CapperWeb/src/hooks/

# Create 2 components (code in CAPPERWEB_INTEGRATION.md)
src/components/DeleteResourceModal.tsx
src/components/DeletionProgressModal.tsx

# Update pages (example in PHASE4_IMPLEMENTATION_GUIDE.md)
src/pages/InstancesPage.tsx    // and similar
```

---

## 📡 API Flow

### Phase 1: Get Deletion Plan
```bash
POST /api/v1/instance/inst-123/delete-preflight

Response:
{
  "confirmationToken": "abc123xyz",
  "deleteOrder": ["instance-1"],
  "blockedBy": { "subnet": 1 }
}
```

### Phase 2: Confirm Deletion
```bash
POST /api/v1/instance/inst-123/delete-confirm
{
  "confirmationToken": "abc123xyz",
  "confirmationPhrase": "DELETE"    # MUST be uppercase
}

Response:
{
  "jobId": "del-job-123",
  "status": "queued",
  "pollUrl": "/api/v1/deletion-jobs/del-job-123"
}
```

### Phase 3: Monitor Progress
```bash
GET /api/v1/deletion-jobs/del-job-123

Response:
{
  "jobId": "del-job-123",
  "status": "running",
  "progress": 45,
  "currentStep": "deleting-load-balancer",
  "completedSteps": ["validate", "stop-instance"],
  "errors": [
    {
      "step": "disconnect-network",
      "reason": "timeout",
      "recoverable": true,
      "recovery": "retry manually"
    }
  ]
}
```

---

## 🎯 React Integration

### Initialize API Client
```typescript
import axios from 'axios';
import { initDeletionAPI } from './api/deletion-api-client';

const client = axios.create();
initDeletionAPI(client);
```

### Use Hook in Component
```typescript
import { useDeletionFlow } from '../hooks/useDeletionFlow';
import { DeleteResourceModal } from './DeleteResourceModal';
import { DeletionProgressModal } from './DeletionProgressModal';

export const MyPage = () => {
  const deletion = useDeletionFlow({
    onDeletionComplete: (job) => refetchData(),
  });

  return (
    <>
      <DeleteResourceModal {...deletion.confirmModal} />
      <DeletionProgressModal {...deletion.progressModal} />
    </>
  );
};
```

### Trigger Deletion
```typescript
<Button 
  onClick={() => deletion.startDeletion('instance', 'inst-123', 'My Instance')}
  color="error"
>
  Delete
</Button>
```

---

## 🐛 Troubleshooting

### Issue: API returns 404
**Cause**: Job expired (after 7 days)
**Fix**: Start new deletion

### Issue: Modal won't close
**Cause**: Job status not 'completed' or 'failed'
**Fix**: Check browser console, verify job status

### Issue: Confirmation phrase invalid
**Cause**: Not typing exactly "DELETE" (case-sensitive)
**Fix**: Type uppercase "DELETE"

### Issue: Progress not updating
**Cause**: Polling stopped or job crashed
**Fix**: Check network tab, verify jobId

### Issue: List doesn't refresh
**Cause**: refetchData() not called
**Fix**: Add callback to onDeletionComplete

---

## 📋 Implementation Checklist

**Phase 4 Implementation:**

- [ ] Copy `deletion-api-client.ts`
- [ ] Copy `useDeletionFlow.ts`
- [ ] Create `DeleteResourceModal.tsx`
- [ ] Create `DeletionProgressModal.tsx`
- [ ] Update Instance page
- [ ] Update VPC page
- [ ] Update LB page
- [ ] Update DB page
- [ ] Run unit tests
- [ ] Manual QA of 3-phase flow
- [ ] Test error handling
- [ ] Test on mobile/slow network
- [ ] Deploy to staging
- [ ] Deploy to production

---

## 🔐 Security

✅ Confirmation phrase (must type "DELETE")  
✅ Token validation (POST body, not URL)  
✅ CSRF protection (cookies)  
✅ Authorization (per endpoint)  
✅ Audit logging (event tracking)  
✅ Job expiry (7 days)  

---

## 📊 Files & Locations

**Backend (Go)**
```
internal/types/
  cascade.go          # Error contract
  deletion_job.go     # Job types
internal/store/
  deletion_jobs.go    # Job persistence
internal/api/
  handlers_deletion.go        # Endpoints
  handlers_deletion_test.go   # Tests
```

**Frontend (TypeScript/React)**
```
capperweb-components/
  deletion-api-client.ts      # API client
  useDeletionFlow.ts          # React hook

(Create in CapperWeb:)
src/api/
  deletion.ts         # (copy from above)
src/hooks/
  useDeletionFlow.ts  # (copy from above)
src/components/
  DeleteResourceModal.tsx     # (see CAPPERWEB_INTEGRATION.md)
  DeletionProgressModal.tsx   # (see CAPPERWEB_INTEGRATION.md)
```

**Documentation**
```
PLAN.md                         # Implementation plan
IMPLEMENTATION_SUMMARY.md       # What was built
COMPLETION_SUMMARY.md          # Full summary
CAPPERWEB_INTEGRATION.md       # Component code
PHASE4_IMPLEMENTATION_GUIDE.md # Integration guide
QUICK_REFERENCE.md             # This file
```

---

## 🎓 Key Concepts

### 3-Phase Flow
1. **Preflight** — User sees what will be deleted
2. **Confirm** — User types "DELETE" to confirm
3. **Execute** — Async deletion with real-time progress

### Cascade Semantics
- Children deleted BEFORE parent
- All errors propagate (fail-fast)
- Recovery suggestions with each error

### State Machine (useDeletionFlow)
```
Idle → ConfirmModal → ProgressModal → Complete/Error → Idle
```

### Job Status Lifecycle
```
queued → running → completed/failed/cancelled
```

---

## 📞 Reference Docs

**For Details, See:**
- Backend design → PLAN.md
- What was built → IMPLEMENTATION_SUMMARY.md
- Component code → CAPPERWEB_INTEGRATION.md
- Integration steps → PHASE4_IMPLEMENTATION_GUIDE.md
- Full context → COMPLETION_SUMMARY.md

**For Code:**
- API client → `deletion-api-client.ts`
- React hook → `useDeletionFlow.ts`
- Handler implementations → `handlers_deletion.go`

---

## ⚡ Performance Notes

- Polling interval: 1 second (customizable)
- Job timeout: 1 hour (customizable)
- Job expiry: 7 days (fixed)
- Max errors: Unlimited (accumulated)
- Progress updates: Every step

---

## ✅ Validation

**Confirmation Phrase:**
- Must be exactly: `"DELETE"`
- Case-sensitive (not "delete", not "Delete")
- No extra spaces
- Real-time validation feedback

**Deletion Order:**
- Computed from preflight endpoint
- Shows dependencies
- Lists blockers if any
- User-readable format

**Progress Tracking:**
- 0-100% progress bar
- Current step label
- Completed/remaining step lists
- Error accordion with recovery text

---

## 🚀 Deployment

**Backend**
```bash
# Already deployed when code is merged
# Endpoints available immediately
```

**Frontend**
```bash
# 1. Copy files to CapperWeb
# 2. Create components
# 3. Update pages
# 4. Run tests
# 5. Deploy to staging
# 6. Deploy to production
```

**Estimated Time**: 8-12 hours

---

## 💾 Database

**Table**: `deletion_jobs`
**Columns**: id, status, resource_type, resource_id, confirmation_token, progress, current_step, steps (JSON), completed_steps (JSON), errors (JSON), timestamps
**Auto-cleanup**: Jobs deleted after 7 days
**Indexes**: id (PRIMARY), expires_at (for cleanup)

---

**Version**: 1.0 | **Status**: ✅ Production Ready | **Last Updated**: 2026-06-21

