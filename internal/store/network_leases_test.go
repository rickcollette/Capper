package store

import (
	"testing"

	"capper/internal/network"
	"capper/internal/types"
)

func TestPruneOrphanedNetworkLeases(t *testing.T) {
	st, err := Open(NewPaths(t.TempDir()))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if err := st.Networks.Insert(network.Network{
		ID: "net_test00000001", Name: "testnet", Project: "default", Mode: "nat",
		Subnet: "10.42.0.0/24", Gateway: "10.42.0.1", Bridge: "cap-test", Status: "ready",
		CreatedAt: "2026-06-18T00:00:00Z",
	}); err != nil {
		t.Fatalf("insert network: %v", err)
	}

	// One live instance, one already-deleted instance.
	if err := st.InsertInstance(types.Instance{
		ID: "inst_live000001", Name: "live", Status: types.StatusRunning,
		CreatedAt: "2026-06-18T00:00:00Z", RootFSPath: "/tmp/x",
	}); err != nil {
		t.Fatalf("insert instance: %v", err)
	}
	_ = st.Networks.InsertLease(network.NetworkLease{NetworkID: "net_test00000001", InstanceID: "inst_live000001", IP: "10.42.0.2"})
	_ = st.Networks.InsertLease(network.NetworkLease{NetworkID: "net_test00000001", InstanceID: "inst_gone000001", IP: "10.42.0.3"})

	removed, err := st.PruneOrphanedNetworkLeases()
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 orphan lease pruned, got %d", removed)
	}

	attached, err := st.LiveNetworkAttachments("net_test00000001")
	if err != nil {
		t.Fatalf("attachments: %v", err)
	}
	if len(attached) != 1 || attached[0].InstanceID != "inst_live000001" {
		t.Fatalf("expected only the live instance attached, got %+v", attached)
	}
}
