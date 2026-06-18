package queue_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"capper/internal/queue"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newManager(t *testing.T) *queue.Manager {
	t.Helper()
	return queue.NewManager(queue.NewStore(openDB(t)))
}

func TestCreateAndList(t *testing.T) {
	m := newManager(t)
	q, err := m.Create("work-queue", "proj1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if q.Name != "work-queue" {
		t.Errorf("name: %q", q.Name)
	}
	queues, err := m.List("proj1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(queues) != 1 {
		t.Errorf("List: got %d, want 1", len(queues))
	}
}

func TestPublishAndConsume(t *testing.T) {
	m := newManager(t)
	if _, err := m.Create("q1", "proj1"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"msg1", "msg2", "msg3"} {
		if _, err := m.Publish("q1", "proj1", body); err != nil {
			t.Fatalf("Publish %q: %v", body, err)
		}
	}
	msgs, err := m.Consume("q1", "proj1", 2)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("Consume: got %d messages, want 2", len(msgs))
	}
}

func TestConsumeOrdering(t *testing.T) {
	m := newManager(t)
	if _, err := m.Create("ordered", "proj1"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"first", "second", "third"} {
		if _, err := m.Publish("ordered", "proj1", body); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
	msgs, _ := m.Consume("ordered", "proj1", 10)
	if len(msgs) < 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Body != "first" {
		t.Errorf("FIFO ordering: first msg got %q", msgs[0].Body)
	}
}

func TestDepth(t *testing.T) {
	m := newManager(t)
	if _, err := m.Create("depth-q", "proj1"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := m.Publish("depth-q", "proj1", "msg"); err != nil {
			t.Fatal(err)
		}
	}
	d, err := m.Depth("depth-q", "proj1")
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if d != 5 {
		t.Errorf("Depth: got %d, want 5", d)
	}
}

func TestDelete(t *testing.T) {
	m := newManager(t)
	if _, err := m.Create("to-del", "proj1"); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete("to-del", "proj1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	queues, _ := m.List("proj1")
	if len(queues) != 0 {
		t.Errorf("expected 0 queues after delete, got %d", len(queues))
	}
}
