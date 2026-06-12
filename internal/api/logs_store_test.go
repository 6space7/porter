package api_test

import (
	"context"
	"database/sql"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/store"
)

func TestStoreLogServiceReturnsBuildLogWithDeploymentStatus(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: filepath.Join(t.TempDir(), "porter.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	seedAppForLogs(t, ctx, queries)
	if _, err := queries.CreateDeployment(ctx, store.CreateDeploymentParams{
		ID:       "dep_1",
		AppID:    "app_1",
		Status:   "failed",
		Stage:    "building",
		BuildLog: "error: docker build failed\n",
		ImageTag: sql.NullString{},
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	logs := api.NewStoreLogService(queries, &fakeRuntimeLogStreamer{})
	response, err := logs.GetBuildLog(ctx, "dep_1")
	if err != nil {
		t.Fatalf("get build log: %v", err)
	}

	if response.DeploymentID != "dep_1" || response.AppID != "app_1" {
		t.Fatalf("response ids = %#v", response)
	}
	if response.Status != "failed" || response.Stage != "building" {
		t.Fatalf("response status/stage = %#v", response)
	}
	if response.BuildLog != "error: docker build failed\n" {
		t.Fatalf("build log = %q", response.BuildLog)
	}
}

func TestStoreLogServiceStreamsRuntimeLogsForExistingApp(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{Path: filepath.Join(t.TempDir(), "porter.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	queries := store.New(db.SQL())
	seedAppForLogs(t, ctx, queries)
	streamer := &fakeRuntimeLogStreamer{stream: io.NopCloser(strings.NewReader("live\n"))}

	logs := api.NewStoreLogService(queries, streamer)
	stream, err := logs.StreamRuntimeLogs(ctx, "app_1")
	if err != nil {
		t.Fatalf("stream runtime logs: %v", err)
	}
	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if string(body) != "live\n" {
		t.Fatalf("runtime log body = %q", body)
	}
	if streamer.appID != "app_1" {
		t.Fatalf("streamed app id = %q", streamer.appID)
	}
}

func seedAppForLogs(t *testing.T, ctx context.Context, queries *store.Queries) {
	t.Helper()

	if _, err := queries.CreateProject(ctx, store.CreateProjectParams{ID: "proj_1", Name: "demo"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := queries.CreateApp(ctx, store.CreateAppParams{
		ID:           "app_1",
		ProjectID:    "proj_1",
		ServerID:     "local",
		Name:         "web",
		GitUrl:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
		Status:       "running",
	}); err != nil {
		t.Fatalf("create app: %v", err)
	}
}

type fakeRuntimeLogStreamer struct {
	appID  string
	stream io.ReadCloser
}

func (streamer *fakeRuntimeLogStreamer) StreamRuntimeLogs(_ context.Context, appID string) (io.ReadCloser, error) {
	streamer.appID = appID
	if streamer.stream != nil {
		return streamer.stream, nil
	}
	return io.NopCloser(strings.NewReader("")), nil
}
