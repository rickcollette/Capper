package control

import (
	"context"
	"fmt"
	"os"
	"time"

	"capper/internal/capinit"
	capperdns "capper/internal/dns"
	"capper/internal/health"
	hsprovider "capper/internal/hostsec/provider"
	"capper/internal/hoststorage"
	"capper/internal/manager"
	"capper/internal/network"
	"capper/internal/store"
	capstore "capper/internal/storage"
	"capper/internal/supervisor"
	"capper/internal/topology"
)

// Daemon is the top-level Capper control plane. It owns the event bus,
// reconciler loop, admission chain, and the instance supervisor.
type Daemon struct {
	Store       *store.Store
	Bus         *Bus
	Reconcilers *ReconcilerLoop
	Admission   *AdmissionChain
	Supervisor  *supervisor.Supervisor
	IMDS        *capinit.Server
}

// DaemonOptions configures the Daemon.
type DaemonOptions struct {
	SupervisorInterval time.Duration
	ReconcilerInterval time.Duration
}

// DefaultDaemonOptions returns sensible defaults.
func DefaultDaemonOptions() DaemonOptions {
	return DaemonOptions{
		SupervisorInterval: 5 * time.Second,
		ReconcilerInterval: 30 * time.Second,
	}
}

// NewDaemon assembles a Daemon from the provided store and instance manager.
// Callers may Register additional reconcilers and admission hooks before Run.
func NewDaemon(st *store.Store, instMgr manager.InstanceManager, opts DaemonOptions) *Daemon {
	d := &Daemon{
		Store:       st,
		Bus:         NewBus(),
		Reconcilers: NewReconcilerLoop(opts.ReconcilerInterval),
		Admission:   &AdmissionChain{},
		Supervisor:  supervisor.New(st, instMgr),
	}
	d.Supervisor.Interval = opts.SupervisorInterval
	d.Reconcilers.Register(&hostHeartbeatReconciler{st: st})
	st.LB.SetInstanceLister(st.ListInstances)
	st.LB.SetNodeHealthChecker(func(nodeID string) bool {
		node, err := st.Topology.Store().GetNode(nodeID)
		if err != nil {
			return true // unknown node — fail open
		}
		return node.Status != topology.StatusOffline &&
			node.Status != topology.StatusUnhealthy &&
			node.Status != topology.StatusLost
	})
	d.Reconcilers.Register(&lbReconciler{st: st})
	healthRec := health.NewReconciler(st, st.Health)
	healthRec.OnUnhealthy = func(instanceID string) {
		inst, ierr := st.ResolveInstance(instanceID)
		if ierr != nil || inst.NetworkIP == "" {
			return
		}
		// Remove from all LB backends.
		lbs, err := st.LB.Store().List("")
		if err == nil {
			for _, lb := range lbs {
				backends, berr := st.LB.ListBackends(lb.ID, lb.Project)
				if berr != nil {
					continue
				}
				for _, b := range backends {
					if len(b.Address) >= len(inst.NetworkIP) &&
						b.Address[:len(inst.NetworkIP)] == inst.NetworkIP {
						_ = st.LB.RemoveBackend(lb.ID, lb.Project, b.Address)
					}
				}
			}
		}
		// Remove DNS A records that point to this instance's IP.
		zones, zerr := st.DNS.ListZones("")
		if zerr == nil {
			for _, z := range zones {
				records, rerr := st.DNS.ListRecords(z.ID)
				if rerr != nil {
					continue
				}
				for _, rec := range records {
					if rec.Type != "A" {
						continue
					}
					for _, v := range rec.Values {
						if v == inst.NetworkIP {
							_ = st.DNS.DeleteRecord(rec.ID)
							break
						}
					}
				}
			}
		}
	}
	d.Reconcilers.Register(healthRec)
	d.Reconcilers.Register(&eventingReconciler{st: st})
	d.Reconcilers.Register(&stackReplicaReconciler{st: st, instMgr: instMgr})
	d.Reconcilers.Register(&capmetaReconciler{
		st:      st,
		servers: make(map[string]*capinit.Server),
		cancels: make(map[string]context.CancelFunc),
	})
	d.Reconcilers.Register(&capdnsReconciler{
		st:      st,
		daemons: make(map[string]*capperdns.Daemon),
	})
	d.Reconcilers.Register(&staleBridgeReconciler{st: st})
	d.Reconcilers.Register(&hostStorageReconciler{st: st})
	d.Reconcilers.Register(&fail2banReconciler{st: st})
	d.Reconcilers.Register(&orphanLeaseReconciler{st: st})
	registerTopologyReconcilers(d, st)
	return d
}

// registerTopologyReconcilers wires up all topology-aware reconcilers.
func registerTopologyReconcilers(d *Daemon, st *store.Store) {
	topoStore := st.Topology.Store()

	// Determine local topology IDs for replica reconcilers.
	localRealm, localRegion, localZone := "rlm_local", "reg_local", "zon_local"
	if realms, err := topoStore.ListRealms(); err == nil && len(realms) > 0 {
		localRealm = realms[0].ID
	}
	if regions, err := topoStore.ListRegions(""); err == nil && len(regions) > 0 {
		localRegion = regions[0].ID
	}
	if zones, err := topoStore.ListZones(""); err == nil && len(zones) > 0 {
		localZone = zones[0].ID
	}

	d.Reconcilers.Register(&topology.InstancePlacementReconciler{
		Topology: topoStore,
		SetPlacement: func(id, realmID, regionID, zoneID, nodeID string) error {
			return st.SetInstancePlacement(id, realmID, regionID, zoneID, nodeID)
		},
		ListInstances: func() ([]topology.PlacedInstance, error) {
			insts, err := st.ListInstances()
			if err != nil {
				return nil, err
			}
			out := make([]topology.PlacedInstance, len(insts))
			for i, inst := range insts {
				out[i] = topology.PlacedInstance{
					ID: inst.ID, RealmID: inst.RealmID,
					RegionID: inst.RegionID, ZoneID: inst.ZoneID, NodeID: inst.NodeID,
				}
			}
			return out, nil
		},
	})

	d.Reconcilers.Register(&topology.NodeHealthReconciler{
		Topology:   topoStore,
		StaleAfter: 2 * time.Minute,
	})
	d.Reconcilers.Register(&topology.ZoneHealthReconciler{Topology: topoStore})
	d.Reconcilers.Register(&topology.RegionHealthReconciler{Topology: topoStore})

	storagePaths := capstore.Paths{
		Volumes:   st.Paths.StorageVolumes,
		Buckets:   st.Paths.StorageBuckets,
		Snapshots: st.Paths.StorageSnapshots,
	}
	storageMgr := capstore.NewManager(st.Storage, storagePaths)

	d.Reconcilers.Register(&topology.ImageReplicaReconciler{
		Topology:      topoStore,
		LocalRealmID:  localRealm,
		LocalRegionID: localRegion,
		LocalZoneID:   localZone,
		ListImages: func() ([]topology.ImageRef, error) {
			imgs, err := st.ListImages()
			if err != nil {
				return nil, err
			}
			out := make([]topology.ImageRef, len(imgs))
			for i, img := range imgs {
				out[i] = topology.ImageRef{
					ID:     img.ID,
					Digest: img.Digest,
					Size:   img.SizeBytes,
					Path:   img.Path,
				}
			}
			return out, nil
		},
	})

	d.Reconcilers.Register(&topology.StorageReplicaReconciler{
		Topology:      topoStore,
		LocalRealmID:  localRealm,
		LocalRegionID: localRegion,
		LocalZoneID:   localZone,
		ListObjects: func() ([]topology.ObjectRef, error) {
			buckets, err := storageMgr.ListBuckets()
			if err != nil {
				return nil, err
			}
			var out []topology.ObjectRef
			for _, b := range buckets {
				objs, err := storageMgr.ListObjects(b.Name, "")
				if err != nil {
					continue
				}
				for _, obj := range objs {
					out = append(out, topology.ObjectRef{
						Bucket: b.Name,
						Key:    obj.Key,
						ETag:   obj.Digest,
						Size:   obj.SizeBytes,
					})
				}
			}
			return out, nil
		},
	})

	d.Reconcilers.Register(&topology.MigrationPlanReconciler{Topology: topoStore})

}

// Run starts the supervisor, reconciler loop, and IMDS server as goroutines,
// then blocks until ctx is cancelled and all have returned.
func (d *Daemon) Run(ctx context.Context) {
	workers := 2
	done := make(chan struct{}, 3)
	go func() { d.Supervisor.Run(ctx); done <- struct{}{} }()
	go func() { d.Reconcilers.Run(ctx); done <- struct{}{} }()
	if d.IMDS != nil {
		workers = 3
		go func() {
			_ = d.IMDS.ListenAndServe(ctx)
			done <- struct{}{}
		}()
	}
	for range workers {
		<-done
	}
}

// SupervisorStats returns per-instance restart counts for this session.
func (d *Daemon) SupervisorStats() map[string]int {
	return d.Supervisor.Stats()
}

// ---- built-in reconcilers ---------------------------------------------------

// lbReconciler starts/stops load balancer proxies to match DB state.
type lbReconciler struct {
	st *store.Store
}

func (r *lbReconciler) Name() string { return "lb-proxy" }

func (r *lbReconciler) Reconcile(ctx context.Context) error {
	return r.st.LB.Reconcile(ctx)
}

// eventingReconciler runs due schedules on every reconcile tick.
type eventingReconciler struct {
	st *store.Store
}

func (r *eventingReconciler) Name() string { return "eventing" }

func (r *eventingReconciler) Reconcile(ctx context.Context) error {
	return r.st.Eventing.RunDueSchedules(ctx, "")
}

// stackReplicaReconciler restarts failed instance replicas owned by stacks.
type stackReplicaReconciler struct {
	st      *store.Store
	instMgr manager.InstanceManager
}

func (r *stackReplicaReconciler) Name() string { return "stack-replicas" }

func (r *stackReplicaReconciler) Reconcile(ctx context.Context) error {
	return r.st.Stack.ReconcileReplicas(ctx, "")
}

// capmetaReconciler manages one capmeta HTTP server per network.
// It starts a server bound to each network's gateway address so that DNAT
// can redirect 169.254.169.254:80 → gatewayIP:80 per bridge.
type capmetaReconciler struct {
	st      *store.Store
	servers map[string]*capinit.Server // network ID → running server
	cancels map[string]context.CancelFunc
}

func (r *capmetaReconciler) Name() string { return "capmeta" }

func (r *capmetaReconciler) Reconcile(ctx context.Context) error {
	nets, err := r.st.Networks.List("")
	if err != nil {
		return err
	}
	active := make(map[string]bool)
	for _, n := range nets {
		active[n.ID] = true
		// Restore all nftables/iptables rules (idempotent; also runs after reboot).
		network.RestoreNetworkRules(n)
		if _, running := r.servers[n.ID]; running {
			continue
		}
		if n.Gateway == "" {
			continue
		}
		addr := n.Gateway + ":8080"
		srv := capinit.NewServerWithAddr(r.st, addr)
		childCtx, cancel := context.WithCancel(ctx)
		r.servers[n.ID] = srv
		r.cancels[n.ID] = cancel
		go func(s *capinit.Server, c context.Context) { _ = s.ListenAndServe(c) }(srv, childCtx)
	}
	// Stop servers for deleted networks.
	for id, cancel := range r.cancels {
		if !active[id] {
			cancel()
			delete(r.servers, id)
			delete(r.cancels, id)
		}
	}
	return nil
}

// hostHeartbeatReconciler bumps last_seen for the local host if registered.
type hostHeartbeatReconciler struct {
	st *store.Store
}

func (r *hostHeartbeatReconciler) Name() string { return "host-heartbeat" }

func (r *hostHeartbeatReconciler) Reconcile(_ context.Context) error {
	hostname, _ := os.Hostname()
	hosts, err := r.st.Hosts.List()
	if err != nil {
		return err
	}
	for _, h := range hosts {
		if h.Hostname == hostname {
			return r.st.Hosts.UpdateSeen(h.ID)
		}
	}
	return nil
}

// staleBridgeReconciler removes Linux bridge interfaces that no longer have a
// corresponding active network record in the store. Stale bridges with the same
// subnet as an active bridge poison the kernel routing table and break NAT.
type staleBridgeReconciler struct {
	st *store.Store
}

func (r *staleBridgeReconciler) Name() string { return "stale-bridges" }

func (r *staleBridgeReconciler) Reconcile(_ context.Context) error {
	nets, err := r.st.Networks.List("")
	if err != nil {
		return err
	}
	activeBridges := make(map[string]bool)
	for _, n := range nets {
		if n.Bridge != "" {
			activeBridges[n.Bridge] = true
		}
	}
	return network.RemoveStaleBridges(activeBridges)
}

// hostStorageReconciler refreshes each storage pool's capacity from its backend
// and marks pools whose backing storage (mountpoint or volume group) has gone
// missing as degraded, so the Admin UI reflects live host state.
type hostStorageReconciler struct {
	st *store.Store
}

func (r *hostStorageReconciler) Name() string { return "host-storage" }

func (r *hostStorageReconciler) Reconcile(ctx context.Context) error {
	return hoststorage.NewManager(r.st.HostStorage).Reconcile(ctx)
}

// orphanLeaseReconciler prunes network leases whose owning instance no longer
// exists, so deleting an instance that skipped lease cleanup can't permanently
// block its network's deletion or starve its IP pool.
type orphanLeaseReconciler struct {
	st *store.Store
}

func (r *orphanLeaseReconciler) Name() string { return "orphan-network-leases" }

func (r *orphanLeaseReconciler) Reconcile(_ context.Context) error {
	_, err := r.st.PruneOrphanedNetworkLeases()
	return err
}

// fail2banReconciler re-applies the admin persistent blocklist so manually-added
// bans survive fail2ban restarts. It uses the process-wide fail2ban worker, so
// it shares the same exclusive serialized queue as the admin API.
type fail2banReconciler struct {
	st *store.Store
}

func (r *fail2banReconciler) Name() string { return "fail2ban-blocklist" }

func (r *fail2banReconciler) Reconcile(ctx context.Context) error {
	w := hsprovider.Fail2ban()
	if !w.Available() {
		return nil
	}
	entries, err := r.st.Fail2ban.ListBlocklist()
	if err != nil || len(entries) == 0 {
		return err
	}
	want := map[string][]string{}
	for _, e := range entries {
		want[e.Jail] = append(want[e.Jail], e.IP)
	}
	_, err = w.EnsureBans(ctx, want)
	return err
}

// buildDNSHealthFilter returns a HealthFilter that returns false for any IP
// whose instance is placed on an unhealthy or offline node/zone.
func buildDNSHealthFilter(st *store.Store) capperdns.HealthFilter {
	return func(ip string) bool {
		insts, err := st.ListInstances()
		if err != nil {
			return true // fail open
		}
		for _, inst := range insts {
			if inst.NetworkIP != ip || inst.NodeID == "" {
				continue
			}
			node, err := st.Topology.Store().GetNode(inst.NodeID)
			if err != nil {
				return true
			}
			if node.Status == topology.StatusOffline || node.Status == topology.StatusUnhealthy || node.Status == topology.StatusLost {
				return false
			}
			if inst.ZoneID != "" {
				zone, err := st.Topology.Store().GetZone(inst.ZoneID)
				if err == nil && (zone.Status == topology.StatusUnhealthy || zone.Status == topology.StatusCordoned || zone.Status == topology.StatusDraining) {
					return false
				}
			}
			return true
		}
		return true // IP not associated with a known instance — pass through
	}
}

// capdnsReconciler manages one DNS daemon per network, bound to gateway:53.
// Instances point their resolv.conf at the gateway; this daemon serves
// Capper zone records and forwards everything else to public DNS.
type capdnsReconciler struct {
	st      *store.Store
	daemons map[string]*capperdns.Daemon // network ID → running daemon
}

func (r *capdnsReconciler) Name() string { return "capdns" }

func (r *capdnsReconciler) Reconcile(_ context.Context) error {
	nets, err := r.st.Networks.List("")
	if err != nil {
		return err
	}
	active := make(map[string]bool)
	for _, n := range nets {
		active[n.ID] = true
		if _, running := r.daemons[n.ID]; running {
			continue
		}
		if n.Gateway == "" {
			continue
		}
		resolver := capperdns.NewNetworkResolver(r.st.DNS, n.ID, nil, []string{"8.8.8.8:53", "8.8.4.4:53"})
		resolver.SetHealthFilter(buildDNSHealthFilter(r.st))
		d := capperdns.NewDaemon(n.Gateway+":53", resolver)
		if err := d.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "capdns: start dns for %s at %s:53: %v\n", n.ID, n.Gateway, err)
			continue
		}
		r.daemons[n.ID] = d
	}
	for id, d := range r.daemons {
		if !active[id] {
			_ = d.Stop()
			delete(r.daemons, id)
		}
	}
	return nil
}
