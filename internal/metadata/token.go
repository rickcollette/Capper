package metadata

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// tokenRecord holds an in-memory token record.
type tokenRecord struct {
	token      string
	instanceID string
	expiresAt  time.Time
}

// TokenManager issues and validates short-lived instance-bound tokens.
type TokenManager struct {
	mu     sync.Mutex
	tokens map[string]tokenRecord // key: SHA-256 hash of token
}

func NewTokenManager() *TokenManager {
	tm := &TokenManager{tokens: make(map[string]tokenRecord)}
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for range t.C {
			now := time.Now()
			tm.mu.Lock()
			for k, rec := range tm.tokens {
				if now.After(rec.expiresAt) {
					delete(tm.tokens, k)
				}
			}
			tm.mu.Unlock()
		}
	}()
	return tm
}

// Issue generates a random token for the given instance with a TTL.
func (tm *TokenManager) Issue(instanceID string, ttl time.Duration) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: rand: %w", err)
	}
	token := hex.EncodeToString(b)
	hash := tokenHash(token)
	tm.mu.Lock()
	tm.tokens[hash] = tokenRecord{token: token, instanceID: instanceID, expiresAt: time.Now().Add(ttl)}
	tm.mu.Unlock()
	return token, nil
}

// Validate checks that the token is valid and belongs to the expected instance.
// Returns the instanceID if valid.
func (tm *TokenManager) Validate(token, instanceID string) bool {
	hash := tokenHash(token)
	tm.mu.Lock()
	rec, ok := tm.tokens[hash]
	tm.mu.Unlock()
	if !ok {
		return false
	}
	if time.Now().After(rec.expiresAt) {
		tm.mu.Lock()
		delete(tm.tokens, hash)
		tm.mu.Unlock()
		return false
	}
	return rec.instanceID == instanceID
}

// TokenHash returns the SHA-256 hex of a token, for storing in the metadata record.
func TokenHash(token string) string { return tokenHash(token) }

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
