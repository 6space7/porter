package docker_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/6space7/porter/internal/deploy"
	dockerstage "github.com/6space7/porter/internal/docker"
)

func TestBuilderBuildsDeterministicImageTagFromSourceDir(t *testing.T) {
	images := &fakeImageBackend{log: "docker build log\n"}
	builder := dockerstage.Builder{Images: images}

	result, err := builder.Build(context.Background(), deploy.BuildRequest{
		AppID:        "app_1",
		DeploymentID: "dep_1",
		SourceDir:    "work/app_1/dep_1/source",
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
	if images.sourceDir != "work/app_1/dep_1/source" || images.imageTag != "porter/app_1:dep_1" {
		t.Fatalf("backend call = source %q tag %q", images.sourceDir, images.imageTag)
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
	if containers.networkName != "porter-app_1" {
		t.Fatalf("network = %q, want porter-app_1", containers.networkName)
	}
	spec := containers.spec
	if spec.Name != "porter-app_1" || spec.ImageTag != "porter/app_1:dep_1" || spec.NetworkName != "porter-app_1" {
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
