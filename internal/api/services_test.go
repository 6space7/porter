package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/services"
)

func TestServiceRoutesListTemplatesDeployAndAttach(t *testing.T) {
	service := &fakeServiceManager{
		templates: []services.Template{{
			Slug:         "postgres",
			Name:         "PostgreSQL",
			Description:  "Relational database",
			Category:     "database",
			Image:        "postgres:16-alpine",
			InternalPort: 5432,
			Provides:     map[string]string{"DATABASE_URL": "postgres://internal"},
		}},
	}
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: serviceTestVerifier(),
		Services:      service,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/service-templates?search=post", nil)
	req.Header.Set("Authorization", "Bearer services-read-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list templates status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var templates []services.Template
	if err := json.Unmarshal(rr.Body.Bytes(), &templates); err != nil {
		t.Fatalf("decode templates: %v", err)
	}
	if len(templates) != 1 || templates[0].Slug != "postgres" {
		t.Fatalf("templates = %#v", templates)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewBufferString(`{
		"project_id":"proj_1",
		"template_slug":"postgres",
		"name":"db",
		"exposed":false
	}`))
	req.Header.Set("Authorization", "Bearer services-write-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create service status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	var created api.CreateServiceResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created service: %v", err)
	}
	if created.Service.ID != "svc_1" || created.Credentials["POSTGRES_PASSWORD"] == "" || created.Provides["DATABASE_URL"] == "" {
		t.Fatalf("created = %#v", created)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/services/svc_1/attach", bytes.NewBufferString(`{"app_id":"app_1"}`))
	req.Header.Set("Authorization", "Bearer services-write-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("attach service status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var attached api.AttachServiceResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &attached); err != nil {
		t.Fatalf("decode attached service: %v", err)
	}
	if attached.AppID != "app_1" || attached.Env["DATABASE_URL"] == "" {
		t.Fatalf("attached = %#v", attached)
	}
}

func TestServiceRoutesRequireScopes(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: serviceTestVerifier(),
		Services:      &fakeServiceManager{},
	})

	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/service-templates", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/service-templates", "Bearer apps-read-token", http.StatusForbidden, "forbidden")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/services", "Bearer services-read-token", http.StatusForbidden, "forbidden")
}

func serviceTestVerifier() api.TokenVerifier {
	return api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		switch token {
		case "services-read-token":
			return api.Principal{TokenID: "tok_read", Scopes: []string{"services:read"}}, nil
		case "services-write-token":
			return api.Principal{TokenID: "tok_write", Scopes: []string{"services:read", "services:write", "apps:write"}}, nil
		case "apps-read-token":
			return api.Principal{TokenID: "tok_apps_read", Scopes: []string{"apps:read"}}, nil
		default:
			return api.Principal{}, api.ErrInvalidToken
		}
	})
}

type fakeServiceManager struct {
	templates []services.Template
	services  []api.ServiceResponse
}

func (manager *fakeServiceManager) ListTemplates(_ context.Context, query string) ([]services.Template, error) {
	return append([]services.Template(nil), manager.templates...), nil
}

func (manager *fakeServiceManager) GetTemplate(_ context.Context, slug string) (services.Template, error) {
	for _, tmpl := range manager.templates {
		if tmpl.Slug == slug {
			return tmpl, nil
		}
	}
	return services.Template{}, api.ErrNotFound
}

func (manager *fakeServiceManager) CreateService(_ context.Context, input api.CreateServiceInput) (api.CreateServiceResponse, error) {
	response := api.ServiceResponse{
		ID:           "svc_1",
		ProjectID:    input.ProjectID,
		TemplateSlug: input.TemplateSlug,
		Name:         input.Name,
		Status:       "running",
		InternalPort: 5432,
	}
	manager.services = append(manager.services, response)
	return api.CreateServiceResponse{
		Service:     response,
		Credentials: map[string]string{"POSTGRES_PASSWORD": "generated"},
		Provides:    map[string]string{"DATABASE_URL": "postgres://internal"},
	}, nil
}

func (manager *fakeServiceManager) ListServices(context.Context) ([]api.ServiceResponse, error) {
	return append([]api.ServiceResponse(nil), manager.services...), nil
}

func (manager *fakeServiceManager) GetService(_ context.Context, id string) (api.ServiceResponse, error) {
	for _, service := range manager.services {
		if service.ID == id {
			return service, nil
		}
	}
	return api.ServiceResponse{}, api.ErrNotFound
}

func (manager *fakeServiceManager) AttachService(_ context.Context, serviceID, appID string) (api.AttachServiceResponse, error) {
	return api.AttachServiceResponse{
		ServiceID: serviceID,
		AppID:     appID,
		Env:       map[string]string{"DATABASE_URL": "postgres://internal"},
	}, nil
}
