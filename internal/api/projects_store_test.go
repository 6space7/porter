package api_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/store"
)

func TestStoreProjectServicePersistsProjects(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	service := api.NewStoreProjectService(store.New(db.SQL()), func() string {
		return "proj_test"
	})

	created, err := service.CreateProject(ctx, "demo")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if created.ID != "proj_test" || created.Name != "demo" {
		t.Fatalf("created project = %#v", created)
	}

	projects, err := service.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 || projects[0].ID != "proj_test" || projects[0].Name != "demo" {
		t.Fatalf("projects = %#v", projects)
	}
}
