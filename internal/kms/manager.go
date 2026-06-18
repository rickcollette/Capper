package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"time"
)

// Manager provides high-level KMS operations. masterKey is the AES-256
// master key (32 bytes) used to wrap/unwrap data keys.
type Manager struct {
	store     *Store
	masterKey []byte
}

func NewManager(s *Store, masterKey []byte) *Manager {
	return &Manager{store: s, masterKey: masterKey}
}

// Create generates a new 32-byte data key, wraps it under the master key,
// and stores it with the given name. Returns an error if an active key with
// that name already exists.
func (m *Manager) Create(name, project string) (Key, error) {
	if name == "" {
		return Key{}, fmt.Errorf("kms: name is required")
	}
	dataKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dataKey); err != nil {
		return Key{}, fmt.Errorf("kms: generate data key: %w", err)
	}
	wrapped, err := wrap(m.masterKey, dataKey)
	if err != nil {
		return Key{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	k := Key{
		ID:           newID(),
		Name:         name,
		Project:      project,
		Status:       KeyStatusActive,
		EncryptedKey: wrapped,
		CreatedAt:    now,
	}
	if err := m.store.Insert(k); err != nil {
		return Key{}, fmt.Errorf("kms: store: %w", err)
	}
	return k, nil
}

// GetDataKey retrieves and unwraps the data key for the named key.
func (m *Manager) GetDataKey(nameOrID, project string) ([]byte, error) {
	k, err := m.store.Get(nameOrID, project)
	if err != nil {
		return nil, err
	}
	return unwrap(m.masterKey, k.EncryptedKey)
}

// List returns all keys for the project (data keys are not exposed).
func (m *Manager) List(project string) ([]Key, error) {
	return m.store.List(project)
}

// Encrypt encrypts plaintext with the named KMS key's data key.
// Returns nonce || ciphertext as a single byte slice.
func (m *Manager) Encrypt(nameOrID, project string, plaintext []byte) ([]byte, error) {
	dataKey, err := m.GetDataKey(nameOrID, project)
	if err != nil {
		return nil, fmt.Errorf("kms: encrypt: %w", err)
	}
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, fmt.Errorf("kms: encrypt: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("kms: encrypt: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("kms: encrypt: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext (nonce || ciphertext format) with the named key.
func (m *Manager) Decrypt(nameOrID, project string, ciphertext []byte) ([]byte, error) {
	dataKey, err := m.GetDataKey(nameOrID, project)
	if err != nil {
		return nil, fmt.Errorf("kms: decrypt: %w", err)
	}
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, fmt.Errorf("kms: decrypt: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("kms: decrypt: gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("kms: decrypt: ciphertext too short")
	}
	plain, err := gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("kms: decrypt: %w", err)
	}
	return plain, nil
}

// Rotate creates a new data key with the same name, marks the old key as
// rotated, and links the new key's RotatedFrom field to the old key's ID.
// The old key's wrapped data key remains in the store (needed to decrypt
// any data encrypted under it).
func (m *Manager) Rotate(nameOrID, project string) (Key, error) {
	old, err := m.store.Get(nameOrID, project)
	if err != nil {
		return Key{}, err
	}
	if old.Status != KeyStatusActive {
		return Key{}, fmt.Errorf("kms: key %q is already rotated", old.Name)
	}
	dataKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dataKey); err != nil {
		return Key{}, fmt.Errorf("kms: generate data key: %w", err)
	}
	wrapped, err := wrap(m.masterKey, dataKey)
	if err != nil {
		return Key{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	// Mark old key rotated before inserting new one (UNIQUE constraint on name).
	if err := m.store.MarkRotated(old.ID); err != nil {
		return Key{}, fmt.Errorf("kms: mark rotated: %w", err)
	}
	// SQLite UNIQUE(name, project) only enforces on active rows if we use a
	// partial index — for simplicity, we include the rotation timestamp in the
	// stored name for old keys so the constraint isn't violated.
	// Rename the old key row to free the name for the new active key.
	_, dbErr := m.store.db.Exec(
		`UPDATE kms_keys SET name=name||'@'||rotated_at WHERE id=?`, old.ID,
	)
	if dbErr != nil {
		return Key{}, fmt.Errorf("kms: rename rotated key: %w", dbErr)
	}
	newKey := Key{
		ID:           newID(),
		Name:         old.Name,
		Project:      project,
		Status:       KeyStatusActive,
		EncryptedKey: wrapped,
		CreatedAt:    now,
		RotatedFrom:  old.ID,
	}
	if err := m.store.Insert(newKey); err != nil {
		return Key{}, fmt.Errorf("kms: store new key: %w", err)
	}
	return newKey, nil
}

// Delete removes a key and its wrapped key material from the store.
// Any data previously encrypted under this key will be permanently unrecoverable.
func (m *Manager) Delete(nameOrID, project string) error {
	return m.store.Delete(nameOrID, project)
}

// wrap encrypts a data key with AES-256-GCM using the master key.
// Output: nonce (12 bytes) || ciphertext.
func wrap(masterKey, dataKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("kms: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("kms: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("kms: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, dataKey, nil), nil
}

// unwrap decrypts a wrapped data key.
func unwrap(masterKey, wrapped []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("kms: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("kms: gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(wrapped) < ns {
		return nil, fmt.Errorf("kms: wrapped key too short")
	}
	return gcm.Open(nil, wrapped[:ns], wrapped[ns:], nil)
}
