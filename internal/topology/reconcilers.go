package topology

import (
	"context"
	"log"
	"time"
)

// InstanceStore is the subset of store.Store used by topology reconcilers.
type InstanceStore interface {
	ListInstances() ([]interface{ GetPlacement() (realmID, regionID, zoneID, nodeID string) }, error)
}

// ---- InstancePlacementReconciler --------------------------------------------

// InstancePlacementReconciler fills realm/region/zone/node IDs on instances
// that were created before the topology system was wired in.
type InstancePlacementReconciler struct {
	Topology *Store
	SetPlacement func(instanceID, realmID, regionID, zoneID, nodeID string) error
	ListInstances func() ([]PlacedInstance, error)
}

type PlacedInstance struct {
	ID       string
	RealmID  string
	RegionID string
	ZoneID   string
	NodeID   string
}

func (r *InstancePlacementReconciler) Name() string { return "instance-placement" }

func (r *InstancePlacementReconciler) Reconcile(_ context.Context) error {
	if r.ListInstances == nil || r.SetPlacement == nil {
		return nil
	}
	instances, err := r.ListInstances()
	if err != nil {
		return err
	}
	for _, inst := range instances {
		if inst.RealmID != "" {
			continue // already placed
		}
		// Pick the first ready node in local topology as fallback placement.
		nodes, err := r.Topology.ListNodes("")
		if err != nil || len(nodes) == 0 {
			continue
		}
		var target *Node
		for i := range nodes {
			if nodes[i].Status == StatusReady {
				target = &nodes[i]
				break
			}
		}
		if target == nil {
			target = &nodes[0]
		}
		if err := r.SetPlacement(inst.ID, target.RealmID, target.RegionID, target.ZoneID, target.ID); err != nil {
			log.Printf("[placement-reconciler] set placement for %s: %v", inst.ID, err)
		}
	}
	return nil
}

// ---- NodeHealthReconciler ---------------------------------------------------

// NodeHealthReconciler marks nodes offline when their health record is stale,
// and marks them ready again when a fresh health record appears.
type NodeHealthReconciler struct {
	Topology  *Store
	StaleAfter time.Duration
}

func (r *NodeHealthReconciler) Name() string { return "node-health" }

func (r *NodeHealthReconciler) Reconcile(_ context.Context) error {
	stale := r.StaleAfter
	if stale <= 0 {
		stale = 2 * time.Minute
	}
	nodes, err := r.Topology.ListNodes("")
	if err != nil {
		return err
	}
	for _, n := range nodes {
		// Look up most recent service_health record for this node.
		health, err := r.Topology.ListServiceHealthByNode("node", "", "", "", n.ID)
		if err != nil {
			continue
		}
		healthy := false
		for _, h := range health {
			if h.NodeID != n.ID {
				continue
			}
			t, err := time.Parse(time.RFC3339, h.CheckedAt)
			if err != nil {
				continue
			}
			if time.Since(t) < stale && h.Status == "healthy" {
				healthy = true
				break
			}
		}

		switch {
		case healthy && n.Status == StatusOffline:
			_ = r.Topology.UpdateNodeStatus(n.ID, StatusReady)
		case !healthy && len(health) > 0 && (n.Status == StatusReady):
			// Only mark offline when we have seen health records that are now stale —
			// a node that has never reported is left in its initial state.
			_ = r.Topology.UpdateNodeStatus(n.ID, StatusOffline)
		}
	}
	return nil
}

// ---- ZoneHealthReconciler ---------------------------------------------------

// ZoneHealthReconciler degrades or recovers a zone based on its node health.
// A zone is degraded when >50% of its nodes are unhealthy/offline.
type ZoneHealthReconciler struct {
	Topology *Store
}

func (r *ZoneHealthReconciler) Name() string { return "zone-health" }

func (r *ZoneHealthReconciler) Reconcile(_ context.Context) error {
	zones, err := r.Topology.ListZones("")
	if err != nil {
		return err
	}
	for _, z := range zones {
		if z.Status == StatusMaintenance || z.Status == StatusCordoned || z.Status == StatusDraining {
			continue
		}
		nodes, err := r.Topology.ListNodes(z.ID)
		if err != nil || len(nodes) == 0 {
			continue
		}
		bad := 0
		for _, n := range nodes {
			if n.Status == StatusOffline || n.Status == StatusUnhealthy || n.Status == StatusLost {
				bad++
			}
		}
		ratio := float64(bad) / float64(len(nodes))
		switch {
		case ratio >= 0.5 && z.Status == StatusActive:
			_ = r.Topology.UpdateZoneStatus(z.ID, StatusDegraded)
		case ratio >= 1.0 && z.Status == StatusDegraded:
			_ = r.Topology.UpdateZoneStatus(z.ID, StatusUnhealthy)
		case ratio < 0.5 && (z.Status == StatusDegraded || z.Status == StatusUnhealthy):
			_ = r.Topology.UpdateZoneStatus(z.ID, StatusActive)
		}
	}
	return nil
}

// ---- RegionHealthReconciler -------------------------------------------------

// RegionHealthReconciler degrades or recovers a region based on its zone health.
type RegionHealthReconciler struct {
	Topology *Store
}

func (r *RegionHealthReconciler) Name() string { return "region-health" }

func (r *RegionHealthReconciler) Reconcile(_ context.Context) error {
	regions, err := r.Topology.ListRegions("")
	if err != nil {
		return err
	}
	for _, reg := range regions {
		if reg.Status == StatusMaintenance || reg.Status == StatusDraining {
			continue
		}
		zones, err := r.Topology.ListZones(reg.ID)
		if err != nil || len(zones) == 0 {
			continue
		}
		bad := 0
		for _, z := range zones {
			if z.Status == StatusUnhealthy || z.Status == StatusOffline {
				bad++
			}
		}
		ratio := float64(bad) / float64(len(zones))
		switch {
		case ratio >= 0.5 && reg.Status == StatusActive:
			_ = r.Topology.UpdateRegionStatus(reg.ID, StatusDegraded)
		case ratio >= 1.0 && reg.Status == StatusDegraded:
			_ = r.Topology.UpdateRegionStatus(reg.ID, StatusUnhealthy)
		case ratio < 0.5 && (reg.Status == StatusDegraded || reg.Status == StatusUnhealthy):
			_ = r.Topology.UpdateRegionStatus(reg.ID, StatusActive)
		}
	}
	return nil
}

// ---- ImageReplicaReconciler -------------------------------------------------

// ImageReplicaReconciler ensures every image has an image_replicas row for the
// local zone with status "ready". In a multi-node setup it would trigger actual
// replication; here it just seeds the tracking record.
type ImageReplicaReconciler struct {
	Topology      *Store
	LocalRealmID  string
	LocalRegionID string
	LocalZoneID   string
	ListImages    func() ([]ImageRef, error)
}

// ImageRef is the minimal image info the reconciler needs.
type ImageRef struct {
	ID     string
	Digest string
	Size   int64
	Path   string
}

func (r *ImageReplicaReconciler) Name() string { return "image-replica" }

func (r *ImageReplicaReconciler) Reconcile(_ context.Context) error {
	if r.ListImages == nil || r.LocalZoneID == "" {
		return nil
	}
	images, err := r.ListImages()
	if err != nil {
		return err
	}
	for _, img := range images {
		replicas, _ := r.Topology.ListImageReplicas(img.ID)
		alreadyLocal := false
		for _, rep := range replicas {
			if rep.ZoneID == r.LocalZoneID && rep.Status == "ready" {
				alreadyLocal = true
				break
			}
		}
		if alreadyLocal {
			continue
		}
		rep := ImageReplica{
			ImageID:   img.ID,
			Digest:    img.Digest,
			RealmID:   r.LocalRealmID,
			RegionID:  r.LocalRegionID,
			ZoneID:    r.LocalZoneID,
			Status:    "ready",
			SizeBytes: img.Size,
			Location:  img.Path,
		}
		if err := r.Topology.InsertImageReplica(rep); err != nil {
			log.Printf("[image-replica-reconciler] insert replica for %s: %v", img.ID, err)
		}
	}
	return nil
}

// ---- StorageReplicaReconciler -----------------------------------------------

// StorageReplicaReconciler ensures every stored object has a storage_replicas
// row for the local zone. In single-zone mode this just seeds tracking records.
type StorageReplicaReconciler struct {
	Topology      *Store
	LocalRealmID  string
	LocalRegionID string
	LocalZoneID   string
	ListObjects   func() ([]ObjectRef, error)
}

// ObjectRef is the minimal object info the reconciler needs.
type ObjectRef struct {
	Bucket string
	Key    string
	ETag   string
	Size   int64
	Path   string
}

func (r *StorageReplicaReconciler) Name() string { return "storage-replica" }

func (r *StorageReplicaReconciler) Reconcile(_ context.Context) error {
	if r.ListObjects == nil || r.LocalZoneID == "" {
		return nil
	}
	objects, err := r.ListObjects()
	if err != nil {
		return err
	}
	for _, obj := range objects {
		replicas, _ := r.Topology.ListStorageReplicas(obj.Bucket, obj.Key)
		alreadyLocal := false
		for _, rep := range replicas {
			if rep.ZoneID == r.LocalZoneID && rep.Status == "ready" {
				alreadyLocal = true
				break
			}
		}
		if alreadyLocal {
			continue
		}
		rep := StorageReplica{
			Bucket:    obj.Bucket,
			ObjectKey: obj.Key,
			ETag:      obj.ETag,
			RealmID:   r.LocalRealmID,
			RegionID:  r.LocalRegionID,
			ZoneID:    r.LocalZoneID,
			Status:    "ready",
			SizeBytes: obj.Size,
			Location:  obj.Path,
		}
		if err := r.Topology.InsertStorageReplica(rep); err != nil {
			log.Printf("[storage-replica-reconciler] insert replica %s/%s: %v", obj.Bucket, obj.Key, err)
		}
	}
	return nil
}

// ---- MigrationPlanReconciler ------------------------------------------------

// MigrationPlanReconciler advances migration plans through their state machine.
// draft → validated → running → completed (or failed).
type MigrationPlanReconciler struct {
	Topology *Store
}

func (r *MigrationPlanReconciler) Name() string { return "migration-plan" }

func (r *MigrationPlanReconciler) Reconcile(_ context.Context) error {
	plans, err := r.Topology.ListAllMigrationPlans()
	if err != nil {
		return err
	}
	for _, plan := range plans {
		switch plan.Status {
		case "draft":
			// Auto-validate plans that have source and target set.
			if plan.SourceRegionID != "" && plan.TargetRegionID != "" {
				_ = r.Topology.UpdateMigrationPlanStatus(plan.ID, "validated")
			}
		case "running":
			// In single-node mode there is no actual cross-region work,
			// so immediately advance running plans to completed.
			_ = r.Topology.UpdateMigrationPlanStatus(plan.ID, "completed")
		}
	}
	return nil
}
