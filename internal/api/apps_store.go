package api

import (
	"context"

	"github.com/6space7/porter/internal/store"
)

type appIDFunc func() string

type storeAppService struct {
	queries *store.Queries
	newID   appIDFunc
}

func NewStoreAppService(queries *store.Queries, newID appIDFunc) AppService {
	if newID == nil {
		newID = func() string {
			return randomPrefixedID("app")
		}
	}
	return storeAppService{queries: queries, newID: newID}
}

func (service storeAppService) CreateApp(ctx context.Context, input CreateAppInput) (AppResponse, error) {
	app, err := service.queries.CreateApp(ctx, store.CreateAppParams{
		ID:           service.newID(),
		ProjectID:    input.ProjectID,
		ServerID:     "local",
		Name:         input.Name,
		GitUrl:       input.GitURL,
		Branch:       input.Branch,
		BuildType:    input.BuildType,
		InternalPort: input.InternalPort,
		Status:       "created",
	})
	if err != nil {
		return AppResponse{}, err
	}
	return appResponse(app), nil
}

func (service storeAppService) ListApps(ctx context.Context) ([]AppResponse, error) {
	apps, err := service.queries.ListApps(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]AppResponse, 0, len(apps))
	for _, app := range apps {
		responses = append(responses, appResponse(app))
	}
	return responses, nil
}

func appResponse(app store.App) AppResponse {
	return AppResponse{
		ID:           app.ID,
		ProjectID:    app.ProjectID,
		Name:         app.Name,
		GitURL:       app.GitUrl,
		Branch:       app.Branch,
		BuildType:    app.BuildType,
		InternalPort: app.InternalPort,
		Status:       app.Status,
	}
}
