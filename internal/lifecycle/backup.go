package lifecycle

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BackupOptions struct {
	DatabasePath string
	BackupDir    string
	Now          func() time.Time
}

type BackupResult struct {
	Path  string
	Bytes int64
}

func BackupSQLite(ctx context.Context, opts BackupOptions) (BackupResult, error) {
	if strings.TrimSpace(opts.DatabasePath) == "" {
		return BackupResult{}, fmt.Errorf("database path is required")
	}
	if strings.TrimSpace(opts.BackupDir) == "" {
		return BackupResult{}, fmt.Errorf("backup dir is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if err := ctx.Err(); err != nil {
		return BackupResult{}, err
	}
	if err := os.MkdirAll(opts.BackupDir, 0o700); err != nil {
		return BackupResult{}, fmt.Errorf("create backup dir: %w", err)
	}

	source, err := os.Open(opts.DatabasePath)
	if err != nil {
		return BackupResult{}, fmt.Errorf("open database: %w", err)
	}
	defer source.Close()

	name := "porter-" + now().UTC().Format("20060102-150405") + ".db"
	path := filepath.Join(opts.BackupDir, name)
	target, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return BackupResult{}, fmt.Errorf("create backup: %w", err)
	}
	defer target.Close()

	bytes, err := io.Copy(target, source)
	if err != nil {
		return BackupResult{}, fmt.Errorf("copy database: %w", err)
	}
	if err := target.Sync(); err != nil {
		return BackupResult{}, fmt.Errorf("sync backup: %w", err)
	}
	return BackupResult{Path: path, Bytes: bytes}, nil
}
