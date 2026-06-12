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

type StoreAppServiceOptions struct {
	NewAppID     func() string
	NewDomainID  func() string
	PublicIP     string
	RouteUpdater RouteUpdater
}

type storeAppService struct {
	queries      *store.Queries
	newAppID     appIDFunc
	newDomainID  appIDFunc
	publicIP     string
	routeUpdater RouteUpdater
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
