package api_test

import (
	"testing"

	"github.com/6space7/porter/internal/api"
)

func TestValidateAppName(t *testing.T) {
	valid := []string{"app", "my-app-1", "a1"}
	for _, name := range valid {
		if err := api.ValidateAppName(name); err != nil {
			t.Fatalf("ValidateAppName(%q) unexpected error: %v", name, err)
		}
	}

	invalid := []string{"", "App", "-app", "app-", "my_app", "app.example", longString("a", 64)}
	for _, name := range invalid {
		if err := api.ValidateAppName(name); err == nil {
			t.Fatalf("ValidateAppName(%q) expected error", name)
		}
	}
}

func TestValidateBuildType(t *testing.T) {
	for _, buildType := range []string{"dockerfile", "nixpacks"} {
		if err := api.ValidateBuildType(buildType); err != nil {
			t.Fatalf("ValidateBuildType(%q) unexpected error: %v", buildType, err)
		}
	}

	for _, buildType := range []string{"", "docker", "compose", "Dockerfile"} {
		if err := api.ValidateBuildType(buildType); err == nil {
			t.Fatalf("ValidateBuildType(%q) expected error", buildType)
		}
	}
}

func TestValidateBranchName(t *testing.T) {
	valid := []string{"main", "feature/ship-it", "release-2026.06"}
	for _, branch := range valid {
		if err := api.ValidateBranchName(branch); err != nil {
			t.Fatalf("ValidateBranchName(%q) unexpected error: %v", branch, err)
		}
	}

	invalid := []string{"", "--upload-pack=touch", "../main", "feature//bad", "bad branch", "bad.lock", "main@{1}", longString("a", 129)}
	for _, branch := range invalid {
		if err := api.ValidateBranchName(branch); err == nil {
			t.Fatalf("ValidateBranchName(%q) expected error", branch)
		}
	}
}

func TestValidateDomainName(t *testing.T) {
	valid := []string{"app.example.com", "my-app.203-0-113-42.sslip.io"}
	for _, domain := range valid {
		if err := api.ValidateDomainName(domain); err != nil {
			t.Fatalf("ValidateDomainName(%q) unexpected error: %v", domain, err)
		}
	}

	invalid := []string{"", "localhost", "bad_domain.com", "-bad.example.com", "bad-.example.com", ".example.com", "example.com.", longString("a", 254)}
	for _, domain := range invalid {
		if err := api.ValidateDomainName(domain); err == nil {
			t.Fatalf("ValidateDomainName(%q) expected error", domain)
		}
	}
}

func longString(value string, count int) string {
	out := ""
	for i := 0; i < count; i++ {
		out += value
	}
	return out
}
