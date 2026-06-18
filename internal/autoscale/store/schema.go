package autoscalestore

import "database/sql"

// InitSchema creates all autoscale tables and migrates compute_groups.
// Safe to call multiple times (idempotent).
func InitSchema(db *sql.DB) error {
	stmts := []string{
		// --- autoscale policies ------------------------------------------------
		`CREATE TABLE IF NOT EXISTS autoscale_policies (
			id                         TEXT PRIMARY KEY,
			project                    TEXT NOT NULL DEFAULT '',
			name                       TEXT NOT NULL,
			group_id                   TEXT NOT NULL,
			enabled                    INTEGER NOT NULL DEFAULT 1,
			policy_type                TEXT NOT NULL DEFAULT 'target',
			metric_name                TEXT NOT NULL DEFAULT '',
			metric_scope               TEXT NOT NULL DEFAULT 'group',
			queue_name                 TEXT NOT NULL DEFAULT '',
			target_value               REAL NOT NULL DEFAULT 0,
			scale_out_threshold        REAL NOT NULL DEFAULT 0,
			scale_in_threshold         REAL NOT NULL DEFAULT 0,
			min_replicas               INTEGER NOT NULL DEFAULT 1,
			max_replicas               INTEGER NOT NULL DEFAULT 10,
			scale_out_step             INTEGER NOT NULL DEFAULT 1,
			scale_in_step              INTEGER NOT NULL DEFAULT 1,
			scale_out_cooldown_seconds INTEGER NOT NULL DEFAULT 60,
			scale_in_cooldown_seconds  INTEGER NOT NULL DEFAULT 300,
			evaluation_window_seconds  INTEGER NOT NULL DEFAULT 300,
			stabilization_window_secs  INTEGER NOT NULL DEFAULT 300,
			schedule_json              TEXT NOT NULL DEFAULT '[]',
			last_scale_at              TEXT NOT NULL DEFAULT '',
			last_decision              TEXT NOT NULL DEFAULT '',
			created_at                 TEXT NOT NULL,
			updated_at                 TEXT NOT NULL,
			UNIQUE(project, name)
		);`,

		// --- autoscale decisions (audit trail) ---------------------------------
		`CREATE TABLE IF NOT EXISTS autoscale_decisions (
			id                   TEXT PRIMARY KEY,
			policy_id            TEXT NOT NULL DEFAULT '',
			group_id             TEXT NOT NULL,
			project              TEXT NOT NULL DEFAULT '',
			old_replicas         INTEGER NOT NULL DEFAULT 0,
			new_replicas         INTEGER NOT NULL DEFAULT 0,
			recommended_replicas INTEGER NOT NULL DEFAULT 0,
			decision             TEXT NOT NULL,
			reason               TEXT NOT NULL DEFAULT '',
			metric_name          TEXT NOT NULL DEFAULT '',
			metric_value         REAL NOT NULL DEFAULT 0,
			target_value         REAL NOT NULL DEFAULT 0,
			blocked              INTEGER NOT NULL DEFAULT 0,
			blocked_reason       TEXT NOT NULL DEFAULT '',
			created_at           TEXT NOT NULL
		);`,

		// --- metric samples (rolling window) -----------------------------------
		`CREATE TABLE IF NOT EXISTS autoscale_metric_samples (
			id          TEXT PRIMARY KEY,
			project     TEXT NOT NULL DEFAULT '',
			scope       TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			metric_name TEXT NOT NULL,
			value       REAL NOT NULL,
			labels_json TEXT NOT NULL DEFAULT '{}',
			sampled_at  TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_metric_samples_resource
			ON autoscale_metric_samples(scope, resource_id, metric_name, sampled_at);`,

		// --- compute_groups autoscale columns (ALTER TABLE, ignore if exists) --
		`ALTER TABLE compute_groups ADD COLUMN scaling_enabled  INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE compute_groups ADD COLUMN lb_id            TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE compute_groups ADD COLUMN drain_seconds    INTEGER NOT NULL DEFAULT 30;`,
		`ALTER TABLE compute_groups ADD COLUMN placement_policy TEXT NOT NULL DEFAULT '';`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			// ALTER TABLE fails with "duplicate column name" if column exists — ignore.
			if isAlterColumnExists(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func isAlterColumnExists(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "duplicate column name") || contains(msg, "already exists")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
