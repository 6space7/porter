package deploy_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/6space7/porter/internal/deploy"
)

func TestValidateGitURLAcceptsHTTPSAndSSHGitHubStyleURLs(t *testing.T) {
	valid := []string{
		"https://github.com/example/repo.git",
		"https://github.com/example/repo",
		"ssh://git@github.com/example/repo.git",
		"git@github.com:example/repo.git",
	}

	for _, gitURL := range valid {
		if err := deploy.ValidateGitURL(gitURL); err != nil {
			t.Fatalf("ValidateGitURL(%q) unexpected error: %v", gitURL, err)
		}
	}
}

func TestValidateGitURLRejectsLocalPathsAndArgumentInjection(t *testing.T) {
	invalid := []string{
		"",
		"file:///etc/passwd",
		"/srv/repo",
		"../repo",
		"--upload-pack=touch /tmp/pwned",
		"https://github.com/example/repo.git --upload-pack=touch",
		"http://github.com/example/repo.git",
		"git://github.com/example/repo.git",
		"https://localhost/example/repo.git",
		"https://127.0.0.1/example/repo.git",
	}

	for _, gitURL := range invalid {
		if err := deploy.ValidateGitURL(gitURL); err == nil {
			t.Fatalf("ValidateGitURL(%q) expected error", gitURL)
		}
	}
}

func TestGitClonerRunsCloneWithArgumentArray(t *testing.T) {
	runner := &fakeCommandRunner{output: "cloned\n"}
	cloner := deploy.GitCloner{
		Root:   t.TempDir(),
		Runner: runner,
	}

	result, err := cloner.Clone(context.Background(), deploy.CloneRequest{
		AppID:        "app_1",
		DeploymentID: "dep_1",
		GitURL:       "https://github.com/example/web.git",
		Branch:       "main",
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if result.Log != "cloned\n" {
		t.Fatalf("log = %q, want cloned newline", result.Log)
	}
	if result.SourceDir == "" {
		t.Fatal("source dir is empty")
	}
	if runner.name != "git" {
		t.Fatalf("command name = %q, want git", runner.name)
	}

	wantPrefix := []string{"clone", "--depth", "1", "--branch", "main", "--", "https://github.com/example/web.git"}
	if len(runner.args) != len(wantPrefix)+1 {
		t.Fatalf("args = %#v", runner.args)
	}
	if !reflect.DeepEqual(runner.args[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("args prefix = %#v, want %#v", runner.args[:len(wantPrefix)], wantPrefix)
	}
	if runner.args[len(runner.args)-1] == "" {
		t.Fatalf("destination arg is empty: %#v", runner.args)
	}
}

func TestGitClonerRejectsUnsafeInputsBeforeRunningCommand(t *testing.T) {
	tests := []deploy.CloneRequest{
		{AppID: "app_1", DeploymentID: "dep_1", GitURL: "file:///etc/passwd", Branch: "main"},
		{AppID: "app_1", DeploymentID: "dep_1", GitURL: "https://github.com/example/web.git", Branch: "--upload-pack=touch"},
		{AppID: "../escape", DeploymentID: "dep_1", GitURL: "https://github.com/example/web.git", Branch: "main"},
	}

	for _, req := range tests {
		runner := &fakeCommandRunner{}
		cloner := deploy.GitCloner{
			Root:   t.TempDir(),
			Runner: runner,
		}

		if _, err := cloner.Clone(context.Background(), req); err == nil {
			t.Fatalf("Clone(%#v) expected error", req)
		}
		if runner.called {
			t.Fatalf("Clone(%#v) ran command before rejecting input", req)
		}
	}
}

type fakeCommandRunner struct {
	called bool
	name   string
	args   []string
	dir    string
	output string
	err    error
}

func (runner *fakeCommandRunner) Run(_ context.Context, name string, args []string, dir string) ([]byte, error) {
	runner.called = true
	runner.name = name
	runner.args = append([]string(nil), args...)
	runner.dir = dir
	return []byte(runner.output), runner.err
}
