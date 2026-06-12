package api_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/store"
)

func TestStoreDeploymentServiceRunsPipelineAndListsDeployments(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDeploymentServiceTest(t, ctx)
	defer closeDB()

	pipeline := deploy.Pipeline{
		Store: deploy.NewStoreDeploymentStore(queries, func() string {
			return "dep_1"
		}),
		Cloner: deploy.ClonerFunc(func(_ context.Context, req deploy.CloneRequest) (string, error) {
			if req.GitURL != "https://github.com/example/web.git" || req.Branch != "main" {
				t.Fatalf("clone request = %#v", req)
			}
			return "cloned\n", nil
		}),
		Builder: deploy.BuilderFunc(func(context.Context, deploy.BuildRequest) (deploy.BuildResult, error) {
			return deploy.BuildResult{ImageTag: "porter/app_1:dep_1", Log: "built\n"}, nil
		}),
		Runner: deploy.RunnerFunc(func(context.Context, deploy.RunRequest) (string, error) {
			return "started\n", nil
		}),
	}
	service := api.NewStoreDeploymentService(queries, pipeline, nil)

	deployment, err := service.DeployApp(ctx, "app_1")
	if err != nil {
		t.Fatalf("deploy app: %v", err)
	}
	if deployment.ID != "dep_1" || deployment.Status != "running" || deployment.Stage != "running" {
		t.Fatalf("deployment = %#v", deployment)
	}

	deployments, err := service.ListDeployments(ctx, "app_1")
	if err != nil {
		t.Fatalf("list deployments: %v", err)
	}
	if len(deployments) != 1 || deployments[0].ImageTag != "porter/app_1:dep_1" {
		t.Fatalf("deployments = %#v", deployments)
	}
}

func setupAppForDeploymentServiceTest(t *testing.T, ctx context.Context) (*store.Queries, func()) {
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
