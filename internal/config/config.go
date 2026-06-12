package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/6space7/porter/internal/install"
)

type Config struct {
	HTTPAddr                   string
	DatabasePath               string
	PublicIP                   string
	WorkspacePath              string
	CaddyAskURL                string
	ManageCaddy                bool
	BootstrapTokenHash         string
	MasterKeyPath              string
	BootstrapAdminEmail        string
	BootstrapAdminPasswordFile string
}

func Load() Config {
	paths := install.DefaultPaths()
	return Config{
		HTTPAddr:                   envOrDefault("PORTER_HTTP_ADDR", ":8080"),
		DatabasePath:               envOrDefault("PORTER_DATABASE_PATH", paths.DatabasePath),
		PublicIP:                   os.Getenv("PORTER_PUBLIC_IP"),
		WorkspacePath:              envOrDefault("PORTER_WORKSPACE_PATH", paths.WorkspacePath),
		CaddyAskURL:                envOrDefault("PORTER_CADDY_ASK_URL", "http://127.0.0.1:8080/api/v1/caddy/ask"),
		ManageCaddy:                boolEnvOrDefault("PORTER_MANAGE_CADDY", true),
		BootstrapTokenHash:         os.Getenv("PORTER_BOOTSTRAP_TOKEN_HASH"),
		MasterKeyPath:              envOrDefault("PORTER_MASTER_KEY_PATH", paths.MasterKeyPath),
		BootstrapAdminEmail:        envOrDefault("PORTER_BOOTSTRAP_ADMIN_EMAIL", "admin@porter.local"),
		BootstrapAdminPasswordFile: os.Getenv("PORTER_BOOTSTRAP_ADMIN_PASSWORD_FILE"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func boolEnvOrDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
