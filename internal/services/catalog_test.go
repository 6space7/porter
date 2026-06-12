package services_test

import (
	"testing"
	"testing/fstest"

	"github.com/6space7/porter/internal/services"
)

func TestCatalogLoadsTemplatesAndSearches(t *testing.T) {
	fsys := fstest.MapFS{
		"postgres.yaml": {Data: []byte(`
slug: postgres
name: PostgreSQL
description: Relational database
category: database
docs_url: https://www.postgresql.org/docs/
logo: postgres
image: postgres:16-alpine
internal_port: 5432
exposed: false
variables:
  POSTGRES_DB: app
  POSTGRES_USER: porter
  POSTGRES_PASSWORD: ${SERVICE_PASSWORD_POSTGRES}
volumes:
  - name: data
    path: /var/lib/postgresql/data
provides:
  DATABASE_URL: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${SERVICE_HOST}:5432/${POSTGRES_DB}
healthcheck:
  command: pg_isready -U ${POSTGRES_USER}
`)},
	}

	catalog, err := services.LoadCatalog(fsys)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	matches := catalog.Search("database")
	if len(matches) != 1 {
		t.Fatalf("matches = %#v", matches)
	}
	tmpl, ok := catalog.Get("postgres")
	if !ok {
		t.Fatal("postgres template missing")
	}
	if tmpl.Name != "PostgreSQL" || tmpl.InternalPort != 5432 || tmpl.Exposed {
		t.Fatalf("template = %#v", tmpl)
	}
	if tmpl.Provides["DATABASE_URL"] == "" || tmpl.Variables["POSTGRES_PASSWORD"] == "" {
		t.Fatalf("template variables/provides = %#v %#v", tmpl.Variables, tmpl.Provides)
	}
	if len(tmpl.Volumes) != 1 || tmpl.Volumes[0].Path != "/var/lib/postgresql/data" {
		t.Fatalf("volumes = %#v", tmpl.Volumes)
	}
}

func TestCatalogRejectsDuplicateSlugs(t *testing.T) {
	fsys := fstest.MapFS{
		"one.yaml": {Data: []byte("slug: postgres\nname: One\nimage: postgres:16\ninternal_port: 5432\n")},
		"two.yaml": {Data: []byte("slug: postgres\nname: Two\nimage: postgres:16\ninternal_port: 5432\n")},
	}

	if _, err := services.LoadCatalog(fsys); err == nil {
		t.Fatal("expected duplicate slug to fail")
	}
}
