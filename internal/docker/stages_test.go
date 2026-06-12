package docker_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/6space7/porter/internal/deploy"
	dockerstage "github.com/6space7/porter/internal/docker"
	"github.com/6space7/porter/internal/services"
)

func TestBuilderBuildsDeterministicImageTagFromSourceDir(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "Dockerfile"), []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	images := &fakeImageBackend{log: "docker build log\n"}
	builder := dockerstage.Builder{Images: images}

	result, err := builder.Build(context.Background(), deploy.BuildRequest{
		AppID:        "app_1",
		DeploymentID: "dep_1",
		SourceDir:    sourceDir,
		BuildType:    "dockerfile",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if result.ImageTag != "porter/app_1:dep_1" {
		t.Fatalf("image tag = %q, want porter/app_1:dep_1", result.ImageTag)
	}
	if result.Log != "docker build log\n" {
		t.Fatalf("log = %q", result.Log)
	}
	if result.BuildType != "dockerfile" {
		t.Fatalf("build type = %q, want dockerfile", result.BuildType)
	}
	if images.sourceDir != sourceDir || images.imageTag != "porter/app_1:dep_1" {
		t.Fatalf("backend call = source %q tag %q", images.sourceDir, images.imageTag)
	}
}

func TestBuilderUsesNixpacksWhenRequested(t *testing.T) {
	nixpacks := &fakeNixpacksBackend{log: "nixpacks build log\n"}
	images := &fakeImageBackend{log: "docker build log\n"}
	builder := dockerstage.Builder{Images: images, Nixpacks: nixpacks}

	result, err := builder.Build(context.Background(), deploy.BuildRequest{
		AppID:        "app_1",
		DeploymentID: "dep_1",
		SourceDir:    t.TempDir(),
		BuildType:    "nixpacks",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if images.called {
		t.Fatal("dockerfile image backend should not be called for explicit nixpacks")
	}
	if !nixpacks.called || nixpacks.imageTag != "porter/app_1:dep_1" {
		t.Fatalf("nixpacks backend call = %#v", nixpacks)
	}
	if result.BuildType != "nixpacks" || result.Log != "nixpacks build log\n" {
		t.Fatalf("result = %#v", result)
	}
}

func TestBuilderFallsBackToNixpacksWhenDockerfileMissing(t *testing.T) {
	nixpacks := &fakeNixpacksBackend{log: "nixpacks fallback log\n"}
	images := &fakeImageBackend{log: "docker build log\n"}
	builder := dockerstage.Builder{Images: images, Nixpacks: nixpacks}

	result, err := builder.Build(context.Background(), deploy.BuildRequest{
		AppID:        "app_1",
		DeploymentID: "dep_1",
		SourceDir:    t.TempDir(),
		BuildType:    "dockerfile",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if images.called {
		t.Fatal("dockerfile image backend should not be called without a Dockerfile")
	}
	if !nixpacks.called {
		t.Fatal("nixpacks backend should be called when Dockerfile is missing")
	}
	if result.BuildType != "nixpacks" {
		t.Fatalf("build type = %q, want nixpacks", result.BuildType)
	}
}

func TestBuilderRejectsMissingSourceDir(t *testing.T) {
	images := &fakeImageBackend{}
	builder := dockerstage.Builder{Images: images}

	if _, err := builder.Build(context.Background(), deploy.BuildRequest{AppID: "app_1", DeploymentID: "dep_1"}); err == nil {
		t.Fatal("expected missing source dir to fail")
	}
	if images.called {
		t.Fatal("backend should not be called when source dir is missing")
	}
}

func TestRunnerCreatesNetworkAndReplacesContainerWithSafeDefaults(t *testing.T) {
	containers := &fakeContainerBackend{log: "container started\n"}
	runner := dockerstage.Runner{Containers: containers}

	log, err := runner.Run(context.Background(), deploy.RunRequest{
		AppID:        "app_1",
		DeploymentID: "dep_1",
		ImageTag:     "porter/app_1:dep_1",
		InternalPort: 8080,
		Env:          map[string]string{"DATABASE_URL": "postgres://internal", "PORT": "8080"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if log != "container started\n" {
		t.Fatalf("log = %q", log)
	}
	if containers.networkName != "porter-proxy" {
		t.Fatalf("network = %q, want porter-proxy", containers.networkName)
	}
	spec := containers.spec
	if spec.Name != "porter-app_1" || spec.ImageTag != "porter/app_1:dep_1" || spec.NetworkName != "porter-proxy" {
		t.Fatalf("spec identity = %#v", spec)
	}
	if spec.InternalPort != 8080 {
		t.Fatalf("internal port = %d, want 8080", spec.InternalPort)
	}
	if !reflect.DeepEqual(spec.Env, []string{"DATABASE_URL=postgres://internal", "PORT=8080"}) {
		t.Fatalf("env = %#v", spec.Env)
	}
	if spec.Privileged {
		t.Fatal("container must not be privileged")
	}
	if !reflect.DeepEqual(spec.CapDrop, []string{"ALL"}) {
		t.Fatalf("cap drop = %#v, want ALL", spec.CapDrop)
	}
	if spec.MemoryBytes == 0 || spec.NanoCPUs == 0 {
		t.Fatalf("resource limits missing: memory=%d nanoCPUs=%d", spec.MemoryBytes, spec.NanoCPUs)
	}
}

func TestServiceRunnerPullsImageAndRunsServiceContainer(t *testing.T) {
	backend := &fakeServiceBackend{log: "service started\n"}
	runner := dockerstage.ServiceRunner{Backend: backend}

	log, err := runner.DeployService(context.Background(), services.DeployRequest{
		ServiceID:     "svc_1",
		Image:         "postgres:16-alpine",
		Command:       []string{"postgres"},
		ContainerName: "porter-svc-svc_1",
		InternalPort:  5432,
		Env:           map[string]string{"POSTGRES_PASSWORD": "secret"},
		Volumes:       []services.VolumeSpec{{Name: "data", Path: "/var/lib/postgresql/data"}},
	})
	if err != nil {
		t.Fatalf("deploy service: %v", err)
	}
	if log != "service started\n" {
		t.Fatalf("log = %q", log)
	}
	if backend.pulledImage != "postgres:16-alpine" || backend.networkName != "porter-proxy" {
		t.Fatalf("backend = %#v", backend)
	}
	spec := backend.spec
	if spec.Name != "porter-svc-svc_1" || spec.ImageTag != "postgres:16-alpine" || spec.InternalPort != 5432 {
		t.Fatalf("spec identity = %#v", spec)
	}
	if !reflect.DeepEqual(spec.Command, []string{"postgres"}) {
		t.Fatalf("command = %#v", spec.Command)
	}
	if !reflect.DeepEqual(spec.Env, []string{"POSTGRES_PASSWORD=secret"}) {
		t.Fatalf("env = %#v", spec.Env)
	}
	if len(spec.Mounts) != 1 || spec.Mounts[0].Source != "porter-svc-svc_1-data" || spec.Mounts[0].Target != "/var/lib/postgresql/data" {
		t.Fatalf("mounts = %#v", spec.Mounts)
	}
	if spec.Privileged || len(spec.CapDrop) != 0 {
		t.Fatalf("unsafe spec = %#v", spec)
	}
}

func TestRuntimeLogsStreamsSanitizedAppContainerLogs(t *testing.T) {
	containers := &fakeRuntimeLogBackend{stream: io.NopCloser(strings.NewReader("live\n"))}
	runtimeLogs := dockerstage.RuntimeLogs{Containers: containers}

	stream, err := runtimeLogs.StreamRuntimeLogs(context.Background(), "App 1")
	if err != nil {
		t.Fatalf("stream runtime logs: %v", err)
	}
	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if string(body) != "live\n" {
		t.Fatalf("body = %q", body)
	}
	if containers.containerName != "porter-app-1" {
		t.Fatalf("container name = %q", containers.containerName)
	}
}

func TestAppControllerUsesSanitizedContainerNames(t *testing.T) {
	containers := &fakeLifecycleBackend{}
	controller := dockerstage.AppController{Containers: containers}

	if err := controller.StopApp(context.Background(), "App 1"); err != nil {
		t.Fatalf("stop app: %v", err)
	}
	if err := controller.StartApp(context.Background(), "App 1"); err != nil {
		t.Fatalf("start app: %v", err)
	}
	if err := controller.RemoveApp(context.Background(), "App 1"); err != nil {
		t.Fatalf("remove app: %v", err)
	}

	if containers.stopped != "porter-app-1" || containers.started != "porter-app-1" || containers.removed != "porter-app-1" {
		t.Fatalf("lifecycle names = start:%q stop:%q remove:%q", containers.started, containers.stopped, containers.removed)
	}
}

func TestProxyNetworkNameMatchesManagedCaddyNetwork(t *testing.T) {
	if got := dockerstage.ProxyNetworkName(); got != "porter-proxy" {
		t.Fatalf("proxy network name = %q, want porter-proxy", got)
	}
}

type fakeImageBackend struct {
	called    bool
	sourceDir string
	imageTag  string
	log       string
}

func (backend *fakeImageBackend) BuildImage(_ context.Context, sourceDir, imageTag string) (string, error) {
	backend.called = true
	backend.sourceDir = sourceDir
	backend.imageTag = imageTag
	return backend.log, nil
}

type fakeNixpacksBackend struct {
	called    bool
	sourceDir string
	imageTag  string
	log       string
}

func (backend *fakeNixpacksBackend) BuildWithNixpacks(_ context.Context, sourceDir, imageTag string) (string, error) {
	backend.called = true
	backend.sourceDir = sourceDir
	backend.imageTag = imageTag
	return backend.log, nil
}

type fakeContainerBackend struct {
	networkName string
	spec        dockerstage.ContainerSpec
	log         string
}

func (backend *fakeContainerBackend) EnsureNetwork(_ context.Context, name string) error {
	backend.networkName = name
	return nil
}

func (backend *fakeContainerBackend) ReplaceContainer(_ context.Context, spec dockerstage.ContainerSpec) (string, error) {
	backend.spec = spec
	return backend.log, nil
}

type fakeServiceBackend struct {
	pulledImage string
	networkName string
	spec        dockerstage.ContainerSpec
	log         string
}

func (backend *fakeServiceBackend) PullImage(_ context.Context, image string) error {
	backend.pulledImage = image
	return nil
}

func (backend *fakeServiceBackend) EnsureNetwork(_ context.Context, name string) error {
	backend.networkName = name
	return nil
}

func (backend *fakeServiceBackend) ReplaceContainer(_ context.Context, spec dockerstage.ContainerSpec) (string, error) {
	backend.spec = spec
	return backend.log, nil
}

type fakeRuntimeLogBackend struct {
	containerName string
	stream        io.ReadCloser
}

func (backend *fakeRuntimeLogBackend) StreamContainerLogs(_ context.Context, containerName string) (io.ReadCloser, error) {
	backend.containerName = containerName
	return backend.stream, nil
}

type fakeLifecycleBackend struct {
	started string
	stopped string
	removed string
}

func (backend *fakeLifecycleBackend) StartContainer(_ context.Context, containerName string) error {
	backend.started = containerName
	return nil
}

func (backend *fakeLifecycleBackend) StopContainer(_ context.Context, containerName string) error {
	backend.stopped = containerName
	return nil
}

func (backend *fakeLifecycleBackend) RemoveContainer(_ context.Context, containerName string) error {
	backend.removed = containerName
	return nil
}
