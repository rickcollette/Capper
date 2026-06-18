package storage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"capper/internal/storage"
)

// mockKMS is a simple XOR "encryption" for testing — not secure, but deterministic.
type mockKMS struct {
	key byte
}

func (m *mockKMS) Encrypt(keyName, project string, plaintext []byte) ([]byte, error) {
	out := make([]byte, len(plaintext))
	for i, b := range plaintext {
		out[i] = b ^ m.key
	}
	return out, nil
}

func (m *mockKMS) Decrypt(keyName, project string, ciphertext []byte) ([]byte, error) {
	return m.Encrypt(keyName, project, ciphertext) // XOR is self-inverse
}

// TestPutObjectWithKMSEncryption verifies that when a bucket has KMSKeyName set,
// PutObject encrypts the stored bytes (GetObject returns ciphertext ≠ plaintext).
func TestPutObjectWithKMSEncryption(t *testing.T) {
	mgr, _ := openManager(t)
	kmsInst := &mockKMS{key: 0xAB}
	mgr.SetKMS(kmsInst)

	// Create a bucket with a KMS key name.
	bucket, err := mgr.CreateBucket(storage.CreateBucketOptions{
		Name:       "encrypted-bucket",
		KMSKeyName: "mykey",
	})
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	plaintext := []byte("this is a secret object payload")
	srcFile := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(srcFile, plaintext, 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if _, err := mgr.PutObject(bucket.Name, "secret.txt", srcFile); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	// GetObject reads from disk — returns the raw (encrypted) bytes.
	outFile := filepath.Join(t.TempDir(), "out.txt")
	if err := mgr.GetObject(bucket.Name, "secret.txt", outFile); err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	stored, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read stored: %v", err)
	}

	// Stored bytes should NOT equal the plaintext.
	if bytes.Equal(stored, plaintext) {
		t.Error("stored bytes equal plaintext — KMS encryption was not applied")
	}

	// Verify the stored bytes decrypt back to the original.
	decrypted, _ := kmsInst.Decrypt("mykey", "", stored)
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypt roundtrip failed: got %q want %q", decrypted, plaintext)
	}
}

// TestPutObjectWithoutKMSStoresPlaintext verifies that a bucket without KMSKeyName
// stores object bytes unchanged.
func TestPutObjectWithoutKMSStoresPlaintext(t *testing.T) {
	mgr, _ := openManager(t)
	// Attach a KMS but don't set KMSKeyName on the bucket — should be a no-op.
	mgr.SetKMS(&mockKMS{key: 0xFF})

	bucket, err := mgr.CreateBucket(storage.CreateBucketOptions{Name: "plain-bucket"})
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	plaintext := []byte("unencrypted payload")
	srcFile := filepath.Join(t.TempDir(), "plain.txt")
	os.WriteFile(srcFile, plaintext, 0o600)

	if _, err := mgr.PutObject(bucket.Name, "plain.txt", srcFile); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "out.txt")
	mgr.GetObject(bucket.Name, "plain.txt", outFile)
	stored, _ := os.ReadFile(outFile)
	if !bytes.Equal(stored, plaintext) {
		t.Error("plain bucket should store unencrypted bytes")
	}
}

// TestPutObjectNoKMSManager verifies that a bucket with KMSKeyName but no KMS
// manager attached still writes the object (unencrypted fallback).
func TestPutObjectNoKMSManager(t *testing.T) {
	mgr, _ := openManager(t)
	// No SetKMS call.

	bucket, err := mgr.CreateBucket(storage.CreateBucketOptions{
		Name:       "kms-missing-bucket",
		KMSKeyName: "somekey",
	})
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	plaintext := []byte("fallback payload")
	srcFile := filepath.Join(t.TempDir(), "f.txt")
	os.WriteFile(srcFile, plaintext, 0o600)

	if _, err := mgr.PutObject(bucket.Name, "f.txt", srcFile); err != nil {
		t.Fatalf("PutObject without KMS manager: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "out.txt")
	mgr.GetObject(bucket.Name, "f.txt", outFile)
	stored, _ := os.ReadFile(outFile)
	if !bytes.Equal(stored, plaintext) {
		t.Error("without KMS manager, bytes should be stored as-is")
	}
}
