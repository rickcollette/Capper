package hoststorage

import (
	"context"
	"database/sql"
	"strings"
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

func TestPoolAllocationAndOvercommit(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s)

	// Use a temp dir as the pool mountpoint; force a small fixed capacity.
	pool, err := mgr.CreatePool(CreatePoolOptions{
		Name: "p1", Mountpoint: t.TempDir(), TotalBytes: 1000,
	})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	a1, err := mgr.Allocate(AllocateOptions{PoolID: pool.ID, Name: "vol-a", Owner: "inst-1", SizeBytes: 600})
	if err != nil {
		t.Fatalf("allocate a1: %v", err)
	}

	// Second allocation that fits.
	if _, err := mgr.Allocate(AllocateOptions{PoolID: pool.ID, Name: "vol-b", SizeBytes: 300}); err != nil {
		t.Fatalf("allocate a2: %v", err)
	}

	// Overcommit must be refused (600+300+200 > 1000).
	if _, err := mgr.Allocate(AllocateOptions{PoolID: pool.ID, Name: "vol-c", SizeBytes: 200}); err == nil {
		t.Fatal("expected overcommit to be refused")
	}

	got, _ := mgr.GetPool(pool.ID)
	if got.AllocatedBytes != 900 || got.AvailableBytes != 100 {
		t.Fatalf("usage: allocated=%d available=%d, want 900/100", got.AllocatedBytes, got.AvailableBytes)
	}

	// Deleting a pool with allocations is refused.
	if err := mgr.DeletePool(pool.ID); err == nil {
		t.Fatal("expected delete to be refused while allocations exist")
	}

	// Release frees capacity and allows a previously-too-big allocation.
	if err := mgr.Release(a1.ID); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := mgr.Allocate(AllocateOptions{PoolID: pool.ID, Name: "vol-c", SizeBytes: 200}); err != nil {
		t.Fatalf("allocate after release: %v", err)
	}
}

func TestCreatePoolRejectsMissingMountpoint(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s)
	if _, err := mgr.CreatePool(CreatePoolOptions{Name: "x", Mountpoint: "/nonexistent/capper/path"}); err == nil {
		t.Fatal("expected error for missing mountpoint")
	}
}

func TestLVMAllocateRelease(t *testing.T) {
	// Stub LVM/mkfs so the LVM path is exercised without real tooling.
	var calls []string
	orig := lvmExec
	lvmExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "vgs" {
			return []byte("1073741824\n"), nil // 1 GiB VG
		}
		return []byte(""), nil
	}
	t.Cleanup(func() { lvmExec = orig })

	s := newStore(t)
	mgr := NewManager(s)

	// Construct the LVM pool directly (CreatePool would gate on lvmAvailable()).
	pool, err := s.InsertPool(StoragePool{Name: "vgpool", Backend: BackendLVM, VGName: "capvg", TotalBytes: 1 << 30})
	if err != nil {
		t.Fatalf("insert pool: %v", err)
	}

	a, err := mgr.Allocate(AllocateOptions{PoolID: pool.ID, Name: "inst-1", Owner: "inst-1", SizeBytes: 256 << 20})
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if a.Device != "/dev/capvg/inst-1" {
		t.Fatalf("expected LV device, got %q", a.Device)
	}
	if !containsCall(calls, "lvcreate") || !containsCall(calls, "mkfs.ext4") {
		t.Fatalf("expected lvcreate+mkfs, got %v", calls)
	}

	got, _ := mgr.GetPool(pool.ID)
	if got.AllocatedBytes != 256<<20 {
		t.Fatalf("allocated=%d, want %d", got.AllocatedBytes, 256<<20)
	}

	if err := mgr.ReleaseByOwner("inst-1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if !containsCall(calls, "lvremove") {
		t.Fatalf("expected lvremove on release, got %v", calls)
	}
}

func TestVGSizeIgnoresLeakedFDWarnings(t *testing.T) {
	// vgs can emit "File descriptor N leaked" notices to stderr; CombinedOutput
	// mixes them with the value. The size must still parse from the trailing token.
	orig := lvmExec
	lvmExec = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("File descriptor 6 (socket:[317230]) leaked on vgs invocation. Parent PID 46002: /usr/local/bin/capper\n" +
			"File descriptor 7 (socket:[316365]) leaked on vgs invocation. Parent PID 46002: /usr/local/bin/capper\n" +
			"  2000397795328\n"), nil
	}
	t.Cleanup(func() { lvmExec = orig })

	n, err := vgSizeBytes(context.Background(), "capvg")
	if err != nil {
		t.Fatalf("vgSizeBytes: %v", err)
	}
	if n != 2000397795328 {
		t.Fatalf("got %d, want 2000397795328", n)
	}
}

func containsCall(calls []string, prefix string) bool {
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}
