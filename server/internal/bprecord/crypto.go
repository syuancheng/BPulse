package bprecord

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

type Keyring struct {
	version string
	key     []byte
}

func NewKeyringFromBase64(version, encoded string) (Keyring, error) {
	if version == "" {
		return Keyring{}, fmt.Errorf("key version is required")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Keyring{}, fmt.Errorf("decode data encryption key: %w", err)
	}
	if len(key) != 32 {
		return Keyring{}, fmt.Errorf("data encryption key must be 32 bytes")
	}
	return Keyring{version: version, key: key}, nil
}

func (k Keyring) Encrypt(plaintext, aad []byte) (string, []byte, []byte, error) {
	block, err := aes.NewCipher(k.key)
	if err != nil {
		return "", nil, nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", nil, nil, fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	return k.version, nonce, gcm.Seal(nil, nonce, plaintext, aad), nil
}

func (k Keyring) Decrypt(keyVersion string, nonce, ciphertext, aad []byte) ([]byte, error) {
	if keyVersion != k.version {
		return nil, ErrDecryptFailed
	}
	block, err := aes.NewCipher(k.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	return plaintext, nil
}
