package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type DeploymentService interface {
	DeployApp(ctx context.Context, appID string) (DeploymentResponse, error)
	ListDeployments(ctx context.Context, appID string) ([]DeploymentResponse, error)
}

type DeploymentResponse struct {
	ID       string `json:"id"`
	AppID    string `json:"app_id"`
	Status   string `json:"status"`
	Stage    string `json:"stage"`
	BuildLog string `json:"build_log"`
	ImageTag string `json:"image_tag"`
}

type deploymentHandler struct {
	deployments DeploymentService
}

func mountDeploymentRoutes(router chi.Router, deployments DeploymentService) {
	handler := deploymentHandler{deployments: deployments}
	router.With(RequireScope("apps:read")).Get("/apps/{appID}/deployments", handler.list)
	router.With(RequireScope("apps:deploy")).Post("/apps/{appID}/deploy", handler.deploy)
}

func (handler deploymentHandler) list(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	deployments, err := handler.deployments.ListDeployments(r.Context(), appID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Deployments could not be loaded.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, deployments)
}

func (handler deploymentHandler) deploy(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	deployment, err := handler.deployments.DeployApp(r.Context(), appID)
	if err != nil && deployment.ID == "" {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Deployment could not be started.", "Try again or check deployment logs.", nil)
		return
	}
	writeJSON(w, http.StatusAccepted, deployment)
}
