package runtime_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/runtime"
)

func TestNewHandlerServesMachineReadableDocs(t *testing.T) {
	ctx := context.Background()
	db, handler, err := runtime.NewHandlerWithOptions(ctx, config.Config{
		DatabasePath:   filepath.Join(t.TempDir(), "porter.db"),
		WorkspacePath:  filepath.Join(t.TempDir(), "work"),
		PlatformDomain: "porter.example.com",
	}, runtime.Options{
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

	req := httptest.NewRequest(http.MethodGet, "/llms.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("llms status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "https://porter.example.com/api/v1/mcp") {
		t.Fatalf("llms response = %s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("docs status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"mcp_endpoint":"https://porter.example.com/api/v1/mcp"`) {
		t.Fatalf("docs response = %s", rr.Body.String())
	}
}
