# Phase 4: CapperWeb Integration — Implementation Guide

This guide provides step-by-step instructions for integrating the cascading deletion framework into CapperWeb.

**Status**: Backend complete ✅ | Frontend in progress ⏳

## Quick Start

### Backend Ready ✅

The backend is fully implemented and ready to use:

```bash
$ cd /home/megalith/CapperVM/Capper
$ go build ./internal/api ./internal/store ./internal/manager
# Success - all code compiles
```

**Available endpoints:**
- `POST /api/v1/{resourceType}/{resourceId}/delete-preflight` — Get deletion plan
- `POST /api/v1/{resourceType}/{resourceId}/delete-confirm` — Confirm & start job
- `GET /api/v1/deletion-jobs/{jobId}` — Poll job status

### Frontend Setup (in CapperWeb)

**Step 1: Copy component files**
```bash
# From /home/megalith/CapperVM/Capper/capperweb-components/
cp deletion-api-client.ts src/api/
cp useDeletionFlow.ts src/hooks/
```

**Step 2: Implement React components**
- DeleteResourceModal.tsx (in src/components/)
- DeletionProgressModal.tsx (in src/components/)

See CAPPERWEB_INTEGRATION.md for complete component code.

**Step 3: Integrate into resource pages**
- Update InstancesPage.tsx
- Update VPCsPage.tsx
- Update LoadBalancersPage.tsx
- etc.

---

## Detailed Implementation Steps

### Step 1: Initialize API Client

**File: src/main.tsx or src/App.tsx**

```typescript
import axios from 'axios';
import { initDeletionAPI } from './api/deletion-api-client';

// Create axios instance (or use your existing one)
const apiClient = axios.create({
  baseURL: process.env.REACT_APP_API_URL || '/api',
  withCredentials: true,
});

// Initialize deletion API
initDeletionAPI(apiClient);
```

### Step 2: Create Deletion Components

**File: src/components/DeleteResourceModal.tsx**

Key features:
- Shows deletion order and blockers from preflight
- Requires typing "DELETE" (case-sensitive)
- Validates confirmation phrase in real-time
- Disables confirm button until phrase matches exactly
- Handles preflight loading/error states

```typescript
// See CAPPERWEB_INTEGRATION.md for full component code
```

**File: src/components/DeletionProgressModal.tsx**

Key features:
- Polls job status every 1-2 seconds
- Shows progress bar (0-100%)
- Displays current step with icon (pending/running/done/error)
- Lists completed and remaining steps
- Accordion for errors with recovery suggestions
- Auto-closes on success (2 second delay)
- Shows timeout/expiry errors gracefully

### Step 3: Create Custom Hook

**File: src/hooks/useDeletionFlow.ts**

Manages the 3-phase state machine:
- Phase 1: Idle
- Phase 2: Confirmation modal
- Phase 3: Progress modal
- Phase 4: Complete/Error

Provides:
- `startDeletion(type, id, name)` — Open modal
- `onConfirmSuccess(jobId)` — Move to progress
- `onDeletionComplete(job)` — Handle completion
- Computed properties: `showConfirmModal`, `showProgressModal`

### Step 4: Update Resource Pages

**Example: src/pages/InstancesPage.tsx**

```typescript
import { useDeletionFlow } from '../hooks/useDeletionFlow';
import { DeleteResourceModal } from '../components/DeleteResourceModal';
import { DeletionProgressModal } from '../components/DeletionProgressModal';

export const InstancesPage: React.FC = () => {
  const deletion = useDeletionFlow({
    onDeletionComplete: (job) => {
      // Refresh instances list
      refetchInstances();
      
      // Show success toast/notification
      toast.success(`Instance ${job.resourceId} deleted`);
    },
    onDeletionFailed: (job, error) => {
      // Show error toast
      toast.error(`Failed to delete: ${error}`);
    },
  });

  const handleDeleteInstance = (instanceId: string, instanceName: string) => {
    deletion.startDeletion('instance', instanceId, instanceName);
  };

  return (
    <Box>
      {/* Instances table/list */}
      <InstancesList 
        onDelete={(id, name) => handleDeleteInstance(id, name)}
      />

      {/* Delete confirmation modal - Phase 1 */}
      <DeleteResourceModal
        open={deletion.showConfirmModal}
        resourceType={deletion.state.resourceType}
        resourceId={deletion.state.resourceId}
        resourceName={deletion.state.resourceName}
        onClose={deletion.closeConfirmModal}
        onSuccess={(jobId) => {
          deletion.closeConfirmModal();
          deletion.onConfirmSuccess(jobId);
        }}
      />

      {/* Progress modal - Phase 2 */}
      {deletion.state.jobId && (
        <DeletionProgressModal
          open={deletion.showProgressModal}
          jobId={deletion.state.jobId}
          resourceType={deletion.state.resourceType}
          resourceId={deletion.state.resourceId}
          onClose={deletion.closeModal}
          onComplete={deletion.onDeletionComplete}
        />
      )}
    </Box>
  );
};
```

### Step 5: Update Delete Buttons

**Before:**
```typescript
<Button
  onClick={() => api.delete(`/instances/${id}`)}
  color="error"
>
  Delete
</Button>
```

**After:**
```typescript
<Button
  onClick={() => deletion.startDeletion('instance', id, instanceName)}
  color="error"
>
  Delete
</Button>
```

### Step 6: Handle Different Resource Types

The framework is generic, so update each resource type:

**Instances:**
```typescript
deletion.startDeletion('instance', instanceId, instanceName);
```

**VPCs:**
```typescript
deletion.startDeletion('vpc', vpcId, vpcName);
```

**Load Balancers:**
```typescript
deletion.startDeletion('load-balancer', lbId, lbName);
```

**Databases:**
```typescript
deletion.startDeletion('database', dbId, dbName);
```

---

## Component Customization

### Styling

All components use Material-UI (MUI). Customize theme:

```typescript
// In DeleteResourceModal.tsx, add custom styles:
const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    confirmPhrase: {
      fontFamily: 'monospace',
      fontSize: '14px',
      fontWeight: 600,
    },
    deleteButton: {
      backgroundColor: theme.palette.error.main,
      '&:hover': {
        backgroundColor: theme.palette.error.dark,
      },
    },
  })
);
```

### Confirmation Phrase

The phrase must be exactly "DELETE" (uppercase). To customize:

```typescript
// In DeleteResourceModal.tsx, change validation:
const CONFIRMATION_PHRASE = 'PERMANENTLY DELETE'; // Change this
if (confirmationPhrase !== CONFIRMATION_PHRASE) {
  setError(`Phrase must be exactly "${CONFIRMATION_PHRASE}"`);
}
```

### Polling Interval

Default is 1 second. To adjust:

```typescript
// In DeletionProgressModal.tsx
await deletionAPI.pollUntilComplete(jobId, {
  interval: 2000, // 2 seconds
  timeout: 3600000, // 1 hour
  onProgress: (job) => setJob(job),
});
```

---

## Testing

### Unit Tests

**Test file: src/components/__tests__/DeleteResourceModal.test.tsx**

```typescript
import { render, screen, userEvent } from '@testing-library/react';
import { DeleteResourceModal } from '../DeleteResourceModal';
import * as deletionAPI from '../../api/deletion-api-client';

describe('DeleteResourceModal', () => {
  beforeEach(() => {
    jest.spyOn(deletionAPI, 'preflight').mockResolvedValue({
      resourceType: 'instance',
      resourceId: 'inst-123',
      confirmationToken: 'token-abc',
      requiresConfirmation: true,
      deleteOrder: ['instance-1'],
    });
  });

  it('should show preflight data', async () => {
    render(
      <DeleteResourceModal
        open={true}
        resourceType="instance"
        resourceId="inst-123"
        onClose={jest.fn()}
      />
    );

    expect(await screen.findByText('instance-1')).toBeInTheDocument();
  });

  it('should require "DELETE" confirmation', async () => {
    const user = userEvent.setup();
    const onSuccess = jest.fn();

    render(
      <DeleteResourceModal
        open={true}
        resourceType="instance"
        resourceId="inst-123"
        onClose={jest.fn()}
        onSuccess={onSuccess}
      />
    );

    const input = await screen.findByPlaceholderText('DELETE');
    
    // Type wrong phrase
    await user.type(input, 'DELETE ME');
    expect(screen.getByText(/must be exactly "DELETE"/i)).toBeInTheDocument();

    // Type correct phrase
    await user.clear(input);
    await user.type(input, 'DELETE');
    expect(screen.queryByText(/must be exactly/i)).not.toBeInTheDocument();
  });

  it('should call onSuccess with jobId when confirmed', async () => {
    const user = userEvent.setup();
    const onSuccess = jest.fn();

    jest.spyOn(deletionAPI, 'confirm').mockResolvedValue({
      jobId: 'del-job-123',
      status: 'queued',
      pollUrl: '/api/v1/deletion-jobs/del-job-123',
    });

    render(
      <DeleteResourceModal
        open={true}
        resourceType="instance"
        resourceId="inst-123"
        onClose={jest.fn()}
        onSuccess={onSuccess}
      />
    );

    const input = await screen.findByPlaceholderText('DELETE');
    await user.type(input, 'DELETE');

    const confirmButton = screen.getByRole('button', { name: /confirm/i });
    await user.click(confirmButton);

    expect(onSuccess).toHaveBeenCalledWith('del-job-123');
  });
});
```

### Integration Tests

**Test file: src/__tests__/deletion-flow.integration.test.tsx**

```typescript
describe('3-Phase Deletion Flow', () => {
  it('should complete full deletion: preflight → confirm → progress', async () => {
    const user = userEvent.setup();
    
    // 1. Click delete button
    const { getByRole } = render(<InstancesPage />);
    await user.click(getByRole('button', { name: /delete.*instance-1/i }));

    // 2. Verify preflight modal shows
    expect(screen.getByText(/resources to be deleted/i)).toBeInTheDocument();

    // 3. Type "DELETE" and confirm
    const input = screen.getByPlaceholderText('DELETE');
    await user.type(input, 'DELETE');
    await user.click(getByRole('button', { name: /confirm/i }));

    // 4. Verify progress modal appears
    expect(await screen.findByText(/deletion in progress/i)).toBeInTheDocument();

    // 5. Poll until completion (mock will auto-complete)
    expect(await screen.findByText(/deletion completed/i)).toBeInTheDocument();

    // 6. Verify modal closes and list refreshes
    await waitFor(() => {
      expect(screen.queryByText(/deletion completed/i)).not.toBeInTheDocument();
    });
  });

  it('should handle errors with recovery suggestions', async () => {
    // Mock error response
    jest.spyOn(deletionAPI, 'getJobStatus').mockResolvedValue({
      ...mockJob,
      status: 'failed',
      errors: [{
        step: 'stop-instance',
        resource: 'instance',
        resourceId: 'inst-123',
        reason: 'kernel panic',
        recoverable: false,
        recovery: 'Check logs and manually recover',
      }],
    });

    // Run deletion flow
    // Should show error with recovery suggestion
    expect(screen.getByText(/kernel panic/i)).toBeInTheDocument();
    expect(screen.getByText(/Check logs and manually recover/i)).toBeInTheDocument();
  });

  it('should auto-close on success', async () => {
    // Run deletion flow
    // Should auto-close after 2 seconds on success
    expect(screen.getByText(/deletion completed/i)).toBeInTheDocument();
    
    await waitFor(() => {
      expect(screen.queryByText(/deletion completed/i)).not.toBeInTheDocument();
    }, { timeout: 3000 });
  });
});
```

### Manual Testing Checklist

- [ ] **Preflight Phase**
  - [ ] Click delete on a resource
  - [ ] Verify modal shows resource name and type
  - [ ] Verify deletion order displays correctly
  - [ ] Verify blockers display if any
  
- [ ] **Confirmation Phase**
  - [ ] Typing wrong phrase shows error
  - [ ] Typing "delete" (lowercase) shows error
  - [ ] Typing "DELETE" (uppercase) removes error
  - [ ] Confirm button enabled only when phrase matches
  - [ ] Click confirm creates deletion job
  
- [ ] **Progress Phase**
  - [ ] Progress modal appears after confirm
  - [ ] Progress bar advances (0→100%)
  - [ ] Current step updates in real-time
  - [ ] Completed steps show checkmarks
  - [ ] Modal auto-closes on success (2 sec)
  
- [ ] **Error Handling**
  - [ ] Network error shows "try again" message
  - [ ] Job timeout after 1 hour shows error
  - [ ] Job expiry (after 7 days) shows 404 error
  - [ ] Errors accordion shows recovery suggestions
  
- [ ] **List Refresh**
  - [ ] List refreshes after successful deletion
  - [ ] Deleted item disappears from list
  - [ ] Count updates correctly

---

## Performance Optimization

### Polling Strategy

```typescript
// Adaptive polling: slower when idle, faster during deletion
let interval = 1000; // Start at 1 second
let noProgressCount = 0;

const adaptivePolling = async () => {
  const job = await getJobStatus(jobId);
  
  if (job.progress === lastProgress) {
    noProgressCount++;
    if (noProgressCount > 5) {
      interval = 5000; // Slow down after 5 no-progress polls
    }
  } else {
    noProgressCount = 0;
    interval = 1000; // Reset to fast when progress detected
  }
  
  lastProgress = job.progress;
};
```

### Memory Management

```typescript
// Clear completed jobs from cache after 1 hour
useEffect(() => {
  if (isComplete) {
    const timer = setTimeout(() => {
      setCompletedJob(null);
    }, 3600000); // 1 hour
    return () => clearTimeout(timer);
  }
}, [isComplete]);
```

---

## Security Checklist

- ✅ Confirmation phrase prevents accidental deletes
- ✅ Token validation in POST body (not URL)
- ✅ CSRF protection via HTTP-only cookies
- ✅ Authorization checked on backend
- ✅ Jobs auto-expire (7 days)
- ✅ All errors logged for audit
- ✅ No sensitive data in localStorage
- ✅ WebSocket polling prevents hanging connections

---

## Troubleshooting

### Modal doesn't appear
- Check `showConfirmModal` state
- Verify `startDeletion` was called
- Check browser console for errors

### Confirmation phrase always invalid
- Verify typing exactly "DELETE" (uppercase)
- Check for extra spaces (trim input)
- Verify comparison is case-sensitive

### Progress not updating
- Check polling interval (default 1 second)
- Verify job exists on backend
- Check network tab for API calls
- Verify jobId is correct

### Modal doesn't close on completion
- Check auto-close timeout (default 2 seconds)
- Verify `onClose` callback
- Check for errors preventing close

### List doesn't refresh
- Verify `refetchInstances` or equivalent called
- Check if list query has cache
- Verify deleted item ID matches

---

## FAQ

**Q: Can I cancel a deletion?**
A: Currently no, but can be added. Backend supports it via `Cancel` method in DeletionJobStore.

**Q: What if I close the browser during deletion?**
A: Job continues on backend. Reopen page and poll with jobId to see status.

**Q: Can I delete multiple resources at once?**
A: Not currently, but use `useBulkDeletionFlow()` hook for queue management.

**Q: How long do jobs persist?**
A: 7 days, then auto-deleted. Stored in SQLite alongside other data.

**Q: What happens if deletion times out?**
A: Returns error after 1 hour. User can manually check status or retry.

---

## Next Steps

1. Copy `deletion-api-client.ts` to CapperWeb
2. Copy `useDeletionFlow.ts` to CapperWeb
3. Implement DeleteResourceModal component
4. Implement DeletionProgressModal component
5. Update resource pages to use components
6. Run unit tests
7. Manual QA of 3-phase flow
8. Deploy to staging
9. Deploy to production

**Estimated effort**: 8-12 hours for full implementation and testing

