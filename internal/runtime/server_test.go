package runtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/runtime"
	"github.com/6space7/porter/internal/store"
)

func TestNewHandlerWiresStoreBackedAuthAndProjects(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "porter.db")

	db, handler, err := runtime.NewHandlerWithOptions(ctx, config.Config{DatabasePath: dbPath, PublicIP: "203.0.113.42"}, runtime.Options{
		Resolver: fakeResolver{
			"custom.example.com": []string{"203.0.113.42"},
		},
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	defer db.Close()

	plaintext, record, err := auth.NewToken("writer", []string{"projects:read", "projects:write", "apps:read", "apps:write"})
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	_, err = store.New(db.SQL()).CreateToken(ctx, store.CreateTokenParams{
		ID:     record.ID,
		Name:   record.Name,
		Hash:   record.Hash,
		Scopes: strings.Join(record.Scopes, ","),
	})
	if err != nil {
		t.Fatalf("store token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"name":"demo"}`))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project response: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/apps", bytes.NewBufferString(`{
		"project_id":"`+project.ID+`",
		"name":"web",
		"git_url":"https://github.com/example/web.git",
		"branch":"main",
		"build_type":"dockerfile",
		"internal_port":3000
	}`))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("app status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var app struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &app); err != nil {
		t.Fatalf("decode app response: %v", err)
	}

	domains, err := store.New(db.SQL()).ListDomainsByApp(ctx, app.ID)
	if err != nil {
		t.Fatalf("list app domains: %v", err)
	}
	if len(domains) != 1 || domains[0].Hostname != "web.203-0-113-42.sslip.io" {
		t.Fatalf("domains = %#v", domains)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/apps/"+app.ID+"/domains", bytes.NewBufferString(`{"hostname":"custom.example.com"}`))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("custom domain status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

type fakeResolver map[string][]string

func (resolver fakeResolver) LookupHost(_ context.Context, hostname string) ([]string, error) {
	return resolver[hostname], nil
}
