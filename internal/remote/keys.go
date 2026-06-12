package remote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type KeyStore interface {
	Put(ctx context.Context, serverID string, privateKeyPEM []byte) (string, error)
}

type FileKeyStore struct {
	Dir string
}

func (store FileKeyStore) Put(ctx context.Context, serverID string, privateKeyPEM []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if store.Dir == "" {
		return "", fmt.Errorf("key store dir is required")
	}
	if serverID == "" {
		return "", fmt.Errorf("server id is required")
	}
	if len(privateKeyPEM) == 0 {
		return "", fmt.Errorf("private key is required")
	}
	if err := os.MkdirAll(store.Dir, 0o700); err != nil {
		return "", fmt.Errorf("create key store dir: %w", err)
	}
	path := filepath.Join(store.Dir, serverID+".pem")
	if err := os.WriteFile(path, privateKeyPEM, 0o600); err != nil {
		return "", fmt.Errorf("write private key: %w", err)
	}
	return path, nil
}
