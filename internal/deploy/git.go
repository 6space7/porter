package deploy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	scpLikeGitURLPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+@([A-Za-z0-9.-]+):[A-Za-z0-9._~/-]+(?:\.git)?$`)
	gitBranchPattern     = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, dir string) ([]byte, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args []string, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

type GitCloner struct {
	Root   string
	Runner CommandRunner
}

func (cloner GitCloner) Clone(ctx context.Context, req CloneRequest) (string, error) {
	if err := ValidateGitURL(req.GitURL); err != nil {
		return "", err
	}
	if err := ValidateGitBranch(req.Branch); err != nil {
		return "", err
	}

	root := cloner.Root
	if root == "" {
		root = os.TempDir()
	}
	dest, err := safeJoin(root, req.AppID, req.DeploymentID, "source")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return "", fmt.Errorf("create clone parent: %w", err)
	}

	runner := cloner.Runner
	if runner == nil {
		runner = ExecCommandRunner{}
	}

	output, err := runner.Run(ctx, "git", []string{
		"clone",
		"--depth",
		"1",
		"--branch",
		req.Branch,
		"--",
		req.GitURL,
		dest,
	}, root)
	if err != nil {
		return string(output), fmt.Errorf("git clone: %w", err)
	}
	return string(output), nil
}

func ValidateGitURL(gitURL string) error {
	if gitURL == "" {
		return fmt.Errorf("git URL is required")
	}
	if hasWhitespace(gitURL) {
		return fmt.Errorf("git URL must not contain whitespace")
	}
	if strings.HasPrefix(gitURL, "-") || strings.HasPrefix(gitURL, "/") || strings.HasPrefix(gitURL, ".") {
		return fmt.Errorf("git URL must not look like a local path or flag")
	}

	if matches := scpLikeGitURLPattern.FindStringSubmatch(gitURL); matches != nil {
		return validateGitHost(matches[1])
	}

	parsed, err := url.Parse(gitURL)
	if err != nil {
		return fmt.Errorf("parse git URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "ssh" {
		return fmt.Errorf("git URL scheme must be https or ssh")
	}
	if parsed.Hostname() == "" || parsed.Path == "" || parsed.Path == "/" {
		return fmt.Errorf("git URL must include a host and repository path")
	}
	if strings.HasPrefix(parsed.Path, "/-") {
		return fmt.Errorf("git URL path must not look like a flag")
	}
	return validateGitHost(parsed.Hostname())
}

func ValidateGitBranch(branch string) error {
	if len(branch) == 0 || len(branch) > 128 {
		return fmt.Errorf("branch name must be 1-128 characters")
	}
	if !gitBranchPattern.MatchString(branch) {
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

func validateGitHost(host string) error {
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return fmt.Errorf("git URL host must not be localhost")
	}

	if ip := net.ParseIP(lower); ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			return fmt.Errorf("git URL host must not be local or private")
		}
	}
	return nil
}

func hasWhitespace(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func safeJoin(root string, parts ...string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}

	allParts := append([]string{absRoot}, parts...)
	target := filepath.Clean(filepath.Join(allParts...))
	rel, err := filepath.Rel(absRoot, target)
	if err != nil {
		return "", fmt.Errorf("resolve target: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("target path escapes root")
	}
	return target, nil
}
