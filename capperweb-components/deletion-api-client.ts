/**
 * API Client for Cascading Deletion Framework
 *
 * Usage:
 *   const preflight = await deletionAPI.preflight('instance', 'inst-123');
 *   const result = await deletionAPI.confirm('instance', 'inst-123', token);
 *   const job = await deletionAPI.getJobStatus(jobId);
 */

// Assuming axios is available as the HTTP client
import axios, { AxiosInstance } from 'axios';

export interface DeletionPreflight {
  resourceType: string;
  resourceId: string;
  confirmationToken: string;
  requiresConfirmation: boolean;
  deleteOrder: string[];
  blockedBy?: {
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

export interface DeletionConfirmResponse {
  jobId: string;
  status: string;
  pollUrl: string;
}

class DeletionAPIClient {
  private client: AxiosInstance;
  private baseURL: string;

  constructor(client: AxiosInstance, baseURL: string = '/api') {
    this.client = client;
    this.baseURL = baseURL;
  }

  /**
   * Get preflight deletion information
   * Shows what will be deleted without actually deleting
   */
  async preflight(
    resourceType: string,
    resourceId: string
  ): Promise<DeletionPreflight> {
    try {
      const response = await this.client.post(
        `${this.baseURL}/v1/${resourceType}/${resourceId}:delete-preflight`
      );
      return response.data.data;
    } catch (error) {
      throw new Error(
        `Failed to get deletion preflight: ${
          error instanceof Error ? error.message : 'Unknown error'
        }`
      );
    }
  }

  /**
   * Confirm deletion with "DELETE" phrase
   * Creates a deletion job and starts async execution
   */
  async confirm(
    resourceType: string,
    resourceId: string,
    confirmationToken: string
  ): Promise<DeletionConfirmResponse> {
    try {
      const response = await this.client.post(
        `${this.baseURL}/v1/${resourceType}/${resourceId}:delete-confirm`,
        {
          confirmationToken,
          confirmationPhrase: 'DELETE',
        }
      );
      return response.data.data;
    } catch (error) {
      if (error instanceof Error && error.message.includes('confirmationPhrase')) {
        throw new Error('Confirmation phrase must be exactly "DELETE" (uppercase)');
      }
      throw new Error(
        `Failed to confirm deletion: ${
          error instanceof Error ? error.message : 'Unknown error'
        }`
      );
    }
  }

  /**
   * Get current status of a deletion job
   * Used for progress polling
   */
  async getJobStatus(jobId: string): Promise<DeletionJob> {
    try {
      const response = await this.client.get(`${this.baseURL}/v1/deletion-jobs/${jobId}`);
      return response.data.data;
    } catch (error) {
      if (error instanceof Error && error.message.includes('404')) {
        throw new Error('Deletion job not found (may have expired after 7 days)');
      }
      throw new Error(
        `Failed to get job status: ${
          error instanceof Error ? error.message : 'Unknown error'
        }`
      );
    }
  }

  /**
   * Poll until deletion completes
   * Automatically retries on timeout until completion or timeout
   */
  async pollUntilComplete(
    jobId: string,
    options: {
      interval?: number; // Poll interval in ms (default: 1000)
      timeout?: number; // Total timeout in ms (default: 3600000 = 1 hour)
      onProgress?: (job: DeletionJob) => void; // Called on each poll
    } = {}
  ): Promise<DeletionJob> {
    const {
      interval = 1000,
      timeout = 3600000,
      onProgress,
    } = options;

    const startTime = Date.now();
    let lastError: Error | null = null;

    while (Date.now() - startTime < timeout) {
      try {
        const job = await this.getJobStatus(jobId);
        onProgress?.(job);

        if (
          job.status === 'completed' ||
          job.status === 'failed' ||
          job.status === 'cancelled'
        ) {
          return job;
        }

        // Wait before next poll
        await this.delay(interval);
      } catch (error) {
        lastError = error instanceof Error ? error : new Error(String(error));
        // Continue polling even on error (network blip, etc)
        await this.delay(interval);
      }
    }

    throw new Error(
      `Deletion job ${jobId} timed out after ${timeout}ms${
        lastError ? ` (last error: ${lastError.message})` : ''
      }`
    );
  }

  /**
   * Helper: sleep for N milliseconds
   */
  private delay(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  /**
   * Helper: check if error is recoverable
   */
  static isRecoverable(error: DeletionJobError): boolean {
    return error.recoverable;
  }

  /**
   * Helper: format error for display
   */
  static formatError(error: DeletionJobError): string {
    return `${error.step} failed: ${error.reason}\nRecovery: ${error.recovery}`;
  }

  /**
   * Helper: count errors by type
   */
  static errorStats(errors: DeletionJobError[]): {
    total: number;
    recoverable: number;
    blocking: number;
  } {
    return {
      total: errors.length,
      recoverable: errors.filter(e => e.recoverable).length,
      blocking: errors.filter(e => !e.recoverable).length,
    };
  }
}

// Export singleton instance (configure with your axios client)
let instance: DeletionAPIClient;

export function initDeletionAPI(client: AxiosInstance, baseURL?: string): DeletionAPIClient {
  instance = new DeletionAPIClient(client, baseURL);
  return instance;
}

export function getDeletionAPI(): DeletionAPIClient {
  if (!instance) {
    throw new Error('DeletionAPI not initialized. Call initDeletionAPI first.');
  }
  return instance;
}

export { DeletionAPIClient };
