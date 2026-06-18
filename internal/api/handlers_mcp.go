package api

import (
	"encoding/json"
	"io"
	"net/http"

	"capper/internal/mcpserver"
)

func (s *Server) mcpStore() *mcpserver.Store { return s.ctrl.Store.MCPServers }
func (s *Server) mcpManager() *mcpserver.Manager {
	return mcpserver.NewManager(s.ctrl.Store.MCPServers)
}

// POST /api/v1/mcp/servers
func (s *Server) handleCreateMCPServer(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "mcp:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var srv mcpserver.Server
	if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
		writeBadRequest(w, err)
		return
	}
	if srv.Name == "" || srv.Runtime == "" {
		writeError(w, http.StatusBadRequest, "name and runtime are required")
		return
	}
	if srv.Project == "" {
		srv.Project = s.project
	}
	created, err := s.mcpStore().CreateServer(srv)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, created, nil)
}

// GET /api/v1/mcp/servers
func (s *Server) handleListMCPServers(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "mcp:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	servers, err := s.mcpStore().ListServers(r.URL.Query().Get("project"))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, servers, nil)
}

// GET /api/v1/mcp/servers/{id}
func (s *Server) handleGetMCPServer(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "mcp:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	srv, err := s.mcpStore().GetServer(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "mcp server not found")
		return
	}
	writeData(w, srv, nil)
}

// DELETE /api/v1/mcp/servers/{id}
func (s *Server) handleDeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "mcp:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.mcpStore().DeleteServer(r.PathValue("id")); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"deleted": r.PathValue("id")}, nil)
}

// GET /api/v1/mcp/servers/{id}/tools
func (s *Server) handleListMCPTools(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "mcp:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	tools, err := s.mcpStore().ListTools(r.PathValue("id"))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, tools, nil)
}

// POST /api/v1/mcp/servers/{id}/tools/sync — register/update the server's tool set.
func (s *Server) handleSyncMCPTools(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "mcp:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Tools []mcpserver.Tool `json:"tools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	serverID := r.PathValue("id")
	out := make([]mcpserver.Tool, 0, len(req.Tools))
	for _, t := range req.Tools {
		t.ServerID = serverID
		saved, err := s.mcpStore().UpsertTool(t)
		if err != nil {
			writeInternal(w, err)
			return
		}
		out = append(out, saved)
	}
	writeData(w, out, nil)
}

// POST /api/v1/mcp/servers/{id}/tools/{toolName}/invoke
func (s *Server) handleInvokeMCPTool(w http.ResponseWriter, r *http.Request) {
	// Note: fine-grained per-tool IAM is enforced inside the manager via the
	// authorize callback; this coarse check ensures the caller may use MCP at all.
	if err := s.authorize(r, "mcp:invoke", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	srv, err := s.mcpStore().GetServer(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "mcp server not found")
		return
	}
	args, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	pt, pid := principalFromContext(r.Context())
	authorize := func(action, resource string) error {
		return s.ctrl.Store.IAM.Authorize(pt, pid, action, resource)
	}
	res, err := s.mcpManager().InvokeTool(srv, r.PathValue("toolName"), args,
		mcpserver.CallContext{Principal: pid, RequestID: r.Header.Get("X-Request-ID")}, authorize)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, res, nil)
}

// GET /api/v1/mcp/servers/{id}/invocations
func (s *Server) handleListMCPInvocations(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "mcp:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	invs, err := s.mcpStore().ListInvocations(r.PathValue("id"), queryInt(r, "limit", 100))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, invs, nil)
}

// GET /api/v1/mcp/approvals
func (s *Server) handleListMCPApprovals(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "mcp:approve", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = mcpserver.ApprovalPending
	}
	approvals, err := s.mcpStore().ListApprovals(status)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, approvals, nil)
}

// POST /api/v1/mcp/approvals/{id}/approve
func (s *Server) handleApproveMCP(w http.ResponseWriter, r *http.Request) {
	s.decideMCPApproval(w, r, mcpserver.ApprovalApproved)
}

// POST /api/v1/mcp/approvals/{id}/deny
func (s *Server) handleDenyMCP(w http.ResponseWriter, r *http.Request) {
	s.decideMCPApproval(w, r, mcpserver.ApprovalDenied)
}

func (s *Server) decideMCPApproval(w http.ResponseWriter, r *http.Request, decision string) {
	if err := s.authorize(r, "mcp:approve", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_, pid := principalFromContext(r.Context())
	appr, err := s.mcpManager().ResolveApproval(r.PathValue("id"), decision, pid, body.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, appr, nil)
}
