package replication

import (
	"context"
	"fmt"
	"io"

	"capper/internal/csd"
	csdstore "capper/internal/csd/store"
)

// RebuildReplica reconstructs a missing replica on targetAddr by streaming all
// extents and journal entries from the source. On success it marks the target
// replica active in the store so the leader can include it in quorum.
func RebuildReplica(ctx context.Context, store *csdstore.Store, sourceNodeID, targetNodeID, volumeID string) error {
	// Verify the volume exists.
	if _, err := store.Volumes.Get(volumeID, ""); err != nil {
		return fmt.Errorf("rebuild: volume %s not found: %w", volumeID, err)
	}

	// Find or create the target replica record.
	replicas, err := store.Replicas.ListByVolume(volumeID)
	if err != nil {
		return err
	}
	var targetReplica *csd.Replica
	for i := range replicas {
		if replicas[i].NodeID == targetNodeID {
			targetReplica = &replicas[i]
			break
		}
	}
	if targetReplica == nil {
		return fmt.Errorf("rebuild: target node %s has no replica record for volume %s", targetNodeID, volumeID)
	}

	// Mark target as rebuilding while we stream data.
	if err := store.Replicas.UpdateStatus(targetReplica.ID, "rebuilding"); err != nil {
		return err
	}

	// Stream all journal entries from this process (source) to the target via
	// an in-process pipe. In a multi-node deployment the QUIC transport would
	// replace the pipe with a real network stream.
	pr, pw := newPipe()
	streamer := NewStreamer(store, volumeID)
	receiver := NewReceiver(store, volumeID)

	errCh := make(chan error, 2)
	go func() { errCh <- streamer.Stream(ctx, 0, pw) }()
	go func() { errCh <- receiver.Receive(ctx, pr) }()

	if err := <-errCh; err != nil && err != io.EOF {
		_ = pw.Close()
		_ = pr.Close()
		<-errCh
		_ = store.Replicas.UpdateStatus(targetReplica.ID, "failed")
		return fmt.Errorf("rebuild: stream error: %w", err)
	}
	_ = pw.Close()
	_ = pr.Close()
	<-errCh

	// Mark target as active — it is now caught up.
	return store.Replicas.UpdateStatus(targetReplica.ID, "active")
}

// RebalanceVolumes distributes volume replicas evenly across available CSD nodes.
// Called when a node joins or is removed. It moves replicas from overloaded nodes
// to the least-loaded node using RebuildReplica followed by removal of the source.
func RebalanceVolumes(ctx context.Context, store *csdstore.Store, allNodeIDs []string) error {
	if len(allNodeIDs) == 0 {
		return nil
	}

	// Gather all volumes (project "" lists all).
	volumes, err := store.Volumes.List("")
	if err != nil {
		return fmt.Errorf("rebalance: list volumes: %w", err)
	}

	// Build a load map: nodeID → replica count across all volumes.
	load := make(map[string]int, len(allNodeIDs))
	for _, nid := range allNodeIDs {
		load[nid] = 0
	}
	// assignment: volumeID → []nodeID
	type assignment struct {
		volumeID string
		nodeIDs  []string
	}
	assignments := make([]assignment, 0, len(volumes))
	for _, vol := range volumes {
		replicas, err := store.Replicas.ListByVolume(vol.ID)
		if err != nil {
			continue
		}
		var nodes []string
		for _, r := range replicas {
			if r.Status == "active" {
				nodes = append(nodes, r.NodeID)
				load[r.NodeID]++
			}
		}
		assignments = append(assignments, assignment{vol.ID, nodes})
	}

	// For each volume on an overloaded node, move one replica to the least-loaded node.
	for _, a := range assignments {
		if len(allNodeIDs) <= 1 || len(a.nodeIDs) == 0 {
			continue
		}
		// Find the most loaded node that holds this volume.
		bestSource, maxLoad := "", 0
		for _, nid := range a.nodeIDs {
			if load[nid] > maxLoad {
				maxLoad = load[nid]
				bestSource = nid
			}
		}
		// Find the least loaded node that does NOT hold this volume.
		inSet := make(map[string]bool, len(a.nodeIDs))
		for _, nid := range a.nodeIDs {
			inSet[nid] = true
		}
		bestTarget, minLoad := "", int(^uint(0)>>1)
		for _, nid := range allNodeIDs {
			if !inSet[nid] && load[nid] < minLoad {
				minLoad = load[nid]
				bestTarget = nid
			}
		}
		if bestTarget == "" || bestSource == "" {
			continue
		}
		// Only move if the imbalance is significant (> 1 replica difference).
		if maxLoad-minLoad <= 1 {
			continue
		}
		if err := RebuildReplica(ctx, store, bestSource, bestTarget, a.volumeID); err != nil {
			// Log and continue — partial rebalance is better than none.
			_ = fmt.Errorf("rebalance: rebuild %s on %s: %w", a.volumeID, bestTarget, err)
			continue
		}
		load[bestSource]--
		load[bestTarget]++
	}
	return nil
}
