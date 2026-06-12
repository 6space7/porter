package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/proxy"
)

func TestDomainRoutesRequireAuthAndScopes(t *testing.T) {
	domains := newFakeDomainService()
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: appTestVerifier(),
		Domains:       domains,
	})

	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/apps/app_1/domains", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/apps/app_1/domains", "Bearer read-token", http.StatusForbidden, "forbidden")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/app_1/domains", bytes.NewBufferString(`{"hostname":"app.example.com"}`))
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("create domain status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var created api.DomainResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created domain: %v", err)
	}
	if created.ID != "dom_1" || created.Hostname != "app.example.com" || created.Type != "custom" {
		t.Fatalf("created domain = %#v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/apps/app_1/domains", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list domains status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var listed []api.DomainResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode listed domains: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "dom_1" {
		t.Fatalf("listed domains = %#v", listed)
	}
}

func TestCreateDomainReturnsStructuredDNSPreflightError(t *testing.T) {
	domains := newFakeDomainService()
	domains.createErr = &proxy.DNSPreflightError{
		Hostname:        "app.example.com",
		RequiredARecord: "203.0.113.42",
		CurrentRecords:  []string{"198.51.100.10"},
	}
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: appTestVerifier(),
		Domains:       domains,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/app_1/domains", bytes.NewBufferString(`{"hostname":"app.example.com"}`))
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Code != "dns_preflight_failed" {
		t.Fatalf("error code = %q", body.Error.Code)
	}
	if body.Error.Details["required_a_record"] != "203.0.113.42" {
		t.Fatalf("details = %#v", body.Error.Details)
	}
}

type fakeDomainService struct {
	domains   []api.DomainResponse
	createErr error
}

func newFakeDomainService() *fakeDomainService {
	return &fakeDomainService{}
}

func (svc *fakeDomainService) AddCustomDomain(_ context.Context, appID, hostname string) (api.DomainResponse, error) {
	if svc.createErr != nil {
		return api.DomainResponse{}, svc.createErr
	}
	domain := api.DomainResponse{
		ID:       "dom_1",
		AppID:    appID,
		Hostname: hostname,
		Type:     "custom",
		Verified: true,
	}
	svc.domains = append(svc.domains, domain)
	return domain, nil
}

func (svc *fakeDomainService) ListDomains(_ context.Context, _ string) ([]api.DomainResponse, error) {
	return append([]api.DomainResponse(nil), svc.domains...), nil
}
