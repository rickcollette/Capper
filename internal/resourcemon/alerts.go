package resourcemon

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ---- alert rules -----------------------------------------------------------

// CreateAlertRule inserts a new alert rule.
func (s *Store) CreateAlertRule(r AlertRule) (AlertRule, error) {
	if r.ID == "" {
		r.ID = "arule_" + uuid.NewString()
	}
	ts := now()
	r.CreatedAt, r.UpdatedAt = ts, ts
	if r.Condition == "" {
		r.Condition = "gt"
	}
	if r.Severity == "" {
		r.Severity = "warning"
	}
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(`INSERT INTO rmon_alert_rules
		(id, name, project, resource_type, metric_name, condition, threshold, duration_seconds,
		 severity, enabled, notification_target, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Name, r.Project, r.ResourceType, r.MetricName, r.Condition, r.Threshold,
		r.DurationSeconds, r.Severity, enabled, r.NotificationTarget, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return AlertRule{}, err
	}
	return r, nil
}

// ListAlertRules returns all alert rules (optionally scoped to a project).
func (s *Store) ListAlertRules(project string) ([]AlertRule, error) {
	q := `SELECT id, name, project, resource_type, metric_name, condition, threshold,
		duration_seconds, severity, enabled, notification_target, created_at, updated_at
		FROM rmon_alert_rules`
	var args []any
	if project != "" {
		q += " WHERE project=?"
		args = append(args, project)
	}
	q += " ORDER BY name"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertRule
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetAlertRule returns a rule by ID.
func (s *Store) GetAlertRule(id string) (AlertRule, error) {
	row := s.db.QueryRow(`SELECT id, name, project, resource_type, metric_name, condition, threshold,
		duration_seconds, severity, enabled, notification_target, created_at, updated_at
		FROM rmon_alert_rules WHERE id=?`, id)
	return scanRule(row)
}

func scanRule(row interface{ Scan(...any) error }) (AlertRule, error) {
	var r AlertRule
	var enabled int
	if err := row.Scan(&r.ID, &r.Name, &r.Project, &r.ResourceType, &r.MetricName, &r.Condition,
		&r.Threshold, &r.DurationSeconds, &r.Severity, &enabled, &r.NotificationTarget,
		&r.CreatedAt, &r.UpdatedAt); err != nil {
		return AlertRule{}, err
	}
	r.Enabled = enabled == 1
	return r, nil
}

// UpdateAlertRule applies the provided field changes to a rule.
func (s *Store) UpdateAlertRule(id string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	allowed := map[string]bool{
		"name": true, "resource_type": true, "metric_name": true, "condition": true,
		"threshold": true, "duration_seconds": true, "severity": true, "enabled": true,
		"notification_target": true,
	}
	var sets []string
	var args []any
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		sets = append(sets, k+"=?")
		args = append(args, v)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at=?")
	args = append(args, now(), id)
	_, err := s.db.Exec(`UPDATE rmon_alert_rules SET `+strings.Join(sets, ", ")+` WHERE id=?`, args...)
	return err
}

// DeleteAlertRule removes a rule.
func (s *Store) DeleteAlertRule(id string) error {
	_, err := s.db.Exec(`DELETE FROM rmon_alert_rules WHERE id=?`, id)
	return err
}

// ---- alerts ----------------------------------------------------------------

// OpenAlert creates a new open alert. If an open alert already exists for the
// same rule+resource, it is returned unchanged (dedupe).
func (s *Store) OpenAlert(a Alert) (Alert, error) {
	if a.RuleID != "" && a.ResourceID != "" {
		var existing string
		err := s.db.QueryRow(`SELECT id FROM rmon_alerts
			WHERE rule_id=? AND resource_id=? AND status='open' LIMIT 1`, a.RuleID, a.ResourceID).Scan(&existing)
		if err == nil && existing != "" {
			return s.GetAlert(existing)
		}
	}
	if a.ID == "" {
		a.ID = "alert_" + uuid.NewString()
	}
	if a.OpenedAt == "" {
		a.OpenedAt = now()
	}
	if a.Status == "" {
		a.Status = "open"
	}
	_, err := s.db.Exec(`INSERT INTO rmon_alerts
		(id, rule_id, project, resource_type, resource_id, severity, status, title, message, opened_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.RuleID, a.Project, a.ResourceType, a.ResourceID, a.Severity, a.Status,
		a.Title, a.Message, a.OpenedAt)
	if err != nil {
		return Alert{}, err
	}
	return a, nil
}

// GetAlert returns an alert by ID.
func (s *Store) GetAlert(id string) (Alert, error) {
	row := s.db.QueryRow(`SELECT id, rule_id, project, resource_type, resource_id, severity, status,
		title, message, opened_at, acknowledged_at, resolved_at FROM rmon_alerts WHERE id=?`, id)
	return scanAlert(row)
}

// ListAlerts returns alerts, optionally filtered by status and/or project.
func (s *Store) ListAlerts(status, project string, limit int) ([]Alert, error) {
	if limit <= 0 {
		limit = 200
	}
	conds := []string{"1=1"}
	args := []any{}
	if status != "" {
		conds = append(conds, "status=?")
		args = append(args, status)
	}
	if project != "" {
		conds = append(conds, "project=?")
		args = append(args, project)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT id, rule_id, project, resource_type, resource_id, severity, status,
		title, message, opened_at, acknowledged_at, resolved_at FROM rmon_alerts
		WHERE `+strings.Join(conds, " AND ")+` ORDER BY opened_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanAlert(row interface{ Scan(...any) error }) (Alert, error) {
	var a Alert
	err := row.Scan(&a.ID, &a.RuleID, &a.Project, &a.ResourceType, &a.ResourceID, &a.Severity,
		&a.Status, &a.Title, &a.Message, &a.OpenedAt, &a.AcknowledgedAt, &a.ResolvedAt)
	return a, err
}

// AckAlert marks an alert acknowledged.
func (s *Store) AckAlert(id string) error {
	_, err := s.db.Exec(`UPDATE rmon_alerts SET status='acknowledged', acknowledged_at=?
		WHERE id=? AND status='open'`, now(), id)
	return err
}

// ResolveAlert marks an alert resolved.
func (s *Store) ResolveAlert(id string) error {
	_, err := s.db.Exec(`UPDATE rmon_alerts SET status='resolved', resolved_at=?
		WHERE id=? AND status!='resolved'`, now(), id)
	return err
}

// evalCondition reports whether value breaches threshold under condition.
func evalCondition(condition string, value, threshold float64) bool {
	switch condition {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	case "ne":
		return value != threshold
	default:
		return false
	}
}

// EvaluateRules checks the latest sample of each enabled rule's metric for the
// given resource and opens alerts for any breach. Returns the alerts opened.
func (s *Store) EvaluateRules(resourceType, resourceID string) ([]Alert, error) {
	rules, err := s.ListAlertRules("")
	if err != nil {
		return nil, err
	}
	var opened []Alert
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.ResourceType != "" && rule.ResourceType != resourceType {
			continue
		}
		latest, ok := s.LatestSample(resourceType, resourceID, rule.MetricName)
		if !ok {
			continue
		}
		if !evalCondition(rule.Condition, latest.Value, rule.Threshold) {
			continue
		}
		a, err := s.OpenAlert(Alert{
			RuleID: rule.ID, Project: rule.Project, ResourceType: resourceType, ResourceID: resourceID,
			Severity: rule.Severity, Title: rule.Name,
			Message: fmt.Sprintf("%s %s %.2f (value %.2f)", rule.MetricName, rule.Condition, rule.Threshold, latest.Value),
		})
		if err != nil {
			return opened, err
		}
		opened = append(opened, a)
	}
	return opened, nil
}

var _ = sql.ErrNoRows
