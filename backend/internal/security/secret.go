package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var ErrInvalidCiphertext = errors.New("invalid encrypted secret")

// EncryptSecret uses AES-256-GCM. The master key is hashed so operators may
// provide a regular environment secret instead of a raw 32-byte key.
func EncryptSecret(masterKey, plaintext string) (string, error) {
	if masterKey == "" {
		return "", errors.New("credential encryption key is empty")
	}
	if plaintext == "" {
		return "", errors.New("secret is empty")
	}
	block, err := newBlock(masterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create secret cipher: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	payload := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawStdEncoding.EncodeToString(payload), nil
}

func DecryptSecret(masterKey, encoded string) (string, error) {
	if masterKey == "" {
		return "", errors.New("credential encryption key is empty")
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	block, err := newBlock(masterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create secret cipher: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", ErrInvalidCiphertext
	}
	plaintext, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plaintext), nil
}

func KeyLast4(value string) string {
	if len(value) <= 4 {
		return value
	}
	return value[len(value)-4:]
}

func newBlock(masterKey string) (cipher.Block, error) {
	key := sha256.Sum256([]byte(masterKey))
	return aes.NewCipher(key[:])
}
