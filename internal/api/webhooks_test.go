package api_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/api"
)

const githubPushPayload = `{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/example/app.git"}}`

func TestGitHubWebhookRequiresSignature(t *testing.T) {
	apps := &fakeWebhookService{config: api.AppWebhookConfig{
		AppID:   "app_1",
		Branch:  "main",
		Secret:  "secret",
		Enabled: true,
	}}
	router := api.NewRouterWithDeps(api.Dependencies{
		Webhooks:    apps,
		Deployments: newFakeDeploymentService(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github/app_1", bytes.NewBufferString(githubPushPayload))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assertErrorCode(t, rr, http.StatusUnauthorized, "invalid_signature")
}

func TestGitHubWebhookRejectsWrongSignature(t *testing.T) {
	apps := &fakeWebhookService{config: api.AppWebhookConfig{
		AppID:   "app_1",
		Branch:  "main",
		Secret:  "secret",
		Enabled: true,
	}}
	router := api.NewRouterWithDeps(api.Dependencies{
		Webhooks:    apps,
		Deployments: newFakeDeploymentService(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github/app_1", bytes.NewBufferString(githubPushPayload))
	req.Header.Set("X-Hub-Signature-256", "sha256=bad")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assertErrorCode(t, rr, http.StatusUnauthorized, "invalid_signature")
}

func TestGitHubWebhookSkipsNonMatchingBranch(t *testing.T) {
	apps := &fakeWebhookService{config: api.AppWebhookConfig{
		AppID:   "app_1",
		Branch:  "release",
		Secret:  "secret",
		Enabled: true,
	}}
	deployments := newFakeDeploymentService()
	router := api.NewRouterWithDeps(api.Dependencies{
		Webhooks:    apps,
		Deployments: deployments,
	})

	req := signedWebhookRequest(githubPushPayload, "secret")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusAccepted, rr.Body.String())
	}
	var body struct {
		Accepted bool   `json:"accepted"`
		Skipped  bool   `json:"skipped"`
		Reason   string `json:"reason"`
		Branch   string `json:"branch"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Accepted || !body.Skipped || body.Reason != "branch_mismatch" || body.Branch != "main" {
		t.Fatalf("response = %#v", body)
	}
	if len(deployments.deployments) != 0 {
		t.Fatalf("deployments = %#v, want none", deployments.deployments)
	}
}

func TestGitHubWebhookDeploysMatchingBranch(t *testing.T) {
	apps := &fakeWebhookService{config: api.AppWebhookConfig{
		AppID:   "app_1",
		Branch:  "main",
		Secret:  "secret",
		Enabled: true,
	}}
	deployments := newFakeDeploymentService()
	router := api.NewRouterWithDeps(api.Dependencies{
		Webhooks:    apps,
		Deployments: deployments,
	})

	req := signedWebhookRequest(githubPushPayload, "secret")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusAccepted, rr.Body.String())
	}
	var body struct {
		Accepted   bool                    `json:"accepted"`
		Skipped    bool                    `json:"skipped"`
		Branch     string                  `json:"branch"`
		Deployment *api.DeploymentResponse `json:"deployment"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Accepted || body.Skipped || body.Branch != "main" || body.Deployment == nil || body.Deployment.ID != "dep_1" {
		t.Fatalf("response = %#v", body)
	}
	if len(deployments.deployments) != 1 || deployments.deployments[0].AppID != "app_1" {
		t.Fatalf("deployments = %#v", deployments.deployments)
	}
}

func signedWebhookRequest(payload, secret string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github/app_1", bytes.NewBufferString(payload))
	req.Header.Set("X-Hub-Signature-256", githubSignature(secret, []byte(payload)))
	return req
}

func githubSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func assertErrorCode(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rr.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, wantStatus, rr.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", body.Error.Code, wantCode)
	}
}

type fakeWebhookService struct {
	config api.AppWebhookConfig
}

func (service *fakeWebhookService) GetAppWebhook(_ context.Context, id string) (api.AppWebhookConfig, error) {
	if service.config.AppID == id {
		return service.config, nil
	}
	return api.AppWebhookConfig{}, api.ErrNotFound
}

func (service *fakeWebhookService) UpdateAppWebhook(_ context.Context, id string, input api.UpdateAppWebhookInput) (api.AppWebhookConfig, error) {
	if service.config.AppID != id {
		return api.AppWebhookConfig{}, api.ErrNotFound
	}
	service.config.Branch = input.Branch
	service.config.Enabled = input.Enabled
	if input.Enabled {
		service.config.Secret = "generated-secret"
	} else {
		service.config.Secret = ""
	}
	return service.config, nil
}
