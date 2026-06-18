package control

import (
	"context"
	"log"
	"time"
)

// Reconciler is implemented by anything that can be driven by the
// ReconcilerLoop. Each call to Reconcile should be idempotent.
type Reconciler interface {
	Name() string
	Reconcile(ctx context.Context) error
}

// ReconcilerLoop runs a set of Reconcilers on a fixed interval until the
// context is cancelled.
type ReconcilerLoop struct {
	Reconcilers []Reconciler
	Interval    time.Duration
}

// NewReconcilerLoop returns a loop with the given interval.
func NewReconcilerLoop(interval time.Duration) *ReconcilerLoop {
	return &ReconcilerLoop{Interval: interval}
}

// Register adds r to the loop. Must be called before Run.
func (l *ReconcilerLoop) Register(r Reconciler) {
	l.Reconcilers = append(l.Reconcilers, r)
}

// Run blocks until ctx is cancelled, invoking each reconciler every Interval.
// Errors are logged but do not stop the loop.
func (l *ReconcilerLoop) Run(ctx context.Context) {
	interval := l.Interval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run all reconcilers once immediately before the first tick.
	l.runAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.runAll(ctx)
		}
	}
}

func (l *ReconcilerLoop) runAll(ctx context.Context) {
	for _, r := range l.Reconcilers {
		if err := r.Reconcile(ctx); err != nil {
			log.Printf("reconciler %s: %v", r.Name(), err)
		}
	}
}
