package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/6space7/porter/internal/deploy"
	"github.com/6space7/porter/internal/services"
)

const (
	defaultMemoryBytes = int64(512 * 1024 * 1024)
	defaultNanoCPUs    = int64(1_000_000_000)
	proxyNetworkName   = "porter-proxy"
)

type ImageBackend interface {
	BuildImage(ctx context.Context, sourceDir, imageTag string) (string, error)
}

type NixpacksBackend interface {
	BuildWithNixpacks(ctx context.Context, sourceDir, imageTag string) (string, error)
}

type ContainerBackend interface {
	EnsureNetwork(ctx context.Context, name string) error
	ReplaceContainer(ctx context.Context, spec ContainerSpec) (string, error)
}

type ServiceBackend interface {
	PullImage(ctx context.Context, image string) error
	EnsureNetwork(ctx context.Context, name string) error
	ReplaceContainer(ctx context.Context, spec ContainerSpec) (string, error)
}

type RuntimeLogBackend interface {
	StreamContainerLogs(ctx context.Context, containerName string) (io.ReadCloser, error)
}

type LifecycleBackend interface {
	StartContainer(ctx context.Context, containerName string) error
	StopContainer(ctx context.Context, containerName string) error
	RemoveContainer(ctx context.Context, containerName string) error
}

type ContainerSpec struct {
	Name         string
	ImageTag     string
	Command      []string
	NetworkName  string
	InternalPort int64
	Env          []string
	Mounts       []VolumeMount
	Privileged   bool
	CapDrop      []string
	MemoryBytes  int64
	NanoCPUs     int64
}

type VolumeMount struct {
	Source string
	Target string
}

type Builder struct {
	Images   ImageBackend
	Nixpacks NixpacksBackend
}

func (builder Builder) Build(ctx context.Context, req deploy.BuildRequest) (deploy.BuildResult, error) {
	if req.SourceDir == "" {
		return deploy.BuildResult{}, fmt.Errorf("source dir is required")
	}

	imageTag := ImageTag(req.AppID, req.DeploymentID)
	buildType, err := builder.resolveBuildType(req)
	if err != nil {
		return deploy.BuildResult{}, err
	}
	if buildType == "nixpacks" {
		if builder.Nixpacks == nil {
			return deploy.BuildResult{}, fmt.Errorf("nixpacks backend is required")
		}
		log, err := builder.Nixpacks.BuildWithNixpacks(ctx, req.SourceDir, imageTag)
		if err != nil {
			return deploy.BuildResult{ImageTag: imageTag, Log: log, BuildType: "nixpacks"}, err
		}
		return deploy.BuildResult{ImageTag: imageTag, Log: log, BuildType: "nixpacks"}, nil
	}
	if builder.Images == nil {
		return deploy.BuildResult{}, fmt.Errorf("image backend is required")
	}
	log, err := builder.Images.BuildImage(ctx, req.SourceDir, imageTag)
	if err != nil {
		return deploy.BuildResult{ImageTag: imageTag, Log: log, BuildType: "dockerfile"}, err
	}
	return deploy.BuildResult{ImageTag: imageTag, Log: log, BuildType: "dockerfile"}, nil
}

func (builder Builder) resolveBuildType(req deploy.BuildRequest) (string, error) {
	if req.BuildType == "nixpacks" {
		return "nixpacks", nil
	}
	hasDockerfile, err := dockerfileExists(req.SourceDir)
	if err != nil {
		return "", err
	}
	if hasDockerfile {
		return "dockerfile", nil
	}
	return "nixpacks", nil
}

func dockerfileExists(sourceDir string) (bool, error) {
	_, err := os.Stat(filepath.Join(sourceDir, "Dockerfile"))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

type Runner struct {
	Containers ContainerBackend
}

type ServiceRunner struct {
	Backend ServiceBackend
}

type RuntimeLogs struct {
	Containers RuntimeLogBackend
}

type AppController struct {
	Containers LifecycleBackend
}

func (runner Runner) Run(ctx context.Context, req deploy.RunRequest) (string, error) {
	if runner.Containers == nil {
		return "", fmt.Errorf("container backend is required")
	}
	if req.ImageTag == "" {
		return "", fmt.Errorf("image tag is required")
	}

	networkName := ProxyNetworkName()
	if err := runner.Containers.EnsureNetwork(ctx, networkName); err != nil {
		return "", err
	}

	internalPort := req.InternalPort
	if internalPort == 0 {
		internalPort = 3000
	}
	return runner.Containers.ReplaceContainer(ctx, ContainerSpec{
		Name:         ContainerName(req.AppID),
		ImageTag:     req.ImageTag,
		NetworkName:  networkName,
		InternalPort: internalPort,
		Env:          envList(req.Env),
		Privileged:   false,
		CapDrop:      []string{"ALL"},
		MemoryBytes:  defaultMemoryBytes,
		NanoCPUs:     defaultNanoCPUs,
	})
}

func (runner ServiceRunner) DeployService(ctx context.Context, req services.DeployRequest) (string, error) {
	if runner.Backend == nil {
		return "", fmt.Errorf("service backend is required")
	}
	if req.Image == "" {
		return "", fmt.Errorf("service image is required")
	}
	if req.ContainerName == "" {
		return "", fmt.Errorf("service container name is required")
	}
	if err := runner.Backend.PullImage(ctx, req.Image); err != nil {
		return "", err
	}
	networkName := ProxyNetworkName()
	if err := runner.Backend.EnsureNetwork(ctx, networkName); err != nil {
		return "", err
	}
	return runner.Backend.ReplaceContainer(ctx, ContainerSpec{
		Name:         req.ContainerName,
		ImageTag:     req.Image,
		Command:      append([]string(nil), req.Command...),
		NetworkName:  networkName,
		InternalPort: req.InternalPort,
		Env:          envList(req.Env),
		Mounts:       serviceMounts(req.ServiceID, req.Volumes),
		Privileged:   false,
		MemoryBytes:  defaultMemoryBytes,
		NanoCPUs:     defaultNanoCPUs,
	})
}

func serviceMounts(serviceID string, volumes []services.VolumeSpec) []VolumeMount {
	if len(volumes) == 0 {
		return nil
	}
	mounts := make([]VolumeMount, 0, len(volumes))
	for _, volume := range volumes {
		mounts = append(mounts, VolumeMount{
			Source: ServiceVolumeName(serviceID, volume.Name),
			Target: volume.Path,
		})
	}
	return mounts
}

func (logs RuntimeLogs) StreamRuntimeLogs(ctx context.Context, appID string) (io.ReadCloser, error) {
	if logs.Containers == nil {
		return nil, fmt.Errorf("container log backend is required")
	}
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	return logs.Containers.StreamContainerLogs(ctx, ContainerName(appID))
}

func (controller AppController) StartApp(ctx context.Context, appID string) error {
	if controller.Containers == nil {
		return fmt.Errorf("container lifecycle backend is required")
	}
	if appID == "" {
		return fmt.Errorf("app id is required")
	}
	return controller.Containers.StartContainer(ctx, ContainerName(appID))
}

func (controller AppController) StopApp(ctx context.Context, appID string) error {
	if controller.Containers == nil {
		return fmt.Errorf("container lifecycle backend is required")
	}
	if appID == "" {
		return fmt.Errorf("app id is required")
	}
	return controller.Containers.StopContainer(ctx, ContainerName(appID))
}

func (controller AppController) RemoveApp(ctx context.Context, appID string) error {
	if controller.Containers == nil {
		return fmt.Errorf("container lifecycle backend is required")
	}
	if appID == "" {
		return fmt.Errorf("app id is required")
	}
	return controller.Containers.RemoveContainer(ctx, ContainerName(appID))
}

func ImageTag(appID, deploymentID string) string {
	return "porter/" + sanitizeDockerName(appID) + ":" + sanitizeDockerName(deploymentID)
}

func ContainerName(appID string) string {
	return "porter-" + sanitizeDockerName(appID)
}

func ServiceContainerName(serviceID string) string {
	return "porter-svc-" + sanitizeDockerName(serviceID)
}

func ServiceVolumeName(serviceID, volumeName string) string {
	return ServiceContainerName(serviceID) + "-" + sanitizeDockerName(volumeName)
}

func ProxyNetworkName() string {
	return proxyNetworkName
}

func envList(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

func sanitizeDockerName(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			out.WriteRune(r)
		default:
			out.WriteRune('-')
		}
	}
	if out.Len() == 0 {
		return "unknown"
	}
	return out.String()
}
