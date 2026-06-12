package config

import "os"

type Config struct {
	HTTPAddr string
}

func Load() Config {
	return Config{
		HTTPAddr: envOrDefault("PORTER_HTTP_ADDR", ":8080"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
