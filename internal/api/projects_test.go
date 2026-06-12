package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/api"
)

func TestProjectRoutesRequireAuthAndScopes(t *testing.T) {
	projects := newFakeProjectService()
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: testVerifier(),
		Projects:      projects,
	})

	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/projects", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/projects", "Bearer read-token", http.StatusForbidden, "forbidden")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"name":"demo"}`))
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var created api.ProjectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created project: %v", err)
	}
	if created.ID != "proj_1" || created.Name != "demo" {
		t.Fatalf("created project = %#v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list projects status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var listed []api.ProjectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode listed projects: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "proj_1" {
		t.Fatalf("listed projects = %#v", listed)
	}
}

func TestCreateProjectValidatesName(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: testVerifier(),
		Projects:      newFakeProjectService(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"name":"Bad Name"}`))
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
			Hint string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Code != "invalid_project_name" || body.Error.Hint == "" {
		t.Fatalf("error = %#v", body.Error)
	}
}

func testVerifier() api.TokenVerifier {
	return api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		switch token {
		case "read-token":
			return api.Principal{TokenID: "tok_read", Scopes: []string{"projects:read"}}, nil
		case "write-token":
			return api.Principal{TokenID: "tok_write", Scopes: []string{"projects:read", "projects:write"}}, nil
		default:
			return api.Principal{}, api.ErrInvalidToken
		}
	})
}

type fakeProjectService struct {
	projects []api.ProjectResponse
}

func newFakeProjectService() *fakeProjectService {
	return &fakeProjectService{}
}

func (svc *fakeProjectService) CreateProject(_ context.Context, name string) (api.ProjectResponse, error) {
	project := api.ProjectResponse{ID: "proj_1", Name: name}
	svc.projects = append(svc.projects, project)
	return project, nil
}

func (svc *fakeProjectService) ListProjects(_ context.Context) ([]api.ProjectResponse, error) {
	return append([]api.ProjectResponse(nil), svc.projects...), nil
}
