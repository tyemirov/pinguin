package tenant

import (
	"errors"
	"io"
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
	_, err = NewSecretKeeper("not-hex")
	if err == nil {
		t.Fatalf("expected error for invalid hex")
	}
	_, err = NewSecretKeeper(strings.Repeat("a", 62))
	if err == nil {
		t.Fatalf("expected error for decoded key length")
	}
}

func TestSecretKeeperReportsCipherErrors(t *testing.T) {
	t.Helper()
	invalidKeeper := &SecretKeeper{key: []byte("short")}
	if _, err := invalidKeeper.Encrypt("payload"); err == nil {
		t.Fatalf("expected encrypt cipher error")
	}
	if _, err := invalidKeeper.Decrypt([]byte("payload long enough")); err == nil {
		t.Fatalf("expected decrypt cipher error")
	}

	keeper := newTestSecretKeeper(t)
	if _, err := keeper.Decrypt([]byte("short")); err == nil {
		t.Fatalf("expected short ciphertext error")
	}
	ciphertext, err := keeper.Encrypt("payload")
	if err != nil {
		t.Fatalf("encrypt payload: %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 0xff
	if _, err := keeper.Decrypt(ciphertext); err == nil {
		t.Fatalf("expected tampered ciphertext error")
	}
}

func TestSecretKeeperReportsNonceSourceFailure(t *testing.T) {
	t.Helper()
	keeper := newTestSecretKeeper(t)
	keeper.random = failingReader{err: io.ErrUnexpectedEOF}
	if _, err := keeper.Encrypt("payload"); err == nil || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected nonce read failure, got %v", err)
	}
}

type failingReader struct {
	err error
}

func (reader failingReader) Read([]byte) (int, error) {
	return 0, reader.err
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
