package deploy

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var scpLikeGitURLPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+@([A-Za-z0-9.-]+):[A-Za-z0-9._~/-]+(?:\.git)?$`)

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
