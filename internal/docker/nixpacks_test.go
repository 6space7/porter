package docker_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	dockerstage "github.com/6space7/porter/internal/docker"
)

func TestNixpacksCLIBuildsImageWithHostCLI(t *testing.T) {
	runner := &fakeCommandRunner{output: "nixpacks built\n"}
	backend := dockerstage.NixpacksCLI{Runner: runner}

	log, err := backend.BuildWithNixpacks(context.Background(), "/var/lib/porter/work/app/source", "porter/app:dep")
	if err != nil {
		t.Fatalf("build with nixpacks: %v", err)
	}

	if log != "nixpacks built\n" {
		t.Fatalf("log = %q", log)
	}
	if runner.name != "nixpacks" {
		t.Fatalf("command = %q, want nixpacks", runner.name)
	}
	wantArgs := []string{
		"build", "/var/lib/porter/work/app/source", "--name", "porter/app:dep",
	}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
	}
}

func TestNixpacksCLIReturnsOutputWithCommandFailure(t *testing.T) {
	runner := &fakeCommandRunner{output: "nixpacks failed\n", err: errors.New("exit status 1")}
	backend := dockerstage.NixpacksCLI{Runner: runner}

	log, err := backend.BuildWithNixpacks(context.Background(), "/src", "porter/app:dep")
	if err == nil {
		t.Fatal("expected nixpacks failure")
	}
	if log != "nixpacks failed\n" {
		t.Fatalf("log = %q", log)
	}
	if !strings.Contains(err.Error(), "nixpacks build failed") {
		t.Fatalf("err = %v", err)
	}
}

type fakeCommandRunner struct {
	name   string
	args   []string
	output string
	err    error
}

func (runner *fakeCommandRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	runner.name = name
	runner.args = append([]string(nil), args...)
	return runner.output, runner.err
}
