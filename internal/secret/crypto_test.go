package secret_test

import (
	"os"
	"path/filepath"
	"testing"

	"capper/internal/secret"
)

// TestPassphraseCustody verifies CAPPER_MASTER_PASSPHRASE custody: keys are
// derived deterministically from passphrase + per-key salt, distinct key paths
// yield distinct keys under one passphrase, and pre-existing file keys are not
// silently overwritten.
func TestPassphraseCustody(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CAPPER_MASTER_PASSPHRASE", "correct horse battery staple")

	keyA := filepath.Join(dir, "secret.key")
	k1, err := secret.LoadOrCreateKey(keyA)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(k1) != 32 {
		t.Fatalf("key length = %d, want 32", len(k1))
	}
	// No plaintext key file is written under passphrase custody; a salt is.
	if _, err := os.Stat(keyA); !os.IsNotExist(err) {
		t.Errorf("plaintext key file should not exist under passphrase custody")
	}
	if _, err := os.Stat(keyA + ".salt"); err != nil {
		t.Errorf("salt file missing: %v", err)
	}

	// Deterministic across reloads (same salt).
	k2, err := secret.LoadOrCreateKey(keyA)
	if err != nil {
		t.Fatalf("re-derive: %v", err)
	}
	if string(k1) != string(k2) {
		t.Errorf("derivation not deterministic across reloads")
	}

	// Distinct key path → distinct salt → distinct key under one passphrase.
	keyB := filepath.Join(dir, "kms.key")
	kb, err := secret.LoadOrCreateKey(keyB)
	if err != nil {
		t.Fatalf("derive B: %v", err)
	}
	if string(kb) == string(k1) {
		t.Errorf("different key paths produced the same key")
	}

	// Round-trip encrypt/decrypt with a derived key.
	ct, err := secret.Encrypt(k1, "hello")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	pt, err := secret.Decrypt(k1, ct)
	if err != nil || pt != "hello" {
		t.Fatalf("decrypt = %q, %v", pt, err)
	}
}

// TestPassphraseRefusesExistingFileKey ensures enabling passphrase custody on a
// deployment that already has a file-based key fails loudly instead of deriving a
// different key (which would orphan existing ciphertext).
func TestPassphraseRefusesExistingFileKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "secret.key")

	// Establish a file-based key first (no passphrase).
	if _, err := secret.LoadOrCreateKey(keyPath); err != nil {
		t.Fatalf("file key: %v", err)
	}

	t.Setenv("CAPPER_MASTER_PASSPHRASE", "later-added-passphrase")
	if _, err := secret.LoadOrCreateKey(keyPath); err == nil {
		t.Fatal("expected refusal when a file key already exists, got nil error")
	}
}
