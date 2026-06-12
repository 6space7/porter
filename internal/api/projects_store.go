package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/6space7/porter/internal/store"
)

type projectIDFunc func() string

type storeProjectService struct {
	queries *store.Queries
	newID   projectIDFunc
}

func NewStoreProjectService(queries *store.Queries, newID projectIDFunc) ProjectService {
	if newID == nil {
		newID = func() string {
			return randomPrefixedID("proj")
		}
	}
	return storeProjectService{queries: queries, newID: newID}
}

func (service storeProjectService) CreateProject(ctx context.Context, name string) (ProjectResponse, error) {
	project, err := service.queries.CreateProject(ctx, store.CreateProjectParams{
		ID:   service.newID(),
		Name: name,
	})
	if err != nil {
		return ProjectResponse{}, err
	}
	return projectResponse(project), nil
}

func (service storeProjectService) ListProjects(ctx context.Context) ([]ProjectResponse, error) {
	projects, err := service.queries.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]ProjectResponse, 0, len(projects))
	for _, project := range projects {
		responses = append(responses, projectResponse(project))
	}
	return responses, nil
}

func projectResponse(project store.Project) ProjectResponse {
	return ProjectResponse{
		ID:   project.ID,
		Name: project.Name,
	}
}

func randomPrefixedID(prefix string) string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(fmt.Sprintf("generate %s id: %v", prefix, err))
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(raw[:])
}
