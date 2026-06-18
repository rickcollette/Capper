package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
)

// ---- KMS keys ---------------------------------------------------------------

func (s *Server) handleListKMSKeys(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "kms:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	keys, err := s.ctrl.Store.KMS.List(s.project)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, keys, nil)
}

func (s *Server) handleCreateKMSKey(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "kms:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	k, err := s.ctrl.Store.KMS.Create(req.Name, s.project)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	// Never expose the wrapped key material to the caller.
	k.EncryptedKey = nil
	s.recordEvent(r, "kms_key", k.ID, "kms.key.created", map[string]any{"name": req.Name})
	writeJSON(w, http.StatusCreated, Envelope{Data: k})
}

func (s *Server) handleRotateKMSKey(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "kms:rotate", "kms/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	k, err := s.ctrl.Store.KMS.Rotate(name, s.project)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	k.EncryptedKey = nil
	s.recordEvent(r, "kms_key", k.ID, "kms.key.rotated", map[string]any{"name": name})
	writeData(w, k, nil)
}

func (s *Server) handleKMSEncrypt(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "kms:encrypt", "kms/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Plaintext string `json:"plaintext"` // base64-encoded bytes
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	plain, err := base64.StdEncoding.DecodeString(req.Plaintext)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	ct, err := s.ctrl.Store.KMS.Encrypt(name, s.project, plain)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeData(w, map[string]string{"ciphertext": base64.StdEncoding.EncodeToString(ct)}, nil)
}

func (s *Server) handleKMSDecrypt(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "kms:decrypt", "kms/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Ciphertext string `json:"ciphertext"` // base64-encoded bytes
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	ct, err := base64.StdEncoding.DecodeString(req.Ciphertext)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	plain, err := s.ctrl.Store.KMS.Decrypt(name, s.project, ct)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeData(w, map[string]string{"plaintext": base64.StdEncoding.EncodeToString(plain)}, nil)
}

func (s *Server) handleDeleteKMSKey(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "kms:delete", "kms/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.ctrl.Store.KMS.Delete(name, s.project); err != nil {
		writeBadRequest(w, err)
		return
	}
	s.recordEvent(r, "kms_key", name, "kms.key.deleted", map[string]any{"name": name})
	w.WriteHeader(http.StatusNoContent)
}

// ---- Governance policies ----------------------------------------------------

func (s *Server) handleListGovernancePolicies(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "governance:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	policies := s.ctrl.Store.Billing.ListGovernancePolicies(s.project)
	writeData(w, policies, nil)
}

func (s *Server) handleCreateGovernancePolicy(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "governance:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name      string `json:"name"`
		Resource  string `json:"resource"`
		Action    string `json:"action"`
		Effect    string `json:"effect"`
		Condition string `json:"condition,omitempty"`
		Priority  int    `json:"priority,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	rule := s.ctrl.Store.Billing.AddGovernancePolicy(
		req.Name, s.project, req.Resource, req.Action, req.Effect, req.Condition, req.Priority,
	)
	s.recordEvent(r, "governance_policy", rule.ID, "governance.policy.created", map[string]any{"name": req.Name})
	writeJSON(w, http.StatusCreated, Envelope{Data: rule})
}

func (s *Server) handleEvaluateGovernance(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "governance:evaluate", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Resource string            `json:"resource"`
		Action   string            `json:"action"`
		Labels   map[string]string `json:"labels,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	allowed, matchedRule := s.ctrl.Store.Billing.EvaluateGovernance(s.project, req.Resource, req.Action, req.Labels)
	writeData(w, map[string]any{
		"allowed":     allowed,
		"matchedRule": matchedRule,
	}, nil)
}
