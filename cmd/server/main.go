package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/6space7/porter/internal/config"
	"github.com/6space7/porter/internal/lifecycle"
	appruntime "github.com/6space7/porter/internal/runtime"
)

func main() {
	if err := run(context.Background(), os.Args, os.Stdout, os.Stderr); err != nil {
		slog.Error("porter failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) > 1 {
		switch args[1] {
		case "backup":
			return runBackup(ctx, args[2:], stdout, stderr)
		case "update":
			return runUpdate(ctx, args[2:], stdout, stderr)
		case "serve":
			return runServer(ctx)
		default:
			return fmt.Errorf("unknown command %q", args[1])
		}
	}
	return runServer(ctx)
}

func runBackup(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg := config.Load()
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	databasePath := flags.String("database", cfg.DatabasePath, "path to porter SQLite database")
	backupDir := flags.String("backup-dir", defaultBackupDir(cfg.DatabasePath), "directory for porter database backups")
	if err := flags.Parse(args); err != nil {
		return err
	}
	result, err := lifecycle.BackupSQLite(ctx, lifecycle.BackupOptions{
		DatabasePath: *databasePath,
		BackupDir:    *backupDir,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Backup: %s (%d bytes)\n", result.Path, result.Bytes)
	return nil
}

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg := config.Load()
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	databasePath := flags.String("database", cfg.DatabasePath, "path to porter SQLite database")
	backupDir := flags.String("backup-dir", defaultBackupDir(cfg.DatabasePath), "directory for porter database backups")
	repo := flags.String("repo", "6space7/porter", "GitHub owner/repo for porter releases")
	version := flags.String("version", "", "release version to install, for example v1.2.3")
	goos := flags.String("goos", goruntime.GOOS, "release operating system")
	goarch := flags.String("goarch", goruntime.GOARCH, "release architecture")
	if err := flags.Parse(args); err != nil {
		return err
	}
	backup, err := lifecycle.BackupSQLite(ctx, lifecycle.BackupOptions{
		DatabasePath: *databasePath,
		BackupDir:    *backupDir,
	})
	if err != nil {
		return err
	}
	plan, err := lifecycle.PlanUpdate(lifecycle.UpdateOptions{
		Repo:    *repo,
		Version: *version,
		GOOS:    *goos,
		GOARCH:  *goarch,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Backup: %s (%d bytes)\n", backup.Path, backup.Bytes)
	fmt.Fprintf(stdout, "Release: %s\n", plan.URL)
	fmt.Fprintln(stdout, "Download and binary replacement will be completed by the release installer path.")
	return nil
}

func runServer(ctx context.Context) error {
	cfg := config.Load()
	db, handler, err := appruntime.NewHandler(ctx, cfg)
	if err != nil {
		return fmt.Errorf("runtime setup failed: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("database close failed", "error", err)
		}
	}()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting porter", "addr", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

func defaultBackupDir(databasePath string) string {
	if databasePath == "" {
		return "backups"
	}
	return filepath.Join(filepath.Dir(databasePath), "backups")
}
