package cappersdk_test

import (
	"testing"

	cappersdk "capper/sdk/go"
)

// TestSDKServiceGroupsList verifies the read paths of the newly-added SDK groups
// against a real server. These exercise SDK → API wiring end to end.
func TestSDKServiceGroupsList(t *testing.T) {
	c := newTestServer(t)

	if _, err := c.Firewalls.List(ctx); err != nil {
		t.Errorf("Firewalls.List: %v", err)
	}
	if _, err := c.Secrets.List(ctx); err != nil {
		t.Errorf("Secrets.List: %v", err)
	}
	if _, err := c.Databases.List(ctx); err != nil {
		t.Errorf("Databases.List: %v", err)
	}
	if _, err := c.Certificates.List(ctx); err != nil {
		t.Errorf("Certificates.List: %v", err)
	}
	if _, err := c.Storage.ListVolumes(ctx, ""); err != nil {
		t.Errorf("Storage.ListVolumes: %v", err)
	}
	if _, err := c.Storage.ListBuckets(ctx); err != nil {
		t.Errorf("Storage.ListBuckets: %v", err)
	}
	if _, err := c.Resources.ListResources(ctx, "", ""); err != nil {
		t.Errorf("Resources.ListResources: %v", err)
	}
	if _, err := c.Functions.List(ctx); err != nil {
		t.Errorf("Functions.List: %v", err)
	}
	if _, err := c.MCP.ListServers(ctx); err != nil {
		t.Errorf("MCP.ListServers: %v", err)
	}
	if _, err := c.IPAM.ListPools(ctx); err != nil {
		t.Errorf("IPAM.ListPools: %v", err)
	}
	if _, err := c.Orgs.List(ctx); err != nil {
		t.Errorf("Orgs.List: %v", err)
	}
	if _, err := c.Stacks.List(ctx); err != nil {
		t.Errorf("Stacks.List: %v", err)
	}
	if _, err := c.Queues.List(ctx); err != nil {
		t.Errorf("Queues.List: %v", err)
	}
	if _, err := c.Backups.List(ctx); err != nil {
		t.Errorf("Backups.List: %v", err)
	}
	if _, err := c.Quotas.List(ctx); err != nil {
		t.Errorf("Quotas.List: %v", err)
	}
	if _, err := c.Ingress.List(ctx); err != nil {
		t.Errorf("Ingress.List: %v", err)
	}
	if _, err := c.AI.ListAgents(ctx); err != nil {
		t.Errorf("AI.ListAgents: %v", err)
	}
	if _, err := c.Autoscale.ListPolicies(ctx); err != nil {
		t.Errorf("Autoscale.ListPolicies: %v", err)
	}
	if _, err := c.Placement.ListPolicies(ctx); err != nil {
		t.Errorf("Placement.ListPolicies: %v", err)
	}
	if _, err := c.InstanceTypes.List(ctx); err != nil {
		t.Errorf("InstanceTypes.List: %v", err)
	}
	if _, err := c.GPU.List(ctx); err != nil {
		t.Errorf("GPU.List: %v", err)
	}
	if _, err := c.Groups.List(ctx); err != nil {
		t.Errorf("Groups.List: %v", err)
	}
	if _, err := c.Governance.ListPolicies(ctx); err != nil {
		t.Errorf("Governance.ListPolicies: %v", err)
	}
	if _, err := c.Marketplace.List(ctx); err != nil {
		t.Errorf("Marketplace.List: %v", err)
	}
	if _, err := c.NodePools.List(ctx); err != nil {
		t.Errorf("NodePools.List: %v", err)
	}
	if _, err := c.Posture.ListFindings(ctx); err != nil {
		t.Errorf("Posture.ListFindings: %v", err)
	}
	if _, err := c.CSD.ListVolumes(ctx); err != nil {
		t.Errorf("CSD.ListVolumes: %v", err)
	}
	if _, err := c.Migrations.List(ctx); err != nil {
		t.Errorf("Migrations.List: %v", err)
	}
	if _, err := c.Health.ListChecks(ctx); err != nil {
		t.Errorf("Health.ListChecks: %v", err)
	}
	if _, err := c.BackupPolicies.ListPolicies(ctx); err != nil {
		t.Errorf("BackupPolicies.ListPolicies: %v", err)
	}
}

func TestSDKSecretLifecycle(t *testing.T) {
	c := newTestServer(t)
	if err := c.Secrets.Create(ctx, "sdk-secret", "s3cr3t"); err != nil {
		t.Fatalf("Secrets.Create: %v", err)
	}
	metas, err := c.Secrets.List(ctx)
	if err != nil {
		t.Fatalf("Secrets.List: %v", err)
	}
	found := false
	for _, m := range metas {
		if m.Name == "sdk-secret" {
			found = true
		}
	}
	if !found {
		t.Error("created secret not present in list")
	}
	if err := c.Secrets.Delete(ctx, "sdk-secret"); err != nil {
		t.Errorf("Secrets.Delete: %v", err)
	}
}

func TestSDKFunctionLifecycle(t *testing.T) {
	c := newTestServer(t)
	fn, err := c.Functions.Create(ctx, cappersdk.Function{Name: "sdk-echo", Runtime: "native", Command: []string{"/bin/cat"}})
	if err != nil {
		t.Fatalf("Functions.Create: %v", err)
	}
	res, err := c.Functions.Invoke(ctx, fn.ID, []byte("ping"))
	if err != nil {
		t.Fatalf("Functions.Invoke: %v", err)
	}
	if res.Status != "succeeded" {
		t.Errorf("expected succeeded, got %q (%s)", res.Status, res.Error)
	}
	if err := c.Functions.Delete(ctx, fn.ID); err != nil {
		t.Errorf("Functions.Delete: %v", err)
	}
}

func TestSDKIPAMLifecycle(t *testing.T) {
	c := newTestServer(t)
	if err := c.IPAM.CreatePool(ctx, cappersdk.IPPool{
		Name: "sdk-pool", CIDR: "203.0.113.0/29", Gateway: "203.0.113.1",
		Usage: []string{"load-balancer"}, AllowAutoAllocate: true,
	}, nil); err != nil {
		t.Fatalf("IPAM.CreatePool: %v", err)
	}
	ip, err := c.IPAM.Reserve(ctx, "sdk-pool", "sdk-ip", "load-balancer", "")
	if err != nil {
		t.Fatalf("IPAM.Reserve: %v", err)
	}
	if ip.Status != "reserved" {
		t.Errorf("expected reserved, got %q", ip.Status)
	}
	if err := c.IPAM.Release(ctx, ip.ID); err != nil {
		t.Errorf("IPAM.Release: %v", err)
	}
}
