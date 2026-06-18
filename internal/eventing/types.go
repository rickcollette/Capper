package eventing

import "database/sql"

// Rule triggers an action when an event pattern matches.
type Rule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Project    string `json:"project"`
	EventType  string `json:"eventType"`  // e.g. "instance.started"
	Action     string `json:"action"`     // "notify" | "webhook" | "run-command"
	ActionArgs string `json:"actionArgs"` // URL, command, etc.
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"createdAt"`
}

// Schedule triggers an action on a cron schedule.
type Schedule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Project    string `json:"project"`
	Cron       string `json:"cron"`       // e.g. "0 * * * *"
	Action     string `json:"action"`     // "backup" | "run-instance" | "webhook"
	ActionArgs string `json:"actionArgs"`
	Enabled    bool   `json:"enabled"`
	LastRunAt  string `json:"lastRunAt,omitempty"`
	NextRunAt  string `json:"nextRunAt,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

// Store persists rules and schedules.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }
