package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/6space7/porter/internal/api"
)

func TestLoginRouteReturnsBearerTokenForAdminCredentials(t *testing.T) {
	auth := &fakeAuthService{
		response: api.LoginResponse{
			Token:   "ptr_login",
			TokenID: "tok_login",
			Scopes:  []string{"projects:read", "apps:deploy"},
		},
	}
	router := api.NewRouterWithDeps(api.Dependencies{Auth: auth})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"secret"}`))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var response api.LoginResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.Token != "ptr_login" || response.TokenID != "tok_login" {
		t.Fatalf("response = %#v", response)
	}
	if auth.email != "admin@example.com" || auth.password != "secret" {
		t.Fatalf("credentials = %q/%q", auth.email, auth.password)
	}
}

func TestLoginRouteSetsSessionAndCSRFCookies(t *testing.T) {
	auth := &fakeAuthService{
		response: api.LoginResponse{
			Token:   "ptr_login",
			TokenID: "tok_login",
			Scopes:  []string{"projects:read"},
		},
	}
	router := api.NewRouterWithDeps(api.Dependencies{Auth: auth})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"secret"}`))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var response api.LoginResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.CSRFToken == "" {
		t.Fatal("login response must include csrf token for browser clients")
	}
	session := responseCookie(t, rr.Result().Cookies(), "porter_session")
	if session.Value != "ptr_login" || !session.HttpOnly || !session.Secure || session.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie = %#v", session)
	}
	csrf := responseCookie(t, rr.Result().Cookies(), "porter_csrf")
	if csrf.Value != response.CSRFToken || csrf.HttpOnly || !csrf.Secure || csrf.SameSite != http.SameSiteStrictMode {
		t.Fatalf("csrf cookie = %#v response token = %q", csrf, response.CSRFToken)
	}
}

func TestLoginRouteRejectsBadCredentials(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{Auth: &fakeAuthService{err: api.ErrInvalidLogin}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"wrong"}`))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestLoginRouteRateLimitsRepeatedFailuresByIP(t *testing.T) {
	limiter := api.NewMemoryFailureLimiter(1, time.Minute, time.Minute)
	router := api.NewRouterWithDeps(api.Dependencies{
		Auth:                &fakeAuthService{err: api.ErrInvalidLogin},
		LoginFailureLimiter: limiter,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"wrong"}`))
	req.RemoteAddr = net.JoinHostPort("203.0.113.10", "12345")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("first status = %d, want %d; body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"wrong"}`))
	req.RemoteAddr = net.JoinHostPort("203.0.113.10", "12346")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d; body=%s", rr.Code, http.StatusTooManyRequests, rr.Body.String())
	}
}

func TestLogoutRouteRevokesCurrentToken(t *testing.T) {
	auth := &fakeAuthService{}
	router := api.NewRouterWithDeps(api.Dependencies{
		Auth:          auth,
		TokenVerifier: deployTestVerifier(),
	})

	assertStatusAndCode(t, router, http.MethodDelete, "/api/v1/auth/session", "", http.StatusUnauthorized, "unauthorized")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNoContent, rr.Body.String())
	}
	if auth.logoutTokenID != "tok_read" {
		t.Fatalf("logout token id = %q", auth.logoutTokenID)
	}
}

func TestCreateTokenRouteRequiresScopeAndReturnsPlaintextOnce(t *testing.T) {
	auth := &fakeAuthService{
		tokenResponse: api.CreateTokenResponse{
			Token:   "ptr_new",
			TokenID: "tok_new",
			Name:    "reader",
			Scopes:  []string{"apps:read"},
		},
	}
	router := api.NewRouterWithDeps(api.Dependencies{
		Auth:          auth,
		TokenVerifier: deployTestVerifier(),
	})

	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/auth/tokens", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/auth/tokens", "Bearer read-token", http.StatusForbidden, "forbidden")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", bytes.NewBufferString(`{"name":"reader","scopes":["apps:read"]}`))
	req.Header.Set("Authorization", "Bearer token-writer")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	var response api.CreateTokenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if response.Token != "ptr_new" || response.TokenID != "tok_new" {
		t.Fatalf("response = %#v", response)
	}
	if auth.tokenName != "reader" || len(auth.tokenScopes) != 1 || auth.tokenScopes[0] != "apps:read" {
		t.Fatalf("token input = %q %#v", auth.tokenName, auth.tokenScopes)
	}
}

func responseCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q missing from %#v", name, cookies)
	return nil
}

type fakeAuthService struct {
	email         string
	password      string
	logoutTokenID string
	tokenName     string
	tokenScopes   []string
	response      api.LoginResponse
	tokenResponse api.CreateTokenResponse
	err           error
}

func (service *fakeAuthService) Login(_ context.Context, email, password string) (api.LoginResponse, error) {
	service.email = email
	service.password = password
	return service.response, service.err
}

func (service *fakeAuthService) Logout(_ context.Context, tokenID string) error {
	service.logoutTokenID = tokenID
	return service.err
}

func (service *fakeAuthService) CreateToken(_ context.Context, name string, scopes []string) (api.CreateTokenResponse, error) {
	service.tokenName = name
	service.tokenScopes = append([]string(nil), scopes...)
	return service.tokenResponse, service.err
}
