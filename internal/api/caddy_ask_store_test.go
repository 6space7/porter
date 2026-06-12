package api_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/store"
)

func TestStoreCaddyAskServiceAllowsOnlyVerifiedDomains(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	if _, err := queries.CreateProject(ctx, store.CreateProjectParams{ID: "proj_1", Name: "demo"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := queries.CreateApp(ctx, store.CreateAppParams{
		ID:           "app_1",
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
	if _, err := queries.CreateDomain(ctx, store.CreateDomainParams{ID: "dom_ok", AppID: "app_1", Hostname: "web.example.com", Type: "custom", Verified: 1}); err != nil {
		t.Fatalf("create verified domain: %v", err)
	}
	if _, err := queries.CreateDomain(ctx, store.CreateDomainParams{ID: "dom_pending", AppID: "app_1", Hostname: "pending.example.com", Type: "custom", Verified: 0}); err != nil {
		t.Fatalf("create pending domain: %v", err)
	}

	service := api.NewStoreCaddyAskService(queries)
	allowed, err := service.IsDomainAllowed(ctx, "web.example.com")
	if err != nil {
		t.Fatalf("check verified domain: %v", err)
	}
	if !allowed {
		t.Fatal("expected verified domain to be allowed")
	}

	allowed, err = service.IsDomainAllowed(ctx, "pending.example.com")
	if err != nil {
		t.Fatalf("check pending domain: %v", err)
	}
	if allowed {
		t.Fatal("expected unverified domain to be denied")
	}

	allowed, err = service.IsDomainAllowed(ctx, "missing.example.com")
	if err != nil {
		t.Fatalf("check missing domain: %v", err)
	}
	if allowed {
		t.Fatal("expected missing domain to be denied")
	}
}
