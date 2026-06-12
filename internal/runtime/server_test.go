package runtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/auth"
	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/runtime"
	"github.com/6space7/porter/internal/store"
)

func TestNewHandlerWiresStoreBackedAuthAndProjects(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "porter.db")

	db, handler, err := runtime.NewHandler(ctx, config.Config{DatabasePath: dbPath})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	defer db.Close()

	plaintext, record, err := auth.NewToken("writer", []string{"projects:read", "projects:write", "apps:read", "apps:write"})
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	_, err = store.New(db.SQL()).CreateToken(ctx, store.CreateTokenParams{
		ID:     record.ID,
		Name:   record.Name,
		Hash:   record.Hash,
		Scopes: strings.Join(record.Scopes, ","),
	})
	if err != nil {
		t.Fatalf("store token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"name":"demo"}`))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project response: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/apps", bytes.NewBufferString(`{
		"project_id":"`+project.ID+`",
		"name":"web",
		"git_url":"https://github.com/example/web.git",
		"branch":"main",
		"build_type":"dockerfile",
		"internal_port":3000
	}`))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("app status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}
