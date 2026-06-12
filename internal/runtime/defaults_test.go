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
	if _, ok := stages.Builder.(dockerstage.Builder); !ok {
		t.Fatalf("builder = %T, want docker.Builder", stages.Builder)
	}
	if _, ok := stages.Runner.(dockerstage.Runner); !ok {
		t.Fatalf("runner = %T, want docker.Runner", stages.Runner)
	}
}
