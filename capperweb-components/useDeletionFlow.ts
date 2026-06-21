/**
 * React Hook for Managing 3-Phase Deletion Flow
 *
 * Manages state for:
 * 1. Delete confirmation modal
 * 2. Progress tracking modal
 * 3. Completion/error handling
 *
 * Usage:
 *   const deletion = useDeletionFlow();
 *
 *   // Start deletion
 *   <DeleteButton onClick={() =>
 *     deletion.startDeletion('instance', 'inst-123', 'my-instance')
 *   } />
 *
 *   // Show modals
 *   <DeleteResourceModal {...deletion.confirmModal} />
 *   <DeletionProgressModal {...deletion.progressModal} />
 */

import { useState, useCallback } from 'react';
import { DeletionJob, getDeletionAPI } from './deletion-api-client';

export enum DeletionPhase {
  Idle = 'idle',
  ConfirmModal = 'confirm_modal',
  ProgressModal = 'progress_modal',
  Complete = 'complete',
  Error = 'error',
}

export interface DeletionFlowState {
  phase: DeletionPhase;
  resourceType: string;
  resourceId: string;
  resourceName?: string;
  jobId?: string;
  completedJob?: DeletionJob;
  error?: string;
}

export interface DeletionFlowCallbacks {
  onPreflightSuccess?: () => void;
  onConfirmSuccess?: (jobId: string) => void;
  onDeletionComplete?: (job: DeletionJob) => void;
  onDeletionFailed?: (job: DeletionJob, error: string) => void;
  onError?: (error: string) => void;
}

export const useDeletionFlow = (callbacks?: DeletionFlowCallbacks) => {
  const [state, setState] = useState<DeletionFlowState>({
    phase: DeletionPhase.Idle,
    resourceType: '',
    resourceId: '',
  });

  // Phase 1: Open confirmation modal
  const startDeletion = useCallback(
    (resourceType: string, resourceId: string, resourceName?: string) => {
      setState({
        phase: DeletionPhase.ConfirmModal,
        resourceType,
        resourceId,
        resourceName,
      });
      callbacks?.onPreflightSuccess?.();
    },
    [callbacks]
  );

  // Close confirmation modal
  const closeConfirmModal = useCallback(() => {
    setState(prev => ({
      ...prev,
      phase: DeletionPhase.Idle,
    }));
  }, []);

  // Phase 2: Confirmation confirmed, open progress modal
  const onConfirmSuccess = useCallback(
    (jobId: string) => {
      setState(prev => ({
        ...prev,
        phase: DeletionPhase.ProgressModal,
        jobId,
      }));
      callbacks?.onConfirmSuccess?.(jobId);
    },
    [callbacks]
  );

  // Phase 3: Deletion complete
  const onDeletionComplete = useCallback(
    (job: DeletionJob) => {
      const isFailed = job.status === 'failed';
      const newPhase = isFailed ? DeletionPhase.Error : DeletionPhase.Complete;

      setState(prev => ({
        ...prev,
        phase: newPhase,
        completedJob: job,
      }));

      if (isFailed) {
        const errorMsg = job.errors?.[0]?.reason || 'Deletion failed';
        callbacks?.onDeletionFailed?.(job, errorMsg);
      } else {
        callbacks?.onDeletionComplete?.(job);
      }
    },
    [callbacks]
  );

  // Handle errors
  const setError = useCallback(
    (error: string) => {
      setState(prev => ({
        ...prev,
        phase: DeletionPhase.Error,
        error,
      }));
      callbacks?.onError?.(error);
    },
    [callbacks]
  );

  // Close progress modal / error dialog
  const closeModal = useCallback(() => {
    setState({
      phase: DeletionPhase.Idle,
      resourceType: '',
      resourceId: '',
    });
  }, []);

  // Reset to idle
  const reset = useCallback(() => {
    setState({
      phase: DeletionPhase.Idle,
      resourceType: '',
      resourceId: '',
    });
  }, []);

  // Computed properties for modal visibility
  const showConfirmModal = state.phase === DeletionPhase.ConfirmModal;
  const showProgressModal = state.phase === DeletionPhase.ProgressModal;
  const showErrorDialog = state.phase === DeletionPhase.Error;
  const isComplete = state.phase === DeletionPhase.Complete;

  return {
    // State
    state,
    showConfirmModal,
    showProgressModal,
    showErrorDialog,
    isComplete,

    // Actions
    startDeletion,
    closeConfirmModal,
    onConfirmSuccess,
    onDeletionComplete,
    closeModal,
    reset,
    setError,

    // Derived data
    hasJob: !!state.jobId,
    hasError: !!state.error,
    jobProgress: state.completedJob?.progress ?? 0,
  };
};

/**
 * Hook for managing multiple active deletions
 * Tracks multiple deletion flows in a queue
 */
export const useBulkDeletionFlow = () => {
  const [deletionQueue, setDeletionQueue] = useState<Map<string, DeletionFlowState>>(
    new Map()
  );

  const startDeletion = useCallback(
    (resourceType: string, resourceId: string, resourceName?: string) => {
      const id = `${resourceType}-${resourceId}`;
      setDeletionQueue(prev => new Map(prev).set(id, {
        phase: DeletionPhase.ConfirmModal,
        resourceType,
        resourceId,
        resourceName,
      }));
    },
    []
  );

  const removeDeletion = useCallback((id: string) => {
    setDeletionQueue(prev => {
      const next = new Map(prev);
      next.delete(id);
      return next;
    });
  }, []);

  const updateDeletion = useCallback((id: string, updates: Partial<DeletionFlowState>) => {
    setDeletionQueue(prev => {
      const next = new Map(prev);
      const current = next.get(id);
      if (current) {
        next.set(id, { ...current, ...updates });
      }
      return next;
    });
  }, []);

  return {
    deletionQueue: Array.from(deletionQueue.values()),
    startDeletion,
    removeDeletion,
    updateDeletion,
    count: deletionQueue.size,
  };
};

/**
 * Hook for automatically refreshing data after successful deletion
 */
export const useDeletionRefresh = (onDelete?: () => Promise<void>) => {
  const [isRefreshing, setIsRefreshing] = useState(false);

  const refresh = useCallback(async () => {
    setIsRefreshing(true);
    try {
      await onDelete?.();
    } finally {
      setIsRefreshing(false);
    }
  }, [onDelete]);

  return { refresh, isRefreshing };
};
