package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext or nonce")
	ErrDecryptionFailed  = errors.New("failed to decrypt ciphertext")
)

type Encryptor struct {
	key []byte
}

func NewEncryptor(masterKeyHex string) (*Encryptor, error) {
	var key []byte
	// If the key is a 64-character hex string (32 bytes), decode it directly
	if len(masterKeyHex) == 64 {
		decoded, err := hex.DecodeString(masterKeyHex)
		if err == nil {
			key = decoded
		}
	}

	// Fallback to SHA-256 hash to ensure exactly 32 bytes for AES-256
	if len(key) != 32 {
		hash := sha256.Sum256([]byte(masterKeyHex))
		key = hash[:]
	}

	return &Encryptor{key: key}, nil
}

// Encrypt encrypts plain text using AES-256-GCM and returns a hex string (nonce + ciphertext)
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(sealed), nil
}

// Decrypt decrypts a hex-encoded string containing (nonce + ciphertext) using AES-256-GCM
func (e *Encryptor) Decrypt(ciphertextHex string) (string, error) {
	data, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}
