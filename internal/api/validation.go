package api

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	appNamePattern     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	domainLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	branchPattern      = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	envKeyPattern      = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,127}$`)
)

func ValidateAppName(name string) error {
	if !appNamePattern.MatchString(name) {
		return fmt.Errorf("app name must be 1-63 lowercase letters, numbers, or hyphens")
	}
	return nil
}

func ValidateProjectName(name string) error {
	if !appNamePattern.MatchString(name) {
		return fmt.Errorf("project name must be 1-63 lowercase letters, numbers, or hyphens")
	}
	return nil
}

func ValidateBuildType(buildType string) error {
	switch buildType {
	case "dockerfile", "nixpacks":
		return nil
	default:
		return fmt.Errorf("build type must be dockerfile or nixpacks")
	}
}

func ValidateBranchName(branch string) error {
	if len(branch) == 0 || len(branch) > 128 {
		return fmt.Errorf("branch name must be 1-128 characters")
	}
	if !branchPattern.MatchString(branch) {
		return fmt.Errorf("branch name contains unsupported characters")
	}
	if strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, "/") || strings.HasPrefix(branch, ".") {
		return fmt.Errorf("branch name has unsafe prefix")
	}
	if strings.Contains(branch, "..") || strings.Contains(branch, "//") || strings.Contains(branch, "@{") {
		return fmt.Errorf("branch name contains unsafe sequence")
	}
	if strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("branch name has unsafe suffix")
	}
	return nil
}

func ValidateDomainName(domain string) error {
	if len(domain) == 0 || len(domain) > 253 {
		return fmt.Errorf("domain must be 1-253 characters")
	}
	if domain != strings.ToLower(domain) {
		return fmt.Errorf("domain must be lowercase")
	}
	if strings.HasSuffix(domain, ".") || strings.HasPrefix(domain, ".") {
		return fmt.Errorf("domain must not start or end with a dot")
	}
	if domain == "localhost" || !strings.Contains(domain, ".") {
		return fmt.Errorf("domain must be a fully qualified public hostname")
	}

	for _, label := range strings.Split(domain, ".") {
		if !domainLabelPattern.MatchString(label) {
			return fmt.Errorf("domain label %q is invalid", label)
		}
	}
	return nil
}

func ValidateEnvKey(key string) error {
	if !envKeyPattern.MatchString(key) {
		return fmt.Errorf("env var key must be uppercase letters, numbers, and underscores")
	}
	return nil
}
