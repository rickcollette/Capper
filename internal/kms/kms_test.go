package kms

import (
	"bytes"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	master := bytes.Repeat([]byte("k"), 32)
	return NewManager(NewStore(openTestDB(t)), master)
}

// TestKMSEncryptDecryptRoundtrip verifies Encrypt/Decrypt produce the original plaintext.
func TestKMSEncryptDecryptRoundtrip(t *testing.T) {
	mgr := newTestManager(t)
	if _, err := mgr.Create("testkey", "proj"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	plaintext := []byte("super-secret-payload-1234")
	ct, err := mgr.Encrypt("testkey", "proj", plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(ct, plaintext) {
		t.Fatal("ciphertext must differ from plaintext")
	}

	pt, err := mgr.Decrypt("testkey", "proj", ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Errorf("roundtrip mismatch: got %q want %q", pt, plaintext)
	}
}

// TestKMSEncryptDeterministicNonce verifies that two encryptions of the same
// plaintext produce different ciphertexts (random nonce).
func TestKMSEncryptDeterministicNonce(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Create("k2", "p")

	plain := []byte("hello")
	ct1, _ := mgr.Encrypt("k2", "p", plain)
	ct2, _ := mgr.Encrypt("k2", "p", plain)

	if bytes.Equal(ct1, ct2) {
		t.Error("two encryptions of same plaintext should produce different ciphertexts (random nonce)")
	}
}

// TestKMSEncryptKeyNotFound verifies Encrypt returns an error for unknown keys.
func TestKMSEncryptKeyNotFound(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.Encrypt("nonexistent", "proj", []byte("data"))
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

// TestKMSDecryptTampered verifies that a tampered ciphertext fails authentication.
func TestKMSDecryptTampered(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Create("k3", "p")

	ct, _ := mgr.Encrypt("k3", "p", []byte("sensitive"))
	ct[len(ct)-1] ^= 0xFF // flip last byte

	_, err := mgr.Decrypt("k3", "p", ct)
	if err == nil {
		t.Error("expected authentication failure for tampered ciphertext")
	}
}

// TestKMSRotate verifies that a rotated key can still encrypt/decrypt new data.
// Rotation generates a new data key, so pre-rotation ciphertext is intentionally
// incompatible — only post-rotation roundtrip is verified here.
func TestKMSRotate(t *testing.T) {
	mgr := newTestManager(t)
	mgr.Create("rotkey", "p")

	if _, err := mgr.Rotate("rotkey", "p"); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	plain := []byte("post-rotation-payload")
	ct, err := mgr.Encrypt("rotkey", "p", plain)
	if err != nil {
		t.Fatalf("Encrypt after Rotate: %v", err)
	}
	pt, err := mgr.Decrypt("rotkey", "p", ct)
	if err != nil {
		t.Fatalf("Decrypt after Rotate: %v", err)
	}
	if !bytes.Equal(pt, plain) {
		t.Errorf("got %q want %q", pt, plain)
	}
}

// TestKMSList verifies created keys appear in List.
func TestKMSList(t *testing.T) {
	mgr := newTestManager(t)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if _, err := mgr.Create(name, "proj"); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}
	keys, err := mgr.List("proj")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}
