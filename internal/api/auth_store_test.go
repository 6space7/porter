package api_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/store"
)

func TestStoreTokenVerifierAuthenticatesHashedToken(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	plaintext, record, err := auth.NewToken("agent", []string{"projects:read", "apps:deploy"})
	if err != nil {
		t.Fatalf("new token: %v", err)
	}

	queries := store.New(db.SQL())
	_, err = queries.CreateToken(ctx, store.CreateTokenParams{
		ID:     record.ID,
		Name:   record.Name,
		Hash:   record.Hash,
		Scopes: strings.Join(record.Scopes, ","),
	})
	if err != nil {
		t.Fatalf("store token: %v", err)
	}

	verifier := api.NewStoreTokenVerifier(queries)
	principal, err := verifier.VerifyBearerToken(ctx, plaintext)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}

	if principal.TokenID != record.ID {
		t.Fatalf("token id = %q, want %q", principal.TokenID, record.ID)
	}
	if len(principal.Scopes) != 2 || principal.Scopes[0] != "projects:read" || principal.Scopes[1] != "apps:deploy" {
		t.Fatalf("scopes = %#v", principal.Scopes)
	}
}

func TestStoreTokenVerifierRejectsUnknownToken(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	verifier := api.NewStoreTokenVerifier(store.New(db.SQL()))
	_, err = verifier.VerifyBearerToken(ctx, "ptr_unknown")

	if !errors.Is(err, api.ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}
