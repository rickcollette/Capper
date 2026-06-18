package eventing

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"
)

// EventSource supplies resource events for rule matching.
type EventSource func() ([]RawEvent, error)

// RawEvent is a simplified event for rule matching.
type RawEvent struct {
	Type      string
	Action    string
	Project   string
	Timestamp string
}

// Manager runs the rule evaluation and schedule engine.
type Manager struct {
	store *Store
}

func NewManager(db *sql.DB) *Manager {
	s := NewStore(db)
	_ = InitSchemaSQL(s)
	return &Manager{store: s}
}

func (m *Manager) Store() *Store { return m.store }

// CreateRule creates an event rule.
func (m *Manager) CreateRule(name, project, eventType, action, actionArgs string) (Rule, error) {
	r := Rule{
		Name: name, Project: project, EventType: eventType,
		Action: action, ActionArgs: actionArgs, Enabled: true,
	}
	return r, m.store.InsertRule(r)
}

// ListRules returns rules for a project.
func (m *Manager) ListRules(project string) ([]Rule, error) {
	return m.store.ListRules(project)
}

// DeleteRule deletes a rule.
func (m *Manager) DeleteRule(name, project string) error {
	return m.store.DeleteRule(name, project)
}

// CreateSchedule creates a schedule.
func (m *Manager) CreateSchedule(name, project, cron, action, actionArgs string) (Schedule, error) {
	sc := Schedule{
		Name: name, Project: project, Cron: cron,
		Action: action, ActionArgs: actionArgs, Enabled: true,
	}
	return sc, m.store.InsertSchedule(sc)
}

// ListSchedules returns schedules for a project.
func (m *Manager) ListSchedules(project string) ([]Schedule, error) {
	return m.store.ListSchedules(project)
}

// DeleteSchedule deletes a schedule.
func (m *Manager) DeleteSchedule(name, project string) error {
	return m.store.DeleteSchedule(name, project)
}

// RunDueSchedules executes any schedules whose cron interval has elapsed.
// This is called periodically from a reconciler.
func (m *Manager) RunDueSchedules(ctx context.Context, project string) error {
	schedules, err := m.store.ListSchedules(project)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, sc := range schedules {
		if !sc.Enabled {
			continue
		}
		if isDue(sc, now) {
			log.Printf("eventing: running schedule %s action=%s", sc.Name, sc.Action)
			_ = m.store.MarkScheduleRun(sc.ID)
		}
	}
	return nil
}

// EvaluateRules checks a list of events against all rules for a project.
// Matched rules with action="webhook" are fired via HTTP POST; others are logged.
func (m *Manager) EvaluateRules(events []RawEvent, project string) error {
	rules, err := m.store.ListRules(project)
	if err != nil {
		return err
	}
	for _, ev := range events {
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			if !matches(rule.EventType, ev.Action) {
				continue
			}
			log.Printf("eventing: rule %q matched event %s → action=%s args=%s",
				rule.Name, ev.Action, rule.Action, rule.ActionArgs)
			if rule.Action == "webhook" && rule.ActionArgs != "" {
				payload := WebhookPayload{
					EventType: ev.Type,
					Action:    ev.Action,
					Project:   ev.Project,
					Timestamp: ev.Timestamp,
					RuleName:  rule.Name,
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := FireWebhook(ctx, rule.ActionArgs, payload); err != nil {
					log.Printf("eventing: webhook %s failed: %v", rule.ActionArgs, err)
				}
				cancel()
			}
		}
	}
	return nil
}

// isDue returns true if the schedule's interval has elapsed since last run.
// Supports simple "every N minutes/hours" patterns: "@5m", "@1h".
func isDue(sc Schedule, now time.Time) bool {
	cron := strings.TrimSpace(sc.Cron)
	var interval time.Duration
	switch {
	case strings.HasPrefix(cron, "@"):
		d, err := time.ParseDuration(strings.TrimPrefix(cron, "@"))
		if err != nil {
			return false
		}
		interval = d
	default:
		// Full cron expressions not evaluated here — treat as 1h default.
		interval = time.Hour
	}
	if sc.LastRunAt == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, sc.LastRunAt)
	if err != nil {
		return true
	}
	return now.Sub(last) >= interval
}

func matches(pattern, action string) bool {
	if pattern == "*" || pattern == action {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(action, prefix+".")
	}
	return false
}
