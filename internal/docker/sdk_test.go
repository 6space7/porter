package docker_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	dockerstage "github.com/6space7/porter/internal/docker"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestSDKBackendImplementsStageBackends(t *testing.T) {
	var _ dockerstage.ImageBackend = (*dockerstage.SDKBackend)(nil)
	var _ dockerstage.ContainerBackend = (*dockerstage.SDKBackend)(nil)
	var _ dockerstage.LifecycleBackend = (*dockerstage.SDKBackend)(nil)
}

func TestSDKBackendBuildImageSendsTaggedDockerBuild(t *testing.T) {
	client := &fakeDockerClient{
		buildResponse: build.ImageBuildResponse{Body: io.NopCloser(strings.NewReader(`{"stream":"built\n"}`))},
	}
	backend := dockerstage.NewSDKBackendWithClient(client)

	log, err := backend.BuildImage(context.Background(), t.TempDir(), "porter/app_1:dep_1")
	if err != nil {
		t.Fatalf("build image: %v", err)
	}

	if !strings.Contains(log, "built") {
		t.Fatalf("log = %q, want build output", log)
	}
	if len(client.buildOptions.Tags) != 1 || client.buildOptions.Tags[0] != "porter/app_1:dep_1" {
		t.Fatalf("build tags = %#v", client.buildOptions.Tags)
	}
	if !client.buildOptions.Remove || !client.buildOptions.ForceRemove {
		t.Fatalf("build cleanup options = remove:%v force:%v", client.buildOptions.Remove, client.buildOptions.ForceRemove)
	}
	if client.buildContext == nil {
		t.Fatal("build context was nil")
	}
}

func TestSDKBackendBuildImageReturnsStreamedBuildError(t *testing.T) {
	client := &fakeDockerClient{
		buildResponse: build.ImageBuildResponse{Body: io.NopCloser(strings.NewReader(`{"stream":"Step 1/1\n"}` + "\n" + `{"error":"dockerfile failed","errorDetail":{"message":"dockerfile failed"}}`))},
	}
	backend := dockerstage.NewSDKBackendWithClient(client)

	log, err := backend.BuildImage(context.Background(), t.TempDir(), "porter/app_1:dep_1")
	if err == nil {
		t.Fatal("expected streamed build error")
	}
	if !strings.Contains(err.Error(), "dockerfile failed") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(log, "dockerfile failed") {
		t.Fatalf("log = %q", log)
	}
}

func TestSDKBackendReplaceContainerUsesSafeOptions(t *testing.T) {
	client := &fakeDockerClient{createID: "container_1"}
	backend := dockerstage.NewSDKBackendWithClient(client)

	log, err := backend.ReplaceContainer(context.Background(), dockerstage.ContainerSpec{
		Name:         "porter-app_1",
		ImageTag:     "porter/app_1:dep_1",
		NetworkName:  "porter-app_1",
		InternalPort: 8080,
		Env:          []string{"PORT=8080"},
		Privileged:   false,
		CapDrop:      []string{"ALL"},
		MemoryBytes:  512 * 1024 * 1024,
		NanoCPUs:     1_000_000_000,
	})
	if err != nil {
		t.Fatalf("replace container: %v", err)
	}

	if log != "container container_1 started\n" {
		t.Fatalf("log = %q", log)
	}
	if client.removedName != "porter-app_1" || !client.removeOptions.Force {
		t.Fatalf("remove = %q %#v", client.removedName, client.removeOptions)
	}
	if client.createdName != "porter-app_1" || client.containerConfig.Image != "porter/app_1:dep_1" {
		t.Fatalf("create = %q %#v", client.createdName, client.containerConfig)
	}
	if client.hostConfig.Privileged {
		t.Fatal("container must not be privileged")
	}
	if len(client.hostConfig.CapDrop) != 1 || client.hostConfig.CapDrop[0] != "ALL" {
		t.Fatalf("cap drop = %#v", client.hostConfig.CapDrop)
	}
	if client.hostConfig.Resources.Memory == 0 || client.hostConfig.Resources.NanoCPUs == 0 {
		t.Fatalf("resources = %#v", client.hostConfig.Resources)
	}
	if client.hostConfig.NetworkMode != container.NetworkMode("porter-app_1") {
		t.Fatalf("network mode = %q", client.hostConfig.NetworkMode)
	}
	if client.startedID != "container_1" {
		t.Fatalf("started id = %q", client.startedID)
	}
}

func TestSDKBackendReplaceContainerIgnoresMissingExistingContainer(t *testing.T) {
	client := &fakeDockerClient{
		createID:    "container_1",
		removeError: errdefs.NotFound(errors.New("no such container")),
	}
	backend := dockerstage.NewSDKBackendWithClient(client)

	if _, err := backend.ReplaceContainer(context.Background(), dockerstage.ContainerSpec{
		Name:        "porter-app_1",
		ImageTag:    "porter/app_1:dep_1",
		NetworkName: "porter-proxy",
	}); err != nil {
		t.Fatalf("replace container: %v", err)
	}

	if client.createdName != "porter-app_1" || client.startedID != "container_1" {
		t.Fatalf("create/start = %q/%q", client.createdName, client.startedID)
	}
}

func TestSDKBackendEnsureNetworkIgnoresExistingNetwork(t *testing.T) {
	client := &fakeDockerClient{
		networkError: errdefs.Conflict(errors.New("network exists")),
	}
	backend := dockerstage.NewSDKBackendWithClient(client)

	if err := backend.EnsureNetwork(context.Background(), "porter-proxy"); err != nil {
		t.Fatalf("ensure network: %v", err)
	}
	if client.networkName != "porter-proxy" {
		t.Fatalf("network name = %q", client.networkName)
	}
}

func TestSDKBackendControlsExistingContainers(t *testing.T) {
	client := &fakeDockerClient{}
	backend := dockerstage.NewSDKBackendWithClient(client)

	if err := backend.StopContainer(context.Background(), "porter-app_1"); err != nil {
		t.Fatalf("stop container: %v", err)
	}
	if err := backend.StartContainer(context.Background(), "porter-app_1"); err != nil {
		t.Fatalf("start container: %v", err)
	}
	if err := backend.RemoveContainer(context.Background(), "porter-app_1"); err != nil {
		t.Fatalf("remove container: %v", err)
	}

	if client.stoppedID != "porter-app_1" || client.startedID != "porter-app_1" || client.removedName != "porter-app_1" {
		t.Fatalf("container calls = stop:%q start:%q remove:%q", client.stoppedID, client.startedID, client.removedName)
	}
	if !client.removeOptions.Force {
		t.Fatalf("remove options = %#v", client.removeOptions)
	}
}

func TestSDKBackendRemoveContainerIgnoresMissingContainer(t *testing.T) {
	client := &fakeDockerClient{
		removeError: errdefs.NotFound(errors.New("no such container")),
	}
	backend := dockerstage.NewSDKBackendWithClient(client)

	if err := backend.RemoveContainer(context.Background(), "porter-app_1"); err != nil {
		t.Fatalf("remove missing container: %v", err)
	}
}

func TestSDKBackendRemovesImagesForRetention(t *testing.T) {
	client := &fakeDockerClient{}
	backend := dockerstage.NewSDKBackendWithClient(client)

	if err := backend.RemoveImage(context.Background(), "porter/app_1:dep_1"); err != nil {
		t.Fatalf("remove image: %v", err)
	}

	if client.removedImage != "porter/app_1:dep_1" {
		t.Fatalf("removed image = %q", client.removedImage)
	}
	if !client.imageRemoveOptions.Force || !client.imageRemoveOptions.PruneChildren {
		t.Fatalf("image remove options = %#v", client.imageRemoveOptions)
	}
}

func TestSDKBackendRemoveImageIgnoresMissingImage(t *testing.T) {
	client := &fakeDockerClient{
		imageRemoveError: errdefs.NotFound(errors.New("no such image")),
	}
	backend := dockerstage.NewSDKBackendWithClient(client)

	if err := backend.RemoveImage(context.Background(), "porter/app_1:dep_1"); err != nil {
		t.Fatalf("remove missing image: %v", err)
	}
}

func TestSDKBackendStreamsContainerLogs(t *testing.T) {
	client := &fakeDockerClient{
		logsResponse: io.NopCloser(strings.NewReader(multiplexedStdout("runtime line\n"))),
	}
	backend := dockerstage.NewSDKBackendWithClient(client)

	stream, err := backend.StreamContainerLogs(context.Background(), "porter-app_1")
	if err != nil {
		t.Fatalf("stream container logs: %v", err)
	}
	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if string(body) != "runtime line\n" {
		t.Fatalf("body = %q", body)
	}
	if client.logsContainer != "porter-app_1" {
		t.Fatalf("logs container = %q", client.logsContainer)
	}
	if !client.logsOptions.ShowStdout || !client.logsOptions.ShowStderr || !client.logsOptions.Follow || client.logsOptions.Tail != "100" {
		t.Fatalf("logs options = %#v", client.logsOptions)
	}
}

type fakeDockerClient struct {
	buildContext  io.Reader
	buildOptions  build.ImageBuildOptions
	buildResponse build.ImageBuildResponse

	removedName        string
	removeOptions      container.RemoveOptions
	removeError        error
	createdName        string
	createID           string
	startedID          string
	stoppedID          string
	removedImage       string
	imageRemoveOptions image.RemoveOptions
	imageRemoveError   error

	containerConfig container.Config
	hostConfig      container.HostConfig
	networkConfig   network.NetworkingConfig
	networkName     string
	networkError    error

	logsContainer string
	logsOptions   container.LogsOptions
	logsResponse  io.ReadCloser
}

func (client *fakeDockerClient) ImageBuild(_ context.Context, buildContext io.Reader, options build.ImageBuildOptions) (build.ImageBuildResponse, error) {
	client.buildContext = buildContext
	client.buildOptions = options
	return client.buildResponse, nil
}

func (client *fakeDockerClient) ImageRemove(_ context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	client.removedImage = imageID
	client.imageRemoveOptions = options
	return nil, client.imageRemoveError
}

func (client *fakeDockerClient) ContainerRemove(_ context.Context, containerID string, options container.RemoveOptions) error {
	client.removedName = containerID
	client.removeOptions = options
	return client.removeError
}

func (client *fakeDockerClient) NetworkCreate(_ context.Context, name string, _ network.CreateOptions) (network.CreateResponse, error) {
	client.networkName = name
	return network.CreateResponse{ID: "network_1"}, client.networkError
}

func (client *fakeDockerClient) ContainerCreate(_ context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, _ *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	client.createdName = containerName
	client.containerConfig = *config
	client.hostConfig = *hostConfig
	client.networkConfig = *networkingConfig
	return container.CreateResponse{ID: client.createID}, nil
}

func (client *fakeDockerClient) ContainerStart(_ context.Context, containerID string, _ container.StartOptions) error {
	client.startedID = containerID
	return nil
}

func (client *fakeDockerClient) ContainerStop(_ context.Context, containerID string, _ container.StopOptions) error {
	client.stoppedID = containerID
	return nil
}

func (client *fakeDockerClient) ContainerLogs(_ context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	client.logsContainer = containerID
	client.logsOptions = options
	return client.logsResponse, nil
}

func multiplexedStdout(message string) string {
	size := len(message)
	header := []byte{1, 0, 0, 0, byte(size >> 24), byte(size >> 16), byte(size >> 8), byte(size)}
	return string(header) + message
}
