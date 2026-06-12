package api

import (
	"context"

	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/store"
)

type storeDeploymentService struct {
	queries      *store.Queries
	pipeline     deploy.Pipeline
	envVars      EnvVarService
	routeUpdater RouteUpdater
	imagePruner  ImagePruner
	retention    int
}

type StoreDeploymentServiceOptions struct {
	RouteUpdater   RouteUpdater
	ImagePruner    ImagePruner
	ImageRetention int
}

type ImagePruner interface {
	RemoveImage(ctx context.Context, imageTag string) error
}

func NewStoreDeploymentService(queries *store.Queries, pipeline deploy.Pipeline, envVars EnvVarService) DeploymentService {
	return NewStoreDeploymentServiceWithOptions(queries, pipeline, envVars, StoreDeploymentServiceOptions{})
}

func NewStoreDeploymentServiceWithOptions(queries *store.Queries, pipeline deploy.Pipeline, envVars EnvVarService, opts StoreDeploymentServiceOptions) DeploymentService {
	return storeDeploymentService{
		queries:      queries,
		pipeline:     pipeline,
		envVars:      envVars,
		routeUpdater: opts.RouteUpdater,
		imagePruner:  opts.ImagePruner,
		retention:    opts.ImageRetention,
	}
}

func (service storeDeploymentService) DeployApp(ctx context.Context, appID string) (DeploymentResponse, error) {
	app, err := service.queries.GetApp(ctx, appID)
	if err != nil {
		return DeploymentResponse{}, err
	}

	env, secrets, err := service.envAndSecretValues(ctx, appID)
	if err != nil {
		return DeploymentResponse{}, err
	}

	record, err := service.pipeline.Run(ctx, deploy.Request{
		AppID:        app.ID,
		GitURL:       app.GitUrl,
		Branch:       app.Branch,
		InternalPort: app.InternalPort,
		Env:          env,
		Secrets:      secrets,
	})
	if err != nil {
		if record.ID != "" {
			_ = service.persistAppStatus(ctx, app.ID, string(record.Status))
		}
		return deploymentResponseFromRecord(record), err
	}
	if err := service.persistAppStatus(ctx, app.ID, string(record.Status)); err != nil {
		return deploymentResponseFromRecord(record), err
	}
	if err := service.persistDetectedPort(ctx, app.ID, app.InternalPort, record.InternalPort); err != nil {
		return deploymentResponseFromRecord(record), err
	}
	if err := service.pruneDeploymentImages(ctx, app.ID); err != nil {
		return deploymentResponseFromRecord(record), err
	}
	return deploymentResponseFromRecord(record), nil
}

func (service storeDeploymentService) RollbackApp(ctx context.Context, appID, deploymentID string) (DeploymentResponse, error) {
	app, err := service.queries.GetApp(ctx, appID)
	if err != nil {
		return DeploymentResponse{}, mapStoreNotFound(err)
	}
	target, err := service.queries.GetDeployment(ctx, deploymentID)
	if err != nil {
		return DeploymentResponse{}, mapStoreNotFound(err)
	}
	if target.AppID != appID || target.Status != string(deploy.StatusRunning) || !target.ImageTag.Valid || target.ImageTag.String == "" {
		return DeploymentResponse{}, ErrInvalidRollbackTarget
	}

	env, secrets, err := service.envAndSecretValues(ctx, appID)
	if err != nil {
		return DeploymentResponse{}, err
	}

	record, err := service.pipeline.Rollback(ctx, deploy.RollbackRequest{
		AppID:              app.ID,
		TargetDeploymentID: target.ID,
		ImageTag:           target.ImageTag.String,
		InternalPort:       app.InternalPort,
		Env:                env,
		Secrets:            secrets,
	})
	if err != nil {
		if record.ID != "" {
			_ = service.persistAppStatus(ctx, app.ID, string(record.Status))
		}
		return deploymentResponseFromRecord(record), err
	}
	if err := service.persistAppStatus(ctx, app.ID, string(record.Status)); err != nil {
		return deploymentResponseFromRecord(record), err
	}
	if err := service.pruneDeploymentImages(ctx, app.ID); err != nil {
		return deploymentResponseFromRecord(record), err
	}
	return deploymentResponseFromRecord(record), nil
}

func (service storeDeploymentService) persistAppStatus(ctx context.Context, appID, status string) error {
	return service.queries.UpdateAppStatus(ctx, store.UpdateAppStatusParams{
		Status: status,
		ID:     appID,
	})
}

func (service storeDeploymentService) persistDetectedPort(ctx context.Context, appID string, storedPort, detectedPort int64) error {
	if detectedPort == 0 || detectedPort == storedPort {
		return nil
	}
	if err := service.queries.UpdateAppInternalPort(ctx, store.UpdateAppInternalPortParams{
		InternalPort: detectedPort,
		ID:           appID,
	}); err != nil {
		return err
	}
	if service.routeUpdater != nil {
		return service.routeUpdater.Reconcile(ctx)
	}
	return nil
}

func (service storeDeploymentService) ListDeployments(ctx context.Context, appID string) ([]DeploymentResponse, error) {
	rows, err := service.queries.ListDeploymentsByApp(ctx, appID)
	if err != nil {
		return nil, err
	}

	responses := make([]DeploymentResponse, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, deploymentResponseFromStore(row))
	}
	return responses, nil
}

func (service storeDeploymentService) pruneDeploymentImages(ctx context.Context, appID string) error {
	if service.imagePruner == nil {
		return nil
	}
	retention := service.retention
	if retention <= 0 {
		retention = 5
	}
	rows, err := service.queries.ListDeploymentsByApp(ctx, appID)
	if err != nil {
		return err
	}

	retained := make(map[string]bool)
	for _, row := range rows {
		if row.Status != string(deploy.StatusRunning) || !row.ImageTag.Valid || row.ImageTag.String == "" {
			continue
		}
		if retained[row.ImageTag.String] {
			continue
		}
		if len(retained) >= retention {
			continue
		}
		retained[row.ImageTag.String] = true
	}

	removed := make(map[string]bool)
	for _, row := range rows {
		if row.Status != string(deploy.StatusRunning) || !row.ImageTag.Valid || row.ImageTag.String == "" {
			continue
		}
		if retained[row.ImageTag.String] {
			continue
		}
		if !removed[row.ImageTag.String] {
			if err := service.imagePruner.RemoveImage(ctx, row.ImageTag.String); err != nil {
				return err
			}
			removed[row.ImageTag.String] = true
		}
		if err := service.queries.ClearDeploymentImageTag(ctx, row.ID); err != nil {
			return err
		}
	}
	return nil
}

func (service storeDeploymentService) envAndSecretValues(ctx context.Context, appID string) (map[string]string, []string, error) {
	if service.envVars == nil {
		return nil, nil, nil
	}
	envVars, err := service.envVars.ListEnvVars(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	env := make(map[string]string, len(envVars))
	secrets := make([]string, 0, len(envVars))
	for _, envVar := range envVars {
		env[envVar.Key] = envVar.Value
		if envVar.IsSecret {
			secrets = append(secrets, envVar.Value)
		}
	}
	return env, secrets, nil
}

func deploymentResponseFromRecord(record deploy.DeploymentRecord) DeploymentResponse {
	return DeploymentResponse{
		ID:       record.ID,
		AppID:    record.AppID,
		Status:   string(record.Status),
		Stage:    string(record.Stage),
		BuildLog: record.BuildLog,
		ImageTag: record.ImageTag,
	}
}

func deploymentResponseFromStore(row store.Deployment) DeploymentResponse {
	imageTag := ""
	if row.ImageTag.Valid {
		imageTag = row.ImageTag.String
	}
	return DeploymentResponse{
		ID:       row.ID,
		AppID:    row.AppID,
		Status:   row.Status,
		Stage:    row.Stage,
		BuildLog: row.BuildLog,
		ImageTag: imageTag,
	}
}
