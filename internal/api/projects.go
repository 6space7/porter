package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ProjectService interface {
	CreateProject(ctx context.Context, name string) (ProjectResponse, error)
	ListProjects(ctx context.Context) ([]ProjectResponse, error)
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
