package api

import (
	"context"
	"database/sql"
	"strings"

	"github.com/6space7/porter/internal/remote"
	"github.com/6space7/porter/internal/store"
)

type StoreServerServiceOptions struct {
	NewServerID func() string
	Validator   remote.Validator
	KeyStore    remote.KeyStore
}

type storeServerService struct {
	queries     *store.Queries
	newServerID func() string
	validator   remote.Validator
	keyStore    remote.KeyStore
}

func NewStoreServerService(queries *store.Queries, validator remote.Validator) ServerService {
	return NewStoreServerServiceWithOptions(queries, StoreServerServiceOptions{Validator: validator})
}

func NewStoreServerServiceWithOptions(queries *store.Queries, opts StoreServerServiceOptions) storeServerService {
	newServerID := opts.NewServerID
	if newServerID == nil {
		newServerID = func() string {
			return randomPrefixedID("srv")
		}
	}
	return storeServerService{
		queries:     queries,
		newServerID: newServerID,
		validator:   opts.Validator,
		keyStore:    opts.KeyStore,
	}
}

func (service storeServerService) ListServers(ctx context.Context) ([]ServerResponse, error) {
	rows, err := service.queries.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]ServerResponse, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, serverResponse(row, remote.CheckResult{}))
	}
	return responses, nil
}

func (service storeServerService) CreateServer(ctx context.Context, input CreateServerInput) (ServerResponse, error) {
	check := remote.CheckResult{}
	if service.validator != nil {
		var err error
		check, err = service.validator.Check(ctx, remote.CheckRequest{
			Host:          input.Host,
			User:          input.SSHUser,
			PrivateKeyPEM: input.PrivateKeyPEM,
		})
		if err != nil {
			return ServerResponse{}, err
		}
	}

	serverID := service.newServerID()
	keyRef := sql.NullString{}
	if service.keyStore != nil {
		ref, err := service.keyStore.Put(ctx, serverID, input.PrivateKeyPEM)
		if err != nil {
			return ServerResponse{}, err
		}
		keyRef = sql.NullString{String: ref, Valid: true}
	}

	row, err := service.queries.CreateServer(ctx, store.CreateServerParams{
		ID:        serverID,
		Name:      strings.TrimSpace(input.Name),
		Host:      strings.TrimSpace(input.Host),
		SshKeyRef: keyRef,
		Status:    "healthy",
	})
	if err != nil {
		return ServerResponse{}, err
	}
	return serverResponse(row, check), nil
}

func serverResponse(row store.Server, check remote.CheckResult) ServerResponse {
	return ServerResponse{
		ID:            row.ID,
		Name:          row.Name,
		Host:          row.Host,
		Status:        row.Status,
		DockerVersion: check.DockerVersion,
		OS:            check.OS,
	}
}
