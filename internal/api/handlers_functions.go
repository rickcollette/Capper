package api

import (
	"encoding/json"
	"io"
	"net/http"

	"capper/internal/functions"
)

func (s *Server) fnStore() *functions.Store { return s.ctrl.Store.Functions }
func (s *Server) fnManager() *functions.Manager {
	return functions.NewManager(s.ctrl.Store.Functions, nil)
}

// POST /api/v1/functions
func (s *Server) handleCreateFunction(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var fn functions.Function
	if err := json.NewDecoder(r.Body).Decode(&fn); err != nil {
		writeBadRequest(w, err)
		return
	}
	if fn.Name == "" || fn.Runtime == "" {
		writeError(w, http.StatusBadRequest, "name and runtime are required")
		return
	}
	if fn.Project == "" {
		fn.Project = s.project
	}
	created, err := s.fnStore().CreateFunction(fn)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, created, nil)
}

// GET /api/v1/functions
func (s *Server) handleListFunctions(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	fns, err := s.fnStore().ListFunctions(r.URL.Query().Get("project"))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, fns, nil)
}

// GET /api/v1/functions/{id}
func (s *Server) handleGetFunction(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	fn, err := s.fnStore().GetFunction(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "function not found")
		return
	}
	writeData(w, fn, nil)
}

// PATCH /api/v1/functions/{id}
func (s *Server) handlePatchFunction(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var fields map[string]any
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.fnStore().UpdateFunction(r.PathValue("id"), fields); err != nil {
		writeInternal(w, err)
		return
	}
	fn, _ := s.fnStore().GetFunction(r.PathValue("id"))
	writeData(w, fn, nil)
}

// DELETE /api/v1/functions/{id}
func (s *Server) handleDeleteFunction(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.fnStore().DeleteFunction(r.PathValue("id")); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"deleted": r.PathValue("id")}, nil)
}

// POST /api/v1/functions/{id}/versions
func (s *Server) handleCreateFunctionVersion(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var v functions.FunctionVersion
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeBadRequest(w, err)
		return
	}
	v.FunctionID = r.PathValue("id")
	created, err := s.fnStore().AddVersion(v)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, created, nil)
}

// GET /api/v1/functions/{id}/versions
func (s *Server) handleListFunctionVersions(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	versions, err := s.fnStore().ListVersions(r.PathValue("id"))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, versions, nil)
}

// POST /api/v1/functions/{id}/invoke
func (s *Server) handleInvokeFunction(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:invoke", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	fn, err := s.fnStore().GetFunction(r.PathValue("id"))
	if err != nil {
		writeNotFound(w, "function not found")
		return
	}
	payload, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20)) // cap payload at 4 MiB
	_, pid := principalFromContext(r.Context())
	res, err := s.fnManager().Invoke(r.Context(), fn, payload, "", pid, "http")
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, res, nil)
}

// POST /api/v1/functions/{id}/triggers
func (s *Server) handleCreateTrigger(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var t functions.Trigger
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeBadRequest(w, err)
		return
	}
	t.FunctionID = r.PathValue("id")
	if t.Project == "" {
		t.Project = s.project
	}
	if t.Type == "" {
		writeError(w, http.StatusBadRequest, "trigger type is required")
		return
	}
	created, err := s.fnStore().AddTrigger(t)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, created, nil)
}

// GET /api/v1/functions/{id}/triggers
func (s *Server) handleListTriggers(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	triggers, err := s.fnStore().ListTriggers(r.PathValue("id"))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, triggers, nil)
}

// DELETE /api/v1/functions/{id}/triggers/{triggerId}
func (s *Server) handleDeleteTrigger(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:update", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.fnStore().DeleteTrigger(r.PathValue("triggerId")); err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, map[string]any{"deleted": r.PathValue("triggerId")}, nil)
}

// GET /api/v1/functions/{id}/invocations
func (s *Server) handleListInvocations(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "function:read", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	invs, err := s.fnStore().ListInvocations(r.PathValue("id"), queryInt(r, "limit", 100))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, invs, nil)
}
