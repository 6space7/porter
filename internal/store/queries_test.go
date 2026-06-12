package store_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/store"
)

func TestGeneratedQueriesCreateProjectsAndTokens(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())

	project, err := queries.CreateProject(ctx, store.CreateProjectParams{
		ID:   "proj_1",
		Name: "Demo",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if project.ID != "proj_1" || project.Name != "Demo" {
		t.Fatalf("project = %#v", project)
	}

	projects, err := queries.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 || projects[0].ID != "proj_1" {
		t.Fatalf("projects = %#v", projects)
	}

	token, err := queries.CreateToken(ctx, store.CreateTokenParams{
		ID:     "tok_1",
		Name:   "reader",
		Hash:   "0123456789abcdef",
		Scopes: "apps:read",
	})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if token.Hash != "0123456789abcdef" {
		t.Fatalf("token hash = %q", token.Hash)
	}

	found, err := queries.GetTokenByHash(ctx, "0123456789abcdef")
	if err != nil {
		t.Fatalf("get token by hash: %v", err)
	}
	if found.ID != "tok_1" || found.Scopes != "apps:read" {
		t.Fatalf("found token = %#v", found)
	}
}
