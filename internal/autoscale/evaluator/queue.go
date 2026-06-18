package evaluator

import (
	"context"
	"fmt"
	"math"

	"capper/internal/autoscale"
)

// QueueDepthFn returns the current depth of a named queue.
type QueueDepthFn func(ctx context.Context, queueName string) (int64, error)

// QueueEvaluator scales worker groups based on queue depth.
//
//	recommendedWorkers = ceil(queue_depth / target_per_worker)
type QueueEvaluator struct {
	depth QueueDepthFn
}

func NewQueueEvaluator(depth QueueDepthFn) *QueueEvaluator {
	return &QueueEvaluator{depth: depth}
}

func (e *QueueEvaluator) Evaluate(
	ctx context.Context,
	policy autoscale.AutoscalePolicy,
	current int,
) (autoscale.ScaleRecommendation, error) {
	rec := autoscale.ScaleRecommendation{
		OldReplicas: current,
		MetricName:  autoscale.MetricQueueDepth,
		TargetValue: policy.TargetValue,
	}

	if !policy.Enabled {
		rec.Decision = autoscale.DecisionDisabled
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, nil
	}

	if policy.QueueName == "" {
		rec.Decision = autoscale.DecisionError
		rec.Reason = "queue policy requires queueName"
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, fmt.Errorf("autoscale: queue policy %q missing queueName", policy.Name)
	}

	depth, err := e.depth(ctx, policy.QueueName)
	if err != nil {
		rec.Decision = autoscale.DecisionError
		rec.Reason = fmt.Sprintf("queue depth query failed: %v", err)
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, err
	}
	rec.MetricValue = float64(depth)

	target := policy.TargetValue
	if target <= 0 {
		target = 50
	}

	var recommended int
	if depth == 0 {
		// Empty queue: scale in to min.
		recommended = policy.MinReplicas
	} else {
		recommended = int(math.Ceil(float64(depth) / target))
	}
	recommended = clamp(recommended, policy.MinReplicas, policy.MaxReplicas)
	rec.RecommendedReplicas = recommended

	switch {
	case recommended > current:
		rec.Decision = autoscale.DecisionScaleOut
		rec.Reason = fmt.Sprintf("queue %q depth=%d target-per-worker=%.0f → scale out %d→%d",
			policy.QueueName, depth, target, current, recommended)
	case recommended < current:
		rec.Decision = autoscale.DecisionScaleIn
		rec.Reason = fmt.Sprintf("queue %q depth=%d target-per-worker=%.0f → scale in %d→%d",
			policy.QueueName, depth, target, current, recommended)
	default:
		rec.Decision = autoscale.DecisionHold
		rec.Reason = fmt.Sprintf("queue %q depth=%d → hold at %d",
			policy.QueueName, depth, current)
	}
	rec.NewReplicas = recommended
	return rec, nil
}
