package deploy_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/store"
)

func TestStoreDeploymentStoreCreatesAndUpdatesDeploymentRows(t *testing.T) {
	ctx := context.Background()
	queries, closeDB := setupAppForDeploymentTest(t, ctx)
	defer closeDB()

	deployments := deploy.NewStoreDeploymentStore(queries, func() string {
		return "dep_1"
	})

	record, err := deployments.CreateDeployment(ctx, "app_1")
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	if record.ID != "dep_1" || record.Stage != deploy.StageQueued || record.Status != deploy.StatusRunning {
		t.Fatalf("record = %#v", record)
	}

	record.Status = deploy.StatusFailed
	record.Stage = deploy.StageBuilding
	record.BuildLog = "docker build failed"
	record.ImageTag = "porter/app_1:dep_1"
	if err := deployments.UpdateDeployment(ctx, record); err != nil {
		t.Fatalf("update deployment: %v", err)
	}

	row, err := queries.GetDeployment(ctx, "dep_1")
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if row.Status != "failed" || row.Stage != "building" || row.BuildLog != "docker build failed" {
		t.Fatalf("row = %#v", row)
	}
	if !row.ImageTag.Valid || row.ImageTag.String != "porter/app_1:dep_1" {
		t.Fatalf("image tag = %#v", row.ImageTag)
	}
}

func setupAppForDeploymentTest(t *testing.T, ctx context.Context) (*store.Queries, func()) {
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
