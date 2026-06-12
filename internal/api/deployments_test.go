package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/api"
)

func TestDeploymentRoutesRequireAuthAndDeployScope(t *testing.T) {
	deployments := newFakeDeploymentService()
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: deployTestVerifier(),
		Deployments:   deployments,
	})

	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/apps/app_1/deploy", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/apps/app_1/deploy", "Bearer read-token", http.StatusForbidden, "forbidden")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/app_1/deploy", nil)
	req.Header.Set("Authorization", "Bearer deploy-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("deploy status = %d, want %d; body=%s", rr.Code, http.StatusAccepted, rr.Body.String())
	}
	var deployment api.DeploymentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &deployment); err != nil {
		t.Fatalf("decode deployment: %v", err)
	}
	if deployment.ID != "dep_1" || deployment.Status != "running" || deployment.Stage != "queued" {
		t.Fatalf("deployment = %#v", deployment)
	}
}

func TestListDeploymentsRequiresReadScope(t *testing.T) {
	deployments := newFakeDeploymentService()
	deployments.deployments = []api.DeploymentResponse{
		{ID: "dep_1", AppID: "app_1", Status: "failed", Stage: "building", BuildLog: "docker build failed"},
	}
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: deployTestVerifier(),
		Deployments:   deployments,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/apps/app_1/deployments", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list deployments status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var listed []api.DeploymentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode deployments: %v", err)
	}
	if len(listed) != 1 || listed[0].Stage != "building" || listed[0].BuildLog == "" {
		t.Fatalf("deployments = %#v", listed)
	}
}

func deployTestVerifier() api.TokenVerifier {
	return api.TokenVerifierFunc(func(_ context.Context, token string) (api.Principal, error) {
		switch token {
		case "read-token":
			return api.Principal{TokenID: "tok_read", Scopes: []string{"apps:read"}}, nil
		case "deploy-token":
			return api.Principal{TokenID: "tok_deploy", Scopes: []string{"apps:read", "apps:deploy"}}, nil
		case "deploy-only-token":
			return api.Principal{TokenID: "tok_deploy_only", Scopes: []string{"apps:deploy"}}, nil
		default:
			return api.Principal{}, api.ErrInvalidToken
		}
	})
}

type fakeDeploymentService struct {
	deployments []api.DeploymentResponse
}

func newFakeDeploymentService() *fakeDeploymentService {
	return &fakeDeploymentService{}
}

func (svc *fakeDeploymentService) DeployApp(_ context.Context, appID string) (api.DeploymentResponse, error) {
	deployment := api.DeploymentResponse{ID: "dep_1", AppID: appID, Status: "running", Stage: "queued"}
	svc.deployments = append(svc.deployments, deployment)
	return deployment, nil
}

func (svc *fakeDeploymentService) ListDeployments(_ context.Context, _ string) ([]api.DeploymentResponse, error) {
	return append([]api.DeploymentResponse(nil), svc.deployments...), nil
}
