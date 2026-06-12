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

	loaded, err := service.GetProject(ctx, "proj_test")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if loaded.Name != "demo" {
		t.Fatalf("loaded project = %#v", loaded)
	}

	updated, err := service.UpdateProject(ctx, "proj_test", "renamed")
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	if updated.Name != "renamed" {
		t.Fatalf("updated project = %#v", updated)
	}

	if err := service.DeleteProject(ctx, "proj_test"); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	projects, err = service.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list projects after delete: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("projects after delete = %#v", projects)
	}
}
