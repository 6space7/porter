package runtime

import (
	"testing"

	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/deploy"
	dockerstage "github.com/6space7/porter/internal/docker"
)

func TestDefaultDeploymentStagesUseGitAndDockerBackends(t *testing.T) {
	stages, err := defaultDeploymentStages(config.Config{WorkspacePath: "C:/porter/work"})
	if err != nil {
		t.Fatalf("default stages: %v", err)
	}

	cloner, ok := stages.Cloner.(deploy.GitCloner)
	if !ok {
		t.Fatalf("cloner = %T, want deploy.GitCloner", stages.Cloner)
	}
	if cloner.Root != "C:/porter/work" {
		t.Fatalf("cloner root = %q", cloner.Root)
	}
	builder, ok := stages.Builder.(dockerstage.Builder)
	if !ok {
		t.Fatalf("builder = %T, want docker.Builder", stages.Builder)
	}
	if builder.Nixpacks == nil {
		t.Fatal("builder must include a nixpacks backend")
	}
	if _, ok := stages.Runner.(dockerstage.Runner); !ok {
		t.Fatalf("runner = %T, want docker.Runner", stages.Runner)
	}
	if stages.ImagePruner == nil {
		t.Fatal("image pruner must be configured")
	}
}
