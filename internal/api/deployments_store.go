package api

import (
	"context"

	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/store"
)

type storeDeploymentService struct {
	queries  *store.Queries
	pipeline deploy.Pipeline
	envVars  EnvVarService
}

func NewStoreDeploymentService(queries *store.Queries, pipeline deploy.Pipeline, envVars EnvVarService) DeploymentService {
	return storeDeploymentService{queries: queries, pipeline: pipeline, envVars: envVars}
}

func (service storeDeploymentService) DeployApp(ctx context.Context, appID string) (DeploymentResponse, error) {
	app, err := service.queries.GetApp(ctx, appID)
	if err != nil {
		return DeploymentResponse{}, err
	}

	secrets, err := service.secretValues(ctx, appID)
	if err != nil {
		return DeploymentResponse{}, err
	}

	record, err := service.pipeline.Run(ctx, deploy.Request{
		AppID:   app.ID,
		GitURL:  app.GitUrl,
		Branch:  app.Branch,
		Secrets: secrets,
	})
	if err != nil {
		return deploymentResponseFromRecord(record), err
	}
	return deploymentResponseFromRecord(record), nil
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

func (service storeDeploymentService) secretValues(ctx context.Context, appID string) ([]string, error) {
	if service.envVars == nil {
		return nil, nil
	}
	envVars, err := service.envVars.ListEnvVars(ctx, appID)
	if err != nil {
		return nil, err
	}
	secrets := make([]string, 0, len(envVars))
	for _, envVar := range envVars {
		if envVar.IsSecret {
			secrets = append(secrets, envVar.Value)
		}
	}
	return secrets, nil
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
