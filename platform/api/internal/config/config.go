// Package config reads runtime configuration from the environment (web-v1.md §14).
// PostgreSQL settings are read by internal/db from the PG* variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/media"
)

type Config struct {
	Port int
	// DatabaseURL is optional; PG* variables are preferred (see internal/db).
	DatabaseURL string
	// MediaRoot is the persistent media volume (web-v1.md §12.1).
	MediaRoot   string
	MediaLimits media.Limits
}

func Load() (Config, error) {
	cfg := Config{Port: 8080, DatabaseURL: os.Getenv("DATABASE_URL"), MediaRoot: "/data/media", MediaLimits: media.DefaultLimits()}
	if v := os.Getenv("STORAGE_LOCAL_ROOT"); v != "" {
		cfg.MediaRoot = v
	}
	for env, dst := range map[string]*int64{
		"MEDIA_MAX_IMAGE_BYTES": &cfg.MediaLimits.Image, "MEDIA_MAX_VIDEO_BYTES": &cfg.MediaLimits.Video,
		"MEDIA_MAX_RESOURCE_BYTES": &cfg.MediaLimits.Resource,
	} {
		if v := os.Getenv(env); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive number of bytes", env)
			}
			*dst = n
		}
	}
	if v := os.Getenv("MEDIA_FREE_RESERVE_BYTES"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return Config{}, errors.New("MEDIA_FREE_RESERVE_BYTES must be a number of bytes")
		}
		cfg.MediaLimits.FreeReserve = n
	}
	if v := os.Getenv("API_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, errors.New("API_PORT must be a TCP port number")
		}
		cfg.Port = port
	}
	return cfg, nil
}
