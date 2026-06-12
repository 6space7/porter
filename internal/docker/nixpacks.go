package docker

import (
	"context"
	"fmt"
	"os/exec"
)

const defaultNixpacksBinary = "nixpacks"

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type NixpacksCLI struct {
	Runner CommandRunner
	Binary string
}

func (cli NixpacksCLI) BuildWithNixpacks(ctx context.Context, sourceDir, imageTag string) (string, error) {
	if sourceDir == "" {
		return "", fmt.Errorf("source dir is required")
	}
	if imageTag == "" {
		return "", fmt.Errorf("image tag is required")
	}

	runner := cli.Runner
	if runner == nil {
		runner = execRunner{}
	}
	binary := cli.Binary
	if binary == "" {
		binary = defaultNixpacksBinary
	}

	log, err := runner.Run(ctx,
		binary,
		"build", sourceDir, "--name", imageTag,
	)
	if err != nil {
		return log, fmt.Errorf("nixpacks build failed: %w", err)
	}
	return log, nil
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	output, err := command.CombinedOutput()
	return string(output), err
}
