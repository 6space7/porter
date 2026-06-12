package store_test

import (
	"context"
	"testing"

	"github.com/6space7/porter/internal/store"
)

func TestOpenMigratesInitialSchema(t *testing.T) {
	db, err := store.Open(context.Background(), store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	rows, err := db.SQL().QueryContext(context.Background(), `
		select name
		from sqlite_master
		where type = 'table'
	`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		tables[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}

	required := []string{
		"schema_migrations",
		"servers",
		"projects",
		"apps",
		"domains",
		"deployments",
		"env_vars",
		"services",
		"tokens",
		"users",
	}
	for _, name := range required {
		if !tables[name] {
			t.Fatalf("missing required table %q; got %#v", name, tables)
		}
	}
}

func TestOpenSeedsLocalServerIdempotently(t *testing.T) {
	db, err := store.Open(context.Background(), store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("run migrations again: %v", err)
	}

	var count int
	var name, host, status string
	err = db.SQL().QueryRowContext(context.Background(), `
		select count(*), max(name), max(host), max(status)
		from servers
		where id = 'local'
	`).Scan(&count, &name, &host, &status)
	if err != nil {
		t.Fatalf("query local server: %v", err)
	}

	if count != 1 {
		t.Fatalf("local server rows = %d, want 1", count)
	}
	if name != "Local" || host != "localhost" || status != "healthy" {
		t.Fatalf("local server = (%q, %q, %q), want (Local, localhost, healthy)", name, host, status)
	}
}
