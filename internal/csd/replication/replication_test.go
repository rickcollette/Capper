package replication

import (
	"database/sql"
	"errors"
	"testing"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"

	_ "modernc.org/sqlite"
)

// ---- Fencing token (split-brain prevention) --------------------------------

func TestFencingToken_AdvanceAndValidate(t *testing.T) {
	var ft FencingToken

	if got := ft.Current(); got != 0 {
		t.Fatalf("initial token = %d, want 0", got)
	}
	if got := ft.Advance(); got != 1 {
		t.Fatalf("first Advance = %d, want 1", got)
	}
	if got := ft.Advance(); got != 2 {
		t.Fatalf("second Advance = %d, want 2", got)
	}

	// Current and newer tokens are accepted.
	if err := ft.Validate(2); err != nil {
		t.Errorf("Validate(current) returned %v, want nil", err)
	}
	if err := ft.Validate(3); err != nil {
		t.Errorf("Validate(newer) returned %v, want nil", err)
	}

	// A stale token (from an old leader) must be rejected as split-brain.
	err := ft.Validate(1)
	if err == nil {
		t.Fatal("Validate(stale) returned nil, want StaleFenceError")
	}
	var sfe *StaleFenceError
	if !errors.As(err, &sfe) {
		t.Fatalf("error type = %T, want *StaleFenceError", err)
	}
	if sfe.Got != 1 || sfe.Want != 2 {
		t.Errorf("StaleFenceError = {Got:%d Want:%d}, want {1 2}", sfe.Got, sfe.Want)
	}
}

// ---- Election: leader tracking and Bully peer comparison -------------------

func TestElection_HandleCoordinatorUpdatesLeaderAndTerm(t *testing.T) {
	em := NewElectionManager(nil, "node-b", "vol-1")

	if got := em.LeaderID(); got != "" {
		t.Fatalf("initial leader = %q, want empty", got)
	}

	em.HandleCoordinator("node-c", 5)
	if got := em.LeaderID(); got != "node-c" {
		t.Errorf("leader = %q, want node-c", got)
	}
	if got := em.term.Load(); got != 5 {
		t.Errorf("term = %d, want 5 (adopted from coordinator)", got)
	}

	// A coordinator message with a lower term must not regress the term, but the
	// leader identity still reflects the latest declaration.
	em.HandleCoordinator("node-d", 3)
	if got := em.term.Load(); got != 5 {
		t.Errorf("term regressed to %d, want 5", got)
	}
}

func TestElection_PeersWithHigherID(t *testing.T) {
	em := NewElectionManager(nil, "node-b", "vol-1")
	em.WithPeers([]string{"node-a", "node-c", "node-d"}, nil)

	higher := em.peersWithHigherID()
	if len(higher) != 2 {
		t.Fatalf("higher peers = %v, want 2 (node-c, node-d)", higher)
	}
	for _, p := range higher {
		if p <= "node-b" {
			t.Errorf("peer %q is not higher than node-b", p)
		}
	}
}

// ---- Quorum math -----------------------------------------------------------

func newTestStore(t *testing.T) *csdstore.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := csdstore.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return csdstore.New(db)
}

func insertReplica(t *testing.T, st *csdstore.Store, id, vol, status string) {
	t.Helper()
	if err := st.Replicas.Insert(csd.Replica{
		ID: id, VolumeID: vol, NodeID: id, Role: "replica",
		BackendType: "file", BackendPath: "/tmp/" + id, Status: status,
		CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("insert replica %s: %v", id, err)
	}
}

func TestReplicaManager_HasQuorum(t *testing.T) {
	st := newTestStore(t)
	rm := NewReplicaManager(st)

	// No replicas registered → single-node, always quorate.
	if ok, err := rm.HasQuorum("vol-empty"); err != nil || !ok {
		t.Errorf("empty volume: quorum=%v err=%v, want true/nil", ok, err)
	}

	// 3 replicas, 2 active → majority → quorum.
	insertReplica(t, st, "r1", "vol-3", "active")
	insertReplica(t, st, "r2", "vol-3", "active")
	insertReplica(t, st, "r3", "vol-3", "down")
	if ok, err := rm.HasQuorum("vol-3"); err != nil || !ok {
		t.Errorf("2/3 active: quorum=%v err=%v, want true/nil", ok, err)
	}

	// 3 replicas, 1 active → minority → no quorum.
	insertReplica(t, st, "s1", "vol-min", "active")
	insertReplica(t, st, "s2", "vol-min", "down")
	insertReplica(t, st, "s3", "vol-min", "down")
	if ok, err := rm.HasQuorum("vol-min"); err != nil || ok {
		t.Errorf("1/3 active: quorum=%v err=%v, want false/nil", ok, err)
	}
}
