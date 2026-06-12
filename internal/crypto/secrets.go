package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const masterKeySize = 32

type SecretBox struct {
	key []byte
}

func GenerateMasterKey() ([]byte, error) {
	key := make([]byte, masterKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	return key, nil
}

func NewSecretBox(masterKey []byte) (*SecretBox, error) {
	if len(masterKey) != masterKeySize {
		return nil, fmt.Errorf("master key must be %d bytes", masterKeySize)
	}
	key := append([]byte(nil), masterKey...)
	return &SecretBox{key: key}, nil
}

func (box *SecretBox) Encrypt(plaintext string) (string, error) {
	gcm, err := box.gcm()
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return base64.RawStdEncoding.EncodeToString(payload), nil
}

func (box *SecretBox) Decrypt(encoded string) (string, error) {
	gcm, err := box.gcm()
	if err != nil {
		return "", err
	}

	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext is too short")
	}

	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

func MaskSecret() string {
	return "••••"
}

func (box *SecretBox) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(box.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return gcm, nil
}
