package proxy_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/store"
)

func TestStoreRouteSourceListsVerifiedDomainRoutes(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	_, err = queries.CreateProject(ctx, store.CreateProjectParams{ID: "proj_1", Name: "demo"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, err = queries.CreateApp(ctx, store.CreateAppParams{
		ID:           "app_web",
		ProjectID:    "proj_1",
		ServerID:     "local",
		Name:         "web",
		GitUrl:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
		Status:       "running",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	for _, domain := range []store.CreateDomainParams{
		{ID: "dom_generated", AppID: "app_web", Hostname: "web.203-0-113-42.sslip.io", Type: "generated", Verified: 1},
		{ID: "dom_custom", AppID: "app_web", Hostname: "web.example.com", Type: "custom", Verified: 1},
		{ID: "dom_unverified", AppID: "app_web", Hostname: "pending.example.com", Type: "custom", Verified: 0},
	} {
		if _, err := queries.CreateDomain(ctx, domain); err != nil {
			t.Fatalf("create domain %s: %v", domain.ID, err)
		}
	}
	if _, err := queries.CreateService(ctx, store.CreateServiceParams{
		ID:               "svc_flows",
		ProjectID:        "proj_1",
		ServerID:         "local",
		TemplateSlug:     "n8n",
		Name:             "flows",
		Status:           "running",
		GeneratedSecrets: "encrypted",
		InternalPort:     5678,
		Exposed:          1,
		Hostname:         sql.NullString{String: "flows.203-0-113-42.sslip.io", Valid: true},
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	source := proxy.NewStoreRouteSource(queries)
	routes, err := source.ListRoutes(ctx)
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}

	want := []proxy.Route{
		{Hostname: "flows.203-0-113-42.sslip.io", ContainerName: "porter-svc-svc_flows", InternalPort: 5678},
		{Hostname: "web.203-0-113-42.sslip.io", ContainerName: "porter-app_web", InternalPort: 3000},
		{Hostname: "web.example.com", ContainerName: "porter-app_web", InternalPort: 3000},
	}
	if !reflect.DeepEqual(routes, want) {
		t.Fatalf("routes = %#v, want %#v", routes, want)
	}
}
