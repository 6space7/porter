package api_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/store"
)

func TestStoreDeploymentServiceRunsPipelineAndListsDeployments(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDeploymentServiceTest(t, ctx)
	defer closeDB()
	envVars := newFakeEnvVarService()
	envVars.values = []api.EnvVar{
		{AppID: "app_1", Key: "DATABASE_URL", Value: "postgres://internal", IsSecret: true},
		{AppID: "app_1", Key: "PUBLIC_URL", Value: "https://web.example.com", IsSecret: false},
	}

	pipeline := deploy.Pipeline{
		Store: deploy.NewStoreDeploymentStore(queries, func() string {
			return "dep_1"
		}),
		Cloner: deploy.ClonerFunc(func(_ context.Context, req deploy.CloneRequest) (deploy.CloneResult, error) {
			if req.GitURL != "https://github.com/example/web.git" || req.Branch != "main" {
				t.Fatalf("clone request = %#v", req)
			}
			return deploy.CloneResult{SourceDir: "work/app_1/dep_1/source", Log: "cloned\n"}, nil
		}),
		Builder: deploy.BuilderFunc(func(context.Context, deploy.BuildRequest) (deploy.BuildResult, error) {
			return deploy.BuildResult{ImageTag: "porter/app_1:dep_1", Log: "built\n"}, nil
		}),
		Runner: deploy.RunnerFunc(func(_ context.Context, req deploy.RunRequest) (string, error) {
			if req.InternalPort != 3000 {
				t.Fatalf("internal port = %d, want 3000", req.InternalPort)
			}
			if req.Env["DATABASE_URL"] != "postgres://internal" || req.Env["PUBLIC_URL"] != "https://web.example.com" {
				t.Fatalf("env = %#v", req.Env)
			}
			return "started\n", nil
		}),
	}
	service := api.NewStoreDeploymentService(queries, pipeline, envVars)

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

func TestStoreDeploymentServicePersistsDetectedPortAndReconcilesRoutes(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDeploymentServiceTest(t, ctx)
	defer closeDB()
	routeUpdater := &fakeRouteUpdater{}

	pipeline := deploy.Pipeline{
		Store: deploy.NewStoreDeploymentStore(queries, func() string {
			return "dep_1"
		}),
		Cloner: deploy.ClonerFunc(func(context.Context, deploy.CloneRequest) (deploy.CloneResult, error) {
			return deploy.CloneResult{SourceDir: "work/app_1/dep_1/source"}, nil
		}),
		PortDetector: deploy.PortDetectorFunc(func(context.Context, string) (int64, bool, error) {
			return 8080, true, nil
		}),
		Builder: deploy.BuilderFunc(func(context.Context, deploy.BuildRequest) (deploy.BuildResult, error) {
			return deploy.BuildResult{ImageTag: "porter/app_1:dep_1"}, nil
		}),
		Runner: deploy.RunnerFunc(func(_ context.Context, req deploy.RunRequest) (string, error) {
			if req.InternalPort != 8080 {
				t.Fatalf("internal port = %d, want 8080", req.InternalPort)
			}
			return "started\n", nil
		}),
	}
	service := api.NewStoreDeploymentServiceWithOptions(queries, pipeline, nil, api.StoreDeploymentServiceOptions{
		RouteUpdater: routeUpdater,
	})

	if _, err := service.DeployApp(ctx, "app_1"); err != nil {
		t.Fatalf("deploy app: %v", err)
	}
	app, err := queries.GetApp(ctx, "app_1")
	if err != nil {
		t.Fatalf("get app: %v", err)
	}
	if app.InternalPort != 8080 {
		t.Fatalf("stored internal port = %d, want 8080", app.InternalPort)
	}
	if routeUpdater.calls != 1 {
		t.Fatalf("route updater calls = %d, want 1", routeUpdater.calls)
	}
}

func TestStoreDeploymentServiceRollsBackToSuccessfulDeployment(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDeploymentServiceTest(t, ctx)
	defer closeDB()
	envVars := newFakeEnvVarService()
	envVars.values = []api.EnvVar{
		{AppID: "app_1", Key: "DATABASE_URL", Value: "postgres://internal", IsSecret: true},
	}
	if _, err := queries.CreateDeployment(ctx, store.CreateDeploymentParams{
		ID:       "dep_previous",
		AppID:    "app_1",
		Status:   "running",
		Stage:    "running",
		BuildLog: "previous deploy\n",
		ImageTag: sql.NullString{String: "porter/app_1:dep_previous", Valid: true},
	}); err != nil {
		t.Fatalf("create previous deployment: %v", err)
	}

	pipeline := deploy.Pipeline{
		Store: deploy.NewStoreDeploymentStore(queries, func() string {
			return "dep_rollback"
		}),
		Runner: deploy.RunnerFunc(func(_ context.Context, req deploy.RunRequest) (string, error) {
			if req.ImageTag != "porter/app_1:dep_previous" || req.InternalPort != 3000 {
				t.Fatalf("rollback run request = %#v", req)
			}
			if req.Env["DATABASE_URL"] != "postgres://internal" {
				t.Fatalf("rollback env = %#v", req.Env)
			}
			return "started previous image\n", nil
		}),
	}
	service := api.NewStoreDeploymentService(queries, pipeline, envVars)

	deployment, err := service.RollbackApp(ctx, "app_1", "dep_previous")
	if err != nil {
		t.Fatalf("rollback app: %v", err)
	}
	if deployment.ID != "dep_rollback" || deployment.ImageTag != "porter/app_1:dep_previous" || deployment.Status != "running" {
		t.Fatalf("rollback deployment = %#v", deployment)
	}
	if deployment.BuildLog == "" {
		t.Fatalf("rollback deployment missing build log")
	}
}

func TestStoreDeploymentServiceRejectsInvalidRollbackTarget(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDeploymentServiceTest(t, ctx)
	defer closeDB()
	if _, err := queries.CreateDeployment(ctx, store.CreateDeploymentParams{
		ID:       "dep_failed",
		AppID:    "app_1",
		Status:   "failed",
		Stage:    "building",
		BuildLog: "failed deploy\n",
		ImageTag: sql.NullString{String: "porter/app_1:dep_failed", Valid: true},
	}); err != nil {
		t.Fatalf("create failed deployment: %v", err)
	}

	service := api.NewStoreDeploymentService(queries, deploy.Pipeline{}, nil)

	if _, err := service.RollbackApp(ctx, "app_1", "dep_failed"); err != api.ErrInvalidRollbackTarget {
		t.Fatalf("rollback error = %v, want invalid rollback target", err)
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

type fakeRouteUpdater struct {
	calls int
}

func (updater *fakeRouteUpdater) Reconcile(context.Context) error {
	updater.calls++
	return nil
}
