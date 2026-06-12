package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

type TokenRecord struct {
	ID     string
	Name   string
	Hash   string
	Scopes []string
}

func NewToken(name string, scopes []string) (string, TokenRecord, error) {
	id, err := randomID("tok")
	if err != nil {
		return "", TokenRecord{}, err
	}
	secret, err := randomID("ptr")
	if err != nil {
		return "", TokenRecord{}, err
	}

	record := TokenRecord{
		ID:     id,
		Name:   name,
		Hash:   HashToken(secret),
		Scopes: append([]string(nil), scopes...),
	}
	return secret, record, nil
}

func VerifyToken(plaintext, hash string) bool {
	expected := HashToken(plaintext)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(hash)) == 1
}

func HasScope(granted []string, required string) bool {
	for _, scope := range granted {
		if scope == required {
			return true
		}
	}
	return false
}

func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func randomID(prefix string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
