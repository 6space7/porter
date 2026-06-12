package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	coreauth "github.com/6space7/porter/internal/auth"
)

var ErrInvalidToken = errors.New("invalid token")

const (
	SessionCookieName = "porter_session"
	CSRFCookieName    = "porter_csrf"
)

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
	return RequireAuth(verifier, nil)
}

func RequireAuth(verifier TokenVerifier, limiter FailureLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)
			if limiter != nil && !limiter.Allow(key) {
				WriteError(w, http.StatusTooManyRequests, "rate_limited", "Too many authentication failures.", "Wait before retrying authentication.", nil)
				return
			}

			token, source, ok := authToken(r)
			if !ok {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.", "Send a bearer token in the Authorization header or use a valid session cookie.", nil)
				return
			}

			principal, err := verifier.VerifyBearerToken(r.Context(), token)
			if err != nil {
				if limiter != nil {
					limiter.RecordFailure(key)
				}
				WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication failed.", "Create a new token or log in again.", nil)
				return
			}
			if limiter != nil {
				limiter.Reset(key)
			}

			ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
			ctx = context.WithValue(ctx, authSourceContextKey{}, source)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireCSRFForSessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) || authSourceFromContext(r.Context()) != authSourceSession {
			next.ServeHTTP(w, r)
			return
		}

		csrfCookie, err := r.Cookie(CSRFCookieName)
		if err != nil || strings.TrimSpace(csrfCookie.Value) == "" {
			WriteError(w, http.StatusForbidden, "csrf_required", "CSRF token is required.", "Send the X-CSRF-Token header that matches the CSRF cookie.", nil)
			return
		}
		header := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
		if header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(csrfCookie.Value)) != 1 {
			WriteError(w, http.StatusForbidden, "csrf_required", "CSRF token is invalid.", "Send the X-CSRF-Token header that matches the CSRF cookie.", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
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

func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

type principalContextKey struct{}
type authSourceContextKey struct{}
type authSource string

const (
	authSourceBearer  authSource = "bearer"
	authSourceSession authSource = "session"
)

func bearerToken(header string) (string, bool) {
	prefix, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(prefix, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return token, true
}

func authToken(r *http.Request) (string, authSource, bool) {
	if token, ok := bearerToken(r.Header.Get("Authorization")); ok {
		return token, authSourceBearer, true
	}
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", "", false
	}
	return cookie.Value, authSourceSession, true
}

func authSourceFromContext(ctx context.Context) authSource {
	source, _ := ctx.Value(authSourceContextKey{}).(authSource)
	return source
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
