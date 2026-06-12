package runtime_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/config"
	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/runtime"
	"github.com/6space7/porter/internal/store"
)

func TestNewHandlerLoadsSecretBoxFromMasterKeyPath(t *testing.T) {
	ctx := context.Background()
	key, err := secretcrypto.GenerateMasterKey()
	if err != nil {
		t.Fatalf("generate master key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0600); err != nil {
		t.Fatalf("write master key: %v", err)
	}

	db, handler, err := runtime.NewHandlerWithOptions(ctx, config.Config{
		DatabasePath:  filepath.Join(t.TempDir(), "porter.db"),
		WorkspacePath: filepath.Join(t.TempDir(), "work"),
		MasterKeyPath: keyPath,
	}, runtime.Options{
		Cloner: deploy.ClonerFunc(func(context.Context, deploy.CloneRequest) (deploy.CloneResult, error) {
			return deploy.CloneResult{}, nil
		}),
		Builder: deploy.BuilderFunc(func(context.Context, deploy.BuildRequest) (deploy.BuildResult, error) {
			return deploy.BuildResult{}, nil
		}),
		Runner: deploy.RunnerFunc(func(context.Context, deploy.RunRequest) (string, error) {
			return "", nil
		}),
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	seedRuntimeLogApp(t, ctx, queries)
	token := seedSecretToken(t, ctx, queries)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/app_1/env", bytes.NewBufferString(`{"key":"DATABASE_URL","value":"postgres://secret","is_secret":true}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "postgres://secret") || !strings.Contains(rr.Body.String(), secretcrypto.MaskSecret()) {
		t.Fatalf("secret response leaked plaintext or missed mask: %s", rr.Body.String())
	}
}

func seedSecretToken(t *testing.T, ctx context.Context, queries *store.Queries) string {
	t.Helper()

	plaintext, record, err := auth.NewToken("env-writer", []string{"apps:read", "apps:write"})
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	if _, err := queries.CreateToken(ctx, store.CreateTokenParams{
		ID:     record.ID,
		Name:   record.Name,
		Hash:   record.Hash,
		Scopes: strings.Join(record.Scopes, ","),
	}); err != nil {
		t.Fatalf("store token: %v", err)
	}
	return plaintext
}
