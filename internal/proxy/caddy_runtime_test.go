package proxy_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/proxy"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestDockerCaddyRuntimeEnsuresCaddyContainer(t *testing.T) {
	client := &fakeCaddyDockerClient{createID: "caddy_1"}
	runtime := proxy.NewDockerCaddyRuntimeWithClient(client)

	if err := runtime.EnsureCaddy(context.Background(), proxy.DefaultCaddyContainerSpec()); err != nil {
		t.Fatalf("ensure caddy: %v", err)
	}

	if client.networkName != "porter-proxy" {
		t.Fatalf("network name = %q", client.networkName)
	}
	if client.networkOptions.Driver != "bridge" || client.networkOptions.Internal {
		t.Fatalf("network options = %#v", client.networkOptions)
	}
	if client.networkOptions.Labels["porter.managed"] != "true" {
		t.Fatalf("network labels = %#v", client.networkOptions.Labels)
	}
	if client.removedName != "porter-caddy" || !client.removeOptions.Force {
		t.Fatalf("remove = %q %#v", client.removedName, client.removeOptions)
	}
	if client.createdName != "porter-caddy" {
		t.Fatalf("created name = %q", client.createdName)
	}
	if client.containerConfig.Image != "caddy:2-alpine" {
		t.Fatalf("image = %q", client.containerConfig.Image)
	}
	if !containsString(client.containerConfig.Env, "CADDY_ADMIN=0.0.0.0:2019") {
		t.Fatalf("env = %#v", client.containerConfig.Env)
	}
	if client.containerConfig.Labels["porter.managed"] != "true" || client.containerConfig.Labels["porter.component"] != "caddy" {
		t.Fatalf("container labels = %#v", client.containerConfig.Labels)
	}
	assertPortBinding(t, client.hostConfig.PortBindings, "80/tcp", "0.0.0.0", "80")
	assertPortBinding(t, client.hostConfig.PortBindings, "443/tcp", "0.0.0.0", "443")
	assertPortBinding(t, client.hostConfig.PortBindings, "2019/tcp", "127.0.0.1", "2019")
	if !containsString(client.hostConfig.Binds, "porter-caddy-config:/config") {
		t.Fatalf("binds = %#v", client.hostConfig.Binds)
	}
	if !containsString(client.hostConfig.Binds, "porter-caddy-data:/data") {
		t.Fatalf("binds = %#v", client.hostConfig.Binds)
	}
	if client.hostConfig.NetworkMode != container.NetworkMode("porter-proxy") {
		t.Fatalf("network mode = %q", client.hostConfig.NetworkMode)
	}
	if client.hostConfig.Privileged {
		t.Fatal("caddy container must not be privileged")
	}
	if !containsString(client.hostConfig.CapDrop, "ALL") {
		t.Fatalf("cap drop = %#v", client.hostConfig.CapDrop)
	}
	if !containsString(client.hostConfig.SecurityOpt, "no-new-privileges:true") {
		t.Fatalf("security opts = %#v", client.hostConfig.SecurityOpt)
	}
	if client.hostConfig.RestartPolicy.Name != container.RestartPolicyUnlessStopped {
		t.Fatalf("restart policy = %#v", client.hostConfig.RestartPolicy)
	}
	if _, ok := client.networkConfig.EndpointsConfig["porter-proxy"]; !ok {
		t.Fatalf("network config = %#v", client.networkConfig.EndpointsConfig)
	}
	if client.startedID != "caddy_1" {
		t.Fatalf("started id = %q", client.startedID)
	}
}

func assertPortBinding(t *testing.T, portMap nat.PortMap, port, hostIP, hostPort string) {
	t.Helper()

	bindings := portMap[nat.Port(port)]
	if len(bindings) != 1 {
		t.Fatalf("%s bindings = %#v", port, bindings)
	}
	if bindings[0].HostIP != hostIP || bindings[0].HostPort != hostPort {
		t.Fatalf("%s binding = %#v", port, bindings[0])
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type fakeCaddyDockerClient struct {
	networkName    string
	networkOptions network.CreateOptions

	removedName   string
	removeOptions container.RemoveOptions
	createdName   string
	createID      string
	startedID     string

	containerConfig container.Config
	hostConfig      container.HostConfig
	networkConfig   network.NetworkingConfig
}

func (client *fakeCaddyDockerClient) NetworkCreate(_ context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
	client.networkName = name
	client.networkOptions = options
	return network.CreateResponse{ID: "network_1"}, nil
}

func (client *fakeCaddyDockerClient) ContainerRemove(_ context.Context, containerID string, options container.RemoveOptions) error {
	client.removedName = containerID
	client.removeOptions = options
	return nil
}

func (client *fakeCaddyDockerClient) ContainerCreate(_ context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, _ *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	client.createdName = containerName
	client.containerConfig = *config
	client.hostConfig = *hostConfig
	client.networkConfig = *networkingConfig
	return container.CreateResponse{ID: client.createID}, nil
}

func (client *fakeCaddyDockerClient) ContainerStart(_ context.Context, containerID string, _ container.StartOptions) error {
	client.startedID = containerID
	return nil
}
