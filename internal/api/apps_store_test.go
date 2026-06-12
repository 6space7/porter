package api_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/store"
)

func TestStoreAppServicePersistsApps(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	_, err = queries.CreateProject(ctx, store.CreateProjectParams{
		ID:   "proj_1",
		Name: "demo",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	service := api.NewStoreAppService(queries, func() string {
		return "app_test"
	})

	created, err := service.CreateApp(ctx, api.CreateAppInput{
		ProjectID:    "proj_1",
		Name:         "web",
		GitURL:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	if created.ID != "app_test" || created.ProjectID != "proj_1" || created.Status != "created" {
		t.Fatalf("created app = %#v", created)
	}

	apps, err := service.ListApps(ctx)
	if err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps) != 1 || apps[0].ID != "app_test" || apps[0].GitURL != "https://github.com/example/web.git" {
		t.Fatalf("apps = %#v", apps)
	}

	loaded, err := service.GetApp(ctx, "app_test")
	if err != nil {
		t.Fatalf("get app: %v", err)
	}
	if loaded.Name != "web" {
		t.Fatalf("loaded app = %#v", loaded)
	}

	branch := "release"
	internalPort := int64(8080)
	updated, err := service.UpdateApp(ctx, "app_test", api.UpdateAppInput{
		Branch:       &branch,
		InternalPort: &internalPort,
	})
	if err != nil {
		t.Fatalf("update app: %v", err)
	}
	if updated.Branch != "release" || updated.InternalPort != 8080 {
		t.Fatalf("updated app = %#v", updated)
	}

	if err := service.DeleteApp(ctx, "app_test"); err != nil {
		t.Fatalf("delete app: %v", err)
	}
	apps, err = service.ListApps(ctx)
	if err != nil {
		t.Fatalf("list apps after delete: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("apps after delete = %#v", apps)
	}
}

func TestStoreAppServiceCreatesGeneratedDomainWhenPublicIPConfigured(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	_, err = queries.CreateProject(ctx, store.CreateProjectParams{
		ID:   "proj_1",
		Name: "demo",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	service := api.NewStoreAppServiceWithOptions(queries, api.StoreAppServiceOptions{
		NewAppID: func() string {
			return "app_test"
		},
		NewDomainID: func() string {
			return "dom_test"
		},
		PublicIP: "203.0.113.42",
	})

	_, err = service.CreateApp(ctx, api.CreateAppInput{
		ProjectID:    "proj_1",
		Name:         "web",
		GitURL:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	domains, err := queries.ListDomainsByApp(ctx, "app_test")
	if err != nil {
		t.Fatalf("list domains: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("domains = %#v, want one generated domain", domains)
	}
	if domains[0].ID != "dom_test" || domains[0].Hostname != "web.203-0-113-42.sslip.io" {
		t.Fatalf("domain = %#v", domains[0])
	}
	if domains[0].Type != "generated" || domains[0].Verified != 1 {
		t.Fatalf("domain type/verified = %q/%d", domains[0].Type, domains[0].Verified)
	}
}

func TestStoreAppServiceLifecycleUsesRuntimeAndPersistsStatus(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	_, err = queries.CreateProject(ctx, store.CreateProjectParams{
		ID:   "proj_1",
		Name: "demo",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	runtime := &fakeAppRuntime{}
	service := api.NewStoreAppServiceWithOptions(queries, api.StoreAppServiceOptions{
		NewAppID: func() string {
			return "app_test"
		},
		Runtime: runtime,
	})
	if _, err := service.CreateApp(ctx, api.CreateAppInput{
		ProjectID:    "proj_1",
		Name:         "web",
		GitURL:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
	}); err != nil {
		t.Fatalf("create app: %v", err)
	}

	stopped, err := service.StopApp(ctx, "app_test")
	if err != nil {
		t.Fatalf("stop app: %v", err)
	}
	if stopped.Status != "stopped" || runtime.stopped != "app_test" {
		t.Fatalf("stopped = %#v runtime=%#v", stopped, runtime)
	}

	started, err := service.StartApp(ctx, "app_test")
	if err != nil {
		t.Fatalf("start app: %v", err)
	}
	if started.Status != "running" || runtime.started != "app_test" {
		t.Fatalf("started = %#v runtime=%#v", started, runtime)
	}

	if _, err := service.RestartApp(ctx, "app_test"); err != nil {
		t.Fatalf("restart app: %v", err)
	}
	if runtime.stopCalls != 2 || runtime.startCalls != 2 {
		t.Fatalf("runtime calls = stop:%d start:%d", runtime.stopCalls, runtime.startCalls)
	}

	if err := service.DeleteApp(ctx, "app_test"); err != nil {
		t.Fatalf("delete app: %v", err)
	}
	if runtime.removed != "app_test" {
		t.Fatalf("removed app = %q", runtime.removed)
	}
}

type fakeAppRuntime struct {
	started    string
	stopped    string
	removed    string
	startCalls int
	stopCalls  int
}

func (runtime *fakeAppRuntime) StartApp(_ context.Context, appID string) error {
	runtime.started = appID
	runtime.startCalls++
	return nil
}

func (runtime *fakeAppRuntime) StopApp(_ context.Context, appID string) error {
	runtime.stopped = appID
	runtime.stopCalls++
	return nil
}

func (runtime *fakeAppRuntime) RemoveApp(_ context.Context, appID string) error {
	runtime.removed = appID
	return nil
}
