package runtime_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/runtime"
	"github.com/6space7/porter/internal/store"
)

func TestNewHandlerReconcilesCaddyRoutesFromSQLite(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "porter.db")
	seedVerifiedDomain(t, ctx, dbPath)

	admin := &fakeCaddyAdmin{}
	db, _, err := runtime.NewHandlerWithOptions(ctx, config.Config{
		DatabasePath:  dbPath,
		WorkspacePath: filepath.Join(t.TempDir(), "work"),
		CaddyAskURL:   "http://127.0.0.1:8080/api/v1/caddy/ask",
	}, runtime.Options{
		CaddyAdmin: admin,
		Cloner: deploy.ClonerFunc(func(context.Context, deploy.CloneRequest) (deploy.CloneResult, error) {
			return deploy.CloneResult{}, nil
		}),
		Builder: deploy.BuilderFunc(func(context.Context, deploy.BuildRequest) (deploy.BuildResult, error) {
			return deploy.BuildResult{}, nil
		}),
		Runner: deploy.RunnerFunc(func(context.Context, deploy.RunRequest) (string, error) {
			return "", nil
		}),
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	defer db.Close()

	if !admin.called {
		t.Fatal("caddy admin was not called")
	}
	if len(admin.config.HTTP.Routes) != 1 {
		t.Fatalf("routes = %#v", admin.config.HTTP.Routes)
	}
	route := admin.config.HTTP.Routes[0]
	if route.Hostname != "web.example.com" || route.UpstreamDial != "porter-app_web:3000" {
		t.Fatalf("route = %#v", route)
	}
	if admin.config.HTTP.AskURL != "http://127.0.0.1:8080/api/v1/caddy/ask" {
		t.Fatalf("ask url = %q", admin.config.HTTP.AskURL)
	}
}

func seedVerifiedDomain(t *testing.T, ctx context.Context, dbPath string) {
	t.Helper()

	db, err := store.Open(ctx, store.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	if _, err := queries.CreateProject(ctx, store.CreateProjectParams{ID: "proj_1", Name: "demo"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := queries.CreateApp(ctx, store.CreateAppParams{
		ID:           "app_web",
		ProjectID:    "proj_1",
		ServerID:     "local",
		Name:         "web",
		GitUrl:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
		Status:       "running",
	}); err != nil {
		t.Fatalf("create app: %v", err)
	}
	if _, err := queries.CreateDomain(ctx, store.CreateDomainParams{
		ID:       "dom_1",
		AppID:    "app_web",
		Hostname: "web.example.com",
		Type:     "custom",
		Verified: 1,
	}); err != nil {
		t.Fatalf("create domain: %v", err)
	}
}

type fakeCaddyAdmin struct {
	called bool
	config proxy.CaddyConfig
}

func (admin *fakeCaddyAdmin) ApplyConfig(_ context.Context, config proxy.CaddyConfig) error {
	admin.called = true
	admin.config = config
	return nil
}
