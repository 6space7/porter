package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/6space7/porter/internal/deploy"
	"github.com/go-chi/chi/v5"
)

type AppService interface {
	CreateApp(ctx context.Context, input CreateAppInput) (AppResponse, error)
	ListApps(ctx context.Context) ([]AppResponse, error)
}

type CreateAppInput struct {
	ProjectID    string
	Name         string
	GitURL       string
	Branch       string
	BuildType    string
	InternalPort int64
}

type AppResponse struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	Name         string `json:"name"`
	GitURL       string `json:"git_url"`
	Branch       string `json:"branch"`
	BuildType    string `json:"build_type"`
	InternalPort int64  `json:"internal_port"`
	Status       string `json:"status"`
}

type appHandler struct {
	apps AppService
}

func mountAppRoutes(router chi.Router, apps AppService) {
	handler := appHandler{apps: apps}
	router.With(RequireScope("apps:read")).Get("/apps", handler.list)
	router.With(RequireScope("apps:write")).Post("/apps", handler.create)
}

func (handler appHandler) list(w http.ResponseWriter, r *http.Request) {
	apps, err := handler.apps.ListApps(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Apps could not be loaded.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

func (handler appHandler) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectID    string `json:"project_id"`
		Name         string `json:"name"`
		GitURL       string `json:"git_url"`
		Branch       string `json:"branch"`
		BuildType    string `json:"build_type"`
		InternalPort int64  `json:"internal_port"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with app settings.", nil)
		return
	}

	appInput := CreateAppInput{
		ProjectID:    input.ProjectID,
		Name:         input.Name,
		GitURL:       input.GitURL,
		Branch:       defaultString(input.Branch, "main"),
		BuildType:    defaultString(input.BuildType, "dockerfile"),
		InternalPort: defaultPort(input.InternalPort),
	}
	if !validID(appInput.ProjectID) {
		WriteError(w, http.StatusBadRequest, "invalid_project_id", "Project id is invalid.", "Use a valid project id returned by the API.", map[string]any{"field": "project_id"})
		return
	}
	if err := ValidateAppName(appInput.Name); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_app_name", "App name is invalid.", "Use lowercase letters, numbers, and hyphens.", map[string]any{"field": "name"})
		return
	}
	if err := deploy.ValidateGitURL(appInput.GitURL); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_git_url", "Git URL is invalid.", "Use an https or ssh Git URL that does not target local resources.", map[string]any{"field": "git_url"})
		return
	}
	if err := ValidateBranchName(appInput.Branch); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_branch", "Branch name is invalid.", "Use a simple Git branch name without spaces, flags, or traversal.", map[string]any{"field": "branch"})
		return
	}
	if err := ValidateBuildType(appInput.BuildType); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_build_type", "Build type is invalid.", "Use dockerfile or nixpacks.", map[string]any{"field": "build_type"})
		return
	}
	if appInput.InternalPort < 1 || appInput.InternalPort > 65535 {
		WriteError(w, http.StatusBadRequest, "invalid_internal_port", "Internal port is invalid.", "Use a TCP port between 1 and 65535.", map[string]any{"field": "internal_port"})
		return
	}

	app, err := handler.apps.CreateApp(r.Context(), appInput)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "App could not be created.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultPort(value int64) int64 {
	if value == 0 {
		return 3000
	}
	return value
}

func validID(value string) bool {
	return len(value) >= 3 && len(value) <= 128
}
