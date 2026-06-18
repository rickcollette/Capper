package ipam

import "testing"

func newExclPool(t *testing.T, mgr *Manager) RoutableIPPool {
	t.Helper()
	pool, _, err := mgr.CreatePool(CreatePoolOptions{
		Pool: RoutableIPPool{
			Name: "public-main", CIDR: "203.0.113.0/29", Gateway: "203.0.113.1",
			Usage: []string{UsageLoadBalancer, UsageReserved}, AllowAutoAllocate: true, Status: PoolActive,
		},
	})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	return pool
}

func TestAddExclusionDropsAddressFromAllocation(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s)
	newExclPool(t, mgr)

	// .2 is the first auto-allocatable address (.0 net, .1 gw, .7 bcast excluded).
	if _, err := mgr.AddExclusion(IPExclusion{Address: "203.0.113.2", Reason: "Capper Server Host"}); err != nil {
		t.Fatalf("add exclusion: %v", err)
	}
	ip, err := s.GetIPByAddress("203.0.113.2")
	if err != nil {
		t.Fatalf("get ip: %v", err)
	}
	if ip.Status != IPExcluded {
		t.Fatalf("expected excluded, got %s", ip.Status)
	}

	// Auto-reserve must skip the excluded address.
	got, err := mgr.Reserve(ReserveOptions{PoolID: ip.PoolID, Purpose: UsageLoadBalancer})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if got.Address == "203.0.113.2" {
		t.Fatal("auto-allocation handed out an excluded address")
	}
}

func TestRemoveExclusionReturnsAddress(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s)
	newExclPool(t, mgr)

	e, err := mgr.AddExclusion(IPExclusion{Address: "203.0.113.3"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := mgr.RemoveExclusion(e.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	ip, _ := s.GetIPByAddress("203.0.113.3")
	if ip.Status != IPAvailable {
		t.Fatalf("expected available after un-exclude, got %s", ip.Status)
	}
}

func TestAddExclusionRefusesClaimedAddress(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s)
	pool := newExclPool(t, mgr)

	// A reserved (claimed-but-unbound) address must not be silently pulled.
	reserved, err := mgr.Reserve(ReserveOptions{PoolID: pool.ID, Purpose: UsageLoadBalancer})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if _, err := mgr.AddExclusion(IPExclusion{Address: reserved.Address}); err == nil {
		t.Fatal("expected refusal to exclude a reserved address")
	}

	// An attached address is likewise refused.
	attached, err := mgr.Reserve(ReserveOptions{PoolID: pool.ID, Purpose: UsageLoadBalancer})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if _, err := mgr.Attach(attached.ID, IPBinding{TargetType: "load-balancer", TargetID: "lb-1",
		BindingMode: ModeVIP, Protocol: "tcp", ExternalPort: 443}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, err := mgr.AddExclusion(IPExclusion{Address: attached.Address}); err == nil {
		t.Fatal("expected refusal to exclude an attached address")
	}
}

func TestCreatePoolHonorsStandingGlobalExclusion(t *testing.T) {
	s := newStore(t)
	mgr := NewManager(s)
	// Exclude an address before any pool containing it exists.
	if _, err := mgr.AddExclusion(IPExclusion{Address: "203.0.113.2", Reason: "reserved by host"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	pool, _, err := mgr.CreatePool(CreatePoolOptions{
		Pool: RoutableIPPool{Name: "p", CIDR: "203.0.113.0/29", Gateway: "203.0.113.1",
			Usage: []string{UsageLoadBalancer}, AllowAutoAllocate: true, Status: PoolActive},
	})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if _, err := s.GetIPByAddress("203.0.113.2"); err == nil {
		t.Fatal("excluded address should not have been materialized")
	}
	for _, ip := range mustList(t, s, pool.ID) {
		if ip.Address == "203.0.113.2" {
			t.Fatal("standing exclusion not applied at pool creation")
		}
	}
}

func mustList(t *testing.T, s *Store, poolID string) []RoutableIP {
	t.Helper()
	ips, err := s.ListIPs(poolID, "")
	if err != nil {
		t.Fatalf("list ips: %v", err)
	}
	return ips
}
