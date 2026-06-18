package api

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"capper/internal/store"
)

// handleCapsuleTypeAudit returns resource events for a specific capsule type.
// Satisfies GET /api/v1/capsule-types/{name}/audit used by InstanceTypeDetail.
func (s *Server) handleCapsuleTypeAudit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "capsule-type:inspect", "capsule-type/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	events, err := s.ctrl.Store.Events.List(store.ListEventsOptions{
		ResourceType: "capsule-type",
		ResourceID:   name,
		Limit:        queryInt(r, "limit", 100),
	})
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, events, nil)
}

// handleMarketplaceScans returns the scan result fields for a marketplace listing.
// Satisfies GET /api/v1/marketplace/images/{id}/scans used by MarketplaceReview.
func (s *Server) handleMarketplaceScans(w http.ResponseWriter, r *http.Request) {
	l, err := s.ctrl.Store.Marketplace.Get(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "listing not found")
		return
	}
	writeData(w, map[string]any{
		"status":     l.ScanStatus,
		"findings":   l.ScanFindings,
		"severities": l.ScanSeverities,
		"scannedAt":  l.ScanScannedAt,
		"sbomDigest": l.SBOMDigest,
	}, nil)
}

// handleInstanceMetadataTab returns a named sub-section of the instance's metadata.
// Satisfies GET /api/v1/instances/{id}/metadata/{tab} used by InstanceMetadata page.
func (s *Server) handleInstanceMetadataTab(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tab := r.PathValue("tab")

	switch tab {
	case "status":
		s.handleInstanceMetadataStatus(w, r, id)
	case "overview", "networking", "storage", "metadata":
		// These tabs are served from the per-instance metadata JSON file.
		meta, ok := loadInstanceMetadata(s, id)
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("metadata tab %q not available for instance %s", tab, id))
			return
		}
		if val, exists := meta[tab]; exists {
			writeData(w, val, nil)
			return
		}
		writeError(w, http.StatusNotFound, fmt.Sprintf("metadata tab %q not available for instance %s", tab, id))
	case "monitoring":
		s.handleInstanceMetadataMonitoring(w, r, id)
	case "logs":
		s.handleInstanceMetadataLogs(w, r, id, "console.log")
	case "console":
		s.handleInstanceMetadataLogs(w, r, id, "console.log")
	case "events":
		s.handleInstanceMetadataEvents(w, r, id)
	default:
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown metadata tab: %q", tab))
	}
}

func (s *Server) handleInstanceMetadataStatus(w http.ResponseWriter, _ *http.Request, id string) {
	inst, err := s.ctrl.Store.ResolveInstance(id)
	running := err == nil && inst != nil && inst.Status == "running"
	writeData(w, map[string]any{
		"running":    running,
		"address":    "http://169.254.169.254/capper/v1",
		"tokenState": "valid",
	}, nil)
}

func (s *Server) handleInstanceMetadataMonitoring(w http.ResponseWriter, _ *http.Request, id string) {
	inst, err := s.ctrl.Store.ResolveInstance(id)
	if err != nil || inst == nil {
		writeNotFound(w, "instance not found")
		return
	}
	// Return last known resource usage from the instance record.
	// Detailed metrics come from the node agent heartbeat when available.
	writeData(w, map[string]any{
		"instanceId": inst.ID,
		"status":     inst.Status,
		"nodeId":     inst.NodeID,
		"cpu": map[string]any{
			"timeSecs": inst.Resources.CPUTimeSecs,
		},
		"memory": map[string]any{
			"allocatedBytes": inst.Resources.MemoryBytes,
		},
	}, nil)
}

func (s *Server) handleInstanceMetadataLogs(w http.ResponseWriter, r *http.Request, id, filename string) {
	logPath := filepath.Join(s.ctrl.Store.Paths.Root, "instances", id, filename)
	tail := queryInt(r, "tail", 100)

	lines, err := tailFile(logPath, tail)
	if err != nil {
		// Return empty log rather than 500 when the log file simply doesn't exist yet.
		writeData(w, map[string]any{"lines": []string{}}, nil)
		return
	}
	writeData(w, map[string]any{"lines": lines}, nil)
}

func (s *Server) handleInstanceMetadataEvents(w http.ResponseWriter, r *http.Request, id string) {
	events, err := s.ctrl.Store.Events.List(store.ListEventsOptions{
		ResourceType: "instance",
		ResourceID:   id,
		Limit:        queryInt(r, "limit", 100),
	})
	if err != nil {
		writeInternal(w, err)
		return
	}
	if events == nil {
		events = []store.ResourceEvent{}
	}
	writeData(w, events, nil)
}

// tailFile reads the last n lines from path. Returns an error only if the file
// cannot be opened at all — a missing file is handled by the callers.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return lines, err
	}
	if len(lines) <= n {
		return lines, nil
	}
	return lines[len(lines)-n:], nil
}
