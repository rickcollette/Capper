package evaluator_test

import (
	"context"
	"errors"
	"testing"

	"capper/internal/autoscale"
	"capper/internal/autoscale/evaluator"
)

// fixedMetric returns a MetricQuerier that always returns value v.
func fixedMetric(v float64) evaluator.MetricQuerier {
	return func(_ context.Context, _, _ string) (float64, error) {
		return v, nil
	}
}

func errMetric(msg string) evaluator.MetricQuerier {
	return func(_ context.Context, _, _ string) (float64, error) {
		return 0, errors.New(msg)
	}
}

func basePolicy() autoscale.AutoscalePolicy {
	return autoscale.AutoscalePolicy{
		ID:          "pol-1",
		GroupID:     "grp-1",
		Enabled:     true,
		PolicyType:  autoscale.PolicyTypeTarget,
		MetricName:  autoscale.MetricCPUAvgPercent,
		TargetValue: 70.0,
		MinReplicas: 1,
		MaxReplicas: 10,
	}
}

// ---- TargetEvaluator --------------------------------------------------------

func TestTargetEvaluator_ScaleOut(t *testing.T) {
	// current=2 replicas, cpu=140% target=70 → ceil(2*140/70)=4 → scale out
	ev := evaluator.NewTargetEvaluator(fixedMetric(140.0))
	rec, err := ev.Evaluate(context.Background(), basePolicy(), 2)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.Decision != autoscale.DecisionScaleOut {
		t.Errorf("expected scale-out, got %q", rec.Decision)
	}
	if rec.NewReplicas != 4 {
		t.Errorf("new replicas: got %d, want 4", rec.NewReplicas)
	}
}

func TestTargetEvaluator_ScaleIn(t *testing.T) {
	// current=4, cpu=35 target=70 → ceil(4*35/70)=2 → scale in
	ev := evaluator.NewTargetEvaluator(fixedMetric(35.0))
	rec, err := ev.Evaluate(context.Background(), basePolicy(), 4)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.Decision != autoscale.DecisionScaleIn {
		t.Errorf("expected scale-in, got %q", rec.Decision)
	}
	if rec.NewReplicas != 2 {
		t.Errorf("new replicas: got %d, want 2", rec.NewReplicas)
	}
}

func TestTargetEvaluator_Hold(t *testing.T) {
	// current=2, cpu=70 target=70 → ceil(2*70/70)=2 → hold
	ev := evaluator.NewTargetEvaluator(fixedMetric(70.0))
	rec, err := ev.Evaluate(context.Background(), basePolicy(), 2)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.Decision != autoscale.DecisionHold {
		t.Errorf("expected hold, got %q", rec.Decision)
	}
}

func TestTargetEvaluator_ClampToMin(t *testing.T) {
	// cpu very low → would scale to 0, clamped to MinReplicas=1
	ev := evaluator.NewTargetEvaluator(fixedMetric(1.0))
	p := basePolicy()
	p.MinReplicas = 1
	rec, err := ev.Evaluate(context.Background(), p, 4)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.NewReplicas < 1 {
		t.Errorf("new replicas below min: got %d", rec.NewReplicas)
	}
}

func TestTargetEvaluator_ClampToMax(t *testing.T) {
	// cpu=10000 → would scale to huge number, clamped to MaxReplicas=10
	ev := evaluator.NewTargetEvaluator(fixedMetric(10000.0))
	rec, err := ev.Evaluate(context.Background(), basePolicy(), 2)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.NewReplicas > 10 {
		t.Errorf("new replicas above max: got %d", rec.NewReplicas)
	}
}

func TestTargetEvaluator_Disabled(t *testing.T) {
	ev := evaluator.NewTargetEvaluator(fixedMetric(200.0))
	p := basePolicy()
	p.Enabled = false
	rec, err := ev.Evaluate(context.Background(), p, 3)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.Decision != autoscale.DecisionDisabled {
		t.Errorf("expected disabled, got %q", rec.Decision)
	}
	if rec.NewReplicas != 3 {
		t.Errorf("replicas should be unchanged: got %d, want 3", rec.NewReplicas)
	}
}

func TestTargetEvaluator_MetricError(t *testing.T) {
	ev := evaluator.NewTargetEvaluator(errMetric("prometheus unreachable"))
	_, err := ev.Evaluate(context.Background(), basePolicy(), 2)
	if err == nil {
		t.Error("expected error when metric query fails")
	}
}

// ---- ThresholdEvaluator -----------------------------------------------------

func TestThresholdEvaluator_ScaleOut(t *testing.T) {
	ev := evaluator.NewThresholdEvaluator(fixedMetric(90.0))
	p := basePolicy()
	p.PolicyType = autoscale.PolicyTypeThreshold
	p.ScaleOutThreshold = 80.0
	p.ScaleInThreshold = 20.0
	p.ScaleOutStep = 2
	rec, err := ev.Evaluate(context.Background(), p, 3)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.Decision != autoscale.DecisionScaleOut {
		t.Errorf("expected scale-out, got %q", rec.Decision)
	}
	if rec.NewReplicas != 5 {
		t.Errorf("new replicas: got %d, want 5 (3+2)", rec.NewReplicas)
	}
}

func TestThresholdEvaluator_ScaleIn(t *testing.T) {
	ev := evaluator.NewThresholdEvaluator(fixedMetric(10.0))
	p := basePolicy()
	p.PolicyType = autoscale.PolicyTypeThreshold
	p.ScaleOutThreshold = 80.0
	p.ScaleInThreshold = 20.0
	p.ScaleInStep = 1
	rec, err := ev.Evaluate(context.Background(), p, 4)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.Decision != autoscale.DecisionScaleIn {
		t.Errorf("expected scale-in, got %q", rec.Decision)
	}
	if rec.NewReplicas != 3 {
		t.Errorf("new replicas: got %d, want 3 (4-1)", rec.NewReplicas)
	}
}

func TestThresholdEvaluator_Hold(t *testing.T) {
	// metric between thresholds → hold
	ev := evaluator.NewThresholdEvaluator(fixedMetric(50.0))
	p := basePolicy()
	p.PolicyType = autoscale.PolicyTypeThreshold
	p.ScaleOutThreshold = 80.0
	p.ScaleInThreshold = 20.0
	rec, err := ev.Evaluate(context.Background(), p, 3)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.Decision != autoscale.DecisionHold {
		t.Errorf("expected hold, got %q", rec.Decision)
	}
}

func TestThresholdEvaluator_Disabled(t *testing.T) {
	ev := evaluator.NewThresholdEvaluator(fixedMetric(90.0))
	p := basePolicy()
	p.Enabled = false
	p.ScaleOutThreshold = 80.0
	rec, err := ev.Evaluate(context.Background(), p, 2)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.Decision != autoscale.DecisionDisabled {
		t.Errorf("expected disabled, got %q", rec.Decision)
	}
}
