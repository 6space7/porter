package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Config struct {
	Path string
}

type DB struct {
	sql *sql.DB
}

func Open(ctx context.Context, cfg Config) (*DB, error) {
	path := cfg.Path
	if path == "" {
		path = ":memory:"
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)

	db := &DB{sql: conn}
	if err := db.Migrate(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) SQL() *sql.DB {
	return db.sql
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) Migrate(ctx context.Context) error {
	if _, err := db.sql.ExecContext(ctx, `
		create table if not exists schema_migrations (
			version integer primary key,
			applied_at text not null default current_timestamp
		)
	`); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}
		applied, err := db.migrationApplied(ctx, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := db.applyMigration(ctx, version, "migrations/"+entry.Name()); err != nil {
			return err
		}
	}

	if _, err := db.sql.ExecContext(ctx, `
		insert or ignore into servers (id, name, host, status)
		values ('local', 'Local', 'localhost', 'healthy')
	`); err != nil {
		return fmt.Errorf("seed local server: %w", err)
	}

	return nil
}

func (db *DB) migrationApplied(ctx context.Context, version int) (bool, error) {
	var count int
	if err := db.sql.QueryRowContext(ctx, `
		select count(*)
		from schema_migrations
		where version = ?
	`, version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %d: %w", version, err)
	}
	return count > 0, nil
}

func (db *DB) applyMigration(ctx context.Context, version int, path string) error {
	body, err := migrationFiles.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", path, err)
	}

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", version, err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return fmt.Errorf("apply migration %d: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, `
		insert into schema_migrations (version)
		values (?)
	`, version); err != nil {
		return fmt.Errorf("record migration %d: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", version, err)
	}
	tx = nil
	return nil
}

func migrationVersion(name string) (int, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("migration %q missing numeric prefix", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("migration %q has invalid version: %w", name, err)
	}
	return version, nil
}
