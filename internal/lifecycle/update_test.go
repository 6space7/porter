package lifecycle_test

import (
	"testing"

	"github.com/6space7/porter/internal/lifecycle"
)

func TestPlanUpdateBuildsReleaseURL(t *testing.T) {
	plan, err := lifecycle.PlanUpdate(lifecycle.UpdateOptions{
		Repo:    "6space7/porter",
		Version: "v1.2.3",
		GOOS:    "linux",
		GOARCH:  "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://github.com/6space7/porter/releases/download/v1.2.3/porter-linux-amd64.tar.gz"
	if plan.URL != want {
		t.Fatalf("url = %q", plan.URL)
	}
	if plan.ArchiveName != "porter-linux-amd64.tar.gz" {
		t.Fatalf("archive = %q", plan.ArchiveName)
	}
}

func TestPlanUpdateValidatesInputs(t *testing.T) {
	if _, err := lifecycle.PlanUpdate(lifecycle.UpdateOptions{}); err == nil {
		t.Fatal("expected validation error")
	}
}
