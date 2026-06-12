package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ProjectService interface {
	CreateProject(ctx context.Context, name string) (ProjectResponse, error)
	ListProjects(ctx context.Context) ([]ProjectResponse, error)
	GetProject(ctx context.Context, id string) (ProjectResponse, error)
	UpdateProject(ctx context.Context, id, name string) (ProjectResponse, error)
	DeleteProject(ctx context.Context, id string) error
}

type ProjectResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type projectHandler struct {
	projects ProjectService
}

func mountProjectRoutes(router chi.Router, projects ProjectService) {
	handler := projectHandler{projects: projects}
	router.With(RequireScope("projects:read")).Get("/projects", handler.list)
	router.With(RequireScope("projects:write")).Post("/projects", handler.create)
	router.With(RequireScope("projects:read")).Get("/projects/{projectID}", handler.get)
	router.With(RequireScope("projects:write")).Patch("/projects/{projectID}", handler.update)
	router.With(RequireScope("projects:write")).Delete("/projects/{projectID}", handler.delete)
}

func (handler projectHandler) list(w http.ResponseWriter, r *http.Request) {
	projects, err := handler.projects.ListProjects(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Projects could not be loaded.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (handler projectHandler) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with a name field.", nil)
		return
	}
	if err := ValidateProjectName(input.Name); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_project_name", "Project name is invalid.", "Use lowercase letters, numbers, and hyphens.", map[string]any{
			"field": "name",
		})
		return
	}

	project, err := handler.projects.CreateProject(r.Context(), input.Name)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "Project could not be created.", "Try again or check server logs.", nil)
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (handler projectHandler) get(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !validID(projectID) {
		WriteError(w, http.StatusBadRequest, "invalid_project_id", "Project id is invalid.", "Use a valid project id returned by the API.", map[string]any{"field": "project_id"})
		return
	}

	project, err := handler.projects.GetProject(r.Context(), projectID)
	if err != nil {
		writeProjectServiceError(w, err, "Project could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (handler projectHandler) update(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !validID(projectID) {
		WriteError(w, http.StatusBadRequest, "invalid_project_id", "Project id is invalid.", "Use a valid project id returned by the API.", map[string]any{"field": "project_id"})
		return
	}

	var input struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", "Send a JSON object with a name field.", nil)
		return
	}
	if err := ValidateProjectName(input.Name); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_project_name", "Project name is invalid.", "Use lowercase letters, numbers, and hyphens.", map[string]any{
			"field": "name",
		})
		return
	}

	project, err := handler.projects.UpdateProject(r.Context(), projectID, input.Name)
	if err != nil {
		writeProjectServiceError(w, err, "Project could not be updated.")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (handler projectHandler) delete(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !validID(projectID) {
		WriteError(w, http.StatusBadRequest, "invalid_project_id", "Project id is invalid.", "Use a valid project id returned by the API.", map[string]any{"field": "project_id"})
		return
	}

	if err := handler.projects.DeleteProject(r.Context(), projectID); err != nil {
		writeProjectServiceError(w, err, "Project could not be deleted.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeProjectServiceError(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, ErrNotFound) {
		WriteError(w, http.StatusNotFound, "not_found", "Project was not found.", "Use a project id returned by the API.", nil)
		return
	}
	WriteError(w, http.StatusInternalServerError, "internal_error", message, "Try again or check server logs.", nil)
}
