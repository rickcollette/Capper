package metadata

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Signer signs metadata documents using Ed25519.
type Signer struct {
	privKey ed25519.PrivateKey
	pubKey  ed25519.PublicKey
}

// SignedBundle contains per-document hashes and the overall signature.
type SignedBundle struct {
	Documents map[string]string `json:"documents"` // name → SHA-256 hex
	Signature string            `json:"signature"`  // Ed25519 sig over sorted JSON of Documents
}

// LoadOrCreateSigner loads an Ed25519 key from dir/metadata.key, creating it if absent.
func LoadOrCreateSigner(dir string) (*Signer, error) {
	keyPath := filepath.Join(dir, "metadata.key")
	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) == ed25519.PrivateKeySize {
		priv := ed25519.PrivateKey(data)
		return &Signer{privKey: priv, pubKey: priv.Public().(ed25519.PublicKey)}, nil
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("metadata: generate key: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, []byte(priv), 0o600); err != nil {
		return nil, err
	}
	return &Signer{privKey: priv, pubKey: pub}, nil
}

// Sign produces a SignedBundle for the given named documents.
func (s *Signer) Sign(docs map[string][]byte) (SignedBundle, error) {
	hashes := make(map[string]string, len(docs))
	for name, body := range docs {
		h := sha256.Sum256(body)
		hashes[name] = hex.EncodeToString(h[:])
	}
	payload, err := json.Marshal(hashes)
	if err != nil {
		return SignedBundle{}, err
	}
	sig := ed25519.Sign(s.privKey, payload)
	return SignedBundle{
		Documents: hashes,
		Signature: hex.EncodeToString(sig),
	}, nil
}

// PublicKeyHex returns the public key as a hex string.
func (s *Signer) PublicKeyHex() string { return hex.EncodeToString(s.pubKey) }

// Verify checks a SignedBundle against the signer's public key.
func (s *Signer) Verify(bundle SignedBundle) bool {
	payload, err := json.Marshal(bundle.Documents)
	if err != nil {
		return false
	}
	sigBytes, err := hex.DecodeString(bundle.Signature)
	if err != nil {
		return false
	}
	return ed25519.Verify(s.pubKey, payload, sigBytes)
}
