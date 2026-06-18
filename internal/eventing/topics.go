package eventing

import (
	"fmt"
	"time"
)

// Topic is a pub/sub channel.
type Topic struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// Subscription registers a webhook or queue endpoint on a topic.
type Subscription struct {
	ID         string `json:"id"`
	TopicID    string `json:"topicId"`
	TopicName  string `json:"topicName"`
	Endpoint   string `json:"endpoint"`   // http(s) URL or "queue:<name>"
	FilterJSON string `json:"filter,omitempty"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"createdAt"`
}

// InitTopicSchema creates the topics and topic_subscriptions tables.
func InitTopicSchema(s *Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS topics (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS topic_subscriptions (
			id          TEXT PRIMARY KEY,
			topic_id    TEXT NOT NULL,
			topic_name  TEXT NOT NULL,
			endpoint    TEXT NOT NULL,
			filter_json TEXT NOT NULL DEFAULT '{}',
			enabled     INTEGER NOT NULL DEFAULT 1,
			created_at  TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("eventing: topic schema: %w", err)
		}
	}
	return nil
}

// CreateTopic creates a new topic.
func (s *Store) CreateTopic(name string) (Topic, error) {
	t := Topic{
		ID:        "topic_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.Exec(
		`INSERT INTO topics (id, name, created_at) VALUES (?, ?, ?)`,
		t.ID, t.Name, t.CreatedAt,
	)
	return t, err
}

// GetTopic retrieves a topic by name or ID.
func (s *Store) GetTopic(nameOrID string) (Topic, error) {
	var t Topic
	err := s.db.QueryRow(
		`SELECT id, name, created_at FROM topics WHERE id=? OR name=?`, nameOrID, nameOrID,
	).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		return t, fmt.Errorf("topic %q not found", nameOrID)
	}
	return t, nil
}

// ListTopics returns all topics.
func (s *Store) ListTopics() ([]Topic, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM topics ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteTopic deletes a topic and all its subscriptions.
func (s *Store) DeleteTopic(nameOrID string) error {
	t, err := s.GetTopic(nameOrID)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`DELETE FROM topic_subscriptions WHERE topic_id=?`, t.ID)
	_, err = s.db.Exec(`DELETE FROM topics WHERE id=?`, t.ID)
	return err
}

// Subscribe registers an endpoint on a topic.
func (s *Store) Subscribe(topicNameOrID, endpoint string) (Subscription, error) {
	t, err := s.GetTopic(topicNameOrID)
	if err != nil {
		return Subscription{}, err
	}
	sub := Subscription{
		ID:        "sub_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		TopicID:   t.ID,
		TopicName: t.Name,
		Endpoint:  endpoint,
		Enabled:   true,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err = s.db.Exec(
		`INSERT INTO topic_subscriptions (id, topic_id, topic_name, endpoint, filter_json, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sub.ID, sub.TopicID, sub.TopicName, sub.Endpoint, "{}", 1, sub.CreatedAt,
	)
	return sub, err
}

// ListSubscriptions returns subscriptions for a topic.
func (s *Store) ListSubscriptions(topicNameOrID string) ([]Subscription, error) {
	t, err := s.GetTopic(topicNameOrID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, topic_id, topic_name, endpoint, filter_json, enabled, created_at
		 FROM topic_subscriptions WHERE topic_id=? ORDER BY created_at`,
		t.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subscription
	for rows.Next() {
		var sub Subscription
		var enabled int
		if err := rows.Scan(&sub.ID, &sub.TopicID, &sub.TopicName, &sub.Endpoint, &sub.FilterJSON, &enabled, &sub.CreatedAt); err != nil {
			return nil, err
		}
		sub.Enabled = enabled != 0
		out = append(out, sub)
	}
	return out, rows.Err()
}

// Unsubscribe removes a subscription.
func (s *Store) Unsubscribe(subID string) error {
	_, err := s.db.Exec(`DELETE FROM topic_subscriptions WHERE id=?`, subID)
	return err
}
