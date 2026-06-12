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
}
