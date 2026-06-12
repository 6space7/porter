package deploy_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/deploy"
)

func TestPipelineRecordsSuccessfulStagesInOrder(t *testing.T) {
	store := &fakeDeploymentStore{}
	pipeline := deploy.Pipeline{
		Store: store,
		Cloner: deploy.ClonerFunc(func(_ context.Context, req deploy.CloneRequest) (string, error) {
			if req.GitURL != "https://github.com/example/web.git" || req.Branch != "main" {
				t.Fatalf("clone request = %#v", req)
			}
			return "cloned\n", nil
		}),
		Builder: deploy.BuilderFunc(func(_ context.Context, req deploy.BuildRequest) (deploy.BuildResult, error) {
			if req.AppID != "app_1" {
				t.Fatalf("build request = %#v", req)
			}
			return deploy.BuildResult{ImageTag: "porter/app_1:dep_1", Log: "built\n"}, nil
		}),
		Runner: deploy.RunnerFunc(func(_ context.Context, req deploy.RunRequest) (string, error) {
			if req.ImageTag != "porter/app_1:dep_1" {
				t.Fatalf("run request = %#v", req)
			}
			return "started\n", nil
		}),
	}

	result, err := pipeline.Run(context.Background(), deploy.Request{
		AppID:  "app_1",
		GitURL: "https://github.com/example/web.git",
		Branch: "main",
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	if result.ID != "dep_1" || result.Status != deploy.StatusRunning || result.Stage != deploy.StageRunning {
		t.Fatalf("result = %#v", result)
	}

	gotStages := store.stages()
	wantStages := []deploy.Stage{
		deploy.StageQueued,
		deploy.StageCloning,
		deploy.StageBuilding,
		deploy.StageStarting,
		deploy.StageRunning,
	}
	if !reflect.DeepEqual(gotStages, wantStages) {
		t.Fatalf("stages = %#v, want %#v", gotStages, wantStages)
	}
}

func TestPipelineFailureRecordsFailedStageAndRedactedLog(t *testing.T) {
	store := &fakeDeploymentStore{}
	pipeline := deploy.Pipeline{
		Store: store,
		Cloner: deploy.ClonerFunc(func(context.Context, deploy.CloneRequest) (string, error) {
			return "cloned\n", nil
		}),
		Builder: deploy.BuilderFunc(func(context.Context, deploy.BuildRequest) (deploy.BuildResult, error) {
			return deploy.BuildResult{Log: "using postgres://secret\n"}, errors.New("docker build failed")
		}),
		Runner: deploy.RunnerFunc(func(context.Context, deploy.RunRequest) (string, error) {
			t.Fatal("runner should not be called after build failure")
			return "", nil
		}),
	}

	result, err := pipeline.Run(context.Background(), deploy.Request{
		AppID:   "app_1",
		GitURL:  "https://github.com/example/web.git",
		Branch:  "main",
		Secrets: []string{"postgres://secret"},
	})
	if err == nil {
		t.Fatal("expected pipeline error")
	}
	if result.Status != deploy.StatusFailed || result.Stage != deploy.StageBuilding {
		t.Fatalf("result = %#v", result)
	}

	last := store.records[len(store.records)-1]
	if last.Status != deploy.StatusFailed || last.Stage != deploy.StageBuilding {
		t.Fatalf("last record = %#v", last)
	}
	if strings.Contains(last.BuildLog, "postgres://secret") {
		t.Fatalf("secret leaked in build log: %s", last.BuildLog)
	}
	if !strings.Contains(last.BuildLog, "[REDACTED]") || !strings.Contains(last.BuildLog, "docker build failed") {
		t.Fatalf("build log missing redaction or error: %s", last.BuildLog)
	}
}

type fakeDeploymentStore struct {
	records []deploy.DeploymentRecord
}

func (store *fakeDeploymentStore) CreateDeployment(_ context.Context, appID string) (deploy.DeploymentRecord, error) {
	record := deploy.DeploymentRecord{
		ID:     "dep_1",
		AppID:  appID,
		Status: deploy.StatusRunning,
		Stage:  deploy.StageQueued,
	}
	store.records = append(store.records, record)
	return record, nil
}

func (store *fakeDeploymentStore) UpdateDeployment(_ context.Context, record deploy.DeploymentRecord) error {
	store.records = append(store.records, record)
	return nil
}

func (store *fakeDeploymentStore) stages() []deploy.Stage {
	stages := make([]deploy.Stage, 0, len(store.records))
	for _, record := range store.records {
		stages = append(stages, record.Stage)
	}
	return stages
}
