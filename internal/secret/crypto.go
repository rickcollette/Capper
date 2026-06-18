package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// passphraseEnv, when set, switches master-key custody from a plaintext key file
// to a passphrase-derived key (see LoadOrCreateKey). The passphrase is supplied
// at runtime (e.g. a systemd credential), so the key root of trust is no longer a
// plaintext file on the same disk as the data.
const passphraseEnv = "CAPPER_MASTER_PASSPHRASE"

// pbkdf2Iterations is the PBKDF2-HMAC-SHA256 work factor for passphrase derivation.
const pbkdf2Iterations = 600_000

// LoadOrCreateKey returns the 32-byte master key for keyPath.
//
// Default custody: read (or create) a random key file at keyPath, 0600.
//
// Passphrase custody: if CAPPER_MASTER_PASSPHRASE is set, derive the key from
// that passphrase and a per-key salt stored at "<keyPath>.salt" (the salt is not
// secret). No plaintext key is written to disk. Each keyPath gets a distinct
// salt, so reusing one passphrase still yields independent keys.
//
// To avoid silently using the wrong key, passphrase mode refuses to start if a
// pre-existing plaintext key file is present without a salt (i.e. a deployment
// that previously used file custody) — migrate those secrets deliberately.
func LoadOrCreateKey(keyPath string) ([]byte, error) {
	if pass := os.Getenv(passphraseEnv); pass != "" {
		return loadPassphraseKey(keyPath, pass)
	}
	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) == 32 {
		return data, nil
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("secret: generate key: %w", err)
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return nil, fmt.Errorf("secret: write key: %w", err)
	}
	return key, nil
}

func loadPassphraseKey(keyPath, passphrase string) ([]byte, error) {
	saltPath := keyPath + ".salt"
	salt, err := os.ReadFile(saltPath)
	switch {
	case err == nil && len(salt) >= 16:
		// established passphrase custody — derive and return
	case os.IsNotExist(err):
		// First run under passphrase custody. Guard against clobbering an
		// existing file-custody deployment.
		if d, derr := os.ReadFile(keyPath); derr == nil && len(d) == 32 {
			return nil, fmt.Errorf("secret: %s is set but a file-based key already exists at %s; "+
				"migrate existing secrets before enabling passphrase custody", passphraseEnv, keyPath)
		}
		salt = make([]byte, 16)
		if _, rerr := io.ReadFull(rand.Reader, salt); rerr != nil {
			return nil, fmt.Errorf("secret: generate salt: %w", rerr)
		}
		if werr := os.WriteFile(saltPath, salt, 0o600); werr != nil {
			return nil, fmt.Errorf("secret: write salt: %w", werr)
		}
	default:
		return nil, fmt.Errorf("secret: read salt %s: %w", saltPath, err)
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, salt, pbkdf2Iterations, 32)
	if err != nil {
		return nil, fmt.Errorf("secret: derive key: %w", err)
	}
	return key, nil
}

// Encrypt encrypts plaintext with AES-256-GCM. The returned bytes are
// nonce (12 bytes) || ciphertext.
func Encrypt(key []byte, plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("secret: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt decrypts a value produced by Encrypt.
func Decrypt(key, ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("secret: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secret: gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return "", fmt.Errorf("secret: ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("secret: decrypt: %w", err)
	}
	return string(plaintext), nil
}
