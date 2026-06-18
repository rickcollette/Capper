package vpcmover

import "sync"

// JobRegistry tracks cancel functions for running jobs.
type JobRegistry struct {
	mu      sync.Mutex
	cancels map[string]func()
}

// NewJobRegistry creates an empty JobRegistry.
func NewJobRegistry() *JobRegistry {
	return &JobRegistry{cancels: make(map[string]func())}
}

// Register stores the cancel function for jobID.
func (r *JobRegistry) Register(jobID string, cancel func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[jobID] = cancel
}

// Cancel calls the cancel function for jobID if it exists.
func (r *JobRegistry) Cancel(jobID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if fn, ok := r.cancels[jobID]; ok {
		fn()
		delete(r.cancels, jobID)
	}
}

// Remove removes the cancel function for jobID without calling it.
func (r *JobRegistry) Remove(jobID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancels, jobID)
}
