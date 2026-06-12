package api_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/store"
)

func TestStoreAuthServiceLogsInAdminAndStoresHashedToken(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: filepath.Join(t.TempDir(), "porter.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	passwordHash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	queries := store.New(db.SQL())
	if _, err := queries.CreateUser(ctx, store.CreateUserParams{
		ID:           "usr_admin",
		Email:        "admin@example.com",
		PasswordHash: passwordHash,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := api.NewStoreAuthService(queries)
	response, err := service.Login(ctx, "admin@example.com", "secret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if response.Token == "" || response.TokenID == "" {
		t.Fatalf("response missing token fields: %#v", response)
	}
	if !hasScope(response.Scopes, "projects:write") || !hasScope(response.Scopes, "apps:deploy") {
		t.Fatalf("scopes = %#v", response.Scopes)
	}

	record, err := queries.GetTokenByHash(ctx, auth.HashToken(response.Token))
	if err != nil {
		t.Fatalf("get token by hash: %v", err)
	}
	if record.Hash == response.Token || strings.Contains(record.Hash, "ptr_") {
		t.Fatalf("stored token hash leaked plaintext: %q", record.Hash)
	}
}

func TestStoreAuthServiceRejectsInvalidPassword(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: filepath.Join(t.TempDir(), "porter.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	passwordHash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	queries := store.New(db.SQL())
	if _, err := queries.CreateUser(ctx, store.CreateUserParams{
		ID:           "usr_admin",
		Email:        "admin@example.com",
		PasswordHash: passwordHash,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := api.NewStoreAuthService(queries)
	_, err = service.Login(ctx, "admin@example.com", "wrong")

	if !errors.Is(err, api.ErrInvalidLogin) {
		t.Fatalf("error = %v, want ErrInvalidLogin", err)
	}
}

func hasScope(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}
