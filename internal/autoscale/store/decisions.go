package autoscalestore

import (
	"database/sql"
	"time"

	"capper/internal/autoscale"
)

// DecisionStore persists autoscale decision audit records.
type DecisionStore struct {
	db *sql.DB
}

func NewDecisionStore(db *sql.DB) *DecisionStore { return &DecisionStore{db: db} }

func (s *DecisionStore) Insert(d autoscale.AutoscaleDecision) error {
	if d.CreatedAt == "" {
		d.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(`
		INSERT INTO autoscale_decisions (
			id, policy_id, group_id, project,
			old_replicas, new_replicas, recommended_replicas,
			decision, reason, metric_name, metric_value, target_value,
			blocked, blocked_reason, created_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.PolicyID, d.GroupID, d.Project,
		d.OldReplicas, d.NewReplicas, d.RecommendedReplicas,
		d.Decision, d.Reason, d.MetricName, d.MetricValue, d.TargetValue,
		boolInt(d.Blocked), d.BlockedReason, d.CreatedAt,
	)
	return err
}

func (s *DecisionStore) ListForGroup(groupID string, limit int) ([]autoscale.AutoscaleDecision, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, policy_id, group_id, project,
		       old_replicas, new_replicas, recommended_replicas,
		       decision, reason, metric_name, metric_value, target_value,
		       blocked, blocked_reason, created_at
		FROM autoscale_decisions
		WHERE group_id=?
		ORDER BY created_at DESC LIMIT ?`, groupID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDecisions(rows)
}

func (s *DecisionStore) ListForPolicy(policyID string, limit int) ([]autoscale.AutoscaleDecision, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, policy_id, group_id, project,
		       old_replicas, new_replicas, recommended_replicas,
		       decision, reason, metric_name, metric_value, target_value,
		       blocked, blocked_reason, created_at
		FROM autoscale_decisions
		WHERE policy_id=?
		ORDER BY created_at DESC LIMIT ?`, policyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDecisions(rows)
}

// ---- helpers ----------------------------------------------------------------

func scanDecision(scan scanFn) (autoscale.AutoscaleDecision, error) {
	var d autoscale.AutoscaleDecision
	var blocked int
	err := scan(
		&d.ID, &d.PolicyID, &d.GroupID, &d.Project,
		&d.OldReplicas, &d.NewReplicas, &d.RecommendedReplicas,
		&d.Decision, &d.Reason, &d.MetricName, &d.MetricValue, &d.TargetValue,
		&blocked, &d.BlockedReason, &d.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return d, autoscale.ErrNotFound
	}
	d.Blocked = blocked != 0
	return d, err
}

func scanDecisions(rows *sql.Rows) ([]autoscale.AutoscaleDecision, error) {
	var out []autoscale.AutoscaleDecision
	for rows.Next() {
		d, err := scanDecision(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
