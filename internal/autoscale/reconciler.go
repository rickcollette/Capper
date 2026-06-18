// Package autoscale implements desired-state autoscaling for Capper instance groups.
// The Reconciler is a normal reconciler loop: it loads all enabled policies,
// evaluates them, and applies the resulting desired replica count to the group.
package autoscale

import (
	"context"
	"fmt"
	"log"
	"time"
)

// GroupScaler can read and update a group's desired replica count.
type GroupScaler interface {
	CurrentReplicas(ctx context.Context, groupID string) (int, error)
	SetDesiredReplicas(ctx context.Context, groupID string, desired int) error
}

// Evaluator turns a policy and current count into a recommendation.
type Evaluator interface {
	Evaluate(ctx context.Context, policy AutoscalePolicy, current int) (ScaleRecommendation, error)
}

// EvaluatorFunc resolves the right Evaluator for a policy type.
type EvaluatorFunc func(policyType string) (Evaluator, error)

// PolicyStore is the minimal interface the Reconciler needs from the policy store.
type PolicyStore interface {
	ListEnabled() ([]AutoscalePolicy, error)
	GetByID(id string) (AutoscalePolicy, error)
	UpdateLastScale(id, decision string) error
}

// DecisionRecorder is the minimal interface for writing decision audit records.
type DecisionRecorder interface {
	Insert(d AutoscaleDecision) error
}

// ReconcilerStore groups the two store interfaces the Reconciler needs.
type ReconcilerStore struct {
	Policies  PolicyStore
	Decisions DecisionRecorder
}

// Reconciler is the top-level autoscaler. Plug it into the daemon's reconcile loop.
type Reconciler struct {
	store   ReconcilerStore
	resolve EvaluatorFunc
	newID   func() string
}

// NewReconciler creates a Reconciler.
// resolve must return an Evaluator for each policy type (target, threshold, schedule, queue).
func NewReconciler(store ReconcilerStore, resolve EvaluatorFunc) *Reconciler {
	return &Reconciler{
		store:   store,
		resolve: resolve,
		newID:   defaultID,
	}
}

func defaultID() string {
	return fmt.Sprintf("asd_%d", time.Now().UnixNano())
}

// Name satisfies the reconciler interface.
func (r *Reconciler) Name() string { return "autoscale" }

// Reconcile evaluates all enabled policies once.
func (r *Reconciler) Reconcile(ctx context.Context, scaler GroupScaler) error {
	policies, err := r.store.Policies.ListEnabled()
	if err != nil {
		return fmt.Errorf("autoscale reconcile: list policies: %w", err)
	}

	for i := range policies {
		p := policies[i]
		if err := r.evaluateOne(ctx, p, scaler); err != nil {
			log.Printf("[autoscale] policy %q: %v", p.Name, err)
		}
	}
	return nil
}

// EvaluatePolicy evaluates a single policy by ID and records the decision.
// Returns the recommendation for callers that want to inspect or dry-run.
func (r *Reconciler) EvaluatePolicy(ctx context.Context, policyID string, scaler GroupScaler) (ScaleRecommendation, error) {
	p, err := r.store.Policies.GetByID(policyID)
	if err != nil {
		return ScaleRecommendation{}, fmt.Errorf("autoscale: policy %q not found", policyID)
	}
	return r.evaluate(ctx, p, scaler, true)
}

func (r *Reconciler) evaluateOne(ctx context.Context, p AutoscalePolicy, scaler GroupScaler) error {
	_, err := r.evaluate(ctx, p, scaler, true)
	return err
}

func (r *Reconciler) evaluate(
	ctx context.Context,
	p AutoscalePolicy,
	scaler GroupScaler,
	applyChange bool,
) (ScaleRecommendation, error) {
	ev, err := r.resolve(p.PolicyType)
	if err != nil {
		return ScaleRecommendation{}, err
	}

	current, err := scaler.CurrentReplicas(ctx, p.GroupID)
	if err != nil {
		return ScaleRecommendation{}, fmt.Errorf("autoscale: get current replicas for group %q: %w", p.GroupID, err)
	}

	rec, evalErr := ev.Evaluate(ctx, p, current)
	if evalErr != nil {
		r.recordDecision(p, rec, current)
		return rec, evalErr
	}

	// Apply cooldown checks.
	if rec.Decision == DecisionScaleOut || rec.Decision == DecisionScaleIn {
		blocked, reason := r.cooldownBlocked(p, rec.Decision)
		if blocked {
			rec.Blocked = true
			rec.BlockedReason = reason
			rec.Decision = DecisionBlocked
			rec.NewReplicas = current
		}
	}

	// Apply the change.
	if applyChange && !rec.Blocked && rec.NewReplicas != current {
		if err := scaler.SetDesiredReplicas(ctx, p.GroupID, rec.NewReplicas); err != nil {
			rec.Blocked = true
			rec.BlockedReason = err.Error()
			rec.Decision = DecisionBlocked
			rec.NewReplicas = current
		} else {
			_ = r.store.Policies.UpdateLastScale(p.ID, rec.Decision)
		}
	}

	r.recordDecision(p, rec, current)
	return rec, nil
}

func (r *Reconciler) cooldownBlocked(p AutoscalePolicy, decision string) (bool, string) {
	if p.LastScaleAt == "" {
		return false, ""
	}
	last, err := time.Parse(time.RFC3339, p.LastScaleAt)
	if err != nil {
		return false, ""
	}
	var cooldown int
	if decision == DecisionScaleOut {
		cooldown = p.ScaleOutCooldownSecs
		if cooldown <= 0 {
			cooldown = 60
		}
	} else {
		cooldown = p.ScaleInCooldownSecs
		if cooldown <= 0 {
			cooldown = 300
		}
	}
	elapsed := time.Since(last)
	needed := time.Duration(cooldown) * time.Second
	if elapsed < needed {
		return true, fmt.Sprintf("%s cooldown: %s remaining",
			decision, (needed - elapsed).Round(time.Second))
	}
	return false, ""
}

func (r *Reconciler) recordDecision(p AutoscalePolicy, rec ScaleRecommendation, oldReplicas int) {
	d := AutoscaleDecision{
		ID:                  r.newID(),
		PolicyID:            p.ID,
		GroupID:             p.GroupID,
		Project:             p.Project,
		OldReplicas:         oldReplicas,
		NewReplicas:         rec.NewReplicas,
		RecommendedReplicas: rec.RecommendedReplicas,
		Decision:            rec.Decision,
		Reason:              rec.Reason,
		MetricName:          rec.MetricName,
		MetricValue:         rec.MetricValue,
		TargetValue:         rec.TargetValue,
		Blocked:             rec.Blocked,
		BlockedReason:       rec.BlockedReason,
		CreatedAt:           time.Now().UTC().Format(time.RFC3339),
	}
	_ = r.store.Decisions.Insert(d)
}
