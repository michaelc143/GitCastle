package config

import "os"

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	RepositoryRoot string
}

func FromEnv() Config {
	return Config{
		HTTPAddr:       valueOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:    valueOrDefault("DATABASE_URL", "postgres://gitcastle:gitcastle@localhost:5432/gitcastle?sslmode=disable"),
		RepositoryRoot: valueOrDefault("REPOSITORY_ROOT", "./var/repositories"),
	}
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
