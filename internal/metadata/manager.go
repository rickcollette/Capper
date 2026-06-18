package metadata

import (
	"time"
)

// Manager wires together the store, token manager, and signer.
type Manager struct {
	Store  *Store
	Tokens *TokenManager
	Signer *Signer
	// Emit is an optional hook for emitting audit events.
	Emit EventEmitter
}

// NewManager creates a metadata Manager.
func NewManager(st *Store, signer *Signer) *Manager {
	return &Manager{Store: st, Tokens: NewTokenManager(), Signer: signer}
}

func (m *Manager) emit(rtype, id, action, project string, meta map[string]any) {
	if m.Emit != nil {
		m.Emit(rtype, id, action, project, meta)
	}
}

// CreateRecord inserts an instance metadata record and issues a launch token.
// Returns the token that should be injected into the instance environment.
func (m *Manager) CreateRecord(meta InstanceMetadata) (token string, err error) {
	token, err = m.Tokens.Issue(meta.InstanceID, 24*time.Hour)
	if err != nil {
		return "", err
	}
	meta.TokenHash = TokenHash(token)
	if meta.CreatedAt == "" {
		meta.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if storeErr := m.Store.Upsert(meta); storeErr != nil {
		return "", storeErr
	}
	m.emit("metadata", meta.InstanceID, "metadata.record.created", meta.Project,
		map[string]any{"instanceId": meta.InstanceID, "hostname": meta.Hostname})
	m.emit("metadata", meta.InstanceID, "metadata.token.created", meta.Project,
		map[string]any{"instanceId": meta.InstanceID})
	return token, nil
}

// DeleteRecord removes the metadata record for an instance.
func (m *Manager) DeleteRecord(instanceID string) error {
	return m.Store.Delete(instanceID)
}

// GetRecord returns the metadata for an instance.
func (m *Manager) GetRecord(instanceID string) (InstanceMetadata, error) {
	return m.Store.Get(instanceID)
}

// LookupByIP resolves instance metadata by the instance's network IP.
func (m *Manager) LookupByIP(ip string) (InstanceMetadata, bool) {
	meta, err := m.Store.GetByIP(ip)
	return meta, err == nil
}
