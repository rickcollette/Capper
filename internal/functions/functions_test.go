package functions

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s := NewStore(db)
	if err := s.InitSchema(); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s
}

func TestFunctionLifecycle(t *testing.T) {
	s := newStore(t)
	fn, err := s.CreateFunction(Function{Project: "prod", Name: "resize", Runtime: "go1.24"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if fn.MemoryBytes != DefaultMemoryBytes || fn.TimeoutMS != DefaultTimeoutMS {
		t.Errorf("defaults not applied: %+v", fn)
	}

	got, err := s.GetFunctionByName("prod", "resize")
	if err != nil || got.ID != fn.ID {
		t.Fatalf("get by name: %v (%q)", err, got.ID)
	}

	v, err := s.AddVersion(FunctionVersion{FunctionID: fn.ID})
	if err != nil || v.Version != "1" {
		t.Fatalf("add version: %v (%q)", err, v.Version)
	}
	v2, _ := s.AddVersion(FunctionVersion{FunctionID: fn.ID})
	if v2.Version != "2" {
		t.Errorf("expected version 2, got %q", v2.Version)
	}

	list, _ := s.ListFunctions("prod")
	if len(list) != 1 {
		t.Errorf("expected 1 function, got %d", len(list))
	}

	if err := s.DeleteFunction(fn.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := s.ListFunctions("prod"); len(list) != 0 {
		t.Errorf("expected 0 functions after delete, got %d", len(list))
	}
}

func TestInvokeAndRecord(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s, ProcessInvoker{})
	// Use /bin/cat as the function body: it echoes stdin to stdout.
	fn, _ := s.CreateFunction(Function{Project: "prod", Name: "echo", Runtime: "native", Command: []string{"/bin/cat"}})

	res, err := mgr.Invoke(context.Background(), fn, []byte("hello"), "", "tester", "manual")
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if res.Status != InvocationSucceeded {
		t.Errorf("expected succeeded, got %q (err=%s)", res.Status, res.Error)
	}
	if res.Output != "hello" {
		t.Errorf("expected output 'hello', got %q", res.Output)
	}

	invs, _ := s.ListInvocations(fn.ID, 10)
	if len(invs) != 1 || invs[0].Status != InvocationSucceeded {
		t.Errorf("expected 1 succeeded invocation, got %+v", invs)
	}
}

func TestInvokeFailureRecorded(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s, ProcessInvoker{})
	fn, _ := s.CreateFunction(Function{Project: "prod", Name: "bad", Runtime: "native",
		Command: []string{"/nonexistent/binary"}})
	res, _ := mgr.Invoke(context.Background(), fn, nil, "", "tester", "manual")
	if res.Status != InvocationFailed {
		t.Errorf("expected failed, got %q", res.Status)
	}
	invs, _ := s.ListInvocations(fn.ID, 10)
	if len(invs) != 1 || invs[0].Status != InvocationFailed {
		t.Errorf("failure not recorded: %+v", invs)
	}
}

func TestTriggerDispatch(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s, ProcessInvoker{})
	fn, _ := s.CreateFunction(Function{Project: "prod", Name: "consumer", Runtime: "native",
		Command: []string{"/bin/cat"}})
	if _, err := s.AddTrigger(Trigger{Project: "prod", FunctionID: fn.ID, Type: "queue",
		Source: "jobs", Enabled: true}); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	results, err := mgr.DispatchEvent(context.Background(), "queue", "jobs", []byte("msg"))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(results) != 1 || results[0].Status != InvocationSucceeded {
		t.Errorf("expected 1 succeeded dispatch, got %+v", results)
	}
	// A disabled/unmatched source dispatches to nothing.
	none, _ := mgr.DispatchEvent(context.Background(), "queue", "other", []byte("x"))
	if len(none) != 0 {
		t.Errorf("expected no dispatch for unmatched source, got %d", len(none))
	}
}
