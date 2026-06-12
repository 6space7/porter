package templates_test

import (
	"testing"

	"github.com/6space7/porter/internal/services"
	"github.com/6space7/porter/templates"
)

func TestEmbeddedCatalogIncludesPhaseFourTemplates(t *testing.T) {
	catalog, err := services.LoadCatalog(templates.FS())
	if err != nil {
		t.Fatalf("load embedded templates: %v", err)
	}

	all := catalog.All()
	if len(all) < 10 {
		t.Fatalf("template count = %d, want at least 10", len(all))
	}
	for _, slug := range []string{"postgres", "mysql", "redis", "mongodb", "n8n", "uptime-kuma", "wordpress", "ghost", "minio", "plausible"} {
		if _, ok := catalog.Get(slug); !ok {
			t.Fatalf("template %q missing", slug)
		}
	}
}
