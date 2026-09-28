// Package config reads runtime configuration from the environment (web-v1.md §14).
// PostgreSQL settings are read by internal/db from the PG* variables.
package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port int
	// DatabaseURL is optional; PG* variables are preferred (see internal/db).
	DatabaseURL string
}

func Load() (Config, error) {
	cfg := Config{Port: 8080, DatabaseURL: os.Getenv("DATABASE_URL")}
	if v := os.Getenv("API_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, errors.New("API_PORT must be a TCP port number")
		}
		cfg.Port = port
	}
	return cfg, nil
}
