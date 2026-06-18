package iam

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"capper/internal/secret"
)

// tokenPayload is the JSON body inside a signed bearer token.
type tokenPayload struct {
	ID            string `json:"id"`
	PrincipalType string `json:"pt"`
	PrincipalID   string `json:"pi"`
	ExpiresAt     string `json:"exp"`
}

// loadOrCreateSigningKey loads the 32-byte token-signing key from
// storeRoot/iam.key. Custody (file or passphrase-derived) is shared with the
// secret/KMS master keys via secret.LoadOrCreateKey, so CAPPER_MASTER_PASSPHRASE
// protects the token-signing root of trust too.
func loadOrCreateSigningKey(storeRoot string) ([]byte, error) {
	key, err := secret.LoadOrCreateKey(filepath.Join(storeRoot, "iam.key"))
	if err != nil {
		return nil, fmt.Errorf("iam: signing key: %w", err)
	}
	return key, nil
}

// Issue creates a new Token record, signs it, and returns the bearer string.
// Format: v1.<base64url(payload)>.<base64url(hmac-sha256)>
func (m *Manager) Issue(name, principalType, principalID string, ttl time.Duration) (string, Token, error) {
	id := newID("tok")
	expiresAt := time.Now().Add(ttl).UTC().Format(time.RFC3339)

	tok := Token{
		ID:            id,
		Name:          name,
		PrincipalType: principalType,
		PrincipalID:   principalID,
		ExpiresAt:     expiresAt,
	}
	if err := m.store.InsertToken(tok); err != nil {
		return "", Token{}, err
	}

	payload := tokenPayload{
		ID:            id,
		PrincipalType: principalType,
		PrincipalID:   principalID,
		ExpiresAt:     expiresAt,
	}
	bearer, err := signPayload(payload, m.sigKey)
	if err != nil {
		return "", Token{}, err
	}
	return bearer, tok, nil
}

// Verify parses and validates a bearer token. Returns principalType and principalID
// on success, or an error if the signature is invalid or the token has expired.
func (m *Manager) Verify(bearer string) (principalType, principalID string, err error) {
	payload, err := verifyPayload(bearer, m.sigKey)
	if err != nil {
		return "", "", err
	}

	// Check expiry.
	exp, err := time.Parse(time.RFC3339, payload.ExpiresAt)
	if err != nil {
		return "", "", fmt.Errorf("iam: token: invalid expiry: %w", err)
	}
	if time.Now().After(exp) {
		return "", "", fmt.Errorf("iam: token %q has expired", payload.ID)
	}

	// Confirm token still exists in the store (it may have been revoked).
	if !m.tokenExists(payload.ID) {
		return "", "", fmt.Errorf("iam: token %q not found (revoked?)", payload.ID)
	}
	return payload.PrincipalType, payload.PrincipalID, nil
}

func signPayload(p tokenPayload, key []byte) (string, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encodedPayload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return "v1." + encodedPayload + "." + sig, nil
}

func verifyPayload(bearer string, key []byte) (tokenPayload, error) {
	// Strip "v1." prefix and split on "."
	if len(bearer) < 3 || bearer[:3] != "v1." {
		return tokenPayload{}, fmt.Errorf("iam: token: invalid format")
	}
	rest := bearer[3:]
	// Find last dot separating payload from signature.
	lastDot := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == '.' {
			lastDot = i
			break
		}
	}
	if lastDot < 0 {
		return tokenPayload{}, fmt.Errorf("iam: token: missing signature")
	}
	encodedPayload := rest[:lastDot]
	encodedSig := rest[lastDot+1:]

	// Verify HMAC.
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encodedPayload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(encodedSig), []byte(expectedSig)) {
		return tokenPayload{}, fmt.Errorf("iam: token: signature mismatch")
	}

	raw, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return tokenPayload{}, fmt.Errorf("iam: token: decode payload: %w", err)
	}
	var p tokenPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return tokenPayload{}, fmt.Errorf("iam: token: parse payload: %w", err)
	}
	return p, nil
}
