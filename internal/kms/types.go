package kms

// KeyStatus represents whether a key is active or has been rotated out.
type KeyStatus string

const (
	KeyStatusActive  KeyStatus = "active"
	KeyStatusRotated KeyStatus = "rotated"
)

// Key is a named symmetric data key stored encrypted under the master key.
// The EncryptedKey field is the raw data key wrapped with AES-256-GCM using
// the store master key (same scheme as secret/crypto.go).
type Key struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Project      string    `json:"project"`
	Status       KeyStatus `json:"status"`
	EncryptedKey []byte    `json:"encryptedKey"`
	CreatedAt    string    `json:"createdAt"`
	RotatedAt    string    `json:"rotatedAt,omitempty"`
	RotatedFrom  string    `json:"rotatedFrom,omitempty"` // predecessor key ID
}
