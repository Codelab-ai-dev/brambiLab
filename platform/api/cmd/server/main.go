// Command server runs the BrambiLab API.
//
//	server [serve]    start the HTTP API (default)
//	server migrate    apply database migrations and exit (release task)
//	server healthcheck probe the local liveness endpoint (for distroless images)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/config"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/health"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if err := run(cmd, logger); err != nil {
		logger.Error("command failed", "command", cmd, "error", err)
		os.Exit(1)
	}
}

func run(cmd string, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "serve":
		return serve(ctx, cfg, logger)
	case "migrate":
		if err := cfg.RequireDatabase(); err != nil {
			return err
		}
		return migrations.Up(ctx, cfg.DatabaseURL, logger)
	case "healthcheck":
		return healthcheck(cfg)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if err := cfg.RequireDatabase(); err != nil {
		return err
	}
	// The pool connects lazily, so the API starts and reports "not ready" while PostgreSQL is down.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure database pool: %w", err)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.NewRouter(logger, health.Handler{DB: pool, Logger: logger}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", srv.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func healthcheck(cfg config.Config) error {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/health/live", cfg.Port))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("liveness returned %d", resp.StatusCode)
	}
	return nil
}
