package api

import (
	"context"
	"fmt"
	"net"

	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/store"
)

type StoreDomainServiceOptions struct {
	Resolver     proxy.Resolver
	ServerIP     string
	NewDomainID  func() string
	RouteUpdater RouteUpdater
}

type storeDomainService struct {
	queries      *store.Queries
	resolver     proxy.Resolver
	serverIP     string
	newDomainID  func() string
	routeUpdater RouteUpdater
}

func NewStoreDomainService(queries *store.Queries, opts StoreDomainServiceOptions) DomainService {
	resolver := opts.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	newDomainID := opts.NewDomainID
	if newDomainID == nil {
		newDomainID = func() string {
			return randomPrefixedID("dom")
		}
	}
	return storeDomainService{
		queries:      queries,
		resolver:     resolver,
		serverIP:     opts.ServerIP,
		newDomainID:  newDomainID,
		routeUpdater: opts.RouteUpdater,
	}
}

func (service storeDomainService) AddCustomDomain(ctx context.Context, appID, hostname string) (DomainResponse, error) {
	if service.serverIP == "" {
		return DomainResponse{}, fmt.Errorf("server public IP is required for domain preflight")
	}
	if err := proxy.PreflightCustomDomain(ctx, service.resolver, hostname, service.serverIP); err != nil {
		return DomainResponse{}, err
	}

	domain, err := service.queries.CreateDomain(ctx, store.CreateDomainParams{
		ID:       service.newDomainID(),
		AppID:    appID,
		Hostname: hostname,
		Type:     "custom",
		Verified: 1,
	})
	if err != nil {
		return DomainResponse{}, err
	}
	if service.routeUpdater != nil {
		if err := service.routeUpdater.Reconcile(ctx); err != nil {
			return DomainResponse{}, err
		}
	}
	return domainResponse(domain), nil
}

func (service storeDomainService) ListDomains(ctx context.Context, appID string) ([]DomainResponse, error) {
	domains, err := service.queries.ListDomainsByApp(ctx, appID)
	if err != nil {
		return nil, err
	}

	responses := make([]DomainResponse, 0, len(domains))
	for _, domain := range domains {
		responses = append(responses, domainResponse(domain))
	}
	return responses, nil
}

func (service storeDomainService) DeleteDomain(ctx context.Context, appID, domainID string) error {
	domain, err := service.queries.GetDomain(ctx, domainID)
	if err != nil {
		return mapStoreNotFound(err)
	}
	if domain.AppID != appID {
		return ErrNotFound
	}
	if err := service.queries.DeleteDomain(ctx, domainID); err != nil {
		return err
	}
	if service.routeUpdater != nil {
		return service.routeUpdater.Reconcile(ctx)
	}
	return nil
}

func (service storeDomainService) VerifyDomain(ctx context.Context, appID, domainID string) (DomainResponse, error) {
	domain, err := service.queries.GetDomain(ctx, domainID)
	if err != nil {
		return DomainResponse{}, mapStoreNotFound(err)
	}
	if domain.AppID != appID {
		return DomainResponse{}, ErrNotFound
	}
	if service.serverIP == "" {
		return DomainResponse{}, fmt.Errorf("server public IP is required for domain preflight")
	}
	if err := proxy.PreflightCustomDomain(ctx, service.resolver, domain.Hostname, service.serverIP); err != nil {
		return DomainResponse{}, err
	}

	updated, err := service.queries.UpdateDomainVerified(ctx, store.UpdateDomainVerifiedParams{
		Verified: 1,
		ID:       domainID,
	})
	if err != nil {
		return DomainResponse{}, mapStoreNotFound(err)
	}
	if service.routeUpdater != nil {
		if err := service.routeUpdater.Reconcile(ctx); err != nil {
			return DomainResponse{}, err
		}
	}
	return domainResponse(updated), nil
}

func domainResponse(domain store.Domain) DomainResponse {
	return DomainResponse{
		ID:       domain.ID,
		AppID:    domain.AppID,
		Hostname: domain.Hostname,
		Type:     domain.Type,
		Verified: domain.Verified == 1,
	}
}
