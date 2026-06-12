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
	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/runtime"
	"github.com/6space7/porter/internal/store"
)

func TestNewHandlerWiresStoreBackedAuthAndProjects(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "porter.db")
	masterKey, err := secretcrypto.GenerateMasterKey()
	if err != nil {
		t.Fatalf("generate master key: %v", err)
	}
	secretBox, err := secretcrypto.NewSecretBox(masterKey)
	if err != nil {
		t.Fatalf("new secret box: %v", err)
	}

	db, handler, err := runtime.NewHandlerWithOptions(ctx, config.Config{DatabasePath: dbPath, PublicIP: "203.0.113.42"}, runtime.Options{
		Resolver: fakeResolver{
			"custom.example.com": []string{"203.0.113.42"},
		},
		SecretBox: secretBox,
		Cloner: deploy.ClonerFunc(func(context.Context, deploy.CloneRequest) (deploy.CloneResult, error) {
			return deploy.CloneResult{SourceDir: "work/app/source", Log: "cloned\n"}, nil
		}),
		Builder: deploy.BuilderFunc(func(context.Context, deploy.BuildRequest) (deploy.BuildResult, error) {
			return deploy.BuildResult{ImageTag: "porter/app:dep", Log: "built\n"}, nil
		}),
		Runner: deploy.RunnerFunc(func(context.Context, deploy.RunRequest) (string, error) {
			return "started\n", nil
		}),
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	defer db.Close()

	plaintext, record, err := auth.NewToken("writer", []string{"projects:read", "projects:write", "apps:read", "apps:write", "apps:deploy"})
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

	req = httptest.NewRequest(http.MethodPost, "/api/v1/apps/"+app.ID+"/env", bytes.NewBufferString(`{"key":"DATABASE_URL","value":"postgres://secret","is_secret":true}`))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("env var status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "••••") || strings.Contains(rr.Body.String(), "postgres://secret") {
		t.Fatalf("env var response leaked secret or missed mask: %s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/apps/"+app.ID+"/deploy", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("deploy status = %d, want %d; body=%s", rr.Code, http.StatusAccepted, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"stage":"running"`) {
		t.Fatalf("deploy response missing running stage: %s", rr.Body.String())
	}
}

type fakeResolver map[string][]string

func (resolver fakeResolver) LookupHost(_ context.Context, hostname string) ([]string, error) {
	return resolver[hostname], nil
}
