// Package evaluator contains scaling algorithm implementations.
package evaluator

import (
	"context"
	"fmt"
	"math"

	"capper/internal/autoscale"
)

// MetricQuerier returns the current value of a named metric for a group.
type MetricQuerier func(ctx context.Context, groupID, metricName string) (float64, error)

// TargetEvaluator implements target-tracking scaling:
//
//	recommendedReplicas = ceil(currentReplicas * currentMetric / targetMetric)
type TargetEvaluator struct {
	query MetricQuerier
}

func NewTargetEvaluator(query MetricQuerier) *TargetEvaluator {
	return &TargetEvaluator{query: query}
}

func (e *TargetEvaluator) Evaluate(
	ctx context.Context,
	policy autoscale.AutoscalePolicy,
	current int,
) (autoscale.ScaleRecommendation, error) {
	rec := autoscale.ScaleRecommendation{
		OldReplicas: current,
		MetricName:  policy.MetricName,
		TargetValue: policy.TargetValue,
	}

	if !policy.Enabled {
		rec.Decision = autoscale.DecisionDisabled
		rec.Reason = "policy disabled"
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, nil
	}

	val, err := e.query(ctx, policy.GroupID, policy.MetricName)
	if err != nil {
		rec.Decision = autoscale.DecisionError
		rec.Reason = fmt.Sprintf("metric query failed: %v", err)
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, err
	}
	rec.MetricValue = val

	if policy.TargetValue <= 0 {
		rec.Decision = autoscale.DecisionHold
		rec.Reason = "target value is zero"
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, nil
	}

	// Compute recommended replicas.
	recommended := int(math.Ceil(float64(current) * val / policy.TargetValue))
	if current == 0 && val > 0 {
		recommended = policy.MinReplicas
	}

	// Clamp to [min, max].
	recommended = clamp(recommended, policy.MinReplicas, policy.MaxReplicas)
	rec.RecommendedReplicas = recommended

	switch {
	case recommended > current:
		rec.Decision = autoscale.DecisionScaleOut
		rec.Reason = fmt.Sprintf("%s=%.2f above target %.2f → scale out from %d to %d",
			policy.MetricName, val, policy.TargetValue, current, recommended)
	case recommended < current:
		rec.Decision = autoscale.DecisionScaleIn
		rec.Reason = fmt.Sprintf("%s=%.2f below target %.2f → scale in from %d to %d",
			policy.MetricName, val, policy.TargetValue, current, recommended)
	default:
		rec.Decision = autoscale.DecisionHold
		rec.Reason = fmt.Sprintf("%s=%.2f near target %.2f → no change",
			policy.MetricName, val, policy.TargetValue)
	}
	rec.NewReplicas = recommended
	return rec, nil
}

// ThresholdEvaluator scales when a metric crosses a threshold.
type ThresholdEvaluator struct {
	query MetricQuerier
}

func NewThresholdEvaluator(query MetricQuerier) *ThresholdEvaluator {
	return &ThresholdEvaluator{query: query}
}

func (e *ThresholdEvaluator) Evaluate(
	ctx context.Context,
	policy autoscale.AutoscalePolicy,
	current int,
) (autoscale.ScaleRecommendation, error) {
	rec := autoscale.ScaleRecommendation{
		OldReplicas: current,
		MetricName:  policy.MetricName,
	}

	if !policy.Enabled {
		rec.Decision = autoscale.DecisionDisabled
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, nil
	}

	val, err := e.query(ctx, policy.GroupID, policy.MetricName)
	if err != nil {
		rec.Decision = autoscale.DecisionError
		rec.Reason = err.Error()
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, err
	}
	rec.MetricValue = val

	step := policy.ScaleOutStep
	if step <= 0 {
		step = 1
	}
	inStep := policy.ScaleInStep
	if inStep <= 0 {
		inStep = 1
	}

	var recommended int
	switch {
	case policy.ScaleOutThreshold > 0 && val >= policy.ScaleOutThreshold:
		recommended = clamp(current+step, policy.MinReplicas, policy.MaxReplicas)
		rec.Decision = autoscale.DecisionScaleOut
		rec.Reason = fmt.Sprintf("%s=%.2f >= out-threshold %.2f → +%d",
			policy.MetricName, val, policy.ScaleOutThreshold, step)
	case policy.ScaleInThreshold > 0 && val <= policy.ScaleInThreshold:
		recommended = clamp(current-inStep, policy.MinReplicas, policy.MaxReplicas)
		rec.Decision = autoscale.DecisionScaleIn
		rec.Reason = fmt.Sprintf("%s=%.2f <= in-threshold %.2f → -%d",
			policy.MetricName, val, policy.ScaleInThreshold, inStep)
	default:
		recommended = current
		rec.Decision = autoscale.DecisionHold
		rec.Reason = fmt.Sprintf("%s=%.2f within thresholds [%.2f, %.2f]",
			policy.MetricName, val, policy.ScaleInThreshold, policy.ScaleOutThreshold)
	}
	rec.RecommendedReplicas = recommended
	rec.NewReplicas = recommended
	rec.TargetValue = policy.ScaleOutThreshold
	return rec, nil
}

// ---- helpers ----------------------------------------------------------------

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if max > 0 && v > max {
		return max
	}
	return v
}
