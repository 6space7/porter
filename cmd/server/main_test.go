package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBackupCommand(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "porter.db")
	if err := os.WriteFile(dbPath, []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{
		"porter",
		"backup",
		"--database", dbPath,
		"--backup-dir", filepath.Join(dir, "backups"),
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run backup: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Backup:") || !strings.Contains(stdout.String(), ".db") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunUpdateCommandBacksUpAndPlansRelease(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "porter.db")
	if err := os.WriteFile(dbPath, []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{
		"porter",
		"update",
		"--database", dbPath,
		"--backup-dir", filepath.Join(dir, "backups"),
		"--repo", "6space7/porter",
		"--version", "v1.2.3",
		"--goos", "linux",
		"--goarch", "amd64",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run update: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Backup:") {
		t.Fatalf("stdout missing backup: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "https://github.com/6space7/porter/releases/download/v1.2.3/porter-linux-amd64.tar.gz") {
		t.Fatalf("stdout missing release URL: %q", stdout.String())
	}
}
