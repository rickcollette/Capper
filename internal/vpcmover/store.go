package vpcmover

import (
	"database/sql"
	"fmt"
	"time"
)

// Store handles all database operations for VPC mobility.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by an already-initialised database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InitSchema creates all vpcmover tables. Safe to call multiple times.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS vpc_mobility_plans (
			id                  TEXT PRIMARY KEY,
			org_id              TEXT NOT NULL,
			account_id          TEXT NOT NULL,
			project_id          TEXT NOT NULL DEFAULT '',
			source_vpc_id        TEXT NOT NULL,
			destination_vpc_id   TEXT NOT NULL DEFAULT '',
			operation            TEXT NOT NULL,
			strategy             TEXT NOT NULL DEFAULT '',
			target_realm_id      TEXT NOT NULL DEFAULT '',
			target_region_id     TEXT NOT NULL DEFAULT '',
			target_zone_id       TEXT NOT NULL DEFAULT '',
			status               TEXT NOT NULL DEFAULT 'draft',
			include_json         TEXT NOT NULL DEFAULT '[]',
			exclude_json         TEXT NOT NULL DEFAULT '[]',
			options_json         TEXT NOT NULL DEFAULT '{}',
			inventory_json       TEXT NOT NULL DEFAULT '{}',
			plan_json            TEXT NOT NULL DEFAULT '{}',
			warnings_json        TEXT NOT NULL DEFAULT '[]',
			errors_json          TEXT NOT NULL DEFAULT '[]',
			created_by           TEXT NOT NULL DEFAULT '',
			created_at           TEXT NOT NULL,
			updated_at           TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS vpc_mobility_jobs (
			id                  TEXT PRIMARY KEY,
			plan_id             TEXT NOT NULL,
			org_id              TEXT NOT NULL,
			account_id          TEXT NOT NULL,
			source_vpc_id        TEXT NOT NULL,
			destination_vpc_id   TEXT NOT NULL DEFAULT '',
			operation            TEXT NOT NULL,
			status               TEXT NOT NULL DEFAULT 'queued',
			current_step         TEXT NOT NULL DEFAULT '',
			progress_percent     INTEGER NOT NULL DEFAULT 0,
			rollback_available   INTEGER NOT NULL DEFAULT 0,
			rollback_expires_at  TEXT NOT NULL DEFAULT '',
			started_at           TEXT NOT NULL DEFAULT '',
			finished_at          TEXT NOT NULL DEFAULT '',
			error_message        TEXT NOT NULL DEFAULT '',
			created_by           TEXT NOT NULL DEFAULT '',
			created_at           TEXT NOT NULL,
			updated_at           TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS vpc_mobility_steps (
			id              TEXT PRIMARY KEY,
			job_id          TEXT NOT NULL,
			step_order      INTEGER NOT NULL,
			name            TEXT NOT NULL,
			status          TEXT NOT NULL DEFAULT 'pending',
			started_at      TEXT NOT NULL DEFAULT '',
			finished_at     TEXT NOT NULL DEFAULT '',
			input_json      TEXT NOT NULL DEFAULT '{}',
			output_json     TEXT NOT NULL DEFAULT '{}',
			error_message   TEXT NOT NULL DEFAULT '',
			retry_count     INTEGER NOT NULL DEFAULT 0,
			created_at      TEXT NOT NULL,
			updated_at      TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS vpc_resource_mappings (
			id                    TEXT PRIMARY KEY,
			job_id                TEXT NOT NULL,
			org_id                TEXT NOT NULL,
			account_id            TEXT NOT NULL,
			source_resource_type  TEXT NOT NULL,
			source_resource_id    TEXT NOT NULL,
			dest_resource_type    TEXT NOT NULL,
			dest_resource_id      TEXT NOT NULL,
			mapping_json          TEXT NOT NULL DEFAULT '{}',
			created_at            TEXT NOT NULL,
			UNIQUE(job_id, source_resource_type, source_resource_id)
		)`,
		`CREATE TABLE IF NOT EXISTS vpc_locks (
			id          TEXT PRIMARY KEY,
			org_id      TEXT NOT NULL,
			account_id  TEXT NOT NULL,
			vpc_id      TEXT NOT NULL,
			lock_type   TEXT NOT NULL,
			reason      TEXT NOT NULL DEFAULT '',
			job_id      TEXT NOT NULL DEFAULT '',
			expires_at  TEXT NOT NULL DEFAULT '',
			created_by  TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			UNIQUE(vpc_id, lock_type)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("vpcmover: schema: %w", err)
		}
	}
	return nil
}

// ---- plans ------------------------------------------------------------------

func (s *Store) CreatePlan(p MobilityPlan) (MobilityPlan, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if p.ID == "" {
		p.ID = newID("vpcplan")
	}
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Status == "" {
		p.Status = PlanStatusDraft
	}
	setDefaults(&p.IncludeJSON, "[]")
	setDefaults(&p.ExcludeJSON, "[]")
	setDefaults(&p.OptionsJSON, "{}")
	setDefaults(&p.InventoryJSON, "{}")
	setDefaults(&p.PlanJSON, "{}")
	setDefaults(&p.WarningsJSON, "[]")
	setDefaults(&p.ErrorsJSON, "[]")

	_, err := s.db.Exec(
		`INSERT INTO vpc_mobility_plans
		 (id,org_id,account_id,project_id,source_vpc_id,destination_vpc_id,
		  operation,strategy,target_realm_id,target_region_id,target_zone_id,
		  status,include_json,exclude_json,options_json,inventory_json,plan_json,
		  warnings_json,errors_json,created_by,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.OrgID, p.AccountID, p.ProjectID, p.SourceVPCID, p.DestinationVPCID,
		string(p.Operation), p.Strategy, p.TargetRealmID, p.TargetRegionID, p.TargetZoneID,
		p.Status, p.IncludeJSON, p.ExcludeJSON, p.OptionsJSON, p.InventoryJSON, p.PlanJSON,
		p.WarningsJSON, p.ErrorsJSON, p.CreatedBy, p.CreatedAt, p.UpdatedAt,
	)
	return p, err
}

func (s *Store) GetPlan(id string) (MobilityPlan, error) {
	var p MobilityPlan
	err := s.db.QueryRow(
		`SELECT id,org_id,account_id,project_id,source_vpc_id,destination_vpc_id,
		        operation,strategy,target_realm_id,target_region_id,target_zone_id,
		        status,include_json,exclude_json,options_json,inventory_json,plan_json,
		        warnings_json,errors_json,created_by,created_at,updated_at
		 FROM vpc_mobility_plans WHERE id=?`, id,
	).Scan(
		&p.ID, &p.OrgID, &p.AccountID, &p.ProjectID, &p.SourceVPCID, &p.DestinationVPCID,
		&p.Operation, &p.Strategy, &p.TargetRealmID, &p.TargetRegionID, &p.TargetZoneID,
		&p.Status, &p.IncludeJSON, &p.ExcludeJSON, &p.OptionsJSON, &p.InventoryJSON, &p.PlanJSON,
		&p.WarningsJSON, &p.ErrorsJSON, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return p, fmt.Errorf("plan %q not found", id)
	}
	return p, nil
}

func (s *Store) ListPlansByVPC(orgID, accountID, sourceVPCID string) ([]MobilityPlan, error) {
	rows, err := s.db.Query(
		`SELECT id,org_id,account_id,project_id,source_vpc_id,destination_vpc_id,
		        operation,strategy,target_realm_id,target_region_id,target_zone_id,
		        status,include_json,exclude_json,options_json,inventory_json,plan_json,
		        warnings_json,errors_json,created_by,created_at,updated_at
		 FROM vpc_mobility_plans
		 WHERE org_id=? AND account_id=? AND source_vpc_id=?
		 ORDER BY created_at DESC`, orgID, accountID, sourceVPCID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlans(rows)
}

func (s *Store) UpdatePlanStatus(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE vpc_mobility_plans SET status=?, updated_at=? WHERE id=?`,
		status, now, id,
	)
	return err
}

func (s *Store) UpdatePlanFields(id string, updates map[string]string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	for col, val := range updates {
		if _, err := s.db.Exec(
			`UPDATE vpc_mobility_plans SET `+sanitizeCol(col)+`=?, updated_at=? WHERE id=?`,
			val, now, id,
		); err != nil {
			return err
		}
	}
	return nil
}

// ---- jobs -------------------------------------------------------------------

func (s *Store) CreateJob(j MobilityJob) (MobilityJob, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if j.ID == "" {
		j.ID = newID("vpcjob")
	}
	j.CreatedAt = now
	j.UpdatedAt = now
	if j.Status == "" {
		j.Status = JobStatusQueued
	}
	_, err := s.db.Exec(
		`INSERT INTO vpc_mobility_jobs
		 (id,plan_id,org_id,account_id,source_vpc_id,destination_vpc_id,
		  operation,status,current_step,progress_percent,rollback_available,
		  rollback_expires_at,started_at,finished_at,error_message,created_by,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		j.ID, j.PlanID, j.OrgID, j.AccountID, j.SourceVPCID, j.DestinationVPCID,
		string(j.Operation), j.Status, j.CurrentStep, j.ProgressPercent,
		boolInt(j.RollbackAvailable), j.RollbackExpiresAt, j.StartedAt, j.FinishedAt,
		j.ErrorMessage, j.CreatedBy, j.CreatedAt, j.UpdatedAt,
	)
	return j, err
}

func (s *Store) GetJob(id string) (MobilityJob, error) {
	var j MobilityJob
	var rollback int
	err := s.db.QueryRow(
		`SELECT id,plan_id,org_id,account_id,source_vpc_id,destination_vpc_id,
		        operation,status,current_step,progress_percent,rollback_available,
		        rollback_expires_at,started_at,finished_at,error_message,created_by,created_at,updated_at
		 FROM vpc_mobility_jobs WHERE id=?`, id,
	).Scan(
		&j.ID, &j.PlanID, &j.OrgID, &j.AccountID, &j.SourceVPCID, &j.DestinationVPCID,
		&j.Operation, &j.Status, &j.CurrentStep, &j.ProgressPercent, &rollback,
		&j.RollbackExpiresAt, &j.StartedAt, &j.FinishedAt, &j.ErrorMessage,
		&j.CreatedBy, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return j, fmt.Errorf("job %q not found", id)
	}
	j.RollbackAvailable = rollback == 1
	return j, nil
}

func (s *Store) ListJobsByVPC(orgID, accountID, sourceVPCID string) ([]MobilityJob, error) {
	rows, err := s.db.Query(
		`SELECT id,plan_id,org_id,account_id,source_vpc_id,destination_vpc_id,
		        operation,status,current_step,progress_percent,rollback_available,
		        rollback_expires_at,started_at,finished_at,error_message,created_by,created_at,updated_at
		 FROM vpc_mobility_jobs
		 WHERE org_id=? AND account_id=? AND source_vpc_id=?
		 ORDER BY created_at DESC`, orgID, accountID, sourceVPCID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (s *Store) UpdateJobStatus(id, status, step string, progress int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE vpc_mobility_jobs
		 SET status=?, current_step=?, progress_percent=?, updated_at=? WHERE id=?`,
		status, step, progress, now, id,
	)
	return err
}

func (s *Store) FailJob(id, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE vpc_mobility_jobs
		 SET status='failed', error_message=?, finished_at=?, updated_at=? WHERE id=?`,
		errMsg, now, now, id,
	)
	return err
}

func (s *Store) CompleteJob(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE vpc_mobility_jobs
		 SET status='completed', progress_percent=100, finished_at=?, updated_at=? WHERE id=?`,
		now, now, id,
	)
	return err
}

// ---- steps ------------------------------------------------------------------

func (s *Store) CreateStep(step MobilityStep) (MobilityStep, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if step.ID == "" {
		step.ID = newID("vpcstep")
	}
	step.CreatedAt = now
	step.UpdatedAt = now
	if step.Status == "" {
		step.Status = StepStatusPending
	}
	setDefaults(&step.InputJSON, "{}")
	setDefaults(&step.OutputJSON, "{}")
	_, err := s.db.Exec(
		`INSERT INTO vpc_mobility_steps
		 (id,job_id,step_order,name,status,started_at,finished_at,
		  input_json,output_json,error_message,retry_count,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		step.ID, step.JobID, step.StepOrder, step.Name, step.Status,
		step.StartedAt, step.FinishedAt, step.InputJSON, step.OutputJSON,
		step.ErrorMessage, step.RetryCount, step.CreatedAt, step.UpdatedAt,
	)
	return step, err
}

func (s *Store) ListSteps(jobID string) ([]MobilityStep, error) {
	rows, err := s.db.Query(
		`SELECT id,job_id,step_order,name,status,started_at,finished_at,
		        input_json,output_json,error_message,retry_count,created_at,updated_at
		 FROM vpc_mobility_steps WHERE job_id=? ORDER BY step_order`, jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MobilityStep
	for rows.Next() {
		var st MobilityStep
		if err := rows.Scan(
			&st.ID, &st.JobID, &st.StepOrder, &st.Name, &st.Status,
			&st.StartedAt, &st.FinishedAt, &st.InputJSON, &st.OutputJSON,
			&st.ErrorMessage, &st.RetryCount, &st.CreatedAt, &st.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStepStatus(id, status, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	finished := ""
	if status == StepStatusCompleted || status == StepStatusFailed || status == StepStatusRolledBack {
		finished = now
	}
	_, err := s.db.Exec(
		`UPDATE vpc_mobility_steps
		 SET status=?, error_message=?, finished_at=?, updated_at=? WHERE id=?`,
		status, errMsg, finished, now, id,
	)
	return err
}

// ---- resource mappings ------------------------------------------------------

func (s *Store) RecordMapping(m ResourceMapping) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if m.ID == "" {
		m.ID = newID("vpcmap")
	}
	m.CreatedAt = now
	setDefaults(&m.MappingJSON, "{}")
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO vpc_resource_mappings
		 (id,job_id,org_id,account_id,source_resource_type,source_resource_id,
		  dest_resource_type,dest_resource_id,mapping_json,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.JobID, m.OrgID, m.AccountID,
		m.SourceResourceType, m.SourceResourceID,
		m.DestResourceType, m.DestResourceID,
		m.MappingJSON, m.CreatedAt,
	)
	return err
}

func (s *Store) ListMappings(jobID string) ([]ResourceMapping, error) {
	rows, err := s.db.Query(
		`SELECT id,job_id,org_id,account_id,source_resource_type,source_resource_id,
		        dest_resource_type,dest_resource_id,mapping_json,created_at
		 FROM vpc_resource_mappings WHERE job_id=? ORDER BY source_resource_type`, jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceMapping
	for rows.Next() {
		var m ResourceMapping
		if err := rows.Scan(
			&m.ID, &m.JobID, &m.OrgID, &m.AccountID,
			&m.SourceResourceType, &m.SourceResourceID,
			&m.DestResourceType, &m.DestResourceID,
			&m.MappingJSON, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---- locks ------------------------------------------------------------------

func (s *Store) AcquireLock(l VPCLock) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if l.ID == "" {
		l.ID = newID("vpclock")
	}
	l.CreatedAt = now
	_, err := s.db.Exec(
		`INSERT INTO vpc_locks
		 (id,org_id,account_id,vpc_id,lock_type,reason,job_id,expires_at,created_by,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		l.ID, l.OrgID, l.AccountID, l.VPCID, l.LockType,
		l.Reason, l.JobID, l.ExpiresAt, l.CreatedBy, l.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("vpc %q is already locked with type %q", l.VPCID, l.LockType)
	}
	return nil
}

func (s *Store) ReleaseLock(vpcID, lockType string) error {
	res, err := s.db.Exec(
		`DELETE FROM vpc_locks WHERE vpc_id=? AND lock_type=?`, vpcID, lockType,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("lock not found for vpc %q type %q", vpcID, lockType)
	}
	return nil
}

func (s *Store) GetLock(vpcID, lockType string) (VPCLock, error) {
	var l VPCLock
	err := s.db.QueryRow(
		`SELECT id,org_id,account_id,vpc_id,lock_type,reason,job_id,expires_at,created_by,created_at
		 FROM vpc_locks WHERE vpc_id=? AND lock_type=?`, vpcID, lockType,
	).Scan(&l.ID, &l.OrgID, &l.AccountID, &l.VPCID, &l.LockType,
		&l.Reason, &l.JobID, &l.ExpiresAt, &l.CreatedBy, &l.CreatedAt)
	if err != nil {
		return l, fmt.Errorf("lock not found for vpc %q type %q", vpcID, lockType)
	}
	return l, nil
}

func (s *Store) ListLocks(vpcID string) ([]VPCLock, error) {
	rows, err := s.db.Query(
		`SELECT id,org_id,account_id,vpc_id,lock_type,reason,job_id,expires_at,created_by,created_at
		 FROM vpc_locks WHERE vpc_id=?`, vpcID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VPCLock
	for rows.Next() {
		var l VPCLock
		if err := rows.Scan(&l.ID, &l.OrgID, &l.AccountID, &l.VPCID, &l.LockType,
			&l.Reason, &l.JobID, &l.ExpiresAt, &l.CreatedBy, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ---- helpers ----------------------------------------------------------------

func newID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func setDefaults(s *string, def string) {
	if *s == "" {
		*s = def
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func sanitizeCol(col string) string {
	allowed := map[string]bool{
		"status": true, "destination_vpc_id": true, "inventory_json": true,
		"plan_json": true, "warnings_json": true, "errors_json": true,
	}
	if allowed[col] {
		return col
	}
	return "updated_at"
}

func scanPlans(rows *sql.Rows) ([]MobilityPlan, error) {
	var out []MobilityPlan
	for rows.Next() {
		var p MobilityPlan
		if err := rows.Scan(
			&p.ID, &p.OrgID, &p.AccountID, &p.ProjectID, &p.SourceVPCID, &p.DestinationVPCID,
			&p.Operation, &p.Strategy, &p.TargetRealmID, &p.TargetRegionID, &p.TargetZoneID,
			&p.Status, &p.IncludeJSON, &p.ExcludeJSON, &p.OptionsJSON, &p.InventoryJSON, &p.PlanJSON,
			&p.WarningsJSON, &p.ErrorsJSON, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanJobs(rows *sql.Rows) ([]MobilityJob, error) {
	var out []MobilityJob
	for rows.Next() {
		var j MobilityJob
		var rollback int
		if err := rows.Scan(
			&j.ID, &j.PlanID, &j.OrgID, &j.AccountID, &j.SourceVPCID, &j.DestinationVPCID,
			&j.Operation, &j.Status, &j.CurrentStep, &j.ProgressPercent, &rollback,
			&j.RollbackExpiresAt, &j.StartedAt, &j.FinishedAt, &j.ErrorMessage,
			&j.CreatedBy, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, err
		}
		j.RollbackAvailable = rollback == 1
		out = append(out, j)
	}
	return out, rows.Err()
}
