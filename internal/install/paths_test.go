package install_test

import (
	"testing"

	"github.com/6space7/porter/internal/install"
)

func TestDefaultPaths(t *testing.T) {
	paths := install.DefaultPaths()

	if paths.ConfigDir != "/etc/porter" {
		t.Fatalf("config dir = %q", paths.ConfigDir)
	}
	if paths.DataDir != "/var/lib/porter" {
		t.Fatalf("data dir = %q", paths.DataDir)
	}
	if paths.DatabasePath != "/var/lib/porter/porter.db" {
		t.Fatalf("database path = %q", paths.DatabasePath)
	}
	if paths.MasterKeyPath != "/etc/porter/master.key" {
		t.Fatalf("master key path = %q", paths.MasterKeyPath)
	}
}
