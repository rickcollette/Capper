package alert

import (
	"fmt"
	"time"

	"capper/internal/metrics"
)

// EventRecord is the minimal event data the evaluator needs.
type EventRecord struct {
	Action    string
	Timestamp string
}

// Manager wraps the alert store and provides rule CRUD + evaluation.
type Manager struct {
	store *Store
}

func NewManager(s *Store) *Manager { return &Manager{store: s} }

func (m *Manager) Create(name, project string, ruleType RuleType, eventAction string,
	windowSecs, threshold int, metricName string) (Rule, error) {
	if name == "" {
		return Rule{}, fmt.Errorf("alert: name is required")
	}
	r := Rule{
		ID:          newID(),
		Name:        name,
		Project:     project,
		Type:        ruleType,
		EventAction: eventAction,
		WindowSecs:  windowSecs,
		Threshold:   threshold,
		MetricName:  metricName,
		Enabled:     true,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.Insert(r); err != nil {
		return Rule{}, fmt.Errorf("alert: store: %w", err)
	}
	return r, nil
}

func (m *Manager) List(project string) ([]Rule, error) { return m.store.List(project) }
func (m *Manager) Get(nameOrID, project string) (Rule, error) {
	return m.store.Get(nameOrID, project)
}
func (m *Manager) Delete(nameOrID, project string) error {
	return m.store.Delete(nameOrID, project)
}

// Evaluate checks all enabled rules for the project against the provided events
// and instance metrics, returning any firing alerts.
func (m *Manager) Evaluate(project string, events []EventRecord, instMetrics []metrics.InstanceMetrics) ([]Firing, error) {
	rules, err := m.store.List(project)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var firing []Firing
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Type {
		case RuleTypeEventCount:
			cutoff := time.Now().Add(-time.Duration(r.WindowSecs) * time.Second).UTC().Format(time.RFC3339)
			var count int64
			for _, e := range events {
				if e.Timestamp < cutoff {
					continue
				}
				if r.EventAction == "" || e.Action == r.EventAction {
					count++
				}
			}
			if int(count) >= r.Threshold {
				firing = append(firing, Firing{
					RuleID: r.ID, RuleName: r.Name,
					Value: count, Threshold: r.Threshold, FiredAt: now,
				})
			}
		case RuleTypeMetricThreshold:
			var maxVal int64
			for _, im := range instMetrics {
				var v int64
				switch r.MetricName {
				case "cpu_micros":
					v = int64(im.CPUUsageUs)
				case "memory_bytes":
					v = int64(im.MemoryBytes)
				case "pid_count":
					v = int64(im.PIDCount)
				}
				if v > maxVal {
					maxVal = v
				}
			}
			if int(maxVal) >= r.Threshold {
				firing = append(firing, Firing{
					RuleID: r.ID, RuleName: r.Name,
					Value: maxVal, Threshold: r.Threshold, FiredAt: now,
				})
			}
		}
	}
	return firing, nil
}
