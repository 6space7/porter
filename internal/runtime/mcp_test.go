package runtime_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/runtime"
	mcpsdkserver "github.com/mark3labs/mcp-go/server"
)

func TestNewHandlerServesAuthenticatedMCP(t *testing.T) {
	ctx := context.Background()
	db, handler, err := runtime.NewHandlerWithOptions(ctx, config.Config{
		DatabasePath:  filepath.Join(t.TempDir(), "porter.db"),
		WorkspacePath: filepath.Join(t.TempDir(), "work"),
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

	token := storeTokenForRuntimeTest(t, ctx, db)
	createProjectForRuntimeTest(t, handler, token)

	body := `{
		"jsonrpc":"2.0",
		"id":1,
		"method":"initialize",
		"params":{
			"protocolVersion":"2025-11-25",
			"capabilities":{},
			"clientInfo":{"name":"porter-test","version":"1.0.0"}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("MCP initialize status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"name":"porter"`) {
		t.Fatalf("MCP initialize response = %s", rr.Body.String())
	}
	sessionID := rr.Header().Get(mcpsdkserver.HeaderKeySessionID)
	if sessionID == "" {
		t.Fatal("MCP initialize did not return session id")
	}

	body = `{
		"jsonrpc":"2.0",
		"id":2,
		"method":"tools/call",
		"params":{
			"name":"porter_list_projects",
			"arguments":{}
		}
	}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/mcp", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(mcpsdkserver.HeaderKeySessionID, sessionID)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("MCP tool status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"name":"demo"`) {
		t.Fatalf("MCP project tool did not use store-backed deps: %s", rr.Body.String())
	}

	body = `{
		"jsonrpc":"2.0",
		"id":3,
		"method":"tools/call",
		"params":{
			"name":"porter_deploy_service",
			"arguments":{
				"project_id":"proj_1",
				"template_slug":"postgres",
				"name":"db"
			}
		}
	}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/mcp", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(mcpsdkserver.HeaderKeySessionID, sessionID)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("MCP forbidden tool status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"isError":true`) || !strings.Contains(rr.Body.String(), `services:write`) {
		t.Fatalf("MCP service tool did not enforce scope: %s", rr.Body.String())
	}
}
