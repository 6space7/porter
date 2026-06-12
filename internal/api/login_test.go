package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestLoginRouteRejectsBadCredentials(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{Auth: &fakeAuthService{err: api.ErrInvalidLogin}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"wrong"}`))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
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

type fakeAuthService struct {
	email         string
	password      string
	logoutTokenID string
	response      api.LoginResponse
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
