package runtime_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/runtime"
	"github.com/6space7/porter/internal/store"
	"nhooyr.io/websocket"
)

func TestNewHandlerWiresRuntimeLogStreaming(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "porter.db")
	runtimeLogs := &fakeRuntimeLogs{stream: io.NopCloser(strings.NewReader("live\n"))}

	db, handler, err := runtime.NewHandlerWithOptions(ctx, config.Config{
		DatabasePath:  dbPath,
		WorkspacePath: filepath.Join(t.TempDir(), "work"),
	}, runtime.Options{
		RuntimeLogs: runtimeLogs,
		Cloner: deploy.ClonerFunc(func(context.Context, deploy.CloneRequest) (deploy.CloneResult, error) {
			return deploy.CloneResult{}, nil
		}),
		Builder: deploy.BuilderFunc(func(context.Context, deploy.BuildRequest) (deploy.BuildResult, error) {
			return deploy.BuildResult{}, nil
		}),
		Runner: deploy.RunnerFunc(func(context.Context, deploy.RunRequest) (string, error) {
			return "", nil
		}),
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	defer db.Close()

	seedRuntimeLogApp(t, ctx, store.New(db.SQL()))
	token := seedRuntimeLogToken(t, ctx, store.New(db.SQL()))

	server := httptest.NewServer(handler)
	defer server.Close()

	wsCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(wsCtx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/apps/app_1/logs", &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}},
	})
	if err != nil {
		t.Fatalf("dial runtime logs: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, message, err := conn.Read(wsCtx)
	if err != nil {
		t.Fatalf("read runtime logs: %v", err)
	}
	if string(message) != "live\n" {
		t.Fatalf("message = %q", message)
	}
	if runtimeLogs.appID != "app_1" {
		t.Fatalf("runtime log app id = %q", runtimeLogs.appID)
	}
}

func seedRuntimeLogApp(t *testing.T, ctx context.Context, queries *store.Queries) {
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

func seedRuntimeLogToken(t *testing.T, ctx context.Context, queries *store.Queries) string {
	t.Helper()

	plaintext, record, err := auth.NewToken("reader", []string{"apps:read"})
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	if _, err := queries.CreateToken(ctx, store.CreateTokenParams{
		ID:     record.ID,
		Name:   record.Name,
		Hash:   record.Hash,
		Scopes: strings.Join(record.Scopes, ","),
	}); err != nil {
		t.Fatalf("store token: %v", err)
	}
	return plaintext
}

type fakeRuntimeLogs struct {
	appID  string
	stream io.ReadCloser
}

func (logs *fakeRuntimeLogs) StreamRuntimeLogs(_ context.Context, appID string) (io.ReadCloser, error) {
	logs.appID = appID
	return logs.stream, nil
}
