package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerClient interface {
	ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (build.ImageBuildResponse, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
}

type SDKBackend struct {
	client dockerClient
}

func NewSDKBackend() (*SDKBackend, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &SDKBackend{client: cli}, nil
}

func NewSDKBackendWithClient(client dockerClient) *SDKBackend {
	return &SDKBackend{client: client}
}

func (backend *SDKBackend) BuildImage(ctx context.Context, sourceDir, imageTag string) (string, error) {
	if backend == nil || backend.client == nil {
		return "", fmt.Errorf("docker client is required")
	}

	buildContext, err := tarDirectory(sourceDir)
	if err != nil {
		return "", fmt.Errorf("create docker build context: %w", err)
	}

	response, err := backend.client.ImageBuild(ctx, buildContext, build.ImageBuildOptions{
		Tags:        []string{imageTag},
		Remove:      true,
		ForceRemove: true,
	})
	if err != nil {
		return "", fmt.Errorf("build docker image: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read docker build output: %w", err)
	}
	return string(body), nil
}

func (backend *SDKBackend) EnsureNetwork(ctx context.Context, name string) error {
	if backend == nil || backend.client == nil {
		return fmt.Errorf("docker client is required")
	}
	_, err := backend.client.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:   "bridge",
		Internal: false,
		Labels: map[string]string{
			"porter.managed": "true",
		},
	})
	return err
}

func (backend *SDKBackend) ReplaceContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	if backend == nil || backend.client == nil {
		return "", fmt.Errorf("docker client is required")
	}

	if err := backend.client.ContainerRemove(ctx, spec.Name, container.RemoveOptions{Force: true}); err != nil {
		return "", fmt.Errorf("remove existing container: %w", err)
	}

	config := &container.Config{
		Image: spec.ImageTag,
		Env:   spec.Env,
		Labels: map[string]string{
			"porter.managed": "true",
		},
	}
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(spec.NetworkName),
		Privileged:  spec.Privileged,
		CapDrop:     spec.CapDrop,
		Resources: container.Resources{
			Memory:   spec.MemoryBytes,
			NanoCPUs: spec.NanoCPUs,
		},
		SecurityOpt: []string{"no-new-privileges:true"},
	}
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			spec.NetworkName: {},
		},
	}

	created, err := backend.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, spec.Name)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	if err := backend.client.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start container: %w", err)
	}
	return fmt.Sprintf("container %s started\n", created.ID), nil
}

func tarDirectory(root string) (io.Reader, error) {
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)

	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header.Name = strings.ReplaceAll(rel, string(filepath.Separator), "/")
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	}); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}
