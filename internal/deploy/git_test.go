package deploy_test

import (
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
