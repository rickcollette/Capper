package api

import (
	"encoding/json"
	"net/http"

	"capper/internal/vpc"
)

func (s *Server) handleListENIs(w http.ResponseWriter, r *http.Request) {
	vpcID := r.URL.Query().Get("vpcId")
	enis, err := s.ctrl.Store.VPC.ListENIs(vpcID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, enis, nil)
}

func (s *Server) handleCreateENI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VPCID      string   `json:"vpcId"`
		SubnetID   string   `json:"subnetId"`
		PrivateIP  string   `json:"privateIpAddress"`
		SGIDs      []string `json:"securityGroupIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	eni, err := s.ctrl.Store.VPC.CreateENI(req.VPCID, req.SubnetID, req.SGIDs, req.PrivateIP)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: eni})
}

func (s *Server) handleGetENI(w http.ResponseWriter, r *http.Request) {
	eni, err := s.ctrl.Store.VPC.GetENI(r.PathValue("eniId"))
	if err != nil {
		writeNotFound(w, "eni not found")
		return
	}
	writeData(w, eni, nil)
}

func (s *Server) handleDeleteENI(w http.ResponseWriter, r *http.Request) {
	if err := s.ctrl.Store.VPC.DeleteENI(r.PathValue("eniId")); err != nil {
		writeBadRequest(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAttachENI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InstanceID      string `json:"instanceId"`
		AttachmentIndex int    `json:"attachmentIndex"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	eni, err := s.ctrl.Store.VPC.AttachENI(r.PathValue("eniId"), req.InstanceID, req.AttachmentIndex)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeData(w, eni, nil)
}

func (s *Server) handleDetachENI(w http.ResponseWriter, r *http.Request) {
	eni, err := s.ctrl.Store.VPC.DetachENI(r.PathValue("eniId"))
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeData(w, eni, nil)
}

func (s *Server) handleAssignENIPrivateIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrivateIP string `json:"privateIpAddress"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.ctrl.Store.VPC.AssignENIPrivateIP(r.PathValue("eniId"), req.PrivateIP, false); err != nil {
		writeBadRequest(w, err)
		return
	}
	eni, _ := s.ctrl.Store.VPC.GetENI(r.PathValue("eniId"))
	writeData(w, eni, nil)
}

// public IP aliases (Networking spec §12.1)
func (s *Server) handleListPublicIPs(w http.ResponseWriter, r *http.Request) {
	s.handleListIPs(w, r)
}

func (s *Server) handleAllocatePublicIP(w http.ResponseWriter, r *http.Request) {
	s.handleReserveIP(w, r)
}

func (s *Server) handleAssociatePublicIP(w http.ResponseWriter, r *http.Request) {
	s.handleAttachIP(w, r)
}

func (s *Server) handleDisassociatePublicIP(w http.ResponseWriter, r *http.Request) {
	s.handleDetachIP(w, r)
}

func (s *Server) handleReleasePublicIP(w http.ResponseWriter, r *http.Request) {
	s.handleReleaseIP(w, r)
}

// noop import guard for vpc in attach handlers
var _ = vpc.ENIStatusAvailable
