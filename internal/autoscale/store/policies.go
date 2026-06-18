package autoscalestore

import (
	"database/sql"
	"fmt"
	"time"

	"capper/internal/autoscale"
)

// PolicyStore handles persistence for autoscale policies.
type PolicyStore struct {
	db *sql.DB
}

func NewPolicyStore(db *sql.DB) *PolicyStore { return &PolicyStore{db: db} }

func (s *PolicyStore) Insert(p autoscale.AutoscalePolicy) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := s.db.Exec(`
		INSERT INTO autoscale_policies (
			id, project, name, group_id, enabled,
			policy_type, metric_name, metric_scope, queue_name,
			target_value, scale_out_threshold, scale_in_threshold,
			min_replicas, max_replicas, scale_out_step, scale_in_step,
			scale_out_cooldown_seconds, scale_in_cooldown_seconds,
			evaluation_window_seconds, stabilization_window_secs,
			schedule_json, last_scale_at, last_decision,
			created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Project, p.Name, p.GroupID, boolInt(p.Enabled),
		p.PolicyType, p.MetricName, p.MetricScope, p.QueueName,
		p.TargetValue, p.ScaleOutThreshold, p.ScaleInThreshold,
		p.MinReplicas, p.MaxReplicas, p.ScaleOutStep, p.ScaleInStep,
		p.ScaleOutCooldownSecs, p.ScaleInCooldownSecs,
		p.EvalWindowSecs, p.StabWindowSecs,
		p.ScheduleJSON, p.LastScaleAt, p.LastDecision,
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *PolicyStore) Get(nameOrID, project string) (autoscale.AutoscalePolicy, error) {
	row := s.db.QueryRow(`
		SELECT id, project, name, group_id, enabled,
		       policy_type, metric_name, metric_scope, queue_name,
		       target_value, scale_out_threshold, scale_in_threshold,
		       min_replicas, max_replicas, scale_out_step, scale_in_step,
		       scale_out_cooldown_seconds, scale_in_cooldown_seconds,
		       evaluation_window_seconds, stabilization_window_secs,
		       schedule_json, last_scale_at, last_decision,
		       created_at, updated_at
		FROM autoscale_policies
		WHERE (id=? OR name=?) AND project=?`,
		nameOrID, nameOrID, project,
	)
	return scanPolicy(row.Scan)
}

func (s *PolicyStore) GetByID(id string) (autoscale.AutoscalePolicy, error) {
	row := s.db.QueryRow(`
		SELECT id, project, name, group_id, enabled,
		       policy_type, metric_name, metric_scope, queue_name,
		       target_value, scale_out_threshold, scale_in_threshold,
		       min_replicas, max_replicas, scale_out_step, scale_in_step,
		       scale_out_cooldown_seconds, scale_in_cooldown_seconds,
		       evaluation_window_seconds, stabilization_window_secs,
		       schedule_json, last_scale_at, last_decision,
		       created_at, updated_at
		FROM autoscale_policies WHERE id=?`, id)
	return scanPolicy(row.Scan)
}

func (s *PolicyStore) List(project string) ([]autoscale.AutoscalePolicy, error) {
	rows, err := s.db.Query(`
		SELECT id, project, name, group_id, enabled,
		       policy_type, metric_name, metric_scope, queue_name,
		       target_value, scale_out_threshold, scale_in_threshold,
		       min_replicas, max_replicas, scale_out_step, scale_in_step,
		       scale_out_cooldown_seconds, scale_in_cooldown_seconds,
		       evaluation_window_seconds, stabilization_window_secs,
		       schedule_json, last_scale_at, last_decision,
		       created_at, updated_at
		FROM autoscale_policies WHERE project=? ORDER BY name`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPolicies(rows)
}

func (s *PolicyStore) ListEnabled() ([]autoscale.AutoscalePolicy, error) {
	rows, err := s.db.Query(`
		SELECT id, project, name, group_id, enabled,
		       policy_type, metric_name, metric_scope, queue_name,
		       target_value, scale_out_threshold, scale_in_threshold,
		       min_replicas, max_replicas, scale_out_step, scale_in_step,
		       scale_out_cooldown_seconds, scale_in_cooldown_seconds,
		       evaluation_window_seconds, stabilization_window_secs,
		       schedule_json, last_scale_at, last_decision,
		       created_at, updated_at
		FROM autoscale_policies WHERE enabled=1 ORDER BY project, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPolicies(rows)
}

func (s *PolicyStore) ForGroup(groupID string) ([]autoscale.AutoscalePolicy, error) {
	rows, err := s.db.Query(`
		SELECT id, project, name, group_id, enabled,
		       policy_type, metric_name, metric_scope, queue_name,
		       target_value, scale_out_threshold, scale_in_threshold,
		       min_replicas, max_replicas, scale_out_step, scale_in_step,
		       scale_out_cooldown_seconds, scale_in_cooldown_seconds,
		       evaluation_window_seconds, stabilization_window_secs,
		       schedule_json, last_scale_at, last_decision,
		       created_at, updated_at
		FROM autoscale_policies WHERE group_id=? ORDER BY name`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPolicies(rows)
}

func (s *PolicyStore) Update(p autoscale.AutoscalePolicy) error {
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE autoscale_policies SET
			enabled=?, policy_type=?, metric_name=?, metric_scope=?, queue_name=?,
			target_value=?, scale_out_threshold=?, scale_in_threshold=?,
			min_replicas=?, max_replicas=?, scale_out_step=?, scale_in_step=?,
			scale_out_cooldown_seconds=?, scale_in_cooldown_seconds=?,
			evaluation_window_seconds=?, stabilization_window_secs=?,
			schedule_json=?, last_scale_at=?, last_decision=?, updated_at=?
		WHERE id=?`,
		boolInt(p.Enabled), p.PolicyType, p.MetricName, p.MetricScope, p.QueueName,
		p.TargetValue, p.ScaleOutThreshold, p.ScaleInThreshold,
		p.MinReplicas, p.MaxReplicas, p.ScaleOutStep, p.ScaleInStep,
		p.ScaleOutCooldownSecs, p.ScaleInCooldownSecs,
		p.EvalWindowSecs, p.StabWindowSecs,
		p.ScheduleJSON, p.LastScaleAt, p.LastDecision, p.UpdatedAt,
		p.ID,
	)
	return err
}

func (s *PolicyStore) UpdateLastScale(id, decision string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE autoscale_policies SET last_scale_at=?, last_decision=?, updated_at=? WHERE id=?`,
		now, decision, now, id,
	)
	return err
}

func (s *PolicyStore) Delete(nameOrID, project string) error {
	res, err := s.db.Exec(
		`DELETE FROM autoscale_policies WHERE (id=? OR name=?) AND project=?`,
		nameOrID, nameOrID, project,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("autoscale: policy %q not found", nameOrID)
	}
	return nil
}

// ---- helpers ----------------------------------------------------------------

type scanFn func(dest ...any) error

func scanPolicy(scan scanFn) (autoscale.AutoscalePolicy, error) {
	var p autoscale.AutoscalePolicy
	var enabled int
	err := scan(
		&p.ID, &p.Project, &p.Name, &p.GroupID, &enabled,
		&p.PolicyType, &p.MetricName, &p.MetricScope, &p.QueueName,
		&p.TargetValue, &p.ScaleOutThreshold, &p.ScaleInThreshold,
		&p.MinReplicas, &p.MaxReplicas, &p.ScaleOutStep, &p.ScaleInStep,
		&p.ScaleOutCooldownSecs, &p.ScaleInCooldownSecs,
		&p.EvalWindowSecs, &p.StabWindowSecs,
		&p.ScheduleJSON, &p.LastScaleAt, &p.LastDecision,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return p, autoscale.ErrNotFound
	}
	p.Enabled = enabled != 0
	return p, err
}

func scanPolicies(rows *sql.Rows) ([]autoscale.AutoscalePolicy, error) {
	var out []autoscale.AutoscalePolicy
	for rows.Next() {
		p, err := scanPolicy(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
