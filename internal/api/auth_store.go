package api

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/store"
)

type storeTokenVerifier struct {
	queries *store.Queries
}

func NewStoreTokenVerifier(queries *store.Queries) TokenVerifier {
	return storeTokenVerifier{queries: queries}
}

func (verifier storeTokenVerifier) VerifyBearerToken(ctx context.Context, token string) (Principal, error) {
	record, err := verifier.queries.GetTokenByHash(ctx, auth.HashToken(token))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, ErrInvalidToken
		}
		return Principal{}, err
	}

	return Principal{
		TokenID: record.ID,
		Scopes:  parseScopes(record.Scopes),
	}, nil
}

func parseScopes(value string) []string {
	parts := strings.Split(value, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		scope := strings.TrimSpace(part)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}
