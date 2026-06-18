package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"capper/internal/autoscale"
)

// ScheduleEvaluator evaluates schedule-based policies.
// It finds the most-recently-triggered cron entry and uses its desiredReplicas.
type ScheduleEvaluator struct{}

func NewScheduleEvaluator() *ScheduleEvaluator { return &ScheduleEvaluator{} }

func (e *ScheduleEvaluator) Evaluate(
	_ context.Context,
	policy autoscale.AutoscalePolicy,
	current int,
) (autoscale.ScaleRecommendation, error) {
	rec := autoscale.ScaleRecommendation{
		OldReplicas: current,
		MetricName:  "schedule",
	}

	if !policy.Enabled {
		rec.Decision = autoscale.DecisionDisabled
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, nil
	}

	if policy.ScheduleJSON == "" || policy.ScheduleJSON == "[]" {
		rec.Decision = autoscale.DecisionHold
		rec.Reason = "no schedule entries"
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, nil
	}

	var entries []autoscale.ScheduleEntry
	if err := json.Unmarshal([]byte(policy.ScheduleJSON), &entries); err != nil {
		rec.Decision = autoscale.DecisionError
		rec.Reason = fmt.Sprintf("invalid schedule JSON: %v", err)
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, err
	}

	now := time.Now()
	// Find the most recently triggered entry.
	var best *autoscale.ScheduleEntry
	var bestTime time.Time
	for i := range entries {
		e := &entries[i]
		t, ok := lastFired(e.Cron, now)
		if !ok {
			continue
		}
		if best == nil || t.After(bestTime) {
			best = e
			bestTime = t
		}
	}
	if best == nil {
		rec.Decision = autoscale.DecisionHold
		rec.Reason = "no matching schedule entry"
		rec.NewReplicas = current
		rec.RecommendedReplicas = current
		return rec, nil
	}

	desired := clamp(best.DesiredReplicas, policy.MinReplicas, policy.MaxReplicas)
	rec.RecommendedReplicas = desired
	switch {
	case desired > current:
		rec.Decision = autoscale.DecisionScaleOut
		rec.Reason = fmt.Sprintf("schedule %q → %d replicas", best.Cron, desired)
	case desired < current:
		rec.Decision = autoscale.DecisionScaleIn
		rec.Reason = fmt.Sprintf("schedule %q → %d replicas", best.Cron, desired)
	default:
		rec.Decision = autoscale.DecisionHold
		rec.Reason = fmt.Sprintf("schedule %q already at %d replicas", best.Cron, desired)
	}
	rec.NewReplicas = desired
	return rec, nil
}

// lastFired returns the most recent time the cron expression fired before now.
// Supports the standard 5-field cron: minute hour dom month dow.
// Returns (zero, false) if the expression cannot be parsed.
func lastFired(cronExpr string, now time.Time) (time.Time, bool) {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return time.Time{}, false
	}

	// Walk backwards minute by minute up to 1 week to find the last match.
	// This is simple and correct for evaluation intervals << 1 week.
	t := now.Truncate(time.Minute)
	limit := t.Add(-7 * 24 * time.Hour)
	for t.After(limit) {
		if matchCron(fields, t) {
			return t, true
		}
		t = t.Add(-time.Minute)
	}
	return time.Time{}, false
}

func matchCron(fields []string, t time.Time) bool {
	return matchField(fields[0], t.Minute(), 0, 59) &&
		matchField(fields[1], t.Hour(), 0, 23) &&
		matchField(fields[2], t.Day(), 1, 31) &&
		matchField(fields[3], int(t.Month()), 1, 12) &&
		matchField(fields[4], int(t.Weekday()), 0, 6)
}

// matchField checks whether val matches a cron field expression.
// Supports: *, N, */step, N-M, N-M/step.
func matchField(field string, val, min, max int) bool {
	for _, part := range strings.Split(field, ",") {
		if matchPart(part, val, min, max) {
			return true
		}
	}
	return false
}

func matchPart(part string, val, min, max int) bool {
	// */step
	if strings.HasPrefix(part, "*/") {
		step, err := strconv.Atoi(part[2:])
		if err != nil || step <= 0 {
			return false
		}
		return (val-min)%step == 0
	}
	// *
	if part == "*" {
		return true
	}
	// N-M/step or N-M
	if strings.Contains(part, "-") {
		rangePart := part
		step := 1
		if idx := strings.Index(part, "/"); idx >= 0 {
			s, err := strconv.Atoi(part[idx+1:])
			if err != nil || s <= 0 {
				return false
			}
			step = s
			rangePart = part[:idx]
		}
		bounds := strings.SplitN(rangePart, "-", 2)
		if len(bounds) != 2 {
			return false
		}
		lo, e1 := strconv.Atoi(bounds[0])
		hi, e2 := strconv.Atoi(bounds[1])
		if e1 != nil || e2 != nil {
			return false
		}
		return val >= lo && val <= hi && (val-lo)%step == 0
	}
	// exact N
	n, err := strconv.Atoi(part)
	return err == nil && n == val
}
