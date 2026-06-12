package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/api"
)

func TestMCPRouteRequiresAuthAndReceivesPrincipal(t *testing.T) {
	verifier := api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		if token == "agent-token" {
			return api.Principal{TokenID: "tok_agent", Scopes: []string{"apps:read"}}, nil
		}
		return api.Principal{}, api.ErrInvalidToken
	})
	var gotPrincipal api.Principal
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: verifier,
		MCP: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := api.PrincipalFromContext(r.Context())
			if !ok {
				t.Fatal("MCP handler did not receive auth principal")
			}
			gotPrincipal = principal
			w.WriteHeader(http.StatusAccepted)
		}),
	})

	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/mcp", "", http.StatusUnauthorized, "unauthorized")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp", nil)
	req.Header.Set("Authorization", "Bearer agent-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("MCP status = %d, want %d; body=%s", rr.Code, http.StatusAccepted, rr.Body.String())
	}
	if gotPrincipal.TokenID != "tok_agent" {
		t.Fatalf("principal = %#v", gotPrincipal)
	}
}
