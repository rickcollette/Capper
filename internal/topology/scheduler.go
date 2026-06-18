package topology

import "context"

// Scheduler selects nodes for instance placement using filter + score.
type Scheduler struct {
	store *Store
}

func NewScheduler(store *Store) *Scheduler { return &Scheduler{store: store} }

// Simulate runs the filter+score pipeline without committing any state.
func (sched *Scheduler) Simulate(_ context.Context, req PlacementRequest) PlacementResult {
	result := PlacementResult{Candidates: []Candidate{}, Rejections: []Rejection{}}

	// Collect candidate nodes.
	var nodes []Node
	var err error
	if req.Zone != "" {
		z, zerr := sched.store.GetZone(req.Zone)
		if zerr != nil {
			result.Rejections = append(result.Rejections, Rejection{Reason: "zone not found: " + req.Zone})
			return result
		}
		nodes, err = sched.store.ListNodes(z.ID)
	} else if req.Region != "" {
		reg, rerr := sched.store.GetRegion(req.Region)
		if rerr != nil {
			result.Rejections = append(result.Rejections, Rejection{Reason: "region not found: " + req.Region})
			return result
		}
		zones, zerr := sched.store.ListZones(reg.ID)
		if zerr == nil {
			for _, z := range zones {
				zNodes, _ := sched.store.ListNodes(z.ID)
				nodes = append(nodes, zNodes...)
			}
		}
	} else {
		nodes, err = sched.store.ListNodes("")
	}
	if err != nil {
		result.Rejections = append(result.Rejections, Rejection{Reason: "store error: " + err.Error()})
		return result
	}

	// Region/zone lookup maps for candidate output.
	regionByID := map[string]Region{}
	zoneByID := map[string]Zone{}

	for i := range nodes {
		n := nodes[i]
		why, ok := sched.filter(n, req)
		if !ok {
			result.Rejections = append(result.Rejections, Rejection{NodeID: n.ID, Node: n.Slug, Reason: why})
			continue
		}

		// Populate region/zone names from cache.
		if _, ok := zoneByID[n.ZoneID]; !ok {
			z, _ := sched.store.GetZone(n.ZoneID)
			zoneByID[n.ZoneID] = z
		}
		z := zoneByID[n.ZoneID]
		if _, ok := regionByID[n.RegionID]; !ok {
			r, _ := sched.store.GetRegion(n.RegionID)
			regionByID[n.RegionID] = r
		}
		reg := regionByID[n.RegionID]

		score, reasons := sched.score(n, req)
		result.Candidates = append(result.Candidates, Candidate{
			RealmID:  n.RealmID,
			RegionID: n.RegionID,
			ZoneID:   n.ZoneID,
			NodeID:   n.ID,
			Region:   reg.Slug,
			Zone:     z.Slug,
			Node:     n.Slug,
			Score:    score,
			Reasons:  reasons,
		})
	}

	// Sort candidates by descending score (simple insertion sort — typically small N).
	for i := 1; i < len(result.Candidates); i++ {
		for j := i; j > 0 && result.Candidates[j].Score > result.Candidates[j-1].Score; j-- {
			result.Candidates[j], result.Candidates[j-1] = result.Candidates[j-1], result.Candidates[j]
		}
	}

	result.Allowed = len(result.Candidates) > 0
	return result
}

// strSliceContains reports whether s is in ss.
func strSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// filter returns ("", true) if the node passes all filters,
// or (reason, false) with the rejection reason otherwise.
func (sched *Scheduler) filter(n Node, req PlacementRequest) (string, bool) {
	if n.Status != StatusReady {
		return "node not ready: status=" + n.Status, false
	}

	// Region / zone filter.
	if req.Region != "" {
		reg, err := sched.store.GetRegion(req.Region)
		if err != nil || (reg.ID != n.RegionID && reg.Slug != req.Region) {
			return "node not in requested region", false
		}
		if reg.Status != StatusActive {
			return "region not active", false
		}
	}
	if req.Zone != "" {
		z, err := sched.store.GetZone(req.Zone)
		if err != nil || (z.ID != n.ZoneID && z.Slug != req.Zone) {
			return "node not in requested zone", false
		}
		if z.Status != StatusActive && z.Status != StatusReady {
			return "zone not active", false
		}
	}

	// Resource capacity.
	if req.CPURequired > 0 && n.CPUCount > 0 && n.CPUCount < req.CPURequired {
		return "insufficient CPU", false
	}
	if req.MemoryBytes > 0 && n.MemoryBytes > 0 && n.MemoryBytes < req.MemoryBytes {
		return "insufficient memory", false
	}
	if req.GPURequired {
		if v, ok := n.Labels["gpu"]; !ok || v != "true" {
			return "node has no GPU", false
		}
	}

	// Required label filter.
	for k, v := range req.RequireLabel {
		if n.Labels[k] != v {
			return "missing required label " + k + "=" + v, false
		}
	}

	// Cordoned node filter.
	if n.Cordoned {
		return "node is cordoned", false
	}

	// Required roles filter.
	if len(req.RequiredRoles) > 0 {
		roles, _ := sched.store.GetNodeRoles(n.ID)
		for _, required := range req.RequiredRoles {
			if !strSliceContains(roles, required) {
				return "missing required role " + required, false
			}
		}
	}

	// Taint filter: reject if node has NoSchedule/NoExecute taint not tolerated.
	taints, _ := sched.store.GetNodeTaints(n.ID)
	for _, taint := range taints {
		if taint.Effect != "NoSchedule" && taint.Effect != "NoExecute" {
			continue
		}
		tolerated := false
		for _, tol := range req.Tolerations {
			if tol.Key == taint.Key && (tol.Value == "" || tol.Value == taint.Value) && (tol.Effect == "" || tol.Effect == taint.Effect) {
				tolerated = true
				break
			}
		}
		if !tolerated {
			return "untolerated taint " + taint.Key + "=" + taint.Value + ":" + taint.Effect, false
		}
	}

	// GPU count filter.
	if req.GPUCount > 0 && n.GPUCount > 0 && n.GPUCount < req.GPUCount {
		return "insufficient GPU count", false
	}

	// Disk filter.
	if req.DiskBytes > 0 && n.DiskBytes > 0 && n.DiskBytes < req.DiskBytes {
		return "insufficient disk", false
	}

	return "", true
}

// score returns a 0-100 score and the reasons that contributed.
func (sched *Scheduler) score(n Node, req PlacementRequest) (int, []string) {
	score := 50
	var reasons []string

	if n.Status == StatusReady {
		score += 20
		reasons = append(reasons, "ready")
	}

	// Prefer nodes with more capacity.
	if req.CPURequired > 0 && n.CPUCount >= req.CPURequired*2 {
		score += 10
		reasons = append(reasons, "ample-cpu")
	}
	if req.MemoryBytes > 0 && n.MemoryBytes >= req.MemoryBytes*2 {
		score += 10
		reasons = append(reasons, "ample-memory")
	}

	// Prefer nodes that have the right labels.
	matched := 0
	for k, v := range req.RequireLabel {
		if n.Labels[k] == v {
			matched++
		}
	}
	if matched > 0 {
		score += matched * 5
		reasons = append(reasons, "label-match")
	}

	// Preferred roles scoring: +5 per matched preferred role.
	if len(req.PreferredRoles) > 0 {
		roles, _ := sched.store.GetNodeRoles(n.ID)
		for _, pref := range req.PreferredRoles {
			if strSliceContains(roles, pref) {
				score += 5
				reasons = append(reasons, "preferred-role:"+pref)
			}
		}
	}

	// GPU bonus: +15 if GPU required and node meets requirement.
	if req.GPURequired && n.GPUCount >= req.GPUCount {
		score += 15
		reasons = append(reasons, "gpu-available")
	}

	// Anti-affinity penalty: -50 if node matches any anti-affinity label.
	for k, v := range req.AntiAffinity {
		if n.Labels[k] == v {
			score -= 50
			reasons = append(reasons, "anti-affinity:"+k)
		}
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	if len(reasons) == 0 {
		reasons = []string{"eligible"}
	}
	return score, reasons
}
