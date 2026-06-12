package lifecycle_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/6space7/porter/internal/lifecycle"
)

func TestBackupSQLiteCopiesDatabaseWithTimestamp(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "porter.db")
	if err := os.WriteFile(dbPath, []byte("sqlite-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := lifecycle.BackupSQLite(context.Background(), lifecycle.BackupOptions{
		DatabasePath: dbPath,
		BackupDir:    filepath.Join(dir, "backups"),
		Now:          func() time.Time { return time.Date(2026, 6, 12, 15, 30, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.Path) != "porter-20260612-153000.db" {
		t.Fatalf("backup path = %s", result.Path)
	}
	if result.Bytes != int64(len("sqlite-bytes")) {
		t.Fatalf("backup bytes = %d", result.Bytes)
	}
	body, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "sqlite-bytes" {
		t.Fatalf("backup body = %q", string(body))
	}
}

func TestBackupSQLiteValidatesRequiredPaths(t *testing.T) {
	if _, err := lifecycle.BackupSQLite(context.Background(), lifecycle.BackupOptions{}); err == nil {
		t.Fatal("expected validation error")
	}
}
