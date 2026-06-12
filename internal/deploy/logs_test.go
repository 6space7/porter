package deploy_test

import (
	"strings"
	"testing"

	"github.com/6space7/porter/internal/deploy"
)

func TestRedactSecretsScrubsKnownValues(t *testing.T) {
	log := strings.Join([]string{
		"starting build",
		"DATABASE_URL=postgres://porter:secret@db:5432/app",
		"API_KEY=abc123",
		"done",
	}, "\n")

	redacted := deploy.RedactSecrets(log, []string{
		"postgres://porter:secret@db:5432/app",
		"abc123",
	})

	if strings.Contains(redacted, "postgres://porter:secret@db:5432/app") {
		t.Fatalf("database URL leaked in redacted log: %s", redacted)
	}
	if strings.Contains(redacted, "abc123") {
		t.Fatalf("API key leaked in redacted log: %s", redacted)
	}
	if strings.Count(redacted, "[REDACTED]") != 2 {
		t.Fatalf("redacted marker count = %d, want 2 in %s", strings.Count(redacted, "[REDACTED]"), redacted)
	}
}

func TestRedactSecretsIgnoresEmptyValues(t *testing.T) {
	log := "line one\nline two"

	redacted := deploy.RedactSecrets(log, []string{"", "line two"})

	if strings.HasPrefix(redacted, "[REDACTED]") {
		t.Fatalf("empty secret should not redact every boundary: %s", redacted)
	}
	if strings.Contains(redacted, "line two") {
		t.Fatalf("expected non-empty secret to be redacted: %s", redacted)
	}
}
