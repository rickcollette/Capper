package eventing

import (
	"fmt"
	"time"
)

// InitSchemaSQL creates all eventing tables.
func InitSchemaSQL(s *Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS event_rules (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			project     TEXT NOT NULL,
			event_type  TEXT NOT NULL,
			action      TEXT NOT NULL DEFAULT 'notify',
			action_args TEXT NOT NULL DEFAULT '',
			enabled     INTEGER NOT NULL DEFAULT 1,
			created_at  TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
		`CREATE TABLE IF NOT EXISTS schedules (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			project     TEXT NOT NULL,
			cron        TEXT NOT NULL DEFAULT '',
			action      TEXT NOT NULL DEFAULT 'backup',
			action_args TEXT NOT NULL DEFAULT '',
			enabled     INTEGER NOT NULL DEFAULT 1,
			last_run_at TEXT NOT NULL DEFAULT '',
			next_run_at TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
	}
	for _, s2 := range stmts {
		if _, err := s.db.Exec(s2); err != nil {
			return err
		}
	}
	if err := InitQueueSchema(s); err != nil {
		return err
	}
	return InitTopicSchema(s)
}

func (s *Store) InsertRule(r Rule) error {
	if r.ID == "" {
		r.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
	}
	if r.CreatedAt == "" {
		r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO event_rules (id, name, project, event_type, action, action_args, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Project, r.EventType, r.Action, r.ActionArgs, boolInt(r.Enabled), r.CreatedAt,
	)
	return err
}

func (s *Store) ListRules(project string) ([]Rule, error) {
	rows, err := s.db.Query(
		`SELECT id, name, project, event_type, action, action_args, enabled, created_at
		 FROM event_rules WHERE project=? ORDER BY name`, project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Project, &r.EventType, &r.Action, &r.ActionArgs, &enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRule(name, project string) error {
	_, err := s.db.Exec(`DELETE FROM event_rules WHERE name=? AND project=?`, name, project)
	return err
}

func (s *Store) InsertSchedule(sc Schedule) error {
	if sc.ID == "" {
		sc.ID = fmt.Sprintf("sched_%d", time.Now().UnixNano())
	}
	if sc.CreatedAt == "" {
		sc.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO schedules (id, name, project, cron, action, action_args, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sc.ID, sc.Name, sc.Project, sc.Cron, sc.Action, sc.ActionArgs, boolInt(sc.Enabled), sc.CreatedAt,
	)
	return err
}

func (s *Store) ListSchedules(project string) ([]Schedule, error) {
	rows, err := s.db.Query(
		`SELECT id, name, project, cron, action, action_args, enabled, last_run_at, next_run_at, created_at
		 FROM schedules WHERE project=? ORDER BY name`, project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		var sc Schedule
		var enabled int
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Project, &sc.Cron, &sc.Action, &sc.ActionArgs,
			&enabled, &sc.LastRunAt, &sc.NextRunAt, &sc.CreatedAt); err != nil {
			return nil, err
		}
		sc.Enabled = enabled != 0
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSchedule(name, project string) error {
	_, err := s.db.Exec(`DELETE FROM schedules WHERE name=? AND project=?`, name, project)
	return err
}

func (s *Store) MarkScheduleRun(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE schedules SET last_run_at=? WHERE id=?`, now, id)
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
