package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/6space7/porter/internal/api"
	"nhooyr.io/websocket"
)

func TestBuildLogRouteReturnsStoredFailureLog(t *testing.T) {
	logs := &fakeLogService{
		buildLog: api.BuildLogResponse{
			DeploymentID: "dep_1",
			AppID:        "app_1",
			Status:       "failed",
			Stage:        "building",
			BuildLog:     "step 1\nerror: docker build failed\n",
		},
	}
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: deployTestVerifier(),
		Logs:          logs,
	})

	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/deployments/dep_1/build-log", "", http.StatusUnauthorized, "unauthorized")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/dep_1/build-log", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("build log status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"failed"`) || !strings.Contains(rr.Body.String(), `"stage":"building"`) {
		t.Fatalf("build log response missing status/stage: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "docker build failed") {
		t.Fatalf("build log response missing log: %s", rr.Body.String())
	}
	if logs.buildDeploymentID != "dep_1" {
		t.Fatalf("build log deployment id = %q", logs.buildDeploymentID)
	}
}

func TestRuntimeLogRouteRequiresReadScopeBeforeWebSocketUpgrade(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: deployTestVerifier(),
		Logs:          &fakeLogService{},
	})

	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/apps/app_1/logs", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/apps/app_1/logs", "Bearer deploy-only-token", http.StatusForbidden, "forbidden")
}

func TestRuntimeLogRouteStreamsLogsOverWebSocket(t *testing.T) {
	stream := newChunkThenBlockStream("runtime line\n")
	defer stream.Close()
	logs := &fakeLogService{runtimeStream: stream}
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: deployTestVerifier(),
		Logs:          logs,
	})
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/apps/app_1/logs", &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer read-token"}},
	})
	if err != nil {
		t.Fatalf("dial logs websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, message, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read websocket log message: %v", err)
	}
	if string(message) != "runtime line\n" {
		t.Fatalf("message = %q", message)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	if logs.runtimeAppID != "app_1" {
		t.Fatalf("runtime app id = %q", logs.runtimeAppID)
	}
}

type fakeLogService struct {
	buildDeploymentID string
	buildLog          api.BuildLogResponse
	runtimeAppID      string
	runtimeStream     io.ReadCloser
}

type chunkThenBlockStream struct {
	chunk []byte
	sent  bool
	done  chan struct{}
	once  sync.Once
}

func newChunkThenBlockStream(chunk string) *chunkThenBlockStream {
	return &chunkThenBlockStream{
		chunk: []byte(chunk),
		done:  make(chan struct{}),
	}
}

func (stream *chunkThenBlockStream) Read(p []byte) (int, error) {
	if !stream.sent {
		stream.sent = true
		return copy(p, stream.chunk), nil
	}
	<-stream.done
	return 0, io.EOF
}

func (stream *chunkThenBlockStream) Close() error {
	stream.once.Do(func() {
		close(stream.done)
	})
	return nil
}

func (svc *fakeLogService) GetBuildLog(_ context.Context, deploymentID string) (api.BuildLogResponse, error) {
	svc.buildDeploymentID = deploymentID
	return svc.buildLog, nil
}

func (svc *fakeLogService) StreamRuntimeLogs(_ context.Context, appID string) (io.ReadCloser, error) {
	svc.runtimeAppID = appID
	if svc.runtimeStream != nil {
		return svc.runtimeStream, nil
	}
	return io.NopCloser(strings.NewReader("")), nil
}
