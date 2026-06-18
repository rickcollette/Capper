package cappersdk_test

import (
	"testing"

	"capper/internal/compute"
	"capper/internal/marketplace"
)

// TestSDKFieldRoundTrip seeds resources directly in the store and reads them
// back through the SDK, asserting that key fields actually deserialize. This
// catches JSON-tag mismatches that a no-error List() check would miss.
func TestSDKFieldRoundTrip(t *testing.T) {
	env := newTestEnv(t)

	// Instance type → SDK InstanceTypes.List
	if err := env.Store.Compute.UpsertInstanceType(compute.InstanceType{
		ID: "it_test", Name: "cap-test", Family: "compute", CPUCount: 4, MemoryBytes: 8 << 30, GPUEligible: true,
	}); err != nil {
		t.Fatalf("seed instance type: %v", err)
	}
	its, err := env.Client.InstanceTypes.List(ctx)
	if err != nil {
		t.Fatalf("InstanceTypes.List: %v", err)
	}
	found := false
	for _, x := range its {
		if x.Name == "cap-test" {
			found = true
			if x.CPUCount != 4 || x.MemoryBytes != 8<<30 || x.Family != "compute" || !x.GPUEligible {
				t.Errorf("instance type fields did not deserialize: %+v", x)
			}
		}
	}
	if !found {
		t.Error("seeded instance type not returned by SDK")
	}

	// GPU device → SDK GPU.List
	if err := env.Store.Compute.UpsertGPUDevice(compute.GPUDevice{
		ID: "gpu_test", Vendor: "nvidia", Model: "A100", MemoryBytes: 40 << 30, Status: "available",
	}); err != nil {
		t.Fatalf("seed gpu: %v", err)
	}
	gpus, err := env.Client.GPU.List(ctx)
	if err != nil {
		t.Fatalf("GPU.List: %v", err)
	}
	gpuFound := false
	for _, g := range gpus {
		if g.ID == "gpu_test" {
			gpuFound = true
			if g.Vendor != "nvidia" || g.Model != "A100" || g.MemoryBytes != 40<<30 || g.Status != "available" {
				t.Errorf("gpu fields did not deserialize: %+v", g)
			}
		}
	}
	if !gpuFound {
		t.Error("seeded gpu not returned by SDK")
	}

	// Marketplace listing seeded via the SQLite store must appear via the SDK —
	// this is the regression guard for the CLI/API dual-backend unification.
	if err := env.Store.Marketplace.Insert(marketplace.MarketplaceListing{
		ID: "mkt_test", Name: "test-image", Version: "1.0.0", Status: "approved",
		ScanStatus: "pass", ScanSeverities: map[string]int{"high": 0, "medium": 1},
	}); err != nil {
		t.Fatalf("seed marketplace listing: %v", err)
	}
	listings, err := env.Client.Marketplace.List(ctx)
	if err != nil {
		t.Fatalf("Marketplace.List: %v", err)
	}
	mktFound := false
	for _, l := range listings {
		if l.ID == "mkt_test" {
			mktFound = true
			if l.Name != "test-image" || l.Status != "approved" || l.ScanStatus != "pass" {
				t.Errorf("marketplace fields did not deserialize: %+v", l)
			}
		}
	}
	if !mktFound {
		t.Error("seeded marketplace listing not returned by SDK (CLI/API backends still split?)")
	}
}
