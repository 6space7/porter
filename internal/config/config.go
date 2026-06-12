package config

import "os"

type Config struct {
	HTTPAddr     string
	DatabasePath string
}

func Load() Config {
	return Config{
		HTTPAddr:     envOrDefault("PORTER_HTTP_ADDR", ":8080"),
		DatabasePath: envOrDefault("PORTER_DATABASE_PATH", "porter.db"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
