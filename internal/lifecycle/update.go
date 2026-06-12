package lifecycle

import (
	"fmt"
	"strings"
)

type UpdateOptions struct {
	Repo    string
	Version string
	GOOS    string
	GOARCH  string
}

type UpdatePlan struct {
	URL         string
	ArchiveName string
}

func PlanUpdate(opts UpdateOptions) (UpdatePlan, error) {
	repo := strings.Trim(strings.TrimSpace(opts.Repo), "/")
	version := strings.TrimSpace(opts.Version)
	goos := strings.TrimSpace(opts.GOOS)
	goarch := strings.TrimSpace(opts.GOARCH)
	if repo == "" {
		return UpdatePlan{}, fmt.Errorf("repo is required")
	}
	if version == "" {
		return UpdatePlan{}, fmt.Errorf("version is required")
	}
	if goos == "" {
		return UpdatePlan{}, fmt.Errorf("goos is required")
	}
	if goarch == "" {
		return UpdatePlan{}, fmt.Errorf("goarch is required")
	}
	archive := fmt.Sprintf("porter-%s-%s.tar.gz", goos, goarch)
	return UpdatePlan{
		URL:         fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, archive),
		ArchiveName: archive,
	}, nil
}
