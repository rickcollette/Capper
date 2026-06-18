package network

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testNetwork(id, name string) Network {
	return Network{
		ID:        id,
		Name:      name,
		Project:   "default",
		Mode:      ModeNAT,
		Subnet:    "10.42.0.0/24",
		Gateway:   "10.42.0.1",
		Bridge:    BridgeName(name),
		Status:    StatusActive,
		CreatedAt: "2024-01-01T00:00:00Z",
	}
}

// ---------------------------------------------------------------------------
// Store tests
// ---------------------------------------------------------------------------

func TestStoreInsertAndGet(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_aabbccdd11223344", "mynet")

	if err := s.Insert(n); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := s.Get(n.ID, "default")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Name != n.Name {
		t.Errorf("name mismatch: got %q want %q", got.Name, n.Name)
	}

	got2, err := s.Get(n.Name, "default")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got2.ID != n.ID {
		t.Errorf("id mismatch: got %q want %q", got2.ID, n.ID)
	}
}

func TestStoreUpdateStatus(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_aabb1122ccdd3344", "testnet")
	if err := s.Insert(n); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.UpdateStatus(n.ID, StatusDeleted); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, err := s.Get(n.ID, "default")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusDeleted {
		t.Errorf("status: got %q want %q", got.Status, StatusDeleted)
	}
}

func TestStoreList(t *testing.T) {
	s := NewStore(openTestDB(t))
	s.Insert(testNetwork("net_0000000000000001", "alpha"))
	s.Insert(testNetwork("net_0000000000000002", "beta"))

	nets, err := s.List("default")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(nets) != 2 {
		t.Errorf("expected 2 networks, got %d", len(nets))
	}
}

func TestStoreDeleteBlockedByLeases(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_deadbeef12345678", "busynet")
	if err := s.Insert(n); err != nil {
		t.Fatalf("insert: %v", err)
	}
	lease := NetworkLease{
		NetworkID:  n.ID,
		InstanceID: "inst_cafe",
		IP:         "10.42.0.2",
		MAC:        "02:00:00:00:00:01",
		CreatedAt:  "2024-01-01T00:00:00Z",
	}
	if err := s.InsertLease(lease); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	if err := s.Delete(n.ID); err == nil {
		t.Error("expected error deleting network with active leases")
	}
}

func TestStoreDeleteEmptyNetwork(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_1122334455667788", "emptynet")
	if err := s.Insert(n); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.Delete(n.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(n.ID, "default"); err == nil {
		t.Error("expected error getting deleted network")
	}
}

func TestStoreAllLeases(t *testing.T) {
	s := NewStore(openTestDB(t))
	a := testNetwork("net_all1111all1111a", "allnet1")
	b := testNetwork("net_all2222all2222b", "allnet2")
	s.Insert(a)
	s.Insert(b)
	_ = s.InsertLease(NetworkLease{NetworkID: a.ID, InstanceID: "inst_a", IP: "10.42.0.2"})
	_ = s.InsertLease(NetworkLease{NetworkID: b.ID, InstanceID: "inst_b", IP: "10.43.0.2"})

	all, err := s.AllLeases()
	if err != nil {
		t.Fatalf("all leases: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 leases across networks, got %d", len(all))
	}
}

func TestStoreLeases(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_aabb1122aabb1122", "leasenet")
	s.Insert(n)

	leases := []NetworkLease{
		{NetworkID: n.ID, InstanceID: "inst_01", IP: "10.42.0.2", MAC: "02:00:00:00:00:02", CreatedAt: "2024-01-01T00:00:00Z"},
		{NetworkID: n.ID, InstanceID: "inst_02", IP: "10.42.0.3", MAC: "02:00:00:00:00:03", CreatedAt: "2024-01-01T00:00:00Z"},
	}
	for _, l := range leases {
		if err := s.InsertLease(l); err != nil {
			t.Fatalf("insert lease: %v", err)
		}
	}

	got, err := s.LeasesForNetwork(n.ID)
	if err != nil {
		t.Fatalf("leases for network: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 leases, got %d", len(got))
	}

	if err := s.DeleteLease(n.ID, "inst_01"); err != nil {
		t.Fatalf("delete lease: %v", err)
	}
	got2, err := s.LeasesForNetwork(n.ID)
	if err != nil {
		t.Fatalf("leases after delete: %v", err)
	}
	if len(got2) != 1 {
		t.Errorf("expected 1 lease after delete, got %d", len(got2))
	}
}

func TestStoreAllocatedIPs(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_cafebabe00000001", "ipnet")
	s.Insert(n)

	s.InsertLease(NetworkLease{NetworkID: n.ID, InstanceID: "inst_x", IP: "10.42.0.5", MAC: "02:aa:bb:cc:dd:01", CreatedAt: "2024-01-01T00:00:00Z"})
	allocated, err := s.AllocatedIPs(n.ID)
	if err != nil {
		t.Fatalf("allocated IPs: %v", err)
	}
	if !allocated["10.42.0.5"] {
		t.Error("expected 10.42.0.5 to be in allocated set")
	}
	if allocated["10.42.0.6"] {
		t.Error("10.42.0.6 should not be allocated")
	}
}

func TestInitSchemaIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatalf("second init: %v", err)
	}
}

// ---------------------------------------------------------------------------
// IPAM tests (pure logic, no OS calls)
// ---------------------------------------------------------------------------

func TestGatewayForSubnet(t *testing.T) {
	cases := []struct {
		cidr string
		want string
	}{
		{"10.42.0.0/24", "10.42.0.1"},
		{"192.168.100.0/24", "192.168.100.1"},
		{"172.16.0.0/16", "172.16.0.1"},
	}
	for _, c := range cases {
		got, err := GatewayForSubnet(c.cidr)
		if err != nil {
			t.Errorf("GatewayForSubnet(%q): %v", c.cidr, err)
			continue
		}
		if got != c.want {
			t.Errorf("GatewayForSubnet(%q) = %q, want %q", c.cidr, got, c.want)
		}
	}
}

func TestBridgeNameTruncation(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"short", "capbr-short"},
		{"exactly9ch", "capbr-exactly9c"}, // 15 chars total
		{"toolongname", "capbr-toolongna"},  // 15 chars total
	}
	for _, c := range cases {
		got := BridgeName(c.name)
		if got != c.want {
			t.Errorf("BridgeName(%q) = %q, want %q", c.name, got, c.want)
		}
		if len(got) > 15 {
			t.Errorf("BridgeName(%q) = %q (len %d), exceeds 15 chars", c.name, got, len(got))
		}
	}
}

func TestRandomMACFormat(t *testing.T) {
	for i := 0; i < 10; i++ {
		mac, err := RandomMAC()
		if err != nil {
			t.Fatalf("RandomMAC: %v", err)
		}
		parsed, err := net.ParseMAC(mac)
		if err != nil {
			t.Errorf("RandomMAC produced invalid MAC %q: %v", mac, err)
		}
		// LAA bit set
		if parsed[0]&0x02 == 0 {
			t.Errorf("MAC %q: LAA bit not set", mac)
		}
		// multicast bit clear
		if parsed[0]&0x01 != 0 {
			t.Errorf("MAC %q: multicast bit set", mac)
		}
		// colon-separated hex octets
		parts := strings.Split(mac, ":")
		if len(parts) != 6 {
			t.Errorf("MAC %q: expected 6 octets, got %d", mac, len(parts))
		}
	}
}

func TestAllocateIPSequential(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_seqalloc0000001", "seqnet")
	n.Subnet = "10.99.0.0/29" // 6 host addrs: .1(gw), .2, .3, .4, .5, .6; .7 = broadcast
	n.Gateway = "10.99.0.1"
	s.Insert(n)

	// Allocate all 5 non-gateway usable addresses.
	for i := 2; i <= 6; i++ {
		lease, err := AllocateIP(s, n, fmt.Sprintf("inst_%02d", i), "")
		if err != nil {
			t.Fatalf("AllocateIP #%d: %v", i, err)
		}
		want := fmt.Sprintf("10.99.0.%d", i)
		if lease.IP != want {
			t.Errorf("lease #%d: got %s, want %s", i, lease.IP, want)
		}
	}

	// Next should fail — exhausted.
	_, err := AllocateIP(s, n, "inst_overflow", "")
	if err == nil {
		t.Error("expected exhaustion error, got nil")
	}
}

func TestAllocateIPPreferred(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_preferred000001", "prefnet")
	n.Subnet = "10.99.1.0/24"
	n.Gateway = "10.99.1.1"
	s.Insert(n)

	lease, err := AllocateIP(s, n, "inst_pref", "10.99.1.50")
	if err != nil {
		t.Fatalf("AllocateIP preferred: %v", err)
	}
	if lease.IP != "10.99.1.50" {
		t.Errorf("got %s, want 10.99.1.50", lease.IP)
	}
}

func TestAllocateIPPreferredCollision(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_collision000001", "collnet")
	n.Subnet = "10.99.2.0/24"
	n.Gateway = "10.99.2.1"
	s.Insert(n)

	if _, err := AllocateIP(s, n, "inst_first", "10.99.2.20"); err != nil {
		t.Fatalf("first alloc: %v", err)
	}
	// same IP for a different instance should fail
	_, err := AllocateIP(s, n, "inst_second", "10.99.2.20")
	if err == nil {
		t.Error("expected collision error, got nil")
	}
}

func TestAllocateIPOutOfRange(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_outofrange00001", "rangenet")
	n.Subnet = "10.99.3.0/24"
	n.Gateway = "10.99.3.1"
	s.Insert(n)

	_, err := AllocateIP(s, n, "inst_oob", "192.168.1.1")
	if err == nil {
		t.Error("expected out-of-range error, got nil")
	}
}

func TestAllocateIPGatewayReserved(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_gwreserved00001", "gwnet")
	n.Subnet = "10.99.4.0/24"
	n.Gateway = "10.99.4.1"
	s.Insert(n)

	_, err := AllocateIP(s, n, "inst_gw", "10.99.4.1")
	if err == nil {
		t.Error("expected error allocating gateway address, got nil")
	}
}

func TestReleaseIP(t *testing.T) {
	s := NewStore(openTestDB(t))
	n := testNetwork("net_release0000001", "relnet")
	n.Subnet = "10.99.5.0/29"
	n.Gateway = "10.99.5.1"
	s.Insert(n)

	lease, err := AllocateIP(s, n, "inst_rel", "")
	if err != nil {
		t.Fatalf("alloc: %v", err)
	}
	if err := ReleaseIP(s, n.ID, "inst_rel"); err != nil {
		t.Fatalf("release: %v", err)
	}
	// Re-allocate — same IP should be available again.
	lease2, err := AllocateIP(s, n, "inst_rel2", "")
	if err != nil {
		t.Fatalf("re-alloc after release: %v", err)
	}
	if lease2.IP != lease.IP {
		t.Errorf("expected re-allocated IP %s, got %s", lease.IP, lease2.IP)
	}
}

// ---------------------------------------------------------------------------
// Bridge/veth tests — skipped unless running as root
// ---------------------------------------------------------------------------

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() != 0 {
		t.Skip("bridge/veth tests require root")
	}
}

func TestCreateDeleteBridge(t *testing.T) {
	requireRoot(t)
	bridge := "capbr-test99"
	if err := CreateBridge(bridge, "10.200.99.1", "10.200.99.0/24", ModeIsolated); err != nil {
		t.Fatalf("CreateBridge: %v", err)
	}
	t.Cleanup(func() { DeleteBridge(bridge, "10.200.99.0/24", ModeIsolated) })
	// Idempotent
	if err := CreateBridge(bridge, "10.200.99.1", "10.200.99.0/24", ModeIsolated); err != nil {
		t.Errorf("CreateBridge (idempotent): %v", err)
	}
	if err := DeleteBridge(bridge, "10.200.99.0/24", ModeIsolated); err != nil {
		t.Errorf("DeleteBridge: %v", err)
	}
}

func TestCreateDeleteVeth(t *testing.T) {
	requireRoot(t)
	bridge := "capbr-vethtest"
	if err := CreateBridge(bridge, "10.200.98.1", "10.200.98.0/24", ModeIsolated); err != nil {
		t.Fatalf("CreateBridge: %v", err)
	}
	t.Cleanup(func() { DeleteBridge(bridge, "10.200.98.0/24", ModeIsolated) })

	if err := CreateVeth(bridge, "cvh-testaaaa", "cvi-testaaaa"); err != nil {
		t.Fatalf("CreateVeth: %v", err)
	}
	if err := DeleteVeth("cvh-testaaaa"); err != nil {
		t.Errorf("DeleteVeth: %v", err)
	}
}
