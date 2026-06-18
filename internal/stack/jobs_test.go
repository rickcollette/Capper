package stack_test

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"capper/internal/stack"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := stack.InitJobSchema(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newJob(name string) stack.Job {
	now := time.Now().UTC().Format(time.RFC3339)
	return stack.Job{
		ID:        "job_" + name,
		Name:      name,
		Project:   "default",
		SpecYAML:  `{"kind":"Job","metadata":{"name":"` + name + `"},"spec":{"steps":[{"run":"echo hi"}]}}`,
		Status:    stack.JobQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestJobStore_InsertAndGet(t *testing.T) {
	db := openTestDB(t)
	s := stack.NewJobStore(db)

	j := newJob("test-insert")
	if err := s.Insert(j); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	got, err := s.Get(j.Name, j.Project)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != j.ID {
		t.Errorf("ID: got %q, want %q", got.ID, j.ID)
	}
}

func TestJobStore_List(t *testing.T) {
	db := openTestDB(t)
	s := stack.NewJobStore(db)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := s.Insert(newJob(name)); err != nil {
			t.Fatalf("Insert %q: %v", name, err)
		}
	}
	jobs, err := s.List("default")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("List: got %d, want 3", len(jobs))
	}
}

func TestJobStore_UpdateStatus(t *testing.T) {
	db := openTestDB(t)
	s := stack.NewJobStore(db)
	j := newJob("status-test")
	_ = s.Insert(j)

	if err := s.UpdateStatus(j.ID, stack.JobRunning); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ := s.Get(j.ID, "")
	if got.Status != stack.JobRunning {
		t.Errorf("status: got %q, want running", got.Status)
	}
}

func TestJobStore_AppendLog(t *testing.T) {
	db := openTestDB(t)
	s := stack.NewJobStore(db)
	j := newJob("log-test")
	_ = s.Insert(j)

	_ = s.AppendLog(j.ID, "line one")
	_ = s.AppendLog(j.ID, "line two")
	got, _ := s.Get(j.ID, "")
	if got.Logs == "" {
		t.Error("expected non-empty logs")
	}
}

func TestJobStore_Delete(t *testing.T) {
	db := openTestDB(t)
	s := stack.NewJobStore(db)
	j := newJob("delete-test")
	_ = s.Insert(j)
	if err := s.Delete(j.Name, j.Project); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := s.Get(j.Name, j.Project)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestParseJobSpec_Valid(t *testing.T) {
	raw := `{"kind":"Job","metadata":{"name":"backup-all"},"spec":{"steps":[{"run":"echo backup"}]}}`
	spec, err := stack.ParseJobSpec(raw)
	if err != nil {
		t.Fatalf("ParseJobSpec: %v", err)
	}
	if spec.Metadata.Name != "backup-all" {
		t.Errorf("name: %q", spec.Metadata.Name)
	}
	if len(spec.Spec.Steps) != 1 {
		t.Errorf("steps: got %d", len(spec.Spec.Steps))
	}
}

func TestParseJobSpec_WrongKind(t *testing.T) {
	raw := `{"kind":"Stack","metadata":{"name":"foo"},"spec":{"steps":[{"run":"x"}]}}`
	_, err := stack.ParseJobSpec(raw)
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestParseJobSpec_NoSteps(t *testing.T) {
	raw := `{"kind":"Job","metadata":{"name":"foo"},"spec":{"steps":[]}}`
	_, err := stack.ParseJobSpec(raw)
	if err == nil {
		t.Fatal("expected error for empty steps")
	}
}

func TestRunJob_Success(t *testing.T) {
	db := openTestDB(t)
	s := stack.NewJobStore(db)
	j := newJob("run-success")
	_ = s.Insert(j)

	if err := stack.RunJob(s, j); err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	got, _ := s.Get(j.ID, "")
	if got.Status != stack.JobDone {
		t.Errorf("status: got %q, want done", got.Status)
	}
}

func TestRunJob_FailingStep(t *testing.T) {
	db := openTestDB(t)
	s := stack.NewJobStore(db)
	now := time.Now().UTC().Format(time.RFC3339)
	j := stack.Job{
		ID:        "job_fail",
		Name:      "fail-test",
		Project:   "default",
		SpecYAML:  `{"kind":"Job","metadata":{"name":"fail-test"},"spec":{"steps":[{"run":"false"}]}}`,
		Status:    stack.JobQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_ = s.Insert(j)

	err := stack.RunJob(s, j)
	if err == nil {
		t.Fatal("expected RunJob to fail for exit-1 step")
	}
	got, _ := s.Get(j.ID, "")
	if got.Status != stack.JobFailed {
		t.Errorf("status: got %q, want failed", got.Status)
	}
}
