package deploy

import (
	"context"
	"fmt"
	"strings"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusFailed  Status = "failed"
)

type Stage string

const (
	StageQueued   Stage = "queued"
	StageCloning  Stage = "cloning"
	StageBuilding Stage = "building"
	StageStarting Stage = "starting"
	StageRunning  Stage = "running"
)

type Request struct {
	AppID   string
	GitURL  string
	Branch  string
	Secrets []string
}

type DeploymentRecord struct {
	ID       string
	AppID    string
	Status   Status
	Stage    Stage
	BuildLog string
	ImageTag string
}

type DeploymentStore interface {
	CreateDeployment(ctx context.Context, appID string) (DeploymentRecord, error)
	UpdateDeployment(ctx context.Context, record DeploymentRecord) error
}

type CloneRequest struct {
	AppID        string
	DeploymentID string
	GitURL       string
	Branch       string
}

type Cloner interface {
	Clone(ctx context.Context, req CloneRequest) (string, error)
}

type ClonerFunc func(ctx context.Context, req CloneRequest) (string, error)

func (fn ClonerFunc) Clone(ctx context.Context, req CloneRequest) (string, error) {
	return fn(ctx, req)
}

type BuildRequest struct {
	AppID        string
	DeploymentID string
}

type BuildResult struct {
	ImageTag string
	Log      string
}

type Builder interface {
	Build(ctx context.Context, req BuildRequest) (BuildResult, error)
}

type BuilderFunc func(ctx context.Context, req BuildRequest) (BuildResult, error)

func (fn BuilderFunc) Build(ctx context.Context, req BuildRequest) (BuildResult, error) {
	return fn(ctx, req)
}

type RunRequest struct {
	AppID        string
	DeploymentID string
	ImageTag     string
}

type Runner interface {
	Run(ctx context.Context, req RunRequest) (string, error)
}

type RunnerFunc func(ctx context.Context, req RunRequest) (string, error)

func (fn RunnerFunc) Run(ctx context.Context, req RunRequest) (string, error) {
	return fn(ctx, req)
}

type Pipeline struct {
	Store   DeploymentStore
	Cloner  Cloner
	Builder Builder
	Runner  Runner
}

func (pipeline Pipeline) Run(ctx context.Context, req Request) (DeploymentRecord, error) {
	record, err := pipeline.Store.CreateDeployment(ctx, req.AppID)
	if err != nil {
		return DeploymentRecord{}, err
	}

	var logs strings.Builder
	if err := pipeline.mark(ctx, &record, StatusRunning, StageCloning, logs.String(), ""); err != nil {
		return record, err
	}
	cloneLog, err := pipeline.Cloner.Clone(ctx, CloneRequest{
		AppID:        req.AppID,
		DeploymentID: record.ID,
		GitURL:       req.GitURL,
		Branch:       req.Branch,
	})
	logs.WriteString(cloneLog)
	if err != nil {
		return pipeline.fail(ctx, record, StageCloning, logs.String(), req.Secrets, err)
	}

	if err := pipeline.mark(ctx, &record, StatusRunning, StageBuilding, RedactSecrets(logs.String(), req.Secrets), ""); err != nil {
		return record, err
	}
	buildResult, err := pipeline.Builder.Build(ctx, BuildRequest{
		AppID:        req.AppID,
		DeploymentID: record.ID,
	})
	logs.WriteString(buildResult.Log)
	if err != nil {
		return pipeline.fail(ctx, record, StageBuilding, logs.String(), req.Secrets, err)
	}

	if err := pipeline.mark(ctx, &record, StatusRunning, StageStarting, RedactSecrets(logs.String(), req.Secrets), buildResult.ImageTag); err != nil {
		return record, err
	}
	runLog, err := pipeline.Runner.Run(ctx, RunRequest{
		AppID:        req.AppID,
		DeploymentID: record.ID,
		ImageTag:     buildResult.ImageTag,
	})
	logs.WriteString(runLog)
	if err != nil {
		return pipeline.fail(ctx, record, StageStarting, logs.String(), req.Secrets, err)
	}

	if err := pipeline.mark(ctx, &record, StatusRunning, StageRunning, RedactSecrets(logs.String(), req.Secrets), buildResult.ImageTag); err != nil {
		return record, err
	}
	return record, nil
}

func (pipeline Pipeline) mark(ctx context.Context, record *DeploymentRecord, status Status, stage Stage, buildLog, imageTag string) error {
	record.Status = status
	record.Stage = stage
	record.BuildLog = buildLog
	record.ImageTag = imageTag
	return pipeline.Store.UpdateDeployment(ctx, *record)
}

func (pipeline Pipeline) fail(ctx context.Context, record DeploymentRecord, stage Stage, log string, secrets []string, cause error) (DeploymentRecord, error) {
	record.Status = StatusFailed
	record.Stage = stage
	record.BuildLog = RedactSecrets(log+fmt.Sprintf("error: %v\n", cause), secrets)
	if err := pipeline.Store.UpdateDeployment(ctx, record); err != nil {
		return record, err
	}
	return record, cause
}
