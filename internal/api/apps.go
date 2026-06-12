package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/6space7/porter/internal/deploy"
	"github.com/go-chi/chi/v5"
)

type AppService interface {
	CreateApp(ctx context.Context, input CreateAppInput) (AppResponse, error)
	ListApps(ctx context.Context) ([]AppResponse, error)
	GetApp(ctx context.Context, id string) (AppResponse, error)
	UpdateApp(ctx context.Context, id string, input UpdateAppInput) (AppResponse, error)
	DeleteApp(ctx context.Context, id string) error
	StopApp(ctx context.Context, id string) (AppResponse, error)
	StartApp(ctx context.Context, id string) (AppResponse, error)
	RestartApp(ctx context.Context, id string) (AppResponse, error)
}

type AppWebhookService interface {
	GetAppWebhook(ctx context.Context, id string) (AppWebhookConfig, error)
	UpdateAppWebhook(ctx context.Context, id string, input UpdateAppWebhookInput) (AppWebhookConfig, error)
}

type CreateAppInput struct {
	ProjectID    string
	Name         string
	GitURL       string
	Branch       string
	BuildType    string
	InternalPort int64
}

type UpdateAppInput struct {
	Name         *string
	GitURL       *string
	Branch       *string
	BuildType    *string
	InternalPort *int64
}

type AppResponse struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	Name             string `json:"name"`
	GitURL           string `json:"git_url"`
	Branch           string `json:"branch"`
	BuildType        string `json:"build_type"`
	InternalPort     int64  `json:"internal_port"`
	Status           string `json:"status"`
	AutoDeployBranch string `json:"auto_deploy_branch"`
}

type UpdateAppWebhookInput struct {
	Branch  string
	Enabled bool
}

type AppWebhookConfig struct {
	AppID   string
	Branch  string
	Secret  string
	Enabled bool
}

type AppWebhookResponse struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret,omitempty"`
	Branch     string `json:"branch"`
	Enabled    bool   `json:"enabled"`
}

type appHandler struct {
	apps     AppService
	webhooks AppWebhookService
}

func mountAppRoutes(router chi.Router, apps AppService, webhooks AppWebhookService) {
	handler := appHandler{apps: apps, webhooks: webhooks}
	router.With(RequireScope("apps:read")).Get("/apps", handler.list)
	router.With(RequireScope("apps:write")).Post("/apps", handler.create)
	router.With(RequireScope("apps:read")).Get("/apps/{appID}", handler.get)
	router.With(RequireScope("apps:write")).Patch("/apps/{appID}", handler.update)
	router.With(RequireScope("apps:write")).Delete("/apps/{appID}", handler.delete)
	if webhooks != nil {
		router.With(RequireScope("apps:write")).Put("/apps/{appID}/webhook", handler.updateWebhook)
	}
	router.With(RequireScope("apps:deploy")).Post("/apps/{appID}/stop", handler.stop)
	router.With(RequireScope("apps:deploy")).Post("/apps/{appID}/start", handler.start)
	router.With(RequireScope("apps:deploy")).Post("/apps/{appID}/restart", handler.restart)
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

func (handler appHandler) get(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	app, err := handler.apps.GetApp(r.Context(), appID)
	if err != nil {
		writeAppServiceError(w, err, "App could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (handler appHandler) update(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	var input struct {
		Name         *string `json:"name"`
		GitURL       *string `json:"git_url"`
		Branch       *string `json:"branch"`
		BuildType    *string `json:"build_type"`
		InternalPort *int64  `json:"internal_port"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with app settings.", nil)
		return
	}

	appInput := UpdateAppInput{
		Name:         input.Name,
		GitURL:       input.GitURL,
		Branch:       input.Branch,
		BuildType:    input.BuildType,
		InternalPort: input.InternalPort,
	}
	if err := validateUpdateAppInput(appInput); err != nil {
		writeAppValidationError(w, err)
		return
	}

	app, err := handler.apps.UpdateApp(r.Context(), appID, appInput)
	if err != nil {
		writeAppServiceError(w, err, "App could not be updated.")
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (handler appHandler) delete(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	if err := handler.apps.DeleteApp(r.Context(), appID); err != nil {
		writeAppServiceError(w, err, "App could not be deleted.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler appHandler) updateWebhook(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	var input struct {
		Branch  string `json:"branch"`
		Enabled bool   `json:"enabled"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with webhook settings.", nil)
		return
	}
	branch := defaultString(input.Branch, "")
	if input.Enabled && branch != "" {
		if err := ValidateBranchName(branch); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_branch", "Branch name is invalid.", "Use a simple Git branch name without spaces, flags, or traversal.", map[string]any{"field": "branch"})
			return
		}
	}

	config, err := handler.webhooks.UpdateAppWebhook(r.Context(), appID, UpdateAppWebhookInput{
		Branch:  branch,
		Enabled: input.Enabled,
	})
	if err != nil {
		writeAppServiceError(w, err, "Webhook settings could not be updated.")
		return
	}
	writeJSON(w, http.StatusOK, AppWebhookResponse{
		WebhookURL: webhookURL(r, appID),
		Secret:     config.Secret,
		Branch:     config.Branch,
		Enabled:    config.Enabled,
	})
}

func (handler appHandler) stop(w http.ResponseWriter, r *http.Request) {
	handler.lifecycle(w, r, handler.apps.StopApp)
}

func (handler appHandler) start(w http.ResponseWriter, r *http.Request) {
	handler.lifecycle(w, r, handler.apps.StartApp)
}

func (handler appHandler) restart(w http.ResponseWriter, r *http.Request) {
	handler.lifecycle(w, r, handler.apps.RestartApp)
}

func (handler appHandler) lifecycle(w http.ResponseWriter, r *http.Request, action func(context.Context, string) (AppResponse, error)) {
	appID := chi.URLParam(r, "appID")
	if !validID(appID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}

	app, err := action(r.Context(), appID)
	if err != nil {
		writeAppServiceError(w, err, "App action could not be completed.")
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func validateUpdateAppInput(input UpdateAppInput) error {
	if input.Name != nil {
		if err := ValidateAppName(*input.Name); err != nil {
			return appValidationError{code: "invalid_app_name", message: "App name is invalid.", hint: "Use lowercase letters, numbers, and hyphens.", field: "name"}
		}
	}
	if input.GitURL != nil {
		if err := deploy.ValidateGitURL(*input.GitURL); err != nil {
			return appValidationError{code: "invalid_git_url", message: "Git URL is invalid.", hint: "Use an https or ssh Git URL that does not target local resources.", field: "git_url"}
		}
	}
	if input.Branch != nil {
		if err := ValidateBranchName(*input.Branch); err != nil {
			return appValidationError{code: "invalid_branch", message: "Branch name is invalid.", hint: "Use a simple Git branch name without spaces, flags, or traversal.", field: "branch"}
		}
	}
	if input.BuildType != nil {
		if err := ValidateBuildType(*input.BuildType); err != nil {
			return appValidationError{code: "invalid_build_type", message: "Build type is invalid.", hint: "Use dockerfile or nixpacks.", field: "build_type"}
		}
	}
	if input.InternalPort != nil {
		if *input.InternalPort < 1 || *input.InternalPort > 65535 {
			return appValidationError{code: "invalid_internal_port", message: "Internal port is invalid.", hint: "Use a TCP port between 1 and 65535.", field: "internal_port"}
		}
	}
	return nil
}

type appValidationError struct {
	code    string
	message string
	hint    string
	field   string
}

func (err appValidationError) Error() string {
	return err.code
}

func writeAppValidationError(w http.ResponseWriter, err error) {
	var validationErr appValidationError
	if errors.As(err, &validationErr) {
		WriteError(w, http.StatusBadRequest, validationErr.code, validationErr.message, validationErr.hint, map[string]any{"field": validationErr.field})
		return
	}
	WriteError(w, http.StatusBadRequest, "invalid_app", "App settings are invalid.", "Check the request fields and try again.", nil)
}

func writeAppServiceError(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, ErrNotFound) {
		WriteError(w, http.StatusNotFound, "not_found", "App was not found.", "Use an app id returned by the API.", nil)
		return
	}
	WriteError(w, http.StatusInternalServerError, "internal_error", message, "Try again or check server logs.", nil)
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
