package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/6space7/porter/internal/api"
	"github.com/go-chi/chi/v5"
)

func TestProtectedRouteRequiresBearerTokenAndScope(t *testing.T) {
	verifier := api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		switch token {
		case "read-token":
			return api.Principal{TokenID: "tok_read", Scopes: []string{"apps:read"}}, nil
		case "deploy-token":
			return api.Principal{TokenID: "tok_deploy", Scopes: []string{"apps:read", "apps:deploy"}}, nil
		default:
			return api.Principal{}, api.ErrInvalidToken
		}
	})

	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(api.RequireBearerToken(verifier))
		r.With(api.RequireScope("apps:read")).Get("/read", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		r.With(api.RequireScope("apps:deploy")).Post("/deploy", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	})

	assertStatusAndCode(t, router, http.MethodGet, "/read", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodGet, "/read", "Bearer bad-token", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodGet, "/read", "Bearer read-token", http.StatusNoContent, "")
	assertStatusAndCode(t, router, http.MethodPost, "/deploy", "Bearer read-token", http.StatusForbidden, "forbidden")
	assertStatusAndCode(t, router, http.MethodPost, "/deploy", "Bearer deploy-token", http.StatusNoContent, "")
}

func TestSessionCookieAuthRequiresCSRFForStateChangingRequests(t *testing.T) {
	verifier := api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		if token == "write-token" {
			return api.Principal{TokenID: "tok_write", Scopes: []string{"projects:write"}}, nil
		}
		return api.Principal{}, api.ErrInvalidToken
	})
	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(api.RequireBearerToken(verifier))
		r.Use(api.RequireCSRFForSessionAuth)
		r.With(api.RequireScope("projects:write")).Post("/projects", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.AddCookie(&http.Cookie{Name: "porter_session", Value: "write-token"})
	req.AddCookie(&http.Cookie{Name: "porter_csrf", Value: "csrf-token"})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.AddCookie(&http.Cookie{Name: "porter_session", Value: "write-token"})
	req.AddCookie(&http.Cookie{Name: "porter_csrf", Value: "csrf-token"})
	req.Header.Set("X-CSRF-Token", "csrf-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("csrf status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestBearerAuthDoesNotRequireCSRFForStateChangingRequests(t *testing.T) {
	verifier := api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		if token == "write-token" {
			return api.Principal{TokenID: "tok_write", Scopes: []string{"projects:write"}}, nil
		}
		return api.Principal{}, api.ErrInvalidToken
	})
	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(api.RequireBearerToken(verifier))
		r.Use(api.RequireCSRFForSessionAuth)
		r.With(api.RequireScope("projects:write")).Post("/projects", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestProtectedRouteTreatsVerifierErrorsAsUnauthorized(t *testing.T) {
	verifier := api.TokenVerifierFunc(func(_ context.Context, _ string) (api.Principal, error) {
		return api.Principal{}, errors.New("store unavailable")
	})

	router := chi.NewRouter()
	router.With(api.RequireBearerToken(verifier)).Get("/read", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	assertStatusAndCode(t, router, http.MethodGet, "/read", "Bearer any-token", http.StatusUnauthorized, "unauthorized")
}

func TestTokenAuthRateLimitsRepeatedFailuresByIP(t *testing.T) {
	verifier := api.TokenVerifierFunc(func(_ context.Context, _ string) (api.Principal, error) {
		return api.Principal{}, api.ErrInvalidToken
	})
	limiter := api.NewMemoryFailureLimiter(1, time.Minute, time.Minute)
	router := chi.NewRouter()
	router.With(api.RequireAuth(verifier, limiter)).Get("/read", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.RemoteAddr = net.JoinHostPort("203.0.113.20", "1111")
	req.Header.Set("Authorization", "Bearer bad-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("first status = %d, want %d; body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/read", nil)
	req.RemoteAddr = net.JoinHostPort("203.0.113.20", "2222")
	req.Header.Set("Authorization", "Bearer bad-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d; body=%s", rr.Code, http.StatusTooManyRequests, rr.Body.String())
	}
}

func assertStatusAndCode(t *testing.T, handler http.Handler, method, path, authorization string, wantStatus int, wantCode string) {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != wantStatus {
		t.Fatalf("%s %s with %q status = %d, want %d; body=%s", method, path, authorization, rr.Code, wantStatus, rr.Body.String())
	}
	if wantCode == "" {
		return
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", body.Error.Code, wantCode)
	}
}
