package api

import (
	"net/http"

	"capper/internal/marketplace"
)

// Marketplace API handlers operate on the SQLite-backed marketplace store
// (internal/marketplace), the same backend used by the `capper market` CLI, so
// listings are consistent across the CLI, API, SDK, and Web UI.

func (s *Server) handleMarketplaceImages(w http.ResponseWriter, r *http.Request) {
	listings, err := s.ctrl.Store.Marketplace.List()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, listings, map[string]bool{"enabled": true})
}

func (s *Server) handleMarketplaceImage(w http.ResponseWriter, r *http.Request) {
	l, err := s.ctrl.Store.Marketplace.Get(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "listing not found")
		return
	}
	writeData(w, l, nil)
}

func (s *Server) handleMarketplaceInstall(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "marketplace:install", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	if _, err := s.ctrl.Store.Marketplace.Get(id); err != nil {
		writeNotFound(w, "listing not found")
		return
	}
	if err := s.ctrl.Store.Marketplace.Install(id, s.project, nil, nil); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]string{"status": "installed", "id": id}, nil)
}

func (s *Server) handleMarketplaceApprove(w http.ResponseWriter, r *http.Request) {
	s.updateMarketplaceStatus(w, r, marketplace.StatusApproved, "marketplace:approve")
}

func (s *Server) handleMarketplaceReject(w http.ResponseWriter, r *http.Request) {
	s.updateMarketplaceStatus(w, r, marketplace.StatusRejected, "marketplace:moderate")
}

func (s *Server) handleMarketplaceQuarantine(w http.ResponseWriter, r *http.Request) {
	s.updateMarketplaceStatus(w, r, marketplace.StatusQuarantined, "marketplace:moderate")
}

func (s *Server) updateMarketplaceStatus(w http.ResponseWriter, r *http.Request, status, action string) {
	if err := s.authorize(r, action, "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	id := r.PathValue("id")
	if _, err := s.ctrl.Store.Marketplace.Get(id); err != nil {
		writeNotFound(w, "listing not found")
		return
	}
	if err := s.ctrl.Store.Marketplace.UpdateStatus(id, status); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]string{"status": status, "id": id}, nil)
}
