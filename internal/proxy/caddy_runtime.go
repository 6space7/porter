package proxy

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type caddyDockerClient interface {
	ImagePull(ctx context.Context, imageName string, options image.PullOptions) (io.ReadCloser, error)
	NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
}

type DockerCaddyRuntime struct {
	client caddyDockerClient
}

func NewDockerCaddyRuntime() (*DockerCaddyRuntime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerCaddyRuntime{client: cli}, nil
}

func NewDockerCaddyRuntimeWithClient(client caddyDockerClient) *DockerCaddyRuntime {
	return &DockerCaddyRuntime{client: client}
}

func (runtime *DockerCaddyRuntime) EnsureCaddy(ctx context.Context, spec CaddyContainerSpec) error {
	if runtime == nil || runtime.client == nil {
		return fmt.Errorf("docker client is required")
	}
	if err := validateCaddySpec(spec); err != nil {
		return err
	}
	if err := runtime.ensureNetwork(ctx, spec.NetworkName); err != nil {
		return err
	}
	if err := runtime.removeExistingContainer(ctx, spec.Name); err != nil {
		return err
	}
	if err := runtime.pullImage(ctx, spec.Image); err != nil {
		return err
	}

	config := &container.Config{
		Image: spec.Image,
		Env: []string{
			"CADDY_ADMIN=0.0.0.0:" + strconv.Itoa(spec.AdminPort),
		},
		ExposedPorts: nat.PortSet{
			tcpPort(spec.HTTPPort):  {},
			tcpPort(spec.HTTPSPort): {},
			tcpPort(spec.AdminPort): {},
		},
		Labels: caddyLabels(),
	}
	hostConfig := &container.HostConfig{
		Binds: []string{
			spec.ConfigVolume + ":/config",
			spec.DataVolume + ":/data",
		},
		NetworkMode: container.NetworkMode(spec.NetworkName),
		ExtraHosts:  []string{"host.docker.internal:host-gateway"},
		PortBindings: nat.PortMap{
			tcpPort(spec.HTTPPort): {
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(spec.HTTPPort)},
			},
			tcpPort(spec.HTTPSPort): {
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(spec.HTTPSPort)},
			},
			tcpPort(spec.AdminPort): {
				{HostIP: spec.AdminHost, HostPort: strconv.Itoa(spec.AdminPort)},
			},
		},
		Privileged: false,
		CapDrop:    []string{"ALL"},
		CapAdd:     []string{"NET_BIND_SERVICE"},
		SecurityOpt: []string{
			"no-new-privileges:true",
		},
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			spec.NetworkName: {},
		},
	}

	created, err := runtime.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, spec.Name)
	if err != nil {
		return fmt.Errorf("create caddy container: %w", err)
	}
	if err := runtime.client.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start caddy container: %w", err)
	}
	return nil
}

func (runtime *DockerCaddyRuntime) pullImage(ctx context.Context, imageName string) error {
	response, err := runtime.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull caddy image: %w", err)
	}
	defer response.Close()
	if _, err := io.Copy(io.Discard, response); err != nil {
		return fmt.Errorf("read caddy image pull output: %w", err)
	}
	return nil
}

func (runtime *DockerCaddyRuntime) ensureNetwork(ctx context.Context, name string) error {
	_, err := runtime.client.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:   "bridge",
		Internal: false,
		Labels:   caddyLabels(),
	})
	if err != nil && !errdefs.IsConflict(err) {
		return fmt.Errorf("create caddy network: %w", err)
	}
	return nil
}

func (runtime *DockerCaddyRuntime) removeExistingContainer(ctx context.Context, name string) error {
	err := runtime.client.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("remove existing caddy container: %w", err)
	}
	return nil
}

func caddyLabels() map[string]string {
	return map[string]string{
		"porter.managed":   "true",
		"porter.component": "caddy",
	}
}

func tcpPort(port int) nat.Port {
	return nat.Port(strconv.Itoa(port) + "/tcp")
}
