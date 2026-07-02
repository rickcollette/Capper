// screendemo seeds a Capper store with deterministic demo data for marketing
// and documentation screenshots.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"capper/internal/ai"
	"capper/internal/backup"
	"capper/internal/compute"
	"capper/internal/database"
	capperdns "capper/internal/dns"
	"capper/internal/firewall"
	"capper/internal/lb"
	"capper/internal/marketplace"
	"capper/internal/network"
	"capper/internal/posture"
	"capper/internal/resourcemon"
	"capper/internal/stack"
	"capper/internal/storage"
	"capper/internal/store"
	"capper/internal/topology"
	"capper/internal/types"
	"capper/internal/vpc"
)

const project = "default"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: screendemo <store-root>")
		os.Exit(2)
	}
	st, err := store.Open(store.NewPaths(os.Args[1]))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()

	if err := seed(st); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func seed(st *store.Store) error {
	if err := cleanup(st); err != nil {
		return err
	}
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	ts := func(mins int) string { return now.Add(time.Duration(mins) * time.Minute).Format(time.RFC3339) }

	if err := seedImages(st, ts); err != nil {
		return err
	}
	if err := seedNetworking(st, ts); err != nil {
		return err
	}
	if err := seedTopology(st, ts); err != nil {
		return err
	}
	if err := seedCompute(st, ts); err != nil {
		return err
	}
	if err := seedStorage(st, ts); err != nil {
		return err
	}
	if err := seedLoadBalancing(st, ts); err != nil {
		return err
	}
	if err := seedPlatform(st, ts); err != nil {
		return err
	}
	return nil
}

func cleanup(st *store.Store) error {
	tables := []struct {
		name string
		col  string
	}{
		{"node_pool_members", "node_id"},
		{"node_pool_roles", "pool_id"},
		{"node_pool_labels", "pool_id"},
		{"node_pools", "id"},
		{"node_heartbeats", "node_id"},
		{"node_services", "node_id"},
		{"node_taints", "node_id"},
		{"node_labels", "node_id"},
		{"node_roles", "node_id"},
		{"nodes", "id"},
		{"zones", "id"},
		{"regions", "id"},
		{"realms", "id"},
		{"instances", "id"},
		{"images", "id"},
		{"networks", "id"},
		{"network_leases", "instance_id"},
		{"capvpc_vpcs", "id"},
		{"capvpc_subnets", "id"},
		{"capvpc_route_tables", "id"},
		{"capvpc_routes", "id"},
		{"capvpc_security_groups", "id"},
		{"capvpc_sg_rules", "id"},
		{"capvpc_internet_gateways", "id"},
		{"storage_volumes", "id"},
		{"storage_buckets", "id"},
		{"storage_snapshots", "id"},
		{"lb_load_balancers", "id"},
		{"lb_backends", "id"},
		{"lb_target_groups", "id"},
		{"lb_listeners", "id"},
		{"lb_target_group_targets", "id"},
		{"ingress_rules", "id"},
		{"waf_rules", "id"},
		{"static_sites", "id"},
		{"dns_zones", "id"},
		{"dns_records", "id"},
		{"dns_service_records", "id"},
		{"firewalls", "network_id"},
		{"firewall_rules", "id"},
		{"managed_databases", "id"},
		{"db_backups", "id"},
		{"backup_records", "id"},
		{"backup_policies", "id"},
		{"stacks", "id"},
		{"posture_findings", "id"},
		{"ai_agents", "id"},
		{"ai_sessions", "id"},
		{"ai_mcp_servers", "id"},
		{"ai_tool_calls", "id"},
		{"marketplace_listings", "id"},
		{"rmon_resources", "id"},
		{"rmon_metric_samples", "id"},
		{"rmon_events", "id"},
		{"account_quotas", "account_id"},
		{"resource_usage", "resource_id"},
		{"storage_pools", "id"},
		{"storage_allocations", "id"},
		{"compute_hosts", "id"},
		{"compute_templates", "id"},
		{"compute_groups", "id"},
	}
	for _, t := range tables {
		if _, err := st.DB.Exec(fmt.Sprintf(`DELETE FROM %s WHERE %s LIKE 'demo-%%'`, t.name, t.col)); err != nil && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("cleanup %s: %w", t.name, err)
		}
	}
	return nil
}

func seedTopology(st *store.Store, ts func(int) string) error {
	top := topology.NewStore(st.DB)
	realm := topology.Realm{
		ID:          "demo-realm-capcloud",
		Slug:        "capcloud",
		Name:        "CapCloud Demo",
		Description: "Demo realm for documentation screenshots",
		Status:      topology.StatusActive,
		Labels:      map[string]string{"environment": "demo", "owner": "platform"},
		CreatedAt:   ts(-220),
	}
	if err := top.InsertRealm(realm); err != nil {
		return err
	}
	region := topology.Region{
		ID:          "demo-region-us-east",
		RealmID:     realm.ID,
		Slug:        "us-east-lab",
		Name:        "US East Lab",
		Description: "Primary beta validation region",
		Location:    "Ashburn, VA",
		Country:     "US",
		RegionCode:  "use1",
		Latitude:    39.0438,
		Longitude:   -77.4874,
		Status:      topology.StatusActive,
		ControlURL:  "https://control.use1.capper.example",
		APIURL:      "https://api.use1.capper.example",
		Labels:      map[string]string{"latency": "low", "tier": "beta"},
		CreatedAt:   ts(-218),
	}
	if err := top.InsertRegion(region); err != nil {
		return err
	}
	for _, z := range []topology.Zone{
		{ID: "demo-zone-use1-a", RealmID: realm.ID, RegionID: region.ID, Slug: "use1-a", Name: "USE1 A", Description: "General compute and edge", FailureDomain: "rack-a", Status: topology.StatusActive, ControlURL: "https://zone-a.use1.capper.example", NetworkCIDR: "10.80.0.0/17", Labels: map[string]string{"class": "mixed"}, CreatedAt: ts(-216)},
		{ID: "demo-zone-use1-b", RealmID: realm.ID, RegionID: region.ID, Slug: "use1-b", Name: "USE1 B", Description: "Storage and database workloads", FailureDomain: "rack-b", Status: topology.StatusActive, ControlURL: "https://zone-b.use1.capper.example", NetworkCIDR: "10.80.128.0/17", Labels: map[string]string{"class": "data"}, CreatedAt: ts(-215)},
	} {
		if err := top.InsertZone(z); err != nil {
			return err
		}
	}
	nodes := []topology.Node{
		{ID: "demo-node-edge-01", RealmID: realm.ID, RegionID: region.ID, ZoneID: "demo-zone-use1-a", Slug: "edge-01", Name: "edge-01", Address: "10.0.0.11", Status: topology.StatusReady, FailureDomain: "rack-a", Labels: map[string]string{"role": "edge", "network": "public"}, CPUCount: 16, MemoryBytes: 64 << 30, DiskBytes: 2 << 40, Roles: []string{"edge", "compute"}, AgentVersion: "0.9.0-beta", LastHeartbeat: ts(-1), CreatedAt: ts(-200)},
		{ID: "demo-node-compute-02", RealmID: realm.ID, RegionID: region.ID, ZoneID: "demo-zone-use1-a", Slug: "compute-02", Name: "compute-02", Address: "10.0.0.12", Status: topology.StatusReady, FailureDomain: "rack-a", Labels: map[string]string{"role": "compute", "pool": "general"}, CPUCount: 32, MemoryBytes: 128 << 30, DiskBytes: 4 << 40, Roles: []string{"compute"}, AgentVersion: "0.9.0-beta", LastHeartbeat: ts(-1), CreatedAt: ts(-198)},
		{ID: "demo-node-storage-01", RealmID: realm.ID, RegionID: region.ID, ZoneID: "demo-zone-use1-b", Slug: "storage-01", Name: "storage-01", Address: "10.0.0.21", Status: topology.StatusCordoned, FailureDomain: "rack-b", Labels: map[string]string{"role": "storage", "maintenance": "planned"}, CPUCount: 24, MemoryBytes: 96 << 30, DiskBytes: 12 << 40, Roles: []string{"storage", "database"}, Cordoned: true, AgentVersion: "0.9.0-beta", LastHeartbeat: ts(-2), CreatedAt: ts(-190)},
		{ID: "demo-node-gpu-01", RealmID: realm.ID, RegionID: region.ID, ZoneID: "demo-zone-use1-b", Slug: "gpu-01", Name: "gpu-01", Address: "10.0.0.31", Status: topology.StatusReady, FailureDomain: "rack-b", Labels: map[string]string{"role": "gpu", "pool": "ai"}, CPUCount: 48, MemoryBytes: 256 << 30, DiskBytes: 8 << 40, Roles: []string{"compute", "gpu"}, AgentVersion: "0.9.0-beta", LastHeartbeat: ts(-1), GPUCount: 4, GPUMemoryBytes: 96 << 30, CreatedAt: ts(-188)},
	}
	for _, n := range nodes {
		created, err := top.InsertNode(n)
		if err != nil {
			return err
		}
		if err := top.SetNodeRoles(created.ID, n.Roles); err != nil {
			return err
		}
		if err := top.UpsertHeartbeat(topology.NodeHeartbeat{NodeID: created.ID, Status: "healthy", CPUUsed: 30 + len(n.Roles)*9, MemoryUsedBytes: n.MemoryBytes / 3, DiskUsedBytes: n.DiskBytes / 4, GPUUsed: n.GPUCount / 2, Message: "demo heartbeat", SeenAt: n.LastHeartbeat}); err != nil {
			return err
		}
	}
	if _, err := top.CreatePool(topology.NodePool{ID: "demo-pool-general", Name: "general-compute", RealmID: realm.ID, RegionID: region.ID, ZoneID: "demo-zone-use1-a", Roles: []string{"compute"}, MinNodes: 2, DesiredNodes: 3, MaxNodes: 8, PlacementPolicy: "spread", Status: topology.StatusActive, CreatedAt: ts(-180)}); err != nil {
		return err
	}
	return nil
}

func seedImages(st *store.Store, ts func(int) string) error {
	for i, img := range []types.ImageRecord{
		{ID: "demo-img-ubuntu-web", Name: "ubuntu-web.cap", Version: "24.04.1", Path: "store/images/ubuntu-web.cap", SizeBytes: 39845888, Digest: "sha256:demo-ubuntu-web"},
		{ID: "demo-img-alpine-worker", Name: "alpine-worker.cap", Version: "3.20.2", Path: "store/images/alpine-worker.cap", SizeBytes: 16564224, Digest: "sha256:demo-alpine-worker"},
		{ID: "demo-img-rocky-db", Name: "rocky-db.cap", Version: "10-beta", Path: "store/images/rocky-db.cap", SizeBytes: 84275200, Digest: "sha256:demo-rocky-db"},
		{ID: "demo-img-homeassistant", Name: "homeassistant.cap", Version: "2026.7", Path: "store/images/homeassistant.cap", SizeBytes: 126615552, Digest: "sha256:demo-homeassistant"},
	} {
		img.CreatedAt = ts(-180 + i*8)
		if err := st.UpsertImage(img); err != nil {
			return err
		}
	}
	return nil
}

func seedNetworking(st *store.Store, ts func(int) string) error {
	networks := []network.Network{
		{ID: "demo-net-prod", Name: "prod-shared", Project: project, Mode: network.ModeNAT, Subnet: "10.42.0.0/24", Gateway: "10.42.0.1", Bridge: "cap-prod0", Labels: map[string]string{"env": "prod", "tier": "shared"}, Status: network.StatusActive, CreatedAt: ts(-160)},
		{ID: "demo-net-edge", Name: "edge-dmz", Project: project, Mode: network.ModeHostExposed, Subnet: "10.43.0.0/24", Gateway: "10.43.0.1", Bridge: "cap-edge0", Labels: map[string]string{"env": "prod", "tier": "edge"}, Status: network.StatusActive, CreatedAt: ts(-150)},
		{ID: "demo-net-lab", Name: "ai-lab", Project: project, Mode: network.ModeIsolated, Subnet: "10.44.0.0/24", Gateway: "10.44.0.1", Bridge: "cap-ai0", Labels: map[string]string{"env": "lab", "team": "platform"}, Status: network.StatusActive, CreatedAt: ts(-140)},
	}
	for _, n := range networks {
		if err := st.Networks.Insert(n); err != nil {
			return err
		}
	}

	vpcStore := vpc.NewStore(st.DB)
	vpcs := []vpc.VPC{
		{ID: "demo-vpc-prod", Project: project, Name: "production", Slug: "production", Description: "Customer-facing workloads", PrimaryIPv4CIDR: "10.80.0.0/16", CIDR: "10.80.0.0/16", DNSDomain: "prod.capper.internal", DNSSupport: true, DNSHostnames: true, Status: vpc.VPCStatusAvailable, EnableFlowLogs: true, Labels: map[string]string{"env": "prod"}, CreatedAt: ts(-155), UpdatedAt: ts(-15)},
		{ID: "demo-vpc-dev", Project: project, Name: "developer-sandbox", Slug: "developer-sandbox", Description: "Ephemeral dev/test workloads", PrimaryIPv4CIDR: "10.90.0.0/16", CIDR: "10.90.0.0/16", DNSDomain: "dev.capper.internal", DNSSupport: true, DNSHostnames: true, Status: vpc.VPCStatusAvailable, Labels: map[string]string{"env": "dev"}, CreatedAt: ts(-120), UpdatedAt: ts(-10)},
	}
	for _, v := range vpcs {
		if err := vpcStore.InsertVPC(v); err != nil {
			return err
		}
	}
	subnets := []vpc.Subnet{
		{ID: "demo-subnet-prod-public-a", VPCID: "demo-vpc-prod", Name: "public-a", CIDR: "10.80.1.0/24", Kind: vpc.SubnetPublic, SubnetType: vpc.SubnetPublic, Zone: "local-a", GatewayIP: "10.80.1.1", BridgeName: "cap-pub-a", AutoAssignPublicIP: true, AvailableIPCount: 212, Status: "available", CreatedAt: ts(-154)},
		{ID: "demo-subnet-prod-app-a", VPCID: "demo-vpc-prod", Name: "app-a", CIDR: "10.80.11.0/24", Kind: vpc.SubnetPrivate, SubnetType: vpc.SubnetPrivate, Zone: "local-a", GatewayIP: "10.80.11.1", BridgeName: "cap-app-a", AvailableIPCount: 197, Status: "available", CreatedAt: ts(-153)},
		{ID: "demo-subnet-prod-data-a", VPCID: "demo-vpc-prod", Name: "data-a", CIDR: "10.80.21.0/24", Kind: vpc.SubnetIsolated, SubnetType: vpc.SubnetIsolated, Zone: "local-a", GatewayIP: "10.80.21.1", BridgeName: "cap-data-a", AvailableIPCount: 226, Status: "available", CreatedAt: ts(-152)},
		{ID: "demo-subnet-dev-a", VPCID: "demo-vpc-dev", Name: "sandbox-a", CIDR: "10.90.10.0/24", Kind: vpc.SubnetPrivate, SubnetType: vpc.SubnetPrivate, Zone: "local-a", GatewayIP: "10.90.10.1", BridgeName: "cap-dev-a", AvailableIPCount: 239, Status: "available", CreatedAt: ts(-119)},
	}
	for _, s := range subnets {
		if err := vpcStore.InsertSubnet(s); err != nil {
			return err
		}
	}
	return nil
}

func seedCompute(st *store.Store, ts func(int) string) error {
	hosts := []compute.Host{
		{ID: "demo-host-edge-01", Name: "edge-01", Address: "10.0.0.11", Status: compute.HostStatusReady, Labels: map[string]string{"zone": "local-a", "role": "edge"}, CPUCount: 16, MemoryBytes: 64 << 30, CreatedAt: ts(-200), UpdatedAt: ts(-5)},
		{ID: "demo-host-compute-02", Name: "compute-02", Address: "10.0.0.12", Status: compute.HostStatusReady, Labels: map[string]string{"zone": "local-a", "role": "compute"}, CPUCount: 32, MemoryBytes: 128 << 30, CreatedAt: ts(-198), UpdatedAt: ts(-4)},
		{ID: "demo-host-storage-01", Name: "storage-01", Address: "10.0.0.21", Status: compute.HostStatusCordoned, Labels: map[string]string{"zone": "local-a", "role": "storage", "maintenance": "planned"}, CPUCount: 24, MemoryBytes: 96 << 30, CreatedAt: ts(-190), UpdatedAt: ts(-2)},
	}
	for _, h := range hosts {
		if err := st.Compute.UpsertHost(h); err != nil {
			return err
		}
	}

	instances := []types.Instance{
		{ID: "demo-inst-web-01", Name: "prod-web-01", Image: "ubuntu-web.cap", ImageID: "demo-img-ubuntu-web", ImageDigest: "sha256:demo-ubuntu-web", PID: 1, Status: types.StatusRunning, CreatedAt: ts(-80), StartedAt: ts(-79), RootFSPath: filepath.Join(st.Paths.Instances, "demo-inst-web-01", "rootfs"), Command: "nginx -g 'daemon off;'", NetworkID: "demo-net-prod", NetworkIP: "10.42.0.21", InstanceType: "small", NodeID: "demo-host-compute-02", Labels: map[string]string{"app": "web", "env": "prod"}},
		{ID: "demo-inst-web-02", Name: "prod-web-02", Image: "ubuntu-web.cap", ImageID: "demo-img-ubuntu-web", ImageDigest: "sha256:demo-ubuntu-web", PID: 1, Status: types.StatusRunning, CreatedAt: ts(-78), StartedAt: ts(-77), RootFSPath: filepath.Join(st.Paths.Instances, "demo-inst-web-02", "rootfs"), Command: "nginx -g 'daemon off;'", NetworkID: "demo-net-prod", NetworkIP: "10.42.0.22", InstanceType: "small", NodeID: "demo-host-edge-01", Labels: map[string]string{"app": "web", "env": "prod"}},
		{ID: "demo-inst-worker-01", Name: "queue-worker-01", Image: "alpine-worker.cap", ImageID: "demo-img-alpine-worker", ImageDigest: "sha256:demo-alpine-worker", PID: 1, Status: types.StatusRunning, CreatedAt: ts(-65), StartedAt: ts(-64), RootFSPath: filepath.Join(st.Paths.Instances, "demo-inst-worker-01", "rootfs"), Command: "/usr/local/bin/worker --queue image-jobs", NetworkID: "demo-net-prod", NetworkIP: "10.42.0.37", InstanceType: "medium", NodeID: "demo-host-compute-02", Labels: map[string]string{"app": "worker", "queue": "image-jobs"}},
		{ID: "demo-inst-db-01", Name: "postgres-primary", Image: "rocky-db.cap", ImageID: "demo-img-rocky-db", ImageDigest: "sha256:demo-rocky-db", PID: 1, Status: types.StatusRunning, CreatedAt: ts(-60), StartedAt: ts(-59), RootFSPath: filepath.Join(st.Paths.Instances, "demo-inst-db-01", "rootfs"), Command: "postgres -D /var/lib/postgresql/data", NetworkID: "demo-net-prod", NetworkIP: "10.42.0.50", InstanceType: "large", NodeID: "demo-host-storage-01", Labels: map[string]string{"app": "postgres", "role": "primary"}},
		{ID: "demo-inst-ai-01", Name: "agent-lab-gpu-01", Image: "ubuntu-web.cap", ImageID: "demo-img-ubuntu-web", ImageDigest: "sha256:demo-ubuntu-web", PID: 0, Status: types.StatusStopped, CreatedAt: ts(-40), StartedAt: ts(-39), RootFSPath: filepath.Join(st.Paths.Instances, "demo-inst-ai-01", "rootfs"), Command: "python serve.py", NetworkID: "demo-net-lab", NetworkIP: "10.44.0.20", InstanceType: "gpu-small", Labels: map[string]string{"app": "ai-lab", "gpu": "true"}},
	}
	for _, inst := range instances {
		if err := os.MkdirAll(filepath.Dir(inst.RootFSPath), 0o755); err != nil {
			return err
		}
		if err := st.InsertInstance(inst); err != nil {
			return err
		}
		if err := st.WriteInstanceJSON(inst); err != nil {
			return err
		}
	}
	leases := []network.NetworkLease{
		{NetworkID: "demo-net-prod", InstanceID: "demo-inst-web-01", IP: "10.42.0.21", MAC: "02:42:0a:2a:00:21", CreatedAt: ts(-79)},
		{NetworkID: "demo-net-prod", InstanceID: "demo-inst-web-02", IP: "10.42.0.22", MAC: "02:42:0a:2a:00:22", CreatedAt: ts(-77)},
		{NetworkID: "demo-net-prod", InstanceID: "demo-inst-worker-01", IP: "10.42.0.37", MAC: "02:42:0a:2a:00:37", CreatedAt: ts(-64)},
		{NetworkID: "demo-net-prod", InstanceID: "demo-inst-db-01", IP: "10.42.0.50", MAC: "02:42:0a:2a:00:50", CreatedAt: ts(-59)},
		{NetworkID: "demo-net-lab", InstanceID: "demo-inst-ai-01", IP: "10.44.0.20", MAC: "02:42:0a:2c:00:20", CreatedAt: ts(-39)},
	}
	for _, l := range leases {
		if err := st.Networks.InsertLease(l); err != nil {
			return err
		}
	}
	if err := st.Compute.InsertTemplate(compute.Template{ID: "demo-tmpl-web", Name: "web-service", Image: "ubuntu-web.cap", Runtime: "bwrap", CreatedAt: ts(-90), Doc: compute.TemplateDoc{Name: "web-service", Image: "ubuntu-web.cap", Runtime: "bwrap", InstanceTypeName: "small", Labels: map[string]string{"app": "web"}}}); err != nil {
		return err
	}
	return st.Compute.InsertGroup(compute.Group{ID: "demo-group-web", Name: "web-tier", TemplateID: "demo-tmpl-web", TemplateName: "web-service", MinSize: 2, DesiredSize: 4, MaxSize: 8, Status: compute.GroupStatusScaling, CreatedAt: ts(-88)})
}

func seedStorage(st *store.Store, ts func(int) string) error {
	vols := []storage.Volume{
		{ID: "demo-vol-pgdata", Name: "pgdata-primary", SizeBytes: 200 << 30, Class: storage.VolumeClassLocalSSD, Backend: storage.VolumeBackendDirectory, Path: filepath.Join(st.Paths.Root, "volumes/pgdata-primary"), Encrypted: true, AttachedInstanceID: "demo-inst-db-01", AttachedPath: "/var/lib/postgresql/data", CreatedAt: ts(-58)},
		{ID: "demo-vol-media", Name: "media-library", SizeBytes: 2 << 40, Class: storage.VolumeClassLocal, Backend: storage.VolumeBackendDirectory, Path: filepath.Join(st.Paths.Root, "volumes/media-library"), Encrypted: true, CreatedAt: ts(-56)},
		{ID: "demo-vol-ai-cache", Name: "ai-model-cache", SizeBytes: 500 << 30, Class: storage.VolumeClassLocalSSD, Backend: storage.VolumeBackendDirectory, Path: filepath.Join(st.Paths.Root, "volumes/ai-model-cache"), Encrypted: false, AttachedInstanceID: "demo-inst-ai-01", AttachedPath: "/models", CreatedAt: ts(-38)},
	}
	for _, v := range vols {
		if err := st.Storage.InsertVolume(v); err != nil {
			return err
		}
	}
	buckets := []storage.Bucket{
		{ID: "demo-bucket-artifacts", Name: "release-artifacts", Backend: storage.BucketBackendLocal, Path: filepath.Join(st.Paths.Root, "buckets/release-artifacts"), Versioning: true, Encrypted: true, QuotaBytes: 500 << 30, CreatedAt: ts(-52)},
		{ID: "demo-bucket-public", Name: "static-site-public", Backend: storage.BucketBackendLocal, Path: filepath.Join(st.Paths.Root, "buckets/static-site-public"), Versioning: true, Encrypted: false, QuotaBytes: 50 << 30, CreatedAt: ts(-51)},
		{ID: "demo-bucket-logs", Name: "audit-log-archive", Backend: storage.BucketBackendLocal, Path: filepath.Join(st.Paths.Root, "buckets/audit-log-archive"), Versioning: true, Encrypted: true, QuotaBytes: 5 << 40, CreatedAt: ts(-49)},
	}
	for _, b := range buckets {
		if err := st.Storage.InsertBucket(b); err != nil {
			return err
		}
	}
	return nil
}

func seedLoadBalancing(st *store.Store, ts func(int) string) error {
	lbStore := lb.NewStore(st.DB)
	lbs := []lb.LoadBalancer{
		{ID: "demo-lb-web", Name: "web-public", Project: project, VPCID: "demo-vpc-prod", SubnetID: "demo-subnet-prod-public-a", Scheme: lb.SchemeInternetFacing, Type: lb.TypeApplication, VIPAddress: "10.80.1.20", DNSName: "web.prod.capper.internal", Mode: lb.ModeHTTP, ListenAddr: "0.0.0.0:8080", Status: lb.StatusActive, Algorithm: lb.AlgoLeastConnections, Selector: "app=web", CreatedAt: ts(-45)},
		{ID: "demo-lb-internal-api", Name: "internal-api", Project: project, VPCID: "demo-vpc-prod", SubnetID: "demo-subnet-prod-app-a", Scheme: lb.SchemeInternal, Type: lb.TypeApplication, VIPAddress: "10.80.11.15", DNSName: "api.prod.capper.internal", Mode: lb.ModeHTTPS, ListenAddr: "10.80.11.15:443", Status: lb.StatusActive, Algorithm: lb.AlgoRoundRobin, Selector: "app=api", TLSCertName: "wildcard-prod", CreatedAt: ts(-42)},
	}
	for _, l := range lbs {
		if err := lbStore.Insert(l); err != nil {
			return err
		}
	}
	if _, err := lbStore.AddBackend("demo-lb-web", "10.42.0.21:8080"); err != nil {
		return err
	}
	if _, err := lbStore.AddBackend("demo-lb-web", "10.42.0.22:8080"); err != nil {
		return err
	}
	if _, err := st.Ingress.Create("web-prod", project, "app.example.test", "/", "web-public", "wildcard-prod", 1200); err != nil {
		return err
	}
	return nil
}

func seedPlatform(st *store.Store, ts func(int) string) error {
	dnsZone := capperdns.Zone{ID: "demo-zone-prod", Name: "prod.capper.internal", Type: capperdns.ZoneTypePrivate, NetworkID: "demo-net-prod", DefaultTTL: 60, Description: "Production service discovery", Labels: map[string]string{"env": "prod"}, CreatedAt: ts(-44)}
	if err := st.DNS.InsertZone(dnsZone); err != nil {
		return err
	}
	for _, r := range []capperdns.Record{
		{ID: "demo-dns-web", ZoneID: dnsZone.ID, Name: "web", FQDN: "web.prod.capper.internal", Type: capperdns.RecordTypeA, Values: []string{"10.80.1.20"}, TTL: 60, Source: capperdns.RecordSourceLB, Enabled: true, CreatedAt: ts(-43)},
		{ID: "demo-dns-db", ZoneID: dnsZone.ID, Name: "postgres", FQDN: "postgres.prod.capper.internal", Type: capperdns.RecordTypeA, Values: []string{"10.42.0.50"}, TTL: 60, Source: capperdns.RecordSourceInstance, Enabled: true, CreatedAt: ts(-42)},
		{ID: "demo-dns-api", ZoneID: dnsZone.ID, Name: "api", FQDN: "api.prod.capper.internal", Type: capperdns.RecordTypeCNAME, Values: []string{"web.prod.capper.internal"}, TTL: 120, Source: capperdns.RecordSourceManual, Enabled: true, CreatedAt: ts(-41)},
	} {
		if err := st.DNS.InsertRecord(r); err != nil {
			return err
		}
	}
	if err := st.Firewalls.Insert(firewall.Firewall{NetworkID: "demo-net-prod", NetworkName: "prod-shared", Mode: firewall.ModeStrict, Backend: "nftables", DefaultIngressPolicy: firewall.ActionDeny, DefaultEgressPolicy: firewall.ActionAllow, DefaultForwardPolicy: firewall.ActionDeny, AllowDNS: true, AllowEstablished: true, NATEnabled: true, Status: firewall.StatusApplied, LastAppliedAt: ts(-9)}); err != nil {
		return err
	}
	if err := st.Firewalls.InsertRule(firewall.Rule{ID: "demo-fw-https", NetworkID: "demo-net-prod", Priority: 100, Enabled: true, Action: firewall.ActionAllow, Direction: firewall.DirectionIngress, From: firewall.Endpoint{Type: firewall.EndpointInternet}, To: firewall.Endpoint{Type: firewall.EndpointLabel, Key: "app", Value: "web"}, Protocol: "tcp", Ports: []int{443, 8080}, Description: "Public HTTPS to web tier", CreatedAt: ts(-8)}); err != nil {
		return err
	}
	dbStore := database.NewStore(st.DB)
	for _, db := range []database.ManagedDB{
		{ID: "demo-db-primary", Name: "orders-primary", Project: project, Engine: database.EnginePostgres, Version: "16", Status: database.DBStatusRunning, NetworkID: "demo-net-prod", InstanceID: "demo-inst-db-01", VolumeID: "demo-vol-pgdata", SecretName: "orders-db-password", DNSName: "postgres.prod.capper.internal", Port: 5432, CreatedAt: ts(-36)},
		{ID: "demo-db-cache", Name: "session-cache", Project: project, Engine: database.EngineRedis, Version: "7", Status: database.DBStatusRunning, NetworkID: "demo-net-prod", DNSName: "redis.prod.capper.internal", Port: 6379, CreatedAt: ts(-35)},
	} {
		if err := dbStore.Insert(db); err != nil {
			return err
		}
	}
	for _, p := range []backup.Policy{
		{ID: "demo-backup-db-nightly", Name: "database-nightly", Project: project, Type: backup.BackupTypeDatabase, TargetPath: "/var/backups/capper/db", Source: "orders-primary", IntervalSecs: 86400, Retention: 14, LastRunAt: ts(-20), CreatedAt: ts(-34)},
		{ID: "demo-backup-store-hourly", Name: "control-plane-hourly", Project: project, Type: backup.BackupTypeStore, TargetPath: "/var/backups/capper/store", IntervalSecs: 3600, Retention: 48, LastRunAt: ts(-1), CreatedAt: ts(-33)},
	} {
		if err := backup.NewStore(st.DB).InsertPolicy(p); err != nil {
			return err
		}
	}
	if err := stack.NewStore(st.DB).Insert(stack.Stack{ID: "demo-stack-prod", Name: "production-web", Project: project, TemplateHash: "sha256:demo-stack-prod", Status: stack.StackActive, Resources: []stack.StackResource{{Type: "vpc", Name: "production", ID: "demo-vpc-prod"}, {Type: "lb", Name: "web-public", ID: "demo-lb-web"}, {Type: "instance", Name: "prod-web-01", ID: "demo-inst-web-01"}}, CreatedAt: ts(-32), UpdatedAt: ts(-2)}); err != nil {
		return err
	}
	if err := posture.NewStore(st.DB).InsertAll([]posture.Finding{
		{ID: "demo-posture-fw", ScanID: "demo-scan-001", Project: project, Check: "public-ingress-review", Severity: posture.SeverityMedium, Target: "web-public:8080", Detail: "Public HTTP listener is open for beta validation; prefer HTTPS-only before production.", ScannedAt: ts(-3)},
		{ID: "demo-posture-storage", ScanID: "demo-scan-001", Project: project, Check: "unencrypted-bucket", Severity: posture.SeverityLow, Target: "static-site-public", Detail: "Static site bucket is intentionally unencrypted and public-readable.", ScannedAt: ts(-3)},
		{ID: "demo-posture-iam", ScanID: "demo-scan-001", Project: project, Check: "broad-admin-role", Severity: posture.SeverityHigh, Target: "platform-admins", Detail: "Admin group has wildcard permissions; review before granting external access.", ScannedAt: ts(-3)},
	}); err != nil {
		return err
	}
	aiStore := ai.NewStore(st.DB)
	if err := aiStore.InsertAgent(ai.Agent{ID: "demo-ai-agent-ops", Name: "ops-copilot", Project: project, Model: "gpt-5-codex", Owner: "platform-team", RoleTemplate: "read-mostly-operator", Status: ai.AgentActive, CreatedAt: ts(-31)}); err != nil {
		return err
	}
	if err := aiStore.InsertSession(ai.Session{ID: "demo-ai-session-001", AgentID: "demo-ai-agent-ops", Project: project, Principal: "alice@example.test", Model: "gpt-5-codex", Status: ai.SessionActive, StartedAt: ts(-12)}); err != nil {
		return err
	}
	if err := aiStore.InsertToolCall(ai.ToolCall{ID: "demo-ai-call-001", SessionID: "demo-ai-session-001", Tool: "capper.instances.list", Action: "read", Resource: "instances/*", Decision: "allowed", Reason: "read-only inventory", CalledAt: ts(-11)}); err != nil {
		return err
	}
	if err := st.Marketplace.Insert(marketplace.MarketplaceListing{ID: "demo-market-pihole", Name: "pihole", Version: "2026.07", Description: "DNS sinkhole appliance recipe", Digest: "sha256:demo-pihole", Status: marketplace.StatusApproved, Labels: map[string]string{"category": "networking"}, ScanStatus: "passed", ScanFindings: 0, SBOMDigest: "sha256:demo-sbom-pihole", CreatedAt: ts(-29), UpdatedAt: ts(-29)}); err != nil {
		return err
	}
	if err := st.Marketplace.Insert(marketplace.MarketplaceListing{ID: "demo-market-jellyfin", Name: "jellyfin", Version: "10.10", Description: "Media server CapStart recipe", Digest: "sha256:demo-jellyfin", Status: marketplace.StatusApproved, Labels: map[string]string{"category": "media"}, ScanStatus: "passed", ScanFindings: 1, ScanSeverities: map[string]int{"low": 1}, CreatedAt: ts(-28), UpdatedAt: ts(-28)}); err != nil {
		return err
	}
	if hasColumn(st, "account_quotas", "resource_type") && hasColumn(st, "resource_usage", "resource_type") {
		if err := st.Quotas.SeedDefaults("demo-acct-local"); err != nil {
			return err
		}
		for _, usage := range []struct {
			key string
			id  string
			val int64
		}{
			{"compute.instances.max", "demo-usage-instances", 5},
			{"storage.buckets.max", "demo-usage-buckets", 3},
			{"vpc.count.max", "demo-usage-vpcs", 2},
			{"lb.count.max", "demo-usage-lb", 2},
		} {
			if err := st.Quotas.RecordUsage("demo-acct-local", usage.key, usage.id, usage.val); err != nil {
				return err
			}
		}
	}
	for _, r := range []resourcemon.Resource{
		{ID: "demo-rmon-prod-web", ResourceType: "instance", Name: "prod-web-01", Project: project, Status: "running", Health: resourcemon.HealthHealthy, Owner: "platform", Labels: map[string]string{"app": "web"}, CreatedAt: ts(-80), UpdatedAt: ts(-1), LastSeenAt: ts(-1)},
		{ID: "demo-rmon-storage-node", ResourceType: "node", Name: "storage-01", Project: project, Status: "cordoned", Health: resourcemon.HealthDegraded, Owner: "sre", Labels: map[string]string{"maintenance": "planned"}, CreatedAt: ts(-190), UpdatedAt: ts(-2), LastSeenAt: ts(-2)},
		{ID: "demo-rmon-lb", ResourceType: "load_balancer", Name: "web-public", Project: project, Status: "active", Health: resourcemon.HealthHealthy, Owner: "edge", CreatedAt: ts(-45), UpdatedAt: ts(-1), LastSeenAt: ts(-1)},
	} {
		if _, err := st.ResourceMon.UpsertResource(r); err != nil {
			return err
		}
	}
	for _, m := range []resourcemon.MetricSample{
		{ID: "demo-metric-web-cpu", Project: project, ResourceType: "instance", ResourceID: "demo-rmon-prod-web", MetricName: "cpu.utilization", Value: 61.5, Unit: "%", SampledAt: ts(-1)},
		{ID: "demo-metric-lb-rps", Project: project, ResourceType: "load_balancer", ResourceID: "demo-rmon-lb", MetricName: "requests_per_second", Value: 284, Unit: "rps", SampledAt: ts(-1)},
	} {
		if err := st.ResourceMon.InsertSample(m); err != nil {
			return err
		}
	}
	return nil
}

func hasColumn(st *store.Store, table, column string) bool {
	rows, err := st.DB.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}
