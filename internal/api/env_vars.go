package api

import (
	"context"
	"encoding/json"
	"net/http"

	secretcrypto "github.com/6space7/porter/internal/crypto"
	"github.com/go-chi/chi/v5"
)

type EnvVarService interface {
	SetEnvVar(ctx context.Context, appID string, input SetEnvVarInput) (EnvVar, error)
	ListEnvVars(ctx context.Context, appID string) ([]EnvVar, error)
}

type SetEnvVarInput struct {
	Key      string
	Value    string
	IsSecret bool
}

type EnvVar struct {
	AppID    string
	Key      string
	Value    string
	IsSecret bool
}

type EnvVarResponse struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

type envVarHandler struct {
	envVars EnvVarService
}

func mountEnvVarRoutes(router chi.Router, envVars EnvVarService) {
	handler := envVarHandler{envVars: envVars}
	router.With(RequireScope("apps:read")).Get("/apps/{appID}/env", handler.list)
	router.With(RequireScope("apps:write")).Post("/apps/{appID}/env", handler.set)
}

func (handler envVarHandler) list(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	envVars, err := handler.envVars.ListEnvVars(r.Context(), appID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Environment variables could not be loaded.", "Try again or check server logs.", nil)
		return
	}

	response := make([]EnvVarResponse, 0, len(envVars))
	for _, envVar := range envVars {
		response = append(response, envVarResponse(envVar))
	}
	writeJSON(w, http.StatusOK, response)
}

func (handler envVarHandler) set(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	var input struct {
		Key      string `json:"key"`
		Value    string `json:"value"`
		IsSecret bool   `json:"is_secret"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with key, value, and is_secret fields.", nil)
		return
	}
	if err := ValidateEnvKey(input.Key); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_env_key", "Environment variable key is invalid.", "Use uppercase letters, numbers, and underscores.", map[string]any{"field": "key"})
		return
	}

	envVar, err := handler.envVars.SetEnvVar(r.Context(), appID, SetEnvVarInput{
		Key:      input.Key,
		Value:    input.Value,
		IsSecret: input.IsSecret,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Environment variable could not be saved.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, envVarResponse(envVar))
}

func envVarResponse(envVar EnvVar) EnvVarResponse {
	value := envVar.Value
	if envVar.IsSecret {
		value = secretcrypto.MaskSecret()
	}
	return EnvVarResponse{
		Key:      envVar.Key,
		Value:    value,
		IsSecret: envVar.IsSecret,
	}
}
