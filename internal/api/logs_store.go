package api

import (
	"context"
	"fmt"
	"io"

	"github.com/6space7/porter/internal/store"
)

type RuntimeLogStreamer interface {
	StreamRuntimeLogs(ctx context.Context, appID string) (io.ReadCloser, error)
}

type storeLogService struct {
	queries     *store.Queries
	runtimeLogs RuntimeLogStreamer
}

func NewStoreLogService(queries *store.Queries, runtimeLogs RuntimeLogStreamer) LogService {
	return storeLogService{queries: queries, runtimeLogs: runtimeLogs}
}

func (service storeLogService) GetBuildLog(ctx context.Context, deploymentID string) (BuildLogResponse, error) {
	deployment, err := service.queries.GetDeployment(ctx, deploymentID)
	if err != nil {
		return BuildLogResponse{}, err
	}
	return BuildLogResponse{
		DeploymentID: deployment.ID,
		AppID:        deployment.AppID,
		Status:       deployment.Status,
		Stage:        deployment.Stage,
		BuildLog:     deployment.BuildLog,
	}, nil
}

func (service storeLogService) StreamRuntimeLogs(ctx context.Context, appID string) (io.ReadCloser, error) {
	if service.runtimeLogs == nil {
		return nil, fmt.Errorf("runtime log streamer is required")
	}
	if _, err := service.queries.GetApp(ctx, appID); err != nil {
		return nil, err
	}
	return service.runtimeLogs.StreamRuntimeLogs(ctx, appID)
}
