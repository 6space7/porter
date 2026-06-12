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

func TestEnvVarRoutesRequireAuthAndMaskSecrets(t *testing.T) {
	envVars := newFakeEnvVarService()
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: appTestVerifier(),
		EnvVars:       envVars,
	})

	assertStatusAndCode(t, router, http.MethodGet, "/api/v1/apps/app_1/env", "", http.StatusUnauthorized, "unauthorized")
	assertStatusAndCode(t, router, http.MethodPost, "/api/v1/apps/app_1/env", "Bearer read-token", http.StatusForbidden, "forbidden")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/app_1/env", bytes.NewBufferString(`{"key":"DATABASE_URL","value":"postgres://secret","is_secret":true}`))
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("set env status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var setResp api.EnvVarResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &setResp); err != nil {
		t.Fatalf("decode set env: %v", err)
	}
	if setResp.Value != "••••" || !setResp.IsSecret {
		t.Fatalf("set response = %#v", setResp)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/apps/app_1/env", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list env status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var listed []api.EnvVarResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode listed env: %v", err)
	}
	if len(listed) != 1 || listed[0].Value != "••••" {
		t.Fatalf("listed env = %#v", listed)
	}
}

func TestSetEnvVarValidatesKey(t *testing.T) {
	router := api.NewRouterWithDeps(api.Dependencies{
		TokenVerifier: appTestVerifier(),
		EnvVars:       newFakeEnvVarService(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/app_1/env", bytes.NewBufferString(`{"key":"bad-key","value":"x","is_secret":false}`))
	req.Header.Set("Authorization", "Bearer write-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Code != "invalid_env_key" {
		t.Fatalf("error code = %q", body.Error.Code)
	}
}

type fakeEnvVarService struct {
	values []api.EnvVar
}

func newFakeEnvVarService() *fakeEnvVarService {
	return &fakeEnvVarService{}
}

func (svc *fakeEnvVarService) SetEnvVar(_ context.Context, appID string, input api.SetEnvVarInput) (api.EnvVar, error) {
	envVar := api.EnvVar{
		AppID:    appID,
		Key:      input.Key,
		Value:    input.Value,
		IsSecret: input.IsSecret,
	}
	svc.values = append(svc.values, envVar)
	return envVar, nil
}

func (svc *fakeEnvVarService) ListEnvVars(_ context.Context, _ string) ([]api.EnvVar, error) {
	return append([]api.EnvVar(nil), svc.values...), nil
}
