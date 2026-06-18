package marketplace

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
)

// VerifySignature reads attestations/signature.json from the artifact tar,
// verifies the embedded ECDSA signature, and returns the signer's public key
// fingerprint as "sha256:<hex>". Returns an error if the artifact is unsigned
// or the signature is invalid.
func VerifySignature(artifactPath string) (signerID string, err error) {
	sigJSON, err := readTarEntry(artifactPath, "attestations/signature.json", 1<<20)
	if err != nil {
		return "", fmt.Errorf("no signature found in artifact")
	}

	var sig struct {
		Payload   string `json:"payload"`
		Signature string `json:"signature"`
		PublicKey string `json:"publicKey"`
	}
	if err := json.Unmarshal(sigJSON, &sig); err != nil {
		return "", fmt.Errorf("signature.json malformed: %w", err)
	}

	payload, err := base64.StdEncoding.DecodeString(sig.Payload)
	if err != nil {
		return "", fmt.Errorf("signature payload: invalid base64: %w", err)
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sig.Signature)
	if err != nil {
		return "", fmt.Errorf("signature bytes: invalid base64: %w", err)
	}

	block, _ := pem.Decode([]byte(sig.PublicKey))
	if block == nil {
		return "", fmt.Errorf("publicKey: no PEM block found")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("publicKey: parse failed: %w", err)
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("publicKey: expected ECDSA, got %T", pub)
	}

	// sigBytes is an ASN.1 DER-encoded (r,s) pair.
	if !verifyECDSASig(ecPub, payload, sigBytes) {
		return "", fmt.Errorf("signature verification failed")
	}

	// Fingerprint = SHA-256 of the DER-encoded public key.
	der, err := x509.MarshalPKIXPublicKey(ecPub)
	if err != nil {
		return "", fmt.Errorf("publicKey: marshal: %w", err)
	}
	sum := sha256.Sum256(der)
	return fmt.Sprintf("sha256:%x", sum), nil
}

func verifyECDSASig(pub *ecdsa.PublicKey, payload, sigBytes []byte) bool {
	digest := sha256.Sum256(payload)
	// sigBytes is ASN.1 DER; parse r and s manually via standard library.
	r, s, err := parseECDSASig(sigBytes)
	if err != nil {
		return false
	}
	return ecdsa.Verify(pub, digest[:], r, s)
}

// parseECDSASig decodes an ASN.1 DER SEQUENCE { INTEGER r, INTEGER s }.
func parseECDSASig(der []byte) (r, s *big.Int, err error) {
	if len(der) < 6 || der[0] != 0x30 {
		return nil, nil, fmt.Errorf("not a DER SEQUENCE")
	}
	// Skip SEQUENCE tag+length.
	off := 2
	if der[1]&0x80 != 0 {
		lenBytes := int(der[1] & 0x7f)
		off += lenBytes
	}
	r, off, err = parseDERInt(der, off)
	if err != nil {
		return nil, nil, err
	}
	s, _, err = parseDERInt(der, off)
	return r, s, err
}

func parseDERInt(der []byte, off int) (*big.Int, int, error) {
	if off >= len(der) || der[off] != 0x02 {
		return nil, off, fmt.Errorf("expected INTEGER tag at offset %d", off)
	}
	off++
	if off >= len(der) {
		return nil, off, fmt.Errorf("truncated DER")
	}
	length := int(der[off])
	off++
	if off+length > len(der) {
		return nil, off, fmt.Errorf("DER INTEGER overflows buffer")
	}
	n := new(big.Int).SetBytes(der[off : off+length])
	return n, off + length, nil
}
