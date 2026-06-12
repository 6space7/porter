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
	assertStatusAndCode(t, router, http.MethodPatch, "/api/v1/apps/app_1", "Bearer read-token", http.StatusForbidden, "forbidden")
	assertStatusAndCode(t, router, http.MethodDelete, "/api/v1/apps/app_1", "Bearer read-token", http.StatusForbidden, "forbidden")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/apps/app_1/stop", "Bearer write-token", http.StatusForbidden, "forbidden")

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

func TestAppDetailUpdateDeleteAndLifecycleRoutes(t *testing.T) {
	apps := newFakeAppService()
	apps.apps = []api.AppResponse{{
		ID:           "app_1",
		ProjectID:    "proj_1",
		Name:         "web",
		GitURL:       "https://github.com/example/web.git",
		Branch:       "main",
		BuildType:    "dockerfile",
		InternalPort: 3000,
		Status:       "running",
	}}
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: appTestVerifier(),
		Apps:          apps,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/apps/app_1", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("get app status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var app api.AppResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &app); err != nil {
		t.Fatalf("decode app: %v", err)
	}
	if app.ID != "app_1" || app.Status != "running" {
		t.Fatalf("app = %#v", app)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/apps/app_1", bytes.NewBufferString(`{"branch":"release","internal_port":8080}`))
	req.Header.Set("Authorization", "Bearer write-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("update app status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &app); err != nil {
		t.Fatalf("decode updated app: %v", err)
	}
	if app.Branch != "release" || app.InternalPort != 8080 {
		t.Fatalf("updated app = %#v", app)
	}

	for _, tc := range []struct {
		path       string
		wantStatus string
	}{
		{path: "/api/v1/apps/app_1/stop", wantStatus: "stopped"},
		{path: "/api/v1/apps/app_1/start", wantStatus: "running"},
		{path: "/api/v1/apps/app_1/restart", wantStatus: "running"},
	} {
		req = httptest.NewRequest(http.MethodPost, tc.path, nil)
		req.Header.Set("Authorization", "Bearer deploy-token")
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d; body=%s", tc.path, rr.Code, http.StatusOK, rr.Body.String())
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &app); err != nil {
			t.Fatalf("decode lifecycle app: %v", err)
		}
		if app.Status != tc.wantStatus {
			t.Fatalf("%s app status = %q, want %q", tc.path, app.Status, tc.wantStatus)
		}
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/apps/app_1", nil)
	req.Header.Set("Authorization", "Bearer write-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete app status = %d, want %d; body=%s", rr.Code, http.StatusNoContent, rr.Body.String())
	}
	if len(apps.apps) != 0 {
		t.Fatalf("apps after delete = %#v", apps.apps)
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
		case "deploy-token":
			return api.Principal{TokenID: "tok_deploy", Scopes: []string{"apps:read", "apps:deploy"}}, nil
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

func (svc *fakeAppService) GetApp(_ context.Context, id string) (api.AppResponse, error) {
	for _, app := range svc.apps {
		if app.ID == id {
			return app, nil
		}
	}
	return api.AppResponse{}, api.ErrNotFound
}

func (svc *fakeAppService) UpdateApp(_ context.Context, id string, input api.UpdateAppInput) (api.AppResponse, error) {
	for i, app := range svc.apps {
		if app.ID == id {
			if input.Name != nil {
				app.Name = *input.Name
			}
			if input.GitURL != nil {
				app.GitURL = *input.GitURL
			}
			if input.Branch != nil {
				app.Branch = *input.Branch
			}
			if input.BuildType != nil {
				app.BuildType = *input.BuildType
			}
			if input.InternalPort != nil {
				app.InternalPort = *input.InternalPort
			}
			svc.apps[i] = app
			return app, nil
		}
	}
	return api.AppResponse{}, api.ErrNotFound
}

func (svc *fakeAppService) DeleteApp(_ context.Context, id string) error {
	for i, app := range svc.apps {
		if app.ID == id {
			svc.apps = append(svc.apps[:i], svc.apps[i+1:]...)
			return nil
		}
	}
	return api.ErrNotFound
}

func (svc *fakeAppService) StopApp(ctx context.Context, id string) (api.AppResponse, error) {
	return svc.setStatus(ctx, id, "stopped")
}

func (svc *fakeAppService) StartApp(ctx context.Context, id string) (api.AppResponse, error) {
	return svc.setStatus(ctx, id, "running")
}

func (svc *fakeAppService) RestartApp(ctx context.Context, id string) (api.AppResponse, error) {
	return svc.setStatus(ctx, id, "running")
}

func (svc *fakeAppService) setStatus(_ context.Context, id, status string) (api.AppResponse, error) {
	for i, app := range svc.apps {
		if app.ID == id {
			svc.apps[i].Status = status
			return svc.apps[i], nil
		}
	}
	return api.AppResponse{}, api.ErrNotFound
}
