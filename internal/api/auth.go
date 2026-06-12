package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	coreauth "github.com/6space7/porter/internal/auth"
)

var ErrInvalidToken = errors.New("invalid token")

type Principal struct {
	TokenID string
	Scopes  []string
}

type TokenVerifier interface {
	VerifyBearerToken(ctx context.Context, token string) (Principal, error)
}

type TokenVerifierFunc func(ctx context.Context, token string) (Principal, error)

func (f TokenVerifierFunc) VerifyBearerToken(ctx context.Context, token string) (Principal, error) {
	return f(ctx, token)
}

func RequireBearerToken(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.", "Send a bearer token in the Authorization header.", nil)
				return
			}

			principal, err := verifier.VerifyBearerToken(r.Context(), token)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication failed.", "Create a new token or log in again.", nil)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal)))
		})
	}
}

func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.", "Send a bearer token in the Authorization header.", nil)
				return
			}
			if !coreauth.HasScope(principal.Scopes, scope) {
				WriteError(w, http.StatusForbidden, "forbidden", "Token scope is insufficient.", "Create or use a token with the required scope.", map[string]any{
					"required_scope": scope,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

type principalContextKey struct{}

func bearerToken(header string) (string, bool) {
	prefix, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(prefix, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return token, true
}
