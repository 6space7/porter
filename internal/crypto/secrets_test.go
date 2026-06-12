package crypto_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	secretcrypto "github.com/6space7/porter/internal/crypto"
)

func TestSecretBoxEncryptsAndDecryptsWithMasterKey(t *testing.T) {
	key, err := secretcrypto.GenerateMasterKey()
	if err != nil {
		t.Fatalf("generate master key: %v", err)
	}

	box, err := secretcrypto.NewSecretBox(key)
	if err != nil {
		t.Fatalf("new secret box: %v", err)
	}

	first, err := box.Encrypt("postgres://porter:secret@db:5432/app")
	if err != nil {
		t.Fatalf("encrypt first value: %v", err)
	}
	second, err := box.Encrypt("postgres://porter:secret@db:5432/app")
	if err != nil {
		t.Fatalf("encrypt second value: %v", err)
	}

	if first == "postgres://porter:secret@db:5432/app" {
		t.Fatal("ciphertext must not equal plaintext")
	}
	if first == second {
		t.Fatal("encrypting the same plaintext twice should produce different ciphertext")
	}

	plaintext, err := box.Decrypt(first)
	if err != nil {
		t.Fatalf("decrypt value: %v", err)
	}
	if plaintext != "postgres://porter:secret@db:5432/app" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestSecretBoxRejectsInvalidMasterKey(t *testing.T) {
	if _, err := secretcrypto.NewSecretBox([]byte("short")); err == nil {
		t.Fatal("expected short master key to fail")
	}
}

func TestLoadSecretBoxReadsHexMasterKeyFile(t *testing.T) {
	key, err := secretcrypto.GenerateMasterKey()
	if err != nil {
		t.Fatalf("generate master key: %v", err)
	}
	path := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)+"\n"), 0600); err != nil {
		t.Fatalf("write master key: %v", err)
	}

	box, err := secretcrypto.LoadSecretBox(path)
	if err != nil {
		t.Fatalf("load secret box: %v", err)
	}

	encrypted, err := box.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	decrypted, err := box.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "secret" {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestMaskSecretUsesFixedMask(t *testing.T) {
	if got := secretcrypto.MaskSecret(); got != "••••" {
		t.Fatalf("mask = %q, want fixed bullet mask", got)
	}
}
