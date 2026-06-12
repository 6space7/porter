package proxy

import (
	"context"

	"github.com/6space7/porter/internal/docker"
	"github.com/6space7/porter/internal/store"
)

type storeRouteSource struct {
	queries *store.Queries
}

func NewStoreRouteSource(queries *store.Queries) RouteSource {
	return storeRouteSource{queries: queries}
}

func (source storeRouteSource) ListRoutes(ctx context.Context) ([]Route, error) {
	rows, err := source.queries.ListVerifiedProxyRoutes(ctx)
	if err != nil {
		return nil, err
	}

	routes := make([]Route, 0, len(rows))
	for _, row := range rows {
		containerName := docker.ContainerName(row.TargetID)
		if row.TargetType == "service" {
			containerName = docker.ServiceContainerName(row.TargetID)
		}
		routes = append(routes, Route{
			Hostname:      row.Hostname,
			ContainerName: containerName,
			InternalPort:  row.InternalPort,
		})
	}
	return routes, nil
}
