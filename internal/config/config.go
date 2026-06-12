package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr      string
	DatabasePath  string
	PublicIP      string
	WorkspacePath string
	CaddyAskURL   string
	ManageCaddy   bool
}

func Load() Config {
	return Config{
		HTTPAddr:      envOrDefault("PORTER_HTTP_ADDR", ":8080"),
		DatabasePath:  envOrDefault("PORTER_DATABASE_PATH", "porter.db"),
		PublicIP:      os.Getenv("PORTER_PUBLIC_IP"),
		WorkspacePath: envOrDefault("PORTER_WORKSPACE_PATH", "data/work"),
		CaddyAskURL:   envOrDefault("PORTER_CADDY_ASK_URL", "http://127.0.0.1:8080/api/v1/caddy/ask"),
		ManageCaddy:   boolEnvOrDefault("PORTER_MANAGE_CADDY", true),
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
