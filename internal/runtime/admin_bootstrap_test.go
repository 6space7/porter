package runtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/runtime"
)

func TestNewHandlerBootstrapsAdminUserFromPasswordFile(t *testing.T) {
	ctx := context.Background()
	passwordPath := filepath.Join(t.TempDir(), "initial-password")
	if err := os.WriteFile(passwordPath, []byte("admin-secret\n"), 0600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	db, handler, err := runtime.NewHandlerWithOptions(ctx, config.Config{
		DatabasePath:               filepath.Join(t.TempDir(), "porter.db"),
		WorkspacePath:              filepath.Join(t.TempDir(), "work"),
		BootstrapAdminEmail:        "admin@example.com",
		BootstrapAdminPasswordFile: passwordPath,
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"admin-secret"}`))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Token == "" {
		t.Fatalf("login response missing token: %s", rr.Body.String())
	}
}
