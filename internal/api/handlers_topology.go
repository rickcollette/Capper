package api

import (
	"encoding/json"
	"net/http"
	"time"

	"capper/internal/topology"
)

// ---- Realm handlers ---------------------------------------------------------

func (s *Server) handleListRealms(w http.ResponseWriter, r *http.Request) {
	realms, err := s.ctrl.Store.Topology.Store().ListRealms()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: realms})
}

func (s *Server) handleCreateRealm(w http.ResponseWriter, r *http.Request) {
	var req topology.Realm
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if err := s.ctrl.Store.Topology.Store().InsertRealm(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	got, _ := s.ctrl.Store.Topology.Store().GetRealm(req.Slug)
	writeJSON(w, http.StatusCreated, Envelope{Data: got})
}

func (s *Server) handleGetRealm(w http.ResponseWriter, r *http.Request) {
	realm, err := s.ctrl.Store.Topology.Store().GetRealm(r.PathValue("realm"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "realm not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: realm})
}

func (s *Server) handlePatchRealm(w http.ResponseWriter, r *http.Request) {
	realm, err := s.ctrl.Store.Topology.Store().GetRealm(r.PathValue("realm"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "realm not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req topology.Realm
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if req.Slug != "" {
		realm.Slug = req.Slug
	}
	if req.Name != "" {
		realm.Name = req.Name
	}
	if req.Description != "" {
		realm.Description = req.Description
	}
	if req.Status != "" {
		realm.Status = req.Status
	}
	if req.Labels != nil {
		realm.Labels = req.Labels
	}
	if err := s.ctrl.Store.Topology.Store().UpdateRealm(realm); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: realm})
}

func (s *Server) handleDeleteRealm(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().DeleteRealm(r.PathValue("realm")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Region handlers --------------------------------------------------------

func (s *Server) handleListRegions(w http.ResponseWriter, r *http.Request) {
	realmFilter := r.URL.Query().Get("realm")
	var realmID string
	if realmFilter != "" {
		realm, err := s.ctrl.Store.Topology.Store().GetRealm(realmFilter)
		if err == nil {
			realmID = realm.ID
		}
	}
	regions, err := s.ctrl.Store.Topology.Store().ListRegions(realmID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: regions})
}

func (s *Server) handleCreateRegion(w http.ResponseWriter, r *http.Request) {
	var req topology.Region
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if err := s.ctrl.Store.Topology.Store().InsertRegion(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	got, _ := s.ctrl.Store.Topology.Store().GetRegion(req.Slug)
	writeJSON(w, http.StatusCreated, Envelope{Data: got})
}

func (s *Server) handleGetRegion(w http.ResponseWriter, r *http.Request) {
	region, err := s.ctrl.Store.Topology.Store().GetRegion(r.PathValue("region"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "region not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: region})
}

func (s *Server) handlePatchRegion(w http.ResponseWriter, r *http.Request) {
	region, err := s.ctrl.Store.Topology.Store().GetRegion(r.PathValue("region"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "region not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req topology.Region
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if req.Name != "" {
		region.Name = req.Name
	}
	if req.Description != "" {
		region.Description = req.Description
	}
	if req.Status != "" {
		region.Status = req.Status
	}
	if req.ControlURL != "" {
		region.ControlURL = req.ControlURL
	}
	if req.APIURL != "" {
		region.APIURL = req.APIURL
	}
	if req.Labels != nil {
		region.Labels = req.Labels
	}
	if err := s.ctrl.Store.Topology.Store().UpdateRegion(region); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: region})
}

func (s *Server) handleDeleteRegion(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().DeleteRegion(r.PathValue("region")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRegionAction(w http.ResponseWriter, r *http.Request, action string) {
	region, err := s.ctrl.Store.Topology.Store().GetRegion(r.PathValue("region"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "region not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	switch action {
	case "drain":
		region.Status = topology.StatusDraining
	case "undrain":
		region.Status = topology.StatusActive
	case "evacuate":
		region.Status = "evacuating"
	case "promote":
		region.Status = topology.StatusActive
	}
	if err := s.ctrl.Store.Topology.Store().UpdateRegion(region); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: region})
}

func (s *Server) handleDrainRegion(w http.ResponseWriter, r *http.Request) {
	s.handleRegionAction(w, r, "drain")
}
func (s *Server) handleUndrainRegion(w http.ResponseWriter, r *http.Request) {
	s.handleRegionAction(w, r, "undrain")
}
func (s *Server) handleEvacuateRegion(w http.ResponseWriter, r *http.Request) {
	s.handleRegionAction(w, r, "evacuate")
}
func (s *Server) handlePromoteRegion(w http.ResponseWriter, r *http.Request) {
	s.handleRegionAction(w, r, "promote")
}

// ---- Zone handlers ----------------------------------------------------------

func (s *Server) handleListZones(w http.ResponseWriter, r *http.Request) {
	regionFilter := r.URL.Query().Get("region")
	var regionID string
	if regionFilter != "" {
		reg, err := s.ctrl.Store.Topology.Store().GetRegion(regionFilter)
		if err == nil {
			regionID = reg.ID
		}
	}
	zones, err := s.ctrl.Store.Topology.Store().ListZones(regionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: zones})
}

func (s *Server) handleCreateZone(w http.ResponseWriter, r *http.Request) {
	var req topology.Zone
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if err := s.ctrl.Store.Topology.Store().InsertZone(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	got, _ := s.ctrl.Store.Topology.Store().GetZone(req.Slug)
	writeJSON(w, http.StatusCreated, Envelope{Data: got})
}

func (s *Server) handleGetZone(w http.ResponseWriter, r *http.Request) {
	zone, err := s.ctrl.Store.Topology.Store().GetZone(r.PathValue("zone"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "zone not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: zone})
}

func (s *Server) handlePatchZone(w http.ResponseWriter, r *http.Request) {
	zone, err := s.ctrl.Store.Topology.Store().GetZone(r.PathValue("zone"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "zone not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req topology.Zone
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if req.Name != "" {
		zone.Name = req.Name
	}
	if req.Description != "" {
		zone.Description = req.Description
	}
	if req.Status != "" {
		zone.Status = req.Status
	}
	if req.ControlURL != "" {
		zone.ControlURL = req.ControlURL
	}
	if req.NetworkCIDR != "" {
		zone.NetworkCIDR = req.NetworkCIDR
	}
	if req.Labels != nil {
		zone.Labels = req.Labels
	}
	if err := s.ctrl.Store.Topology.Store().UpdateZone(zone); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: zone})
}

func (s *Server) handleDeleteZone(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().DeleteZone(r.PathValue("zone")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleZoneAction(w http.ResponseWriter, r *http.Request, status string) {
	if err := s.ctrl.Store.Topology.Store().UpdateZoneStatus(r.PathValue("zone"), status); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	zone, _ := s.ctrl.Store.Topology.Store().GetZone(r.PathValue("zone"))
	writeJSON(w, http.StatusOK, Envelope{Data: zone})
}

func (s *Server) handleCordonZone(w http.ResponseWriter, r *http.Request) {
	s.handleZoneAction(w, r, topology.StatusCordoned)
}
func (s *Server) handleUncordonZone(w http.ResponseWriter, r *http.Request) {
	s.handleZoneAction(w, r, topology.StatusActive)
}
func (s *Server) handleDrainZone(w http.ResponseWriter, r *http.Request) {
	s.handleZoneAction(w, r, topology.StatusDraining)
}
func (s *Server) handleUndrainZone(w http.ResponseWriter, r *http.Request) {
	s.handleZoneAction(w, r, topology.StatusActive)
}
func (s *Server) handleEvacuateZone(w http.ResponseWriter, r *http.Request) {
	s.handleZoneAction(w, r, "evacuating")
}

// ---- Node handlers ----------------------------------------------------------

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	zoneFilter := r.URL.Query().Get("zone")
	var zoneID string
	if zoneFilter != "" {
		z, err := s.ctrl.Store.Topology.Store().GetZone(zoneFilter)
		if err == nil {
			zoneID = z.ID
		}
	}
	nodes, err := s.ctrl.Store.Topology.Store().ListNodes(zoneID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: nodes})
}

func (s *Server) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	var req topology.Node
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	created, err := s.ctrl.Store.Topology.Store().InsertNode(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: created})
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	node, err := s.ctrl.Store.Topology.Store().GetNode(r.PathValue("node"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "node not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: node})
}

func (s *Server) handlePatchNode(w http.ResponseWriter, r *http.Request) {
	node, err := s.ctrl.Store.Topology.Store().GetNode(r.PathValue("node"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "node not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req topology.Node
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if req.Name != "" {
		node.Name = req.Name
	}
	if req.Address != "" {
		node.Address = req.Address
	}
	if req.Status != "" {
		node.Status = req.Status
	}
	if req.Labels != nil {
		node.Labels = req.Labels
	}
	if req.CPUCount != 0 {
		node.CPUCount = req.CPUCount
	}
	if req.MemoryBytes != 0 {
		node.MemoryBytes = req.MemoryBytes
	}
	if req.DiskBytes != 0 {
		node.DiskBytes = req.DiskBytes
	}
	if err := s.ctrl.Store.Topology.Store().UpdateNode(node); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: node})
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().DeleteNode(r.PathValue("node")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleNodeAction(w http.ResponseWriter, r *http.Request, status string) {
	if err := s.ctrl.Store.Topology.Store().UpdateNodeStatus(r.PathValue("node"), status); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	node, _ := s.ctrl.Store.Topology.Store().GetNode(r.PathValue("node"))
	writeJSON(w, http.StatusOK, Envelope{Data: node})
}

func (s *Server) handleCordonNode(w http.ResponseWriter, r *http.Request) {
	s.handleNodeAction(w, r, topology.StatusCordoned)
}
func (s *Server) handleUncordonNode(w http.ResponseWriter, r *http.Request) {
	s.handleNodeAction(w, r, topology.StatusReady)
}
func (s *Server) handleDrainNode(w http.ResponseWriter, r *http.Request) {
	s.handleNodeAction(w, r, topology.StatusDraining)
}
func (s *Server) handleUndrainNode(w http.ResponseWriter, r *http.Request) {
	s.handleNodeAction(w, r, topology.StatusReady)
}

// ---- VPC handlers -----------------------------------------------------------

func (s *Server) handleListVPCs(w http.ResponseWriter, r *http.Request) {
	vpcs, err := s.ctrl.Store.Topology.Store().ListVPCs(s.project)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: vpcs})
}

func (s *Server) handleCreateVPC(w http.ResponseWriter, r *http.Request) {
	var req topology.VPC
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	req.Project = s.project
	if err := s.ctrl.Store.Topology.Store().InsertVPC(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	got, _ := s.ctrl.Store.Topology.Store().GetVPC(s.project, req.Slug)
	writeJSON(w, http.StatusCreated, Envelope{Data: got})
}

func (s *Server) handleGetVPC(w http.ResponseWriter, r *http.Request) {
	vpc, err := s.ctrl.Store.Topology.Store().GetVPC(s.project, r.PathValue("vpc"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "vpc not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: vpc})
}

func (s *Server) handlePatchVPC(w http.ResponseWriter, r *http.Request) {
	vpc, err := s.ctrl.Store.Topology.Store().GetVPC(s.project, r.PathValue("vpc"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "vpc not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req topology.VPC
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if req.Name != "" {
		vpc.Name = req.Name
	}
	if req.Status != "" {
		vpc.Status = req.Status
	}
	if req.MobilityPolicy != "" {
		vpc.MobilityPolicy = req.MobilityPolicy
	}
	if req.Labels != nil {
		vpc.Labels = req.Labels
	}
	if err := s.ctrl.Store.Topology.Store().UpdateVPC(vpc); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: vpc})
}

func (s *Server) handleDeleteVPC(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().DeleteVPC(s.project, r.PathValue("vpc")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListVPCSubnets(w http.ResponseWriter, r *http.Request) {
	vpc, err := s.ctrl.Store.Topology.Store().GetVPC(s.project, r.PathValue("vpc"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "vpc not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	subnets, err := s.ctrl.Store.Topology.Store().ListSubnets(vpc.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: subnets})
}

func (s *Server) handleCreateVPCSubnet(w http.ResponseWriter, r *http.Request) {
	vpc, err := s.ctrl.Store.Topology.Store().GetVPC(s.project, r.PathValue("vpc"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "vpc not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req topology.VPCSubnet
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	req.VPCID = vpc.ID
	req.RealmID = vpc.RealmID
	if err := s.ctrl.Store.Topology.Store().InsertSubnet(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	subnets, _ := s.ctrl.Store.Topology.Store().ListSubnets(vpc.ID)
	var created topology.VPCSubnet
	for _, sub := range subnets {
		if sub.Slug == req.Slug {
			created = sub
		}
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: created})
}

func (s *Server) handleListVPCRoutes(w http.ResponseWriter, r *http.Request) {
	vpc, err := s.ctrl.Store.Topology.Store().GetVPC(s.project, r.PathValue("vpc"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "vpc not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	routes, err := s.ctrl.Store.Topology.Store().ListRoutes(vpc.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: routes})
}

func (s *Server) handleCreateVPCRoute(w http.ResponseWriter, r *http.Request) {
	vpc, err := s.ctrl.Store.Topology.Store().GetVPC(s.project, r.PathValue("vpc"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "vpc not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req topology.VPCRoute
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	req.VPCID = vpc.ID
	req.RealmID = vpc.RealmID
	if err := s.ctrl.Store.Topology.Store().InsertRoute(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	routes, _ := s.ctrl.Store.Topology.Store().ListRoutes(vpc.ID)
	writeJSON(w, http.StatusCreated, Envelope{Data: routes})
}

// ---- Placement policy handlers ----------------------------------------------

func (s *Server) handleListPlacementPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := s.ctrl.Store.Topology.Store().ListPlacementPolicies(s.project)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: policies})
}

func (s *Server) handleCreatePlacementPolicy(w http.ResponseWriter, r *http.Request) {
	var req topology.PlacementPolicy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	req.Project = s.project
	if err := s.ctrl.Store.Topology.Store().InsertPlacementPolicy(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	got, _ := s.ctrl.Store.Topology.Store().GetPlacementPolicy(s.project, req.Slug)
	writeJSON(w, http.StatusCreated, Envelope{Data: got})
}

func (s *Server) handleGetPlacementPolicy(w http.ResponseWriter, r *http.Request) {
	p, err := s.ctrl.Store.Topology.Store().GetPlacementPolicy(s.project, r.PathValue("policy"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "policy not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: p})
}

func (s *Server) handleDeletePlacementPolicy(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().DeletePlacementPolicy(s.project, r.PathValue("policy")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Scheduler handlers -----------------------------------------------------

func (s *Server) handleSchedulerSimulate(w http.ResponseWriter, r *http.Request) {
	var req topology.PlacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if req.Project == "" {
		req.Project = s.project
	}
	sched := topology.NewScheduler(s.ctrl.Store.Topology.Store())
	result := sched.Simulate(r.Context(), req)
	writeJSON(w, http.StatusOK, Envelope{Data: result})
}

func (s *Server) handleSchedulerCapacity(w http.ResponseWriter, r *http.Request) {
	regionFilter := r.URL.Query().Get("region")
	var zoneID string
	if zf := r.URL.Query().Get("zone"); zf != "" {
		z, err := s.ctrl.Store.Topology.Store().GetZone(zf)
		if err == nil {
			zoneID = z.ID
		}
	} else if regionFilter != "" {
		reg, err := s.ctrl.Store.Topology.Store().GetRegion(regionFilter)
		if err != nil {
			writeJSON(w, http.StatusNotFound, Envelope{Error: "region not found"})
			return
		}
		zones, _ := s.ctrl.Store.Topology.Store().ListZones(reg.ID)
		type zoneCapacity struct {
			ZoneID      string `json:"zoneId"`
			Zone        string `json:"zone"`
			NodeCount   int    `json:"nodeCount"`
			TotalCPU    int    `json:"totalCpu"`
			TotalMemory int64  `json:"totalMemory"`
		}
		var caps []zoneCapacity
		for _, z := range zones {
			nodes, _ := s.ctrl.Store.Topology.Store().ListNodes(z.ID)
			cap := zoneCapacity{ZoneID: z.ID, Zone: z.Slug, NodeCount: len(nodes)}
			for _, n := range nodes {
				if n.Status == topology.StatusReady {
					cap.TotalCPU += n.CPUCount
					cap.TotalMemory += n.MemoryBytes
				}
			}
			caps = append(caps, cap)
		}
		writeJSON(w, http.StatusOK, Envelope{Data: caps})
		return
	}
	nodes, _ := s.ctrl.Store.Topology.Store().ListNodes(zoneID)
	var totalCPU int
	var totalMem int64
	ready := 0
	for _, n := range nodes {
		if n.Status == topology.StatusReady {
			ready++
			totalCPU += n.CPUCount
			totalMem += n.MemoryBytes
		}
	}
	writeJSON(w, http.StatusOK, Envelope{Data: map[string]any{
		"totalNodes": len(nodes), "readyNodes": ready,
		"totalCpu": totalCPU, "totalMemory": totalMem,
	}})
}

func (s *Server) handleSchedulerPlacements(w http.ResponseWriter, r *http.Request) {
	// Placement decision history is not persisted; the scheduler computes
	// placement on demand against current node state. This endpoint returns that
	// node inventory (capacity/readiness used for placement), not a decision log.
	nodes, err := s.ctrl.Store.Topology.Store().ListNodes("")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: map[string]any{
		"nodes":            nodes,
		"placementHistory": false,
		"note":             "placement decisions are computed on demand and not retained; this is the current placement-eligible node inventory",
	}})
}

// ---- Service health handlers ------------------------------------------------

func (s *Server) handleListServiceHealth(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	health, err := s.ctrl.Store.Topology.Store().ListServiceHealth(
		q.Get("scope"), q.Get("realm"), q.Get("region"), q.Get("zone"),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: health})
}

func (s *Server) handleUpsertServiceHealth(w http.ResponseWriter, r *http.Request) {
	var req topology.ServiceHealth
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if err := s.ctrl.Store.Topology.Store().UpsertServiceHealth(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Migration plan handlers ------------------------------------------------

func (s *Server) handleListMigrationPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := s.ctrl.Store.Topology.Store().ListMigrationPlans(s.project)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: plans})
}

func (s *Server) handleCreateMigrationPlan(w http.ResponseWriter, r *http.Request) {
	var req topology.MigrationPlan
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	req.Project = s.project
	if err := s.ctrl.Store.Topology.Store().InsertMigrationPlan(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	got, _ := s.ctrl.Store.Topology.Store().GetMigrationPlan(s.project, req.ID)
	writeJSON(w, http.StatusCreated, Envelope{Data: got})
}

func (s *Server) handleGetMigrationPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := s.ctrl.Store.Topology.Store().GetMigrationPlan(s.project, r.PathValue("plan"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "migration plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: plan})
}

// ---- Node heartbeat ---------------------------------------------------------

func (s *Server) handleNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("node")
	node, err := s.ctrl.Store.Topology.Store().GetNode(nodeID)
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "node not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req topology.NodeHeartbeat
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	req.NodeID = node.ID
	if req.SeenAt == "" {
		req.SeenAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := s.ctrl.Store.Topology.Store().UpsertHeartbeat(req); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	status := req.Status
	if status == "" {
		status = topology.StatusReady
	}
	_ = s.ctrl.Store.Topology.Store().UpdateNodeHeartbeat(node.ID, status)
	// Track per-node agent version for rolling-upgrade skew visibility.
	if req.Version != "" && req.Version != node.AgentVersion {
		node.AgentVersion = req.Version
		_ = s.ctrl.Store.Topology.Store().UpdateNode(node)
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Node inventory ---------------------------------------------------------

func (s *Server) handleNodeInventory(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("node")
	node, err := s.ctrl.Store.Topology.Store().GetNode(nodeID)
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "node not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req struct {
		CPUCount       int               `json:"cpuCount"`
		MemoryBytes    int64             `json:"memoryBytes"`
		DiskBytes      int64             `json:"diskBytes"`
		GPUCount       int               `json:"gpuCount"`
		GPUMemoryBytes int64             `json:"gpuMemoryBytes"`
		AgentVersion   string            `json:"agentVersion"`
		Roles          []string          `json:"roles"`
		Labels         map[string]string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if req.CPUCount != 0 {
		node.CPUCount = req.CPUCount
	}
	if req.MemoryBytes != 0 {
		node.MemoryBytes = req.MemoryBytes
	}
	if req.DiskBytes != 0 {
		node.DiskBytes = req.DiskBytes
	}
	if req.GPUCount != 0 {
		node.GPUCount = req.GPUCount
	}
	if req.GPUMemoryBytes != 0 {
		node.GPUMemoryBytes = req.GPUMemoryBytes
	}
	if req.AgentVersion != "" {
		node.AgentVersion = req.AgentVersion
	}
	if err := s.ctrl.Store.Topology.Store().UpdateNode(node); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	if req.Roles != nil {
		_ = s.ctrl.Store.Topology.Store().SetNodeRoles(node.ID, req.Roles)
	}
	if req.Labels != nil {
		_ = s.ctrl.Store.Topology.Store().SetNodeLabels(node.ID, req.Labels)
	}
	got, _ := s.ctrl.Store.Topology.Store().GetNode(node.ID)
	writeJSON(w, http.StatusOK, Envelope{Data: got})
}

// ---- Node services ----------------------------------------------------------

func (s *Server) handlePostNodeServices(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("node")
	node, err := s.ctrl.Store.Topology.Store().GetNode(nodeID)
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "node not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var svcs []topology.NodeService
	if err := json.NewDecoder(r.Body).Decode(&svcs); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	for _, svc := range svcs {
		svc.NodeID = node.ID
		if err := s.ctrl.Store.Topology.Store().UpsertNodeService(svc); err != nil {
			writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListNodeServices(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("node")
	node, err := s.ctrl.Store.Topology.Store().GetNode(nodeID)
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "node not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	svcs, err := s.ctrl.Store.Topology.Store().ListNodeServices(node.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: svcs})
}

// ---- Node approve -----------------------------------------------------------

func (s *Server) handleApproveNode(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().UpdateNodeStatus(r.PathValue("node"), topology.StatusReady); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	node, _ := s.ctrl.Store.Topology.Store().GetNode(r.PathValue("node"))
	writeJSON(w, http.StatusOK, Envelope{Data: node})
}

// ---- Node join --------------------------------------------------------------

func (s *Server) handleNodeJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token        string            `json:"token"`
		Name         string            `json:"name"`
		Address      string            `json:"address"`
		Roles        []string          `json:"roles"`
		Labels       map[string]string `json:"labels"`
		CPUCount     int               `json:"cpuCount"`
		MemoryBytes  int64             `json:"memoryBytes"`
		DiskBytes    int64             `json:"diskBytes"`
		GPUCount     int               `json:"gpuCount"`
		AgentVersion string            `json:"agentVersion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	jt, err := s.ctrl.Store.Topology.Store().ConsumeJoinToken(req.Token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, Envelope{Error: "invalid or expired join token: " + err.Error()})
		return
	}
	node := topology.Node{
		RealmID:      jt.RealmID,
		RegionID:     jt.RegionID,
		ZoneID:       jt.ZoneID,
		Name:         req.Name,
		Slug:         req.Name,
		Address:      req.Address,
		Status:       "pending",
		CPUCount:     req.CPUCount,
		MemoryBytes:  req.MemoryBytes,
		DiskBytes:    req.DiskBytes,
		GPUCount:     req.GPUCount,
		AgentVersion: req.AgentVersion,
		Labels:       req.Labels,
	}
	created, err := s.ctrl.Store.Topology.Store().InsertNode(node)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	roles := jt.Roles
	if len(req.Roles) > 0 {
		roles = req.Roles
	}
	if len(roles) > 0 {
		_ = s.ctrl.Store.Topology.Store().SetNodeRoles(created.ID, roles)
	}
	bearer, tok, err := s.ctrl.Store.IAM.Issue(created.Name, "node", created.ID, 365*24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: "node created but token issuance failed: " + err.Error()})
		return
	}
	certPEM, _, _ := s.ctrl.Store.Certs.IssueNodeCert(
		created.Name+".node.capper.internal",
		[]string{created.Name + ".node.capper.internal", created.Address},
		365*24*time.Hour,
	)
	writeJSON(w, http.StatusCreated, Envelope{Data: map[string]any{
		"node":    created,
		"token":   tok,
		"bearer":  bearer,
		"certPEM": string(certPEM),
	}})
}

// ---- Join tokens ------------------------------------------------------------

func (s *Server) handleListJoinTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.ctrl.Store.Topology.Store().ListJoinTokens()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: tokens})
}

func (s *Server) handleCreateJoinToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RealmID  string   `json:"realmId"`
		RegionID string   `json:"regionId"`
		ZoneID   string   `json:"zoneId"`
		Roles    []string `json:"roles"`
		TTL      string   `json:"ttl"`
		Uses     int      `json:"uses"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	ttl := 24 * time.Hour
	if req.TTL != "" {
		d, err := time.ParseDuration(req.TTL)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, Envelope{Error: "invalid ttl: " + err.Error()})
			return
		}
		ttl = d
	}
	uses := req.Uses
	if uses <= 0 {
		uses = 1
	}
	pt, pid := principalFromContext(r.Context())
	jt := topology.JoinToken{
		RealmID:   req.RealmID,
		RegionID:  req.RegionID,
		ZoneID:    req.ZoneID,
		Roles:     req.Roles,
		UsesLeft:  uses,
		ExpiresAt: time.Now().UTC().Add(ttl).Format(time.RFC3339),
		CreatedBy: pt + ":" + pid,
	}
	created, err := s.ctrl.Store.Topology.Store().CreateJoinToken(jt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: created})
}

func (s *Server) handleDeleteJoinToken(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().DeleteJoinToken(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Node pools -------------------------------------------------------------

func (s *Server) handleListNodePools(w http.ResponseWriter, r *http.Request) {
	pools, err := s.ctrl.Store.Topology.Store().ListPools()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: pools})
}

func (s *Server) handleCreateNodePool(w http.ResponseWriter, r *http.Request) {
	var req topology.NodePool
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	created, err := s.ctrl.Store.Topology.Store().CreatePool(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: created})
}

func (s *Server) handleGetNodePool(w http.ResponseWriter, r *http.Request) {
	pool, err := s.ctrl.Store.Topology.Store().GetPool(r.PathValue("pool"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "pool not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: pool})
}

func (s *Server) handlePatchNodePool(w http.ResponseWriter, r *http.Request) {
	pool, err := s.ctrl.Store.Topology.Store().GetPool(r.PathValue("pool"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "pool not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if err := s.ctrl.Store.Topology.Store().UpdatePool(pool.ID, updates); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	got, _ := s.ctrl.Store.Topology.Store().GetPool(pool.ID)
	writeJSON(w, http.StatusOK, Envelope{Data: got})
}

func (s *Server) handleDeleteNodePool(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.Topology.Store().DeletePool(r.PathValue("pool")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddPoolMember(w http.ResponseWriter, r *http.Request) {
	pool, err := s.ctrl.Store.Topology.Store().GetPool(r.PathValue("pool"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "pool not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var req struct {
		NodeID string `json:"nodeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
		return
	}
	if err := s.ctrl.Store.Topology.Store().AddPoolMember(pool.ID, req.NodeID); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemovePoolMember(w http.ResponseWriter, r *http.Request) {
	pool, err := s.ctrl.Store.Topology.Store().GetPool(r.PathValue("pool"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "pool not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	if err := s.ctrl.Store.Topology.Store().RemovePoolMember(pool.ID, r.PathValue("nodeID")); err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListPoolMembers(w http.ResponseWriter, r *http.Request) {
	pool, err := s.ctrl.Store.Topology.Store().GetPool(r.PathValue("pool"))
	if err == topology.ErrNotFound {
		writeJSON(w, http.StatusNotFound, Envelope{Error: "pool not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	members, err := s.ctrl.Store.Topology.Store().ListPoolMembers(pool.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Envelope{Data: members})
}

// ---- Service nodes ----------------------------------------------------------

func (s *Server) handleListServiceNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.ctrl.Store.Topology.Store().ListNodes("")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	grouped := map[string][]topology.Node{}
	for _, n := range nodes {
		roles, _ := s.ctrl.Store.Topology.Store().GetNodeRoles(n.ID)
		for _, role := range roles {
			grouped[role] = append(grouped[role], n)
		}
	}
	writeJSON(w, http.StatusOK, Envelope{Data: grouped})
}

func (s *Server) handleGetServiceNodesByRole(w http.ResponseWriter, r *http.Request) {
	role := r.PathValue("role")
	nodes, err := s.ctrl.Store.Topology.Store().ListNodes("")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Envelope{Error: err.Error()})
		return
	}
	var out []topology.Node
	for _, n := range nodes {
		roles, _ := s.ctrl.Store.Topology.Store().GetNodeRoles(n.ID)
		for _, nr := range roles {
			if nr == role {
				out = append(out, n)
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, Envelope{Data: out})
}
