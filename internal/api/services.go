package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/6space7/porter/internal/services"
	"github.com/go-chi/chi/v5"
)

type ServiceManager interface {
	ListTemplates(ctx context.Context, query string) ([]services.Template, error)
	GetTemplate(ctx context.Context, slug string) (services.Template, error)
	CreateService(ctx context.Context, input CreateServiceInput) (CreateServiceResponse, error)
	ListServices(ctx context.Context) ([]ServiceResponse, error)
	GetService(ctx context.Context, id string) (ServiceResponse, error)
	AttachService(ctx context.Context, serviceID, appID string) (AttachServiceResponse, error)
}

type CreateServiceInput struct {
	ProjectID    string
	TemplateSlug string
	Name         string
	Exposed      bool
}

type ServiceResponse struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	ServerID     string `json:"server_id"`
	TemplateSlug string `json:"template_slug"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	InternalPort int64  `json:"internal_port"`
	Exposed      bool   `json:"exposed"`
	Hostname     string `json:"hostname,omitempty"`
}

type CreateServiceResponse struct {
	Service     ServiceResponse   `json:"service"`
	Credentials map[string]string `json:"credentials"`
	Provides    map[string]string `json:"provides"`
}

type AttachServiceResponse struct {
	ServiceID string            `json:"service_id"`
	AppID     string            `json:"app_id"`
	Env       map[string]string `json:"env"`
}

type serviceHandler struct {
	services ServiceManager
}

func mountServiceRoutes(router chi.Router, manager ServiceManager) {
	handler := serviceHandler{services: manager}
	router.With(RequireScope("services:read")).Get("/service-templates", handler.listTemplates)
	router.With(RequireScope("services:read")).Get("/service-templates/{slug}", handler.getTemplate)
	router.With(RequireScope("services:read")).Get("/services", handler.listServices)
	router.With(RequireScope("services:write")).Post("/services", handler.createService)
	router.With(RequireScope("services:read")).Get("/services/{serviceID}", handler.getService)
	router.With(RequireScope("services:write")).Post("/services/{serviceID}/attach", handler.attachService)
}

func (handler serviceHandler) listTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := handler.services.ListTemplates(r.Context(), r.URL.Query().Get("search"))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Service templates could not be loaded.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (handler serviceHandler) getTemplate(w http.ResponseWriter, r *http.Request) {
	tmpl, err := handler.services.GetTemplate(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeServiceError(w, err, "Service template could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

func (handler serviceHandler) listServices(w http.ResponseWriter, r *http.Request) {
	services, err := handler.services.ListServices(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Services could not be loaded.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (handler serviceHandler) createService(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectID    string `json:"project_id"`
		TemplateSlug string `json:"template_slug"`
		Name         string `json:"name"`
		Exposed      bool   `json:"exposed"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with service settings.", nil)
		return
	}
	if !validID(input.ProjectID) {
		WriteError(w, http.StatusBadRequest, "invalid_project_id", "Project id is invalid.", "Use a valid project id returned by the API.", map[string]any{"field": "project_id"})
		return
	}
	if err := ValidateAppName(input.Name); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_service_name", "Service name is invalid.", "Use lowercase letters, numbers, and hyphens.", map[string]any{"field": "name"})
		return
	}
	if err := ValidateAppName(input.TemplateSlug); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_template_slug", "Template slug is invalid.", "Use a catalog template slug.", map[string]any{"field": "template_slug"})
		return
	}

	response, err := handler.services.CreateService(r.Context(), CreateServiceInput{
		ProjectID:    input.ProjectID,
		TemplateSlug: input.TemplateSlug,
		Name:         input.Name,
		Exposed:      input.Exposed,
	})
	if err != nil {
		writeServiceError(w, err, "Service could not be created.")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (handler serviceHandler) getService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if !validID(serviceID) {
		WriteError(w, http.StatusBadRequest, "invalid_service_id", "Service id is invalid.", "Use a valid service id returned by the API.", map[string]any{"field": "service_id"})
		return
	}
	service, err := handler.services.GetService(r.Context(), serviceID)
	if err != nil {
		writeServiceError(w, err, "Service could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, service)
}

func (handler serviceHandler) attachService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if !validID(serviceID) {
		WriteError(w, http.StatusBadRequest, "invalid_service_id", "Service id is invalid.", "Use a valid service id returned by the API.", map[string]any{"field": "service_id"})
		return
	}
	var input struct {
		AppID string `json:"app_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with app_id.", nil)
		return
	}
	if !validID(input.AppID) {
		WriteError(w, http.StatusBadRequest, "invalid_app_id", "App id is invalid.", "Use a valid app id returned by the API.", map[string]any{"field": "app_id"})
		return
	}
	response, err := handler.services.AttachService(r.Context(), serviceID, input.AppID)
	if err != nil {
		writeServiceError(w, err, "Service could not be attached.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func writeServiceError(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, ErrNotFound) {
		WriteError(w, http.StatusNotFound, "not_found", "Service was not found.", "Use an id or slug returned by the API.", nil)
		return
	}
	WriteError(w, http.StatusInternalServerError, "internal_error", message, "Try again or check server logs.", nil)
}
