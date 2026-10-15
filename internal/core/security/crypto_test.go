package security

import (
	"path/filepath"
	"testing"
)

func TestCipherService_EncryptDecrypt(t *testing.T) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "master.key")

	cipher, err := NewCipherService(keyFile)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	secret := "MySuperSecretPassword@2026!#$%"
	encrypted, err := cipher.Encrypt(secret)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if encrypted == secret {
		t.Fatalf("encrypted text shouldn't match plaintext")
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != secret {
		t.Fatalf("expected %s, got %s", secret, decrypted)
	}
}

func TestCipherService_Empty(t *testing.T) {
	cipher, err := NewCipherService("")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	enc, err := cipher.Encrypt("")
	if err != nil || enc != "" {
		t.Fatalf("expected empty encrypted for empty input, got %s, err: %v", enc, err)
	}

	dec, err := cipher.Decrypt("")
	if err != nil || dec != "" {
		t.Fatalf("expected empty decrypted for empty input, got %s, err: %v", dec, err)
	}
}
