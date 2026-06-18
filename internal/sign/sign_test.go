package sign

import (
	"archive/tar"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyPairRoundtrip(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "capper.key")
	pubPath := filepath.Join(dir, "capper.pub")

	if err := GenerateKeyPair(keyPath, pubPath); err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	priv, err := LoadPrivateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	pub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}

	msg := []byte("hello capper")
	sig := ed25519.Sign(priv, msg)
	if !ed25519.Verify(pub, msg, sig) {
		t.Fatal("sign/verify roundtrip failed")
	}
}

func TestSignAndVerifyImage(t *testing.T) {
	dir := t.TempDir()
	capPath := filepath.Join(dir, "test.cap")
	buildMinimalCap(t, capPath)

	keyPath := filepath.Join(dir, "capper.key")
	pubPath := filepath.Join(dir, "capper.pub")
	if err := GenerateKeyPair(keyPath, pubPath); err != nil {
		t.Fatal(err)
	}

	priv, err := LoadPrivateKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatal(err)
	}

	// Sign in-place.
	if err := SignImage(capPath, capPath, priv); err != nil {
		t.Fatalf("SignImage: %v", err)
	}

	// Verify with correct key.
	if err := VerifyImage(capPath, pub); err != nil {
		t.Fatalf("VerifyImage: %v", err)
	}
}

func TestVerifyDetectsWrongKey(t *testing.T) {
	dir := t.TempDir()
	capPath := filepath.Join(dir, "test.cap")
	buildMinimalCap(t, capPath)

	keyPath := filepath.Join(dir, "capper.key")
	pubPath := filepath.Join(dir, "capper.pub")
	if err := GenerateKeyPair(keyPath, pubPath); err != nil {
		t.Fatal(err)
	}
	priv, _ := LoadPrivateKey(keyPath)

	if err := SignImage(capPath, capPath, priv); err != nil {
		t.Fatal(err)
	}

	// Generate a different key pair and verify with the wrong public key.
	wrongKey, wrongPub := filepath.Join(dir, "wrong.key"), filepath.Join(dir, "wrong.pub")
	if err := GenerateKeyPair(wrongKey, wrongPub); err != nil {
		t.Fatal(err)
	}
	wrongPubKey, _ := LoadPublicKey(wrongPub)
	if err := VerifyImage(capPath, wrongPubKey); err == nil {
		t.Fatal("expected verification failure with wrong key")
	}
}

func TestVerifyUnsignedImageFails(t *testing.T) {
	dir := t.TempDir()
	capPath := filepath.Join(dir, "test.cap")
	buildMinimalCap(t, capPath)

	pub, _, _ := ed25519.GenerateKey(nil)
	if err := VerifyImage(capPath, pub); err == nil {
		t.Fatal("expected error verifying unsigned image")
	}
}

// buildMinimalCap creates a minimal .cap archive with only capsule.json and checksums.json.
func buildMinimalCap(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tw := tar.NewWriter(f)
	writeEntry := func(name, content string) {
		data := []byte(content)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	writeEntry("capsule.json", `{"capsuleVersion":"0.1","name":"test","version":"0.1.0"}`)
	writeEntry("checksums.json", `{"algorithm":"sha256","files":{"capsule.json":"sha256:abc"}}`)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
}
