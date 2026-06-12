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

func TestAppRoutesRequireAuthAndScopes(t *testing.T) {
	apps := newFakeAppService()
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: appTestVerifier(),
		Apps:          apps,
	})

	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/apps", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/apps", "Bearer read-token", http.StatusForbidden, "forbidden")

	body := `{
		"project_id":"proj_1",
		"name":"web",
		"git_url":"https://github.com/example/web.git",
		"branch":"main",
		"build_type":"dockerfile",
		"internal_port":3000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("create app status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var created api.AppResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created app: %v", err)
	}
	if created.ID != "app_1" || created.Name != "web" || created.InternalPort != 3000 {
		t.Fatalf("created app = %#v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/apps", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list apps status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var listed []api.AppResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode listed apps: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "app_1" {
		t.Fatalf("listed apps = %#v", listed)
	}
}

func TestCreateAppRejectsUnsafeGitURL(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: appTestVerifier(),
		Apps:          newFakeAppService(),
	})

	body := `{
		"project_id":"proj_1",
		"name":"web",
		"git_url":"file:///etc/passwd",
		"branch":"main",
		"build_type":"dockerfile",
		"internal_port":3000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	var bodyResp struct {
		Error struct {
			Code string `json:"code"`
			Hint string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &bodyResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if bodyResp.Error.Code != "invalid_git_url" || bodyResp.Error.Hint == "" {
		t.Fatalf("error = %#v", bodyResp.Error)
	}
}

func appTestVerifier() api.TokenVerifier {
	return api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		switch token {
		case "read-token":
			return api.Principal{TokenID: "tok_read", Scopes: []string{"apps:read"}}, nil
		case "write-token":
			return api.Principal{TokenID: "tok_write", Scopes: []string{"apps:read", "apps:write"}}, nil
		default:
			return api.Principal{}, api.ErrInvalidToken
		}
	})
}

type fakeAppService struct {
	apps []api.AppResponse
}

func newFakeAppService() *fakeAppService {
	return &fakeAppService{}
}

func (svc *fakeAppService) CreateApp(_ context.Context, input api.CreateAppInput) (api.AppResponse, error) {
	app := api.AppResponse{
		ID:           "app_1",
		ProjectID:    input.ProjectID,
		Name:         input.Name,
		GitURL:       input.GitURL,
		Branch:       input.Branch,
		BuildType:    input.BuildType,
		InternalPort: input.InternalPort,
		Status:       "created",
	}
	svc.apps = append(svc.apps, app)
	return app, nil
}

func (svc *fakeAppService) ListApps(_ context.Context) ([]api.AppResponse, error) {
	return append([]api.AppResponse(nil), svc.apps...), nil
}
