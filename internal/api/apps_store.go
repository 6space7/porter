package api

import (
	"context"

	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/store"
)

type appIDFunc func() string

type RouteUpdater interface {
	Reconcile(ctx context.Context) error
}

type AppRuntime interface {
	StartApp(ctx context.Context, appID string) error
	StopApp(ctx context.Context, appID string) error
	RemoveApp(ctx context.Context, appID string) error
}

type StoreAppServiceOptions struct {
	NewAppID     func() string
	NewDomainID  func() string
	PublicIP     string
	RouteUpdater RouteUpdater
	Runtime      AppRuntime
}

type storeAppService struct {
	queries      *store.Queries
	newAppID     appIDFunc
	newDomainID  appIDFunc
	publicIP     string
	routeUpdater RouteUpdater
	runtime      AppRuntime
}

func NewStoreAppService(queries *store.Queries, newID appIDFunc) AppService {
	return NewStoreAppServiceWithOptions(queries, StoreAppServiceOptions{NewAppID: newID})
}

func NewStoreAppServiceWithOptions(queries *store.Queries, opts StoreAppServiceOptions) AppService {
	if opts.NewAppID == nil {
		opts.NewAppID = func() string {
			return randomPrefixedID("app")
		}
	}
	if opts.NewDomainID == nil {
		opts.NewDomainID = func() string {
			return randomPrefixedID("dom")
		}
	}
	return storeAppService{
		queries:      queries,
		newAppID:     opts.NewAppID,
		newDomainID:  opts.NewDomainID,
		publicIP:     opts.PublicIP,
		routeUpdater: opts.RouteUpdater,
		runtime:      opts.Runtime,
	}
}

func (service storeAppService) CreateApp(ctx context.Context, input CreateAppInput) (AppResponse, error) {
	app, err := service.queries.CreateApp(ctx, store.CreateAppParams{
		ID:           service.newAppID(),
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
	if service.publicIP != "" {
		hostname, err := proxy.GenerateSSLIPDomain(input.Name, service.publicIP)
		if err != nil {
			return AppResponse{}, err
		}
		if _, err := service.queries.CreateDomain(ctx, store.CreateDomainParams{
			ID:       service.newDomainID(),
			AppID:    app.ID,
			Hostname: hostname,
			Type:     "generated",
			Verified: 1,
		}); err != nil {
			return AppResponse{}, err
		}
		if service.routeUpdater != nil {
			if err := service.routeUpdater.Reconcile(ctx); err != nil {
				return AppResponse{}, err
			}
		}
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

func (service storeAppService) GetApp(ctx context.Context, id string) (AppResponse, error) {
	app, err := service.queries.GetApp(ctx, id)
	if err != nil {
		return AppResponse{}, mapStoreNotFound(err)
	}
	return appResponse(app), nil
}

func (service storeAppService) UpdateApp(ctx context.Context, id string, input UpdateAppInput) (AppResponse, error) {
	current, err := service.queries.GetApp(ctx, id)
	if err != nil {
		return AppResponse{}, mapStoreNotFound(err)
	}

	name := current.Name
	if input.Name != nil {
		name = *input.Name
	}
	gitURL := current.GitUrl
	if input.GitURL != nil {
		gitURL = *input.GitURL
	}
	branch := current.Branch
	if input.Branch != nil {
		branch = *input.Branch
	}
	buildType := current.BuildType
	if input.BuildType != nil {
		buildType = *input.BuildType
	}
	internalPort := current.InternalPort
	if input.InternalPort != nil {
		internalPort = *input.InternalPort
	}

	updated, err := service.queries.UpdateApp(ctx, store.UpdateAppParams{
		Name:         name,
		GitUrl:       gitURL,
		Branch:       branch,
		BuildType:    buildType,
		InternalPort: internalPort,
		ID:           id,
	})
	if err != nil {
		return AppResponse{}, mapStoreNotFound(err)
	}
	if service.routeUpdater != nil {
		if err := service.routeUpdater.Reconcile(ctx); err != nil {
			return AppResponse{}, err
		}
	}
	return appResponse(updated), nil
}

func (service storeAppService) DeleteApp(ctx context.Context, id string) error {
	if _, err := service.queries.GetApp(ctx, id); err != nil {
		return mapStoreNotFound(err)
	}
	if service.runtime != nil {
		if err := service.runtime.RemoveApp(ctx, id); err != nil {
			return err
		}
	}
	if err := service.queries.DeleteApp(ctx, id); err != nil {
		return err
	}
	if service.routeUpdater != nil {
		return service.routeUpdater.Reconcile(ctx)
	}
	return nil
}

func (service storeAppService) StopApp(ctx context.Context, id string) (AppResponse, error) {
	if _, err := service.queries.GetApp(ctx, id); err != nil {
		return AppResponse{}, mapStoreNotFound(err)
	}
	if service.runtime != nil {
		if err := service.runtime.StopApp(ctx, id); err != nil {
			return AppResponse{}, err
		}
	}
	return service.setStatus(ctx, id, "stopped")
}

func (service storeAppService) StartApp(ctx context.Context, id string) (AppResponse, error) {
	if _, err := service.queries.GetApp(ctx, id); err != nil {
		return AppResponse{}, mapStoreNotFound(err)
	}
	if service.runtime != nil {
		if err := service.runtime.StartApp(ctx, id); err != nil {
			return AppResponse{}, err
		}
	}
	return service.setStatus(ctx, id, "running")
}

func (service storeAppService) RestartApp(ctx context.Context, id string) (AppResponse, error) {
	if _, err := service.queries.GetApp(ctx, id); err != nil {
		return AppResponse{}, mapStoreNotFound(err)
	}
	if service.runtime != nil {
		if err := service.runtime.StopApp(ctx, id); err != nil {
			return AppResponse{}, err
		}
		if err := service.runtime.StartApp(ctx, id); err != nil {
			return AppResponse{}, err
		}
	}
	return service.setStatus(ctx, id, "running")
}

func (service storeAppService) setStatus(ctx context.Context, id, status string) (AppResponse, error) {
	if err := service.queries.UpdateAppStatus(ctx, store.UpdateAppStatusParams{
		Status: status,
		ID:     id,
	}); err != nil {
		return AppResponse{}, err
	}
	return service.GetApp(ctx, id)
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
