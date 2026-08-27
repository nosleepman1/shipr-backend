package crypto

import (
	"testing"
)

func TestAES256GCMEncryptionDecryption(t *testing.T) {
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encryptor, err := NewEncryptor(masterKey)
	if err != nil {
		t.Fatalf("Failed to initialize encryptor: %v", err)
	}

	secret := "DATABASE_PASSWORD=postgres_super_secret_123$!@"

	encrypted, err := encryptor.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if encrypted == secret {
		t.Fatalf("Encrypted text matches plaintext")
	}

	decrypted, err := encryptor.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if decrypted != secret {
		t.Fatalf("Expected '%s', got '%s'", secret, decrypted)
	}
}
