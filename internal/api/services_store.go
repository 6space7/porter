package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	secretcrypto "github.com/6space7/porter/internal/crypto"
	dockerstage "github.com/6space7/porter/internal/docker"
	"github.com/6space7/porter/internal/proxy"
	"github.com/6space7/porter/internal/services"
	"github.com/6space7/porter/internal/store"
)

type ServiceRuntime interface {
	DeployService(ctx context.Context, req services.DeployRequest) (string, error)
}

type StoreServiceManagerOptions struct {
	Runtime         ServiceRuntime
	RouteUpdater    RouteUpdater
	PublicIP        string
	NewServiceID    func() string
	SecretGenerator services.SecretGenerator
}

type storeServiceManager struct {
	queries         *store.Queries
	catalog         services.Catalog
	envVars         EnvVarService
	box             *secretcrypto.SecretBox
	runtime         ServiceRuntime
	routeUpdater    RouteUpdater
	publicIP        string
	newServiceID    func() string
	secretGenerator services.SecretGenerator
}

type storedServiceOutputs struct {
	Credentials map[string]string `json:"credentials"`
	Provides    map[string]string `json:"provides"`
}

func NewStoreServiceManager(queries *store.Queries, catalog services.Catalog, envVars EnvVarService, box *secretcrypto.SecretBox) ServiceManager {
	return NewStoreServiceManagerWithOptions(queries, catalog, envVars, box, StoreServiceManagerOptions{})
}

func NewStoreServiceManagerWithOptions(queries *store.Queries, catalog services.Catalog, envVars EnvVarService, box *secretcrypto.SecretBox, opts StoreServiceManagerOptions) ServiceManager {
	newServiceID := opts.NewServiceID
	if newServiceID == nil {
		newServiceID = func() string {
			return randomPrefixedID("svc")
		}
	}
	secretGenerator := opts.SecretGenerator
	if secretGenerator == nil {
		secretGenerator = services.RandomSecretGenerator{}
	}
	return storeServiceManager{
		queries:         queries,
		catalog:         catalog,
		envVars:         envVars,
		box:             box,
		runtime:         opts.Runtime,
		routeUpdater:    opts.RouteUpdater,
		publicIP:        opts.PublicIP,
		newServiceID:    newServiceID,
		secretGenerator: secretGenerator,
	}
}

func (manager storeServiceManager) ListTemplates(ctx context.Context, query string) ([]services.Template, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return manager.catalog.Search(query), nil
}

func (manager storeServiceManager) GetTemplate(ctx context.Context, slug string) (services.Template, error) {
	if err := ctx.Err(); err != nil {
		return services.Template{}, err
	}
	tmpl, ok := manager.catalog.Get(slug)
	if !ok {
		return services.Template{}, ErrNotFound
	}
	return tmpl, nil
}

func (manager storeServiceManager) CreateService(ctx context.Context, input CreateServiceInput) (CreateServiceResponse, error) {
	tmpl, ok := manager.catalog.Get(input.TemplateSlug)
	if !ok {
		return CreateServiceResponse{}, ErrNotFound
	}

	serviceID := manager.newServiceID()
	containerName := dockerstage.ServiceContainerName(serviceID)
	exposed := input.Exposed || tmpl.Exposed
	hostname := ""
	if exposed {
		if manager.publicIP == "" {
			return CreateServiceResponse{}, fmt.Errorf("public ip is required for exposed services")
		}
		generated, err := proxy.GenerateSSLIPDomain(input.Name, manager.publicIP)
		if err != nil {
			return CreateServiceResponse{}, err
		}
		hostname = generated
	}

	rendered, err := services.RenderTemplate(tmpl, services.RenderInput{
		ServiceID:     serviceID,
		ContainerName: containerName,
		Hostname:      hostname,
	}, manager.secretGenerator)
	if err != nil {
		return CreateServiceResponse{}, err
	}
	if manager.runtime != nil {
		if _, err := manager.runtime.DeployService(ctx, services.DeployRequest{
			ServiceID:     serviceID,
			TemplateSlug:  tmpl.Slug,
			Image:         tmpl.Image,
			Command:       tmpl.Command,
			ContainerName: containerName,
			InternalPort:  tmpl.InternalPort,
			Env:           rendered.Env,
			Volumes:       tmpl.Volumes,
		}); err != nil {
			return CreateServiceResponse{}, err
		}
	}

	outputs := storedServiceOutputs{Credentials: rendered.Generated, Provides: rendered.Provides}
	encodedOutputs, err := manager.encryptOutputs(outputs)
	if err != nil {
		return CreateServiceResponse{}, err
	}
	row, err := manager.queries.CreateService(ctx, store.CreateServiceParams{
		ID:               serviceID,
		ProjectID:        input.ProjectID,
		ServerID:         "local",
		TemplateSlug:     tmpl.Slug,
		Name:             input.Name,
		Status:           "running",
		GeneratedSecrets: encodedOutputs,
		InternalPort:     tmpl.InternalPort,
		Exposed:          boolInt(exposed),
		Hostname:         sql.NullString{String: hostname, Valid: hostname != ""},
	})
	if err != nil {
		return CreateServiceResponse{}, err
	}
	if exposed && manager.routeUpdater != nil {
		if err := manager.routeUpdater.Reconcile(ctx); err != nil {
			return CreateServiceResponse{}, err
		}
	}

	return CreateServiceResponse{
		Service:     serviceResponseFromCreateRow(row),
		Credentials: outputs.Credentials,
		Provides:    outputs.Provides,
	}, nil
}

func (manager storeServiceManager) ListServices(ctx context.Context) ([]ServiceResponse, error) {
	rows, err := manager.queries.ListServices(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]ServiceResponse, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, serviceResponseFromListRow(row))
	}
	return responses, nil
}

func (manager storeServiceManager) GetService(ctx context.Context, id string) (ServiceResponse, error) {
	row, err := manager.queries.GetService(ctx, id)
	if err != nil {
		return ServiceResponse{}, mapStoreNotFound(err)
	}
	return serviceResponseFromGetRow(row), nil
}

func (manager storeServiceManager) AttachService(ctx context.Context, serviceID, appID string) (AttachServiceResponse, error) {
	row, err := manager.queries.GetService(ctx, serviceID)
	if err != nil {
		return AttachServiceResponse{}, mapStoreNotFound(err)
	}
	outputs, err := manager.decryptOutputs(row.GeneratedSecrets)
	if err != nil {
		return AttachServiceResponse{}, err
	}
	if manager.envVars == nil {
		return AttachServiceResponse{}, fmt.Errorf("env var service is required")
	}
	for key, value := range outputs.Provides {
		if _, err := manager.envVars.SetEnvVar(ctx, appID, SetEnvVarInput{
			Key:      key,
			Value:    value,
			IsSecret: true,
		}); err != nil {
			return AttachServiceResponse{}, err
		}
	}
	return AttachServiceResponse{
		ServiceID: serviceID,
		AppID:     appID,
		Env:       outputs.Provides,
	}, nil
}

func (manager storeServiceManager) encryptOutputs(outputs storedServiceOutputs) (string, error) {
	if manager.box == nil {
		return "", fmt.Errorf("secret box is required for generated service secrets")
	}
	body, err := json.Marshal(outputs)
	if err != nil {
		return "", err
	}
	return manager.box.Encrypt(string(body))
}

func (manager storeServiceManager) decryptOutputs(encoded string) (storedServiceOutputs, error) {
	if manager.box == nil {
		return storedServiceOutputs{}, fmt.Errorf("secret box is required for generated service secrets")
	}
	plaintext, err := manager.box.Decrypt(encoded)
	if err != nil {
		return storedServiceOutputs{}, err
	}
	var outputs storedServiceOutputs
	if err := json.Unmarshal([]byte(plaintext), &outputs); err != nil {
		return storedServiceOutputs{}, err
	}
	if outputs.Credentials == nil {
		outputs.Credentials = map[string]string{}
	}
	if outputs.Provides == nil {
		outputs.Provides = map[string]string{}
	}
	return outputs, nil
}

func serviceResponseFromCreateRow(row store.CreateServiceRow) ServiceResponse {
	return serviceResponse(row.ID, row.ProjectID, row.ServerID, row.TemplateSlug, row.Name, row.Status, row.InternalPort, row.Exposed, row.Hostname)
}

func serviceResponseFromListRow(row store.ListServicesRow) ServiceResponse {
	return serviceResponse(row.ID, row.ProjectID, row.ServerID, row.TemplateSlug, row.Name, row.Status, row.InternalPort, row.Exposed, row.Hostname)
}

func serviceResponseFromGetRow(row store.GetServiceRow) ServiceResponse {
	return serviceResponse(row.ID, row.ProjectID, row.ServerID, row.TemplateSlug, row.Name, row.Status, row.InternalPort, row.Exposed, row.Hostname)
}

func serviceResponse(id, projectID, serverID, templateSlug, name, status string, internalPort, exposed int64, hostname sql.NullString) ServiceResponse {
	response := ServiceResponse{
		ID:           id,
		ProjectID:    projectID,
		ServerID:     serverID,
		TemplateSlug: templateSlug,
		Name:         name,
		Status:       status,
		InternalPort: internalPort,
		Exposed:      exposed == 1,
	}
	if hostname.Valid {
		response.Hostname = hostname.String
	}
	return response
}
