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
