package api_test

import (
	"context"
	"errors"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/store"
)

func TestStoreDomainServiceAddsCustomDomainAfterPreflight(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDomainTest(t, ctx)
	defer closeDB()

	service := api.NewStoreDomainService(queries, api.StoreDomainServiceOptions{
		Resolver: fakeResolver{
			"app.example.com": []string{"203.0.113.42"},
		},
		ServerIP: "203.0.113.42",
		NewDomainID: func() string {
			return "dom_custom"
		},
	})

	domain, err := service.AddCustomDomain(ctx, "app_1", "app.example.com")
	if err != nil {
		t.Fatalf("add custom domain: %v", err)
	}
	if domain.ID != "dom_custom" || domain.Hostname != "app.example.com" || domain.Type != "custom" || !domain.Verified {
		t.Fatalf("domain = %#v", domain)
	}

	domains, err := service.ListDomains(ctx, "app_1")
	if err != nil {
		t.Fatalf("list domains: %v", err)
	}
	if len(domains) != 1 || domains[0].ID != "dom_custom" {
		t.Fatalf("domains = %#v", domains)
	}
}

func TestStoreDomainServiceDoesNotStoreFailedPreflight(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDomainTest(t, ctx)
	defer closeDB()

	service := api.NewStoreDomainService(queries, api.StoreDomainServiceOptions{
		Resolver: fakeResolver{
			"app.example.com": []string{"198.51.100.10"},
		},
		ServerIP: "203.0.113.42",
		NewDomainID: func() string {
			return "dom_custom"
		},
	})

	_, err := service.AddCustomDomain(ctx, "app_1", "app.example.com")
	var preflightErr *proxy.DNSPreflightError
	if !errors.As(err, &preflightErr) {
		t.Fatalf("error = %v, want DNSPreflightError", err)
	}

	domains, err := service.ListDomains(ctx, "app_1")
	if err != nil {
		t.Fatalf("list domains: %v", err)
	}
	if len(domains) != 0 {
		t.Fatalf("domains = %#v, want none", domains)
	}
}

func TestStoreDomainServiceDeletesAndReverifiesDomains(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDomainTest(t, ctx)
	defer closeDB()
	routeUpdater := &fakeRouteUpdater{}

	_, err := queries.CreateDomain(ctx, store.CreateDomainParams{
		ID:       "dom_custom",
		AppID:    "app_1",
		Hostname: "app.example.com",
		Type:     "custom",
		Verified: 0,
	})
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}

	service := api.NewStoreDomainService(queries, api.StoreDomainServiceOptions{
		Resolver: fakeResolver{
			"app.example.com": []string{"203.0.113.42"},
		},
		ServerIP:     "203.0.113.42",
		RouteUpdater: routeUpdater,
	})

	verified, err := service.VerifyDomain(ctx, "app_1", "dom_custom")
	if err != nil {
		t.Fatalf("verify domain: %v", err)
	}
	if !verified.Verified {
		t.Fatalf("verified domain = %#v", verified)
	}
	if routeUpdater.calls != 1 {
		t.Fatalf("route updater calls after verify = %d, want 1", routeUpdater.calls)
	}

	if err := service.DeleteDomain(ctx, "app_1", "dom_custom"); err != nil {
		t.Fatalf("delete domain: %v", err)
	}
	domains, err := service.ListDomains(ctx, "app_1")
	if err != nil {
		t.Fatalf("list domains after delete: %v", err)
	}
	if len(domains) != 0 {
		t.Fatalf("domains after delete = %#v", domains)
	}
	if routeUpdater.calls != 2 {
		t.Fatalf("route updater calls after delete = %d, want 2", routeUpdater.calls)
	}
}

func setupAppForDomainTest(t *testing.T, ctx context.Context) (*store.Queries, func()) {
	t.Helper()

	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	queries := store.New(db.SQL())
	_, err = queries.CreateProject(ctx, store.CreateProjectParams{ID: "proj_1", Name: "demo"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, err = queries.CreateApp(ctx, store.CreateAppParams{
		ID:           "app_1",
		ProjectID:    "proj_1",
		ServerID:     "local",
		Name:         "web",
		GitUrl:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
		Status:       "created",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	return queries, func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}

type fakeResolver map[string][]string

func (resolver fakeResolver) LookupHost(_ context.Context, hostname string) ([]string, error) {
	return resolver[hostname], nil
}
