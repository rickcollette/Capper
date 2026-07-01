# CapperWeb Integration Guide — 3-Phase Deletion UI

This guide provides complete TypeScript/React components and API client code for integrating the new cascading deletion framework into CapperWeb.

## API Client (api/deletion.ts)

```typescript
// api/deletion.ts
import { api } from './client';

export interface DeletionPreflight {
  resourceType: string;
  resourceId: string;
  confirmationToken: string;
  requiresConfirmation: boolean;
  deleteOrder: string[];
  blockedBy?: {
    instances?: number;
    subnets?: number;
    [key: string]: number;
  };
}

export interface DeletionJobError {
  step: string;
  resource: string;
  resourceId: string;
  reason: string;
  recoverable: boolean;
  recovery: string;
}

export interface DeletionJob {
  id: string;
  status: 'queued' | 'pre_flight' | 'running' | 'completed' | 'failed' | 'cancelled';
  resourceType: string;
  resourceId: string;
  confirmationToken: string;
  progress: number;
  currentStep: string;
  steps: string[];
  completedSteps: string[];
  remainingSteps: string[];
  errors: DeletionJobError[];
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
  expiresAt: string;
}

export const deletionAPI = {
  // Phase 1: Get preflight info
  preflight: async (
    resourceType: string,
    resourceId: string
  ): Promise<DeletionPreflight> => {
    const response = await api.post(
      `/api/v1/${resourceType}/${resourceId}/delete-preflight`
    );
    return response.data.data;
  },

  // Phase 2: Confirm deletion with "DELETE" phrase
  confirm: async (
    resourceType: string,
    resourceId: string,
    confirmationToken: string
  ): Promise<{ jobId: string; status: string; pollUrl: string }> => {
    const response = await api.post(
      `/api/v1/${resourceType}/${resourceId}/delete-confirm`,
      {
        confirmationToken,
        confirmationPhrase: 'DELETE',
      }
    );
    return response.data.data;
  },

  // Phase 3: Poll job status
  getJobStatus: async (jobId: string): Promise<DeletionJob> => {
    const response = await api.get(`/api/v1/deletion-jobs/${jobId}`);
    return response.data.data;
  },

  // Helper: Poll until completion
  pollUntilComplete: async (
    jobId: string,
    interval: number = 1000,
    timeout: number = 3600000 // 1 hour default
  ): Promise<DeletionJob> => {
    const startTime = Date.now();
    
    while (Date.now() - startTime < timeout) {
      const job = await deletionAPI.getJobStatus(jobId);
      
      if (job.status === 'completed' || job.status === 'failed' || job.status === 'cancelled') {
        return job;
      }
      
      // Wait before next poll
      await new Promise(resolve => setTimeout(resolve, interval));
    }
    
    throw new Error(`Deletion job ${jobId} timed out after ${timeout}ms`);
  },
};
```

## Delete Modal Component (components/DeleteResourceModal.tsx)

```typescript
// components/DeleteResourceModal.tsx
import React, { useState, useEffect } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Alert,
  CircularProgress,
  Box,
  Typography,
  List,
  ListItem,
  Chip,
} from '@mui/material';
import { deletionAPI, DeletionPreflight } from '../api/deletion';

interface DeleteResourceModalProps {
  open: boolean;
  resourceType: string;
  resourceId: string;
  resourceName?: string;
  onClose: () => void;
  onSuccess?: (jobId: string) => void;
}

export const DeleteResourceModal: React.FC<DeleteResourceModalProps> = ({
  open,
  resourceType,
  resourceId,
  resourceName,
  onClose,
  onSuccess,
}) => {
  const [step, setStep] = useState<'preflight' | 'confirm' | 'loading'>('preflight');
  const [preflight, setPreflight] = useState<DeletionPreflight | null>(null);
  const [confirmationPhrase, setConfirmationPhrase] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Load preflight on open
  useEffect(() => {
    if (!open) return;
    
    const loadPreflight = async () => {
      try {
        setLoading(true);
        setError(null);
        const result = await deletionAPI.preflight(resourceType, resourceId);
        setPreflight(result);
        setStep('confirm');
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load deletion info');
        setStep('preflight');
      } finally {
        setLoading(false);
      }
    };

    loadPreflight();
  }, [open, resourceType, resourceId]);

  const handleConfirm = async () => {
    if (confirmationPhrase !== 'DELETE') {
      setError('Confirmation phrase must be exactly "DELETE" (uppercase)');
      return;
    }

    if (!preflight) {
      setError('Missing confirmation token');
      return;
    }

    try {
      setLoading(true);
      setError(null);
      
      const result = await deletionAPI.confirm(
        resourceType,
        resourceId,
        preflight.confirmationToken
      );
      
      setStep('loading');
      onSuccess?.(result.jobId);
      
      // Optional: automatically close and show progress modal
      // setTimeout(() => onClose(), 1000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to confirm deletion');
      setStep('confirm');
    } finally {
      setLoading(false);
    }
  };

  const phraseMatches = confirmationPhrase === 'DELETE';
  const isReady = phraseMatches && !loading;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        Delete {resourceType} {resourceName ? `"${resourceName}"` : resourceId}
      </DialogTitle>
      
      <DialogContent>
        {loading && step === 'preflight' ? (
          <Box display="flex" justifyContent="center" py={3}>
            <CircularProgress />
          </Box>
        ) : step === 'confirm' && preflight ? (
          <Box>
            {/* Show what will be deleted */}
            {preflight.deleteOrder && preflight.deleteOrder.length > 0 && (
              <Box mb={3}>
                <Typography variant="subtitle2" gutterBottom>
                  Resources to be deleted (in order):
                </Typography>
                <List dense>
                  {preflight.deleteOrder.map((item, idx) => (
                    <ListItem key={idx}>
                      <Chip
                        label={item}
                        size="small"
                        icon={<span>{idx + 1}</span>}
                        variant="outlined"
                      />
                    </ListItem>
                  ))}
                </List>
              </Box>
            )}

            {/* Show blockers if any */}
            {preflight.blockedBy && Object.keys(preflight.blockedBy).length > 0 && (
              <Alert severity="warning" sx={{ mb: 2 }}>
                <Typography variant="subtitle2" gutterBottom>
                  The following must be deleted first:
                </Typography>
                <List dense>
                  {Object.entries(preflight.blockedBy).map(([resource, count]) => (
                    <ListItem key={resource}>
                      {count} {resource}
                    </ListItem>
                  ))}
                </List>
              </Alert>
            )}

            {/* Confirmation phrase input */}
            <Box mb={2}>
              <Typography variant="body2" color="textSecondary" gutterBottom>
                To confirm, type <strong>DELETE</strong> (uppercase):
              </Typography>
              <TextField
                fullWidth
                placeholder="DELETE"
                value={confirmationPhrase}
                onChange={(e) => {
                  setConfirmationPhrase(e.target.value);
                  // Clear error when user starts typing
                  if (error) setError(null);
                }}
                autoFocus
                disabled={loading}
              />
              {confirmationPhrase && !phraseMatches && (
                <Typography variant="caption" color="error" sx={{ mt: 1, display: 'block' }}>
                  ❌ Phrase does not match. Type exactly: DELETE
                </Typography>
              )}
              {phraseMatches && (
                <Typography variant="caption" color="success.main" sx={{ mt: 1, display: 'block' }}>
                  ✓ Phrase matches
                </Typography>
              )}
            </Box>

            {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
          </Box>
        ) : step === 'loading' ? (
          <Box display="flex" flexDirection="column" alignItems="center" py={3}>
            <CircularProgress sx={{ mb: 2 }} />
            <Typography>Starting deletion...</Typography>
          </Box>
        ) : null}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        {step === 'confirm' && (
          <Button
            onClick={handleConfirm}
            variant="contained"
            color="error"
            disabled={!isReady}
            loading={loading}
          >
            Confirm Delete
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
};
```

## Progress Modal Component (components/DeletionProgressModal.tsx)

```typescript
// components/DeletionProgressModal.tsx
import React, { useState, useEffect } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  LinearProgress,
  Alert,
  Box,
  Typography,
  Chip,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Accordion,
  AccordionSummary,
  AccordionDetails,
} from '@mui/material';
import {
  CheckCircle as CheckCircleIcon,
  HourglassEmpty as HourglassIcon,
  Error as ErrorIcon,
  ExpandMore as ExpandMoreIcon,
} from '@mui/icons-material';
import { deletionAPI, DeletionJob } from '../api/deletion';

interface DeletionProgressModalProps {
  open: boolean;
  jobId: string;
  resourceType: string;
  resourceId: string;
  onClose: () => void;
  onComplete?: (job: DeletionJob) => void;
}

export const DeletionProgressModal: React.FC<DeletionProgressModalProps> = ({
  open,
  jobId,
  resourceType,
  resourceId,
  onClose,
  onComplete,
}) => {
  const [job, setJob] = useState<DeletionJob | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Poll job status
  useEffect(() => {
    if (!open || !jobId) return;

    setLoading(true);
    setError(null);

    const pollJob = async () => {
      try {
        const job = await deletionAPI.pollUntilComplete(jobId, 1000);
        setJob(job);
        onComplete?.(job);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Polling error');
      } finally {
        setLoading(false);
      }
    };

    pollJob();

    // Also setup interval-based polling as fallback
    const interval = setInterval(async () => {
      try {
        const status = await deletionAPI.getJobStatus(jobId);
        setJob(status);
        
        if (status.status === 'completed' || status.status === 'failed') {
          clearInterval(interval);
          setLoading(false);
          onComplete?.(status);
        }
      } catch (err) {
        // Silent fail on individual polls, error handling happens elsewhere
      }
    }, 2000);

    return () => clearInterval(interval);
  }, [open, jobId, onComplete]);

  if (!job) {
    return (
      <Dialog open={open} maxWidth="sm" fullWidth>
        <DialogTitle>Deletion in Progress</DialogTitle>
        <DialogContent>
          <Box display="flex" justifyContent="center" py={3}>
            {loading ? <CircularProgress /> : null}
          </Box>
        </DialogContent>
      </Dialog>
    );
  }

  const isCompleted = job.status === 'completed' || job.status === 'failed';
  const isFailed = job.status === 'failed';
  const hasErrors = job.errors && job.errors.length > 0;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="sm"
      fullWidth
      disableEscapeKeyDown={!isCompleted}
    >
      <DialogTitle>
        Deleting {resourceType} "{resourceId}"
        {isCompleted && (
          <Chip
            label={isFailed ? 'Failed' : 'Complete'}
            color={isFailed ? 'error' : 'success'}
            size="small"
            sx={{ ml: 1 }}
          />
        )}
      </DialogTitle>

      <DialogContent>
        <Box>
          {/* Progress bar */}
          <Box mb={2}>
            <Box display="flex" justifyContent="space-between" mb={1}>
              <Typography variant="body2">Progress</Typography>
              <Typography variant="body2" color="textSecondary">
                {job.progress}%
              </Typography>
            </Box>
            <LinearProgress
              variant="determinate"
              value={job.progress}
              color={isFailed ? 'error' : 'primary'}
            />
          </Box>

          {/* Current step */}
          {job.currentStep && !isCompleted && (
            <Alert severity="info" sx={{ mb: 2 }}>
              <Typography variant="body2">
                <strong>Current step:</strong> {job.currentStep}
              </Typography>
            </Alert>
          )}

          {/* Completed steps */}
          {job.completedSteps && job.completedSteps.length > 0 && (
            <Box mb={2}>
              <Typography variant="subtitle2" gutterBottom>
                Completed ({job.completedSteps.length})
              </Typography>
              <List dense>
                {job.completedSteps.map((step) => (
                  <ListItem key={step}>
                    <ListItemIcon>
                      <CheckCircleIcon color="success" fontSize="small" />
                    </ListItemIcon>
                    <ListItemText primary={step} />
                  </ListItem>
                ))}
              </List>
            </Box>
          )}

          {/* Remaining steps */}
          {job.remainingSteps && job.remainingSteps.length > 0 && !isCompleted && (
            <Box mb={2}>
              <Typography variant="subtitle2" gutterBottom>
                Remaining ({job.remainingSteps.length})
              </Typography>
              <List dense>
                {job.remainingSteps.map((step) => (
                  <ListItem key={step}>
                    <ListItemIcon>
                      <HourglassIcon fontSize="small" />
                    </ListItemIcon>
                    <ListItemText primary={step} />
                  </ListItem>
                ))}
              </List>
            </Box>
          )}

          {/* Errors */}
          {hasErrors && (
            <Box mb={2}>
              <Accordion defaultExpanded={isFailed}>
                <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                  <ErrorIcon color="error" sx={{ mr: 1 }} />
                  <Typography variant="subtitle2">
                    Errors ({job.errors.length})
                  </Typography>
                </AccordionSummary>
                <AccordionDetails>
                  <List dense sx={{ width: '100%' }}>
                    {job.errors.map((err, idx) => (
                      <ListItem key={idx}>
                        <ListItemText
                          primary={`${err.step}: ${err.reason}`}
                          secondary={
                            <Box>
                              <Typography variant="caption" display="block">
                                Resource: {err.resource} ({err.resourceId})
                              </Typography>
                              {err.recovery && (
                                <Typography variant="caption" display="block" color="success.main">
                                  ✓ {err.recovery}
                                </Typography>
                              )}
                            </Box>
                          }
                        />
                      </ListItem>
                    ))}
                  </List>
                </AccordionDetails>
              </Accordion>
            </Box>
          )}

          {isCompleted && !isFailed && (
            <Alert severity="success">
              ✓ Deletion completed successfully
            </Alert>
          )}

          {isFailed && (
            <Alert severity="error">
              ❌ Deletion failed. Check errors above for recovery steps.
            </Alert>
          )}
        </Box>
      </DialogContent>

      <DialogActions>
        {!isCompleted ? (
          <Button onClick={onClose}>Close</Button>
        ) : (
          <Button onClick={onClose} variant="contained">
            Done
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
};
```

## Hook for Managing Deletion Flow (hooks/useDeletion.ts)

```typescript
// hooks/useDeletion.ts
import { useState } from 'react';
import { DeletionJob } from '../api/deletion';

interface UseDeletionState {
  step: 'idle' | 'delete_modal' | 'progress_modal';
  resourceType: string;
  resourceId: string;
  resourceName?: string;
  jobId?: string;
  completedJob?: DeletionJob;
}

export const useDeletion = () => {
  const [state, setState] = useState<UseDeletionState>({
    step: 'idle',
    resourceType: '',
    resourceId: '',
  });

  const startDeletion = (
    resourceType: string,
    resourceId: string,
    resourceName?: string
  ) => {
    setState({
      step: 'delete_modal',
      resourceType,
      resourceId,
      resourceName,
    });
  };

  const closeDeleteModal = () => {
    setState({ ...state, step: 'idle' });
  };

  const openProgressModal = (jobId: string) => {
    setState({ ...state, step: 'progress_modal', jobId });
  };

  const closeProgressModal = () => {
    setState({ ...state, step: 'idle' });
  };

  const onDeletionComplete = (job: DeletionJob) => {
    setState({ ...state, completedJob: job });
    // Auto-close after 2 seconds on success
    if (job.status === 'completed') {
      setTimeout(() => closeProgressModal(), 2000);
    }
  };

  return {
    state,
    startDeletion,
    closeDeleteModal,
    openProgressModal,
    closeProgressModal,
    onDeletionComplete,
  };
};
```

## Integration in Resource Pages (example: InstancesPage.tsx)

```typescript
// pages/InstancesPage.tsx
import { useDeletion } from '../hooks/useDeletion';
import { DeleteResourceModal } from '../components/DeleteResourceModal';
import { DeletionProgressModal } from '../components/DeletionProgressModal';

export const InstancesPage: React.FC = () => {
  const deletion = useDeletion();
  
  const handleDeleteInstance = (instanceId: string, instanceName: string) => {
    deletion.startDeletion('instance', instanceId, instanceName);
  };

  return (
    <Box>
      {/* Your existing instances list */}
      <InstancesList onDelete={handleDeleteInstance} />

      {/* Delete confirmation modal */}
      <DeleteResourceModal
        open={deletion.state.step === 'delete_modal'}
        resourceType={deletion.state.resourceType}
        resourceId={deletion.state.resourceId}
        resourceName={deletion.state.resourceName}
        onClose={deletion.closeDeleteModal}
        onSuccess={(jobId) => {
          deletion.closeDeleteModal();
          deletion.openProgressModal(jobId);
        }}
      />

      {/* Deletion progress modal */}
      {deletion.state.jobId && (
        <DeletionProgressModal
          open={deletion.state.step === 'progress_modal'}
          jobId={deletion.state.jobId}
          resourceType={deletion.state.resourceType}
          resourceId={deletion.state.resourceId}
          onClose={deletion.closeProgressModal}
          onComplete={(job) => {
            deletion.onDeletionComplete(job);
            // Refresh instances list on success
            if (job.status === 'completed') {
              // refetch instances
            }
          }}
        />
      )}
    </Box>
  );
};
```

## Testing Guide

### Unit Tests (DeleteResourceModal.test.tsx)

```typescript
describe('DeleteResourceModal', () => {
  it('should show deletion order in preflight', () => {
    // Test that preflight data displays correctly
  });

  it('should reject confirmation phrase if not "DELETE"', () => {
    // Test phrase validation
  });

  it('should enable confirm button only when phrase matches', () => {
    // Test button state
  });

  it('should call onSuccess with jobId when confirmed', () => {
    // Test confirmation flow
  });
});
```

### Integration Tests

```typescript
describe('3-Phase Deletion Flow', () => {
  it('should complete full deletion flow: preflight → confirm → progress', () => {
    // 1. Click delete on instance
    // 2. Verify preflight modal shows deletion order
    // 3. Type "DELETE" and confirm
    // 4. Verify progress modal appears
    // 5. Poll until completion
    // 6. Verify modal closes and list refreshes
  });

  it('should handle errors during deletion', () => {
    // Verify error messages with recovery suggestions are shown
  });

  it('should auto-close on successful completion', () => {
    // Verify 2-second auto-close on success
  });
});
```

## Checklist for Integration

- [ ] Copy `api/deletion.ts` to CapperWeb `src/api/deletion.ts`
- [ ] Copy `components/DeleteResourceModal.tsx` to CapperWeb
- [ ] Copy `components/DeletionProgressModal.tsx` to CapperWeb
- [ ] Copy `hooks/useDeletion.ts` to CapperWeb `src/hooks/`
- [ ] Update instance/VPC/LB pages to use `useDeletion` hook
- [ ] Update delete buttons to call `startDeletion()`
- [ ] Test confirmation phrase validation (must be "DELETE")
- [ ] Test progress polling and update
- [ ] Test error handling and recovery suggestions
- [ ] Test list refresh after successful deletion
- [ ] Manual QA: test 3-phase flow end-to-end

## Error Handling Best Practices

1. **Recoverable Errors**: Show recovery suggestion and allow retry
2. **Unrecoverable Errors**: Show error and recovery action, don't allow immediate retry
3. **Timeout**: If job doesn't complete in 1 hour, show timeout error
4. **Job Expiry**: After 7 days, job is auto-deleted; handle 404 gracefully

## Performance Optimization

1. **Polling Interval**: 1-2 seconds during active deletion
2. **Exponential Backoff**: If no progress in 5 minutes, increase poll interval
3. **Connection Reuse**: Use same HTTP client for all deletion API calls
4. **Memory**: Clear completed jobs from cache after 1 hour

## Security Considerations

1. ✅ Confirmation phrase prevents accidental deletes
2. ✅ Token validation prevents CSRF on delete-confirm
3. ✅ No confirmation token in URL (only in POST body)
4. ✅ Jobs auto-expire (7 days max retention)
5. ✅ All errors logged for audit trail

