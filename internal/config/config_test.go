package config_test

import (
	"testing"

	"github.com/6space7/porter/internal/config"
)

func TestLoadEnablesManagedCaddyByDefault(t *testing.T) {
	t.Setenv("PORTER_MANAGE_CADDY", "")

	cfg := config.Load()

	if !cfg.ManageCaddy {
		t.Fatal("managed caddy should be enabled by default")
	}
}

func TestLoadCanDisableManagedCaddy(t *testing.T) {
	t.Setenv("PORTER_MANAGE_CADDY", "false")

	cfg := config.Load()

	if cfg.ManageCaddy {
		t.Fatal("managed caddy should be disabled")
	}
}

func TestLoadUsesInstallPathsByDefault(t *testing.T) {
	t.Setenv("PORTER_DATABASE_PATH", "")
	t.Setenv("PORTER_WORKSPACE_PATH", "")

	cfg := config.Load()

	if cfg.DatabasePath != "/var/lib/porter/porter.db" {
		t.Fatalf("database path = %q", cfg.DatabasePath)
	}
	if cfg.WorkspacePath != "/var/lib/porter/work" {
		t.Fatalf("workspace path = %q", cfg.WorkspacePath)
	}
	if cfg.MasterKeyPath != "/etc/porter/master.key" {
		t.Fatalf("master key path = %q", cfg.MasterKeyPath)
	}
}

func TestLoadReadsBootstrapTokenHash(t *testing.T) {
	t.Setenv("PORTER_BOOTSTRAP_TOKEN_HASH", "abc123")

	cfg := config.Load()

	if cfg.BootstrapTokenHash != "abc123" {
		t.Fatalf("bootstrap token hash = %q", cfg.BootstrapTokenHash)
	}
}

func TestLoadReadsBootstrapAdminPasswordFile(t *testing.T) {
	t.Setenv("PORTER_BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("PORTER_BOOTSTRAP_ADMIN_PASSWORD_FILE", "/etc/porter/initial-password")

	cfg := config.Load()

	if cfg.BootstrapAdminEmail != "admin@example.com" {
		t.Fatalf("bootstrap admin email = %q", cfg.BootstrapAdminEmail)
	}
	if cfg.BootstrapAdminPasswordFile != "/etc/porter/initial-password" {
		t.Fatalf("bootstrap admin password file = %q", cfg.BootstrapAdminPasswordFile)
	}
}
