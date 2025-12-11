package tenant

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// SecretKeeper encrypts and decrypts sensitive strings at rest.
type SecretKeeper struct {
	key []byte
}

// NewSecretKeeper builds a keeper from a raw key. The key must be 32 bytes.
func NewSecretKeeper(rawKey string) (*SecretKeeper, error) {
	keyBytes, err := hex.DecodeString(rawKey)
	if err != nil {
		return nil, fmt.Errorf("tenant: invalid encryption key: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("tenant: encryption key must decode to 32 bytes")
	}
	return &SecretKeeper{key: keyBytes}, nil
}

// Encrypt converts plaintext into ciphertext bytes using AES-GCM.
func (keeper *SecretKeeper) Encrypt(plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(keeper.key)
	if err != nil {
		return nil, fmt.Errorf("tenant: init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("tenant: init gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("tenant: nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}

// Decrypt reverses Encrypt.
func (keeper *SecretKeeper) Decrypt(ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(keeper.key)
	if err != nil {
		return "", fmt.Errorf("tenant: init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("tenant: init gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("tenant: ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	payload := ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("tenant: decrypt: %w", err)
	}
	return string(plaintext), nil
}
