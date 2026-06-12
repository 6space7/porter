package runtime_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/runtime"
)

func TestNewHandlerBootstrapsAdminTokenHash(t *testing.T) {
	ctx := context.Background()
	token := "ptr_bootstrap"

	db, handler, err := runtime.NewHandlerWithOptions(ctx, config.Config{
		DatabasePath:       filepath.Join(t.TempDir(), "porter.db"),
		WorkspacePath:      filepath.Join(t.TempDir(), "work"),
		BootstrapTokenHash: auth.HashToken(token),
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"name":"demo"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}
