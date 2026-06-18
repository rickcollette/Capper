package secret

import (
	"fmt"
	"time"
)

// Manager provides high-level secret operations. The key is the AES-256 master
// key used to encrypt and decrypt secret values.
type Manager struct {
	store *Store
	key   []byte
}

func NewManager(s *Store, key []byte) *Manager {
	return &Manager{store: s, key: key}
}

// Create encrypts plaintext and stores it as a new secret.
// If a secret with the same name already exists in the project, it is updated.
func (m *Manager) Create(name, project, description, plaintext string) (Secret, error) {
	if name == "" {
		return Secret{}, fmt.Errorf("secret: name is required")
	}
	ct, err := Encrypt(m.key, plaintext)
	if err != nil {
		return Secret{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sec := Secret{
		ID:          newID(),
		Name:        name,
		Project:     project,
		Description: description,
		Ciphertext:  ct,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := m.store.Upsert(sec); err != nil {
		return Secret{}, fmt.Errorf("secret: store: %w", err)
	}
	return sec, nil
}

// Get returns a secret by name or ID. The ciphertext is not decrypted; call
// Decrypt to obtain the plaintext value.
func (m *Manager) Get(nameOrID, project string) (Secret, error) {
	return m.store.Get(nameOrID, project)
}

// Decrypt returns the plaintext value of a secret.
func (m *Manager) Decrypt(sec Secret) (string, error) {
	return Decrypt(m.key, sec.Ciphertext)
}

// GetDecrypted returns the plaintext value directly.
func (m *Manager) GetDecrypted(nameOrID, project string) (string, error) {
	sec, err := m.store.Get(nameOrID, project)
	if err != nil {
		return "", err
	}
	return Decrypt(m.key, sec.Ciphertext)
}

// List returns all secrets for the project (ciphertext is included but
// callers should not expose the value — use Decrypt only when authorised).
func (m *Manager) List(project string) ([]Secret, error) {
	return m.store.List(project)
}

// Delete removes a secret by name or ID.
func (m *Manager) Delete(nameOrID, project string) error {
	return m.store.Delete(nameOrID, project)
}
