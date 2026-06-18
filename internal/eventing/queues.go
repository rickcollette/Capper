package eventing

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Queue is a message queue with visibility timeout and optional DLQ.
type Queue struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	VisibilityTimeoutSeconds int    `json:"visibilityTimeoutSeconds"`
	RetentionSeconds         int    `json:"retentionSeconds"`
	DLQName                  string `json:"dlqName,omitempty"`
	CreatedAt                string `json:"createdAt"`
}

// QueueMessage is a single message in a queue.
type QueueMessage struct {
	ID           string `json:"id"`
	QueueID      string `json:"queueId"`
	BodyJSON     string `json:"body"`
	VisibleAt    string `json:"visibleAt"`
	ReceiveCount int    `json:"receiveCount"`
	CreatedAt    string `json:"createdAt"`
}

// InitQueueSchema creates the queues and queue_messages tables.
func InitQueueSchema(s *Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS queues (
			id                         TEXT PRIMARY KEY,
			name                       TEXT NOT NULL UNIQUE,
			visibility_timeout_seconds INTEGER NOT NULL DEFAULT 30,
			retention_seconds          INTEGER NOT NULL DEFAULT 345600,
			dlq_name                   TEXT NOT NULL DEFAULT '',
			created_at                 TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS queue_messages (
			id            TEXT PRIMARY KEY,
			queue_id      TEXT NOT NULL,
			body_json     TEXT NOT NULL,
			visible_at    TEXT NOT NULL,
			receive_count INTEGER NOT NULL DEFAULT 0,
			created_at    TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("eventing: queue schema: %w", err)
		}
	}
	return nil
}

// CreateQueue creates a new queue.
func (s *Store) CreateQueue(name string, visibilityTimeout, retentionSeconds int, dlqName string) (Queue, error) {
	if visibilityTimeout <= 0 {
		visibilityTimeout = 30
	}
	if retentionSeconds <= 0 {
		retentionSeconds = 345600
	}
	q := Queue{
		ID:                       "q_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:                     name,
		VisibilityTimeoutSeconds: visibilityTimeout,
		RetentionSeconds:         retentionSeconds,
		DLQName:                  dlqName,
		CreatedAt:                time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.Exec(
		`INSERT INTO queues (id, name, visibility_timeout_seconds, retention_seconds, dlq_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		q.ID, q.Name, q.VisibilityTimeoutSeconds, q.RetentionSeconds, q.DLQName, q.CreatedAt,
	)
	return q, err
}

// GetQueue retrieves a queue by name or ID.
func (s *Store) GetQueue(nameOrID string) (Queue, error) {
	var q Queue
	err := s.db.QueryRow(
		`SELECT id, name, visibility_timeout_seconds, retention_seconds, dlq_name, created_at
		 FROM queues WHERE id=? OR name=?`, nameOrID, nameOrID,
	).Scan(&q.ID, &q.Name, &q.VisibilityTimeoutSeconds, &q.RetentionSeconds, &q.DLQName, &q.CreatedAt)
	if err != nil {
		return q, fmt.Errorf("queue %q not found", nameOrID)
	}
	return q, nil
}

// ListQueues returns all queues.
func (s *Store) ListQueues() ([]Queue, error) {
	rows, err := s.db.Query(
		`SELECT id, name, visibility_timeout_seconds, retention_seconds, dlq_name, created_at
		 FROM queues ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Queue
	for rows.Next() {
		var q Queue
		if err := rows.Scan(&q.ID, &q.Name, &q.VisibilityTimeoutSeconds, &q.RetentionSeconds, &q.DLQName, &q.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// DeleteQueue deletes a queue and all its messages.
func (s *Store) DeleteQueue(nameOrID string) error {
	q, err := s.GetQueue(nameOrID)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`DELETE FROM queue_messages WHERE queue_id=?`, q.ID)
	_, err = s.db.Exec(`DELETE FROM queues WHERE id=?`, q.ID)
	return err
}

// SendMessage enqueues a message. bodyJSON must be valid JSON.
func (s *Store) SendMessage(queueNameOrID, bodyJSON string) (QueueMessage, error) {
	q, err := s.GetQueue(queueNameOrID)
	if err != nil {
		return QueueMessage{}, err
	}
	now := time.Now().UTC()
	msg := QueueMessage{
		ID:        "msg_" + fmt.Sprintf("%d", now.UnixNano()),
		QueueID:   q.ID,
		BodyJSON:  bodyJSON,
		VisibleAt: now.Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339),
	}
	_, err = s.db.Exec(
		`INSERT INTO queue_messages (id, queue_id, body_json, visible_at, receive_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.QueueID, msg.BodyJSON, msg.VisibleAt, 0, msg.CreatedAt,
	)
	return msg, err
}

// ReceiveMessages returns up to maxMessages visible messages from the queue,
// bumping their visibility timeout and receive_count. The caller must call
// DeleteMessage after successful processing.
func (s *Store) ReceiveMessages(queueNameOrID string, maxMessages int) ([]QueueMessage, error) {
	q, err := s.GetQueue(queueNameOrID)
	if err != nil {
		return nil, err
	}
	if maxMessages <= 0 {
		maxMessages = 1
	}
	now := time.Now().UTC()
	rows, err := s.db.Query(
		`SELECT id, queue_id, body_json, visible_at, receive_count, created_at
		 FROM queue_messages
		 WHERE queue_id=? AND visible_at <= ?
		 ORDER BY created_at
		 LIMIT ?`,
		q.ID, now.Format(time.RFC3339), maxMessages,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueMessage
	for rows.Next() {
		var m QueueMessage
		if err := rows.Scan(&m.ID, &m.QueueID, &m.BodyJSON, &m.VisibleAt, &m.ReceiveCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Extend visibility and increment receive_count.
	nextVisible := now.Add(time.Duration(q.VisibilityTimeoutSeconds) * time.Second).Format(time.RFC3339)
	for _, m := range out {
		_, _ = s.db.Exec(
			`UPDATE queue_messages SET visible_at=?, receive_count=receive_count+1 WHERE id=?`,
			nextVisible, m.ID,
		)
	}

	// Move messages that exceeded max receive count to DLQ.
	if q.DLQName != "" {
		_ = s.moveToDLQ(q, out)
	}

	return out, nil
}

// DeleteMessage removes a processed message from the queue.
func (s *Store) DeleteMessage(messageID string) error {
	_, err := s.db.Exec(`DELETE FROM queue_messages WHERE id=?`, messageID)
	return err
}

// moveToDLQ sends messages exceeding max_receive_count (5) to the DLQ.
func (s *Store) moveToDLQ(q Queue, msgs []QueueMessage) error {
	dlq, err := s.GetQueue(q.DLQName)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		if m.ReceiveCount >= 5 {
			// Only delete the source message once it is safely in the DLQ;
			// otherwise a failed DLQ send followed by an unconditional delete
			// silently loses the message.
			if _, err := s.SendMessage(dlq.Name, strings.TrimSpace(m.BodyJSON)); err != nil {
				slog.Warn("eventing: DLQ send failed; leaving message in source queue",
					"queue", q.Name, "dlq", dlq.Name, "messageId", m.ID, "err", err)
				continue
			}
			if _, err := s.db.Exec(`DELETE FROM queue_messages WHERE id=?`, m.ID); err != nil {
				slog.Warn("eventing: failed to delete message after DLQ move",
					"queue", q.Name, "messageId", m.ID, "err", err)
			}
		}
	}
	return nil
}
