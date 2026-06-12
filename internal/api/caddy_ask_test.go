package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/api"
)

func TestCaddyAskAllowsRegisteredDomainWithoutAuth(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{
		CaddyAsk: fakeCaddyAskService{
			allowed: map[string]bool{"web.example.com": true},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/caddy/ask?domain=web.example.com", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestCaddyAskRejectsUnknownDomainWithoutAuth(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{
		CaddyAsk: fakeCaddyAskService{
			allowed: map[string]bool{"web.example.com": true},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/caddy/ask?domain=unknown.example.com", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
}

func TestCaddyAskValidatesDomain(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{
		CaddyAsk: fakeCaddyAskService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/caddy/ask?domain=bad_domain", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

type fakeCaddyAskService struct {
	allowed map[string]bool
}

func (svc fakeCaddyAskService) IsDomainAllowed(_ context.Context, hostname string) (bool, error) {
	return svc.allowed[hostname], nil
}
