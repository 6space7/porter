package api

import (
	"context"
	"strings"

	"github.com/6space7/porter/internal/store"
)

type storeCaddyAskService struct {
	queries *store.Queries
}

func NewStoreCaddyAskService(queries *store.Queries) CaddyAskService {
	return storeCaddyAskService{queries: queries}
}

func (service storeCaddyAskService) IsDomainAllowed(ctx context.Context, hostname string) (bool, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	routes, err := service.queries.ListVerifiedProxyRoutes(ctx)
	if err != nil {
		return false, err
	}
	for _, route := range routes {
		if strings.ToLower(strings.TrimSpace(route.Hostname)) == hostname {
			return true, nil
		}
	}
	return false, nil
}
