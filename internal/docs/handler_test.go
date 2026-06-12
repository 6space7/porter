package docs_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/docs"
)

func TestHandlerServesLLMSText(t *testing.T) {
	handler := docs.NewHandler(docs.Config{PlatformDomain: "porter.example.com"}, http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/llms.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if contentType := rr.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", contentType)
	}
	for _, want := range []string{
		"/api/v1/mcp",
		"Authorization: Bearer <porter token>",
		"porter_create_app",
		"porter_set_env_var",
		"services:write",
	} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Fatalf("llms.txt missing %q:\n%s", want, rr.Body.String())
		}
	}
}

func TestHandlerServesJSONDocs(t *testing.T) {
	handler := docs.NewHandler(docs.Config{PlatformDomain: "porter.example.com"}, http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if contentType := rr.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("content-type = %q, want application/json", contentType)
	}

	var body struct {
		APIBase     string `json:"api_base"`
		MCPEndpoint string `json:"mcp_endpoint"`
		Auth        struct {
			Header string `json:"header"`
			Scheme string `json:"scheme"`
		} `json:"auth"`
		Tools []struct {
			Name   string   `json:"name"`
			Scopes []string `json:"scopes"`
		} `json:"tools"`
		Examples []struct {
			Name string         `json:"name"`
			Body map[string]any `json:"body"`
		} `json:"examples"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode docs: %v", err)
	}
	if body.APIBase != "/api/v1" {
		t.Fatalf("api_base = %q", body.APIBase)
	}
	if body.MCPEndpoint != "https://porter.example.com/api/v1/mcp" {
		t.Fatalf("mcp_endpoint = %q", body.MCPEndpoint)
	}
	if body.Auth.Header != "Authorization" || body.Auth.Scheme != "Bearer" {
		t.Fatalf("auth = %#v", body.Auth)
	}
	assertToolScope(t, body.Tools, "porter_deploy_service", "services:write")
	assertToolScope(t, body.Tools, "porter_create_app", "apps:write")
	if len(body.Examples) == 0 {
		t.Fatal("expected examples")
	}
}

func assertToolScope(t *testing.T, tools []struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}, name, scope string) {
	t.Helper()
	for _, tool := range tools {
		if tool.Name != name {
			continue
		}
		for _, gotScope := range tool.Scopes {
			if gotScope == scope {
				return
			}
		}
		t.Fatalf("tool %s scopes = %#v, missing %s", name, tool.Scopes, scope)
	}
	t.Fatalf("tool %s not found in %#v", name, tools)
}
