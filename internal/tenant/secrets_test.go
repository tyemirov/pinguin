package tenant

import (
	"strings"
	"testing"
)

func TestSecretKeeperEncryptDecrypt(t *testing.T) {
	t.Helper()
	keeper := newTestSecretKeeper(t)
	plaintext := "super-secret-value"

	ciphertext, err := keeper.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Fatalf("expected ciphertext bytes")
	}

	recovered, err := keeper.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if recovered != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, recovered)
	}
}

func TestSecretKeeperRejectsInvalidKey(t *testing.T) {
	t.Helper()
	_, err := NewSecretKeeper("short")
	if err == nil {
		t.Fatalf("expected error for invalid key size")
	}
}

func newTestSecretKeeper(t *testing.T) *SecretKeeper {
	t.Helper()
	rawKey := strings.Repeat("a", 64)
	keeper, err := NewSecretKeeper(rawKey)
	if err != nil {
		t.Fatalf("secret keeper init error: %v", err)
	}
	return keeper
}
