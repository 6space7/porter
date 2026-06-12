package api

import (
	"context"
	"database/sql"
	"errors"

	"github.com/6space7/porter/internal/store"
)

type storeCaddyAskService struct {
	queries *store.Queries
}

func NewStoreCaddyAskService(queries *store.Queries) CaddyAskService {
	return storeCaddyAskService{queries: queries}
}

func (service storeCaddyAskService) IsDomainAllowed(ctx context.Context, hostname string) (bool, error) {
	domain, err := service.queries.GetDomainByHostname(ctx, hostname)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return domain.Verified == 1, nil
}
