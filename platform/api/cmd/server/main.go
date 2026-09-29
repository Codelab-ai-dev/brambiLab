// Command server runs the BrambiLab API.
//
//	server [serve]    start the HTTP API (default)
//	server migrate    apply database migrations and exit (release task)
//	server healthcheck probe the local liveness endpoint (for distroless images)
//	server preflight  validate the production configuration (names only, never values)
//	server ops-check  run the operational checks (exit 1 if any is critical)
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

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/config"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/contact"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/content"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/db"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/health"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/media"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/ops"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/preflight"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/public"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/publishing"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/site"
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
		dbCfg, err := db.Config(cfg.DatabaseURL)
		if err != nil {
			return err
		}
		return migrations.Up(ctx, dbCfg.ConnConfig, logger)
	case "healthcheck":
		return healthcheck(cfg)
	case "ops-check":
		// Operational checks for cron/monitoring: exit 1 on any critical check.
		return opsCheck(ctx, cfg, logger)
	case "preflight":
		// Production configuration check before a release; prints names, never values.
		results := preflight.Check(os.Getenv, preflight.Writable)
		for _, r := range results {
			fmt.Printf("%-4s %s: %s\n", r.Level, r.Name, r.Message)
		}
		if preflight.Failed(results) {
			return errors.New("preflight failed")
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	authCfg, err := auth.LoadConfig(os.Getenv)
	if err != nil {
		return err
	}
	if !authCfg.Enabled() {
		logger.Warn("owner login disabled: set ADMIN_GITHUB_USER_ID, GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET")
	}
	dbCfg, err := db.Config(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	// The pool connects lazily, so the API starts and reports "not ready" while PostgreSQL is down.
	pool, err := pgxpool.NewWithConfig(ctx, dbCfg)
	if err != nil {
		return fmt.Errorf("configure database pool: %w", err)
	}
	defer pool.Close()

	// Maintenance (backups) and BACKGROUND_JOBS=off (isolated restores) pause every background job.
	maintenance := ops.NewMaintenance(pool, logger, os.Getenv)
	if maintenance.BackgroundDisabled() {
		logger.Warn("background jobs disabled (BACKGROUND_JOBS=off)")
	}
	go maintenance.Run(ctx)
	gate := ops.Gate(maintenance.Allow)

	authHandler := auth.NewHandler(authCfg, pool, logger)
	authHandler.Gate = gate
	go authHandler.RunCleanup(ctx, time.Hour)

	storage, err := media.NewLocal(cfg.MediaRoot)
	if err != nil {
		return err
	}
	mediaHandler := media.NewHandler(media.NewStore(pool), storage, cfg.MediaLimits, logger,
		authHandler.RequireOwner, auth.Actor, authHandler.IsOwner)
	mediaHandler.Gate = gate
	go mediaHandler.RunCleanup(ctx, 30*time.Minute, 2*time.Hour)

	// Contact (WEB-007): disabled unless fully configured; the reason is logged, never the key.
	contactCfg := contact.LoadConfig(os.Getenv)
	contactStore := contact.NewStore(pool, contactCfg)
	contactStore.Gate = gate
	contactHandler := contact.NewHandler(contactStore, contactCfg, logger)
	if contactCfg.Enabled {
		go contact.NewWorker(contactStore, contact.NewResend(contactCfg.APIURL, contactCfg.APIKey), logger).Run(ctx, contact.WorkerTick)
	} else {
		logger.Warn("contact form disabled", "reasons", contactCfg.Problems)
		// Retention still applies to messages stored while it was enabled.
		go contactStore.RunPurge(ctx, time.Hour, logger)
	}

	checker := ops.NewChecker(pool, cfg.MediaRoot, contactCfg.Enabled, !maintenance.BackgroundDisabled())
	webhook, err := ops.WebhookURL(os.Getenv("ALERT_WEBHOOK_URL"))
	if err != nil {
		return err
	}
	if webhook == "" {
		logger.Warn("alert notifications disabled: ALERT_WEBHOOK_URL is not set (problems are still logged)")
	}
	go ops.NewMonitor(checker, webhook, authCfg.PublicOrigin, logger).Run(ctx, 5*time.Minute)

	siteStore := site.NewStore(pool)
	publisher := publishing.NewService(pool)
	publisher.Gate = gate
	go publisher.Run(ctx, publishing.DefaultTick, logger)

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: httpapi.NewRouter(logger, authCfg.PublicOrigin, maintenance.Middleware,
			health.Handler{DB: pool, Logger: logger},
			authHandler,
			content.NewHandler(content.NewStore(pool), logger, authHandler.RequireOwner, auth.Actor),
			mediaHandler,
			publishing.NewHandler(publisher, logger, authHandler.RequireOwner, auth.Actor),
			site.NewHandler(siteStore, logger, authHandler.RequireOwner, auth.Actor),
			public.NewHandler(pool, siteStore, logger),
			contactHandler,
			ops.NewAdminHandler(checker, pool, authHandler.RequireOwner, webhook != ""),
			contact.NewAdminHandler(contactStore, contactHandler, authHandler.RequireOwner, auth.Actor),
		),
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

func opsCheck(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	dbCfg, err := db.Config(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, dbCfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	contactCfg := contact.LoadConfig(os.Getenv)
	m := ops.NewMaintenance(pool, logger, os.Getenv)
	r := ops.NewChecker(pool, cfg.MediaRoot, contactCfg.Enabled, !m.BackgroundDisabled()).Run(ctx)
	for _, c := range r.Checks {
		fmt.Printf("%-8s %s: %s\n", c.Status, c.Name, c.Detail)
	}
	if r.Status == ops.StatusCritical {
		return errors.New("critical operational checks")
	}
	return nil
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
