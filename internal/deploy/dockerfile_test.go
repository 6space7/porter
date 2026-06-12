package deploy_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/6space7/porter/internal/deploy"
)

func TestDockerfilePortDetectorReadsFirstExposePort(t *testing.T) {
	sourceDir := t.TempDir()
	err := os.WriteFile(filepath.Join(sourceDir, "Dockerfile"), []byte(`
FROM node:22-alpine
EXPOSE 8080/tcp 9090
CMD ["node", "server.js"]
`), 0600)
	if err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}

	port, ok, err := deploy.DockerfilePortDetector{}.DetectPort(context.Background(), sourceDir)
	if err != nil {
		t.Fatalf("detect port: %v", err)
	}
	if !ok || port != 8080 {
		t.Fatalf("port = %d/%v, want 8080/true", port, ok)
	}
}

func TestDockerfilePortDetectorIgnoresMissingDockerfile(t *testing.T) {
	port, ok, err := deploy.DockerfilePortDetector{}.DetectPort(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("detect port: %v", err)
	}
	if ok || port != 0 {
		t.Fatalf("port = %d/%v, want 0/false", port, ok)
	}
}
