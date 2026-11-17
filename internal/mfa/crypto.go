package mfa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const KeyBytes = 32

// Encrypt protects the TOTP secret at rest with AES-256-GCM. The key lives in
// a separate 0600 key file or APP_MFA_KEY, not in the SQLite database.
func Encrypt(key, plaintext []byte) (string, error) {
	if len(key) != KeyBytes {
		return "", fmt.Errorf("MFA key must be %d bytes", KeyBytes)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	out := append(nonce, sealed...)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

func Decrypt(key []byte, encoded string) ([]byte, error) {
	if len(key) != KeyBytes {
		return nil, fmt.Errorf("MFA key must be %d bytes", KeyBytes)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("encrypted MFA secret is truncated")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
