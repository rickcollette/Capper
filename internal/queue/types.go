package queue

import (
	"database/sql"
	"fmt"
	"time"
)

// Message is a queue message.
type Message struct {
	ID        string `json:"id"`
	Queue     string `json:"queue"`
	Project   string `json:"project"`
	Body      string `json:"body"`
	Status    string `json:"status"` // "pending", "delivered", "acked"
	CreatedAt string `json:"createdAt"`
}

// Queue is a named message queue.
type Queue struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Project   string `json:"project"`
	CreatedAt string `json:"createdAt"`
}

// Store manages queues and messages in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS queues (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			project    TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(name, project)
		)`,
		`CREATE TABLE IF NOT EXISTS queue_messages (
			id         TEXT PRIMARY KEY,
			queue_name TEXT NOT NULL,
			project    TEXT NOT NULL,
			body       TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL DEFAULT 'pending',
			created_at TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// Manager handles queue operations.
type Manager struct {
	store *Store
}

func NewManager(s *Store) *Manager { return &Manager{store: s} }

func (m *Manager) Create(name, project string) (Queue, error) {
	q := Queue{
		ID: fmt.Sprintf("q_%d", time.Now().UnixNano()), Name: name,
		Project: project, CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err := m.store.db.Exec(
		`INSERT INTO queues (id, name, project, created_at) VALUES (?, ?, ?, ?)`,
		q.ID, q.Name, q.Project, q.CreatedAt,
	)
	return q, err
}

func (m *Manager) List(project string) ([]Queue, error) {
	rows, err := m.store.db.Query(`SELECT id, name, project, created_at FROM queues WHERE project=?`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Queue
	for rows.Next() {
		var q Queue
		if err := rows.Scan(&q.ID, &q.Name, &q.Project, &q.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (m *Manager) Delete(name, project string) error {
	_, err := m.store.db.Exec(`DELETE FROM queues WHERE name=? AND project=?`, name, project)
	return err
}

func (m *Manager) Publish(queueName, project, body string) (Message, error) {
	msg := Message{
		ID: fmt.Sprintf("msg_%d", time.Now().UnixNano()), Queue: queueName,
		Project: project, Body: body, Status: "pending",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err := m.store.db.Exec(
		`INSERT INTO queue_messages (id, queue_name, project, body, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.Queue, msg.Project, msg.Body, msg.Status, msg.CreatedAt,
	)
	return msg, err
}

func (m *Manager) Consume(queueName, project string, max int) ([]Message, error) {
	if max <= 0 {
		max = 10
	}
	rows, err := m.store.db.Query(
		`SELECT id, queue_name, project, body, status, created_at FROM queue_messages
		 WHERE queue_name=? AND project=? AND status='pending' ORDER BY created_at LIMIT ?`,
		queueName, project, max,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.Queue, &msg.Project, &msg.Body, &msg.Status, &msg.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	// Mark as delivered
	for _, msg := range out {
		_, _ = m.store.db.Exec(`UPDATE queue_messages SET status='delivered' WHERE id=?`, msg.ID)
	}
	return out, nil
}

// Depth returns the number of pending messages in the named queue.
func (m *Manager) Depth(queueName, project string) (int64, error) {
	row := m.store.db.QueryRow(
		`SELECT COUNT(*) FROM queue_messages WHERE queue_name=? AND project=? AND status='pending'`,
		queueName, project,
	)
	var n int64
	return n, row.Scan(&n)
}
