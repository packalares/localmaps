// Command localmaps is the gateway binary. When LOCALMAPS_MODE=worker
// it re-execs the worker entry point instead.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/hibiken/asynq"

	"github.com/packalares/localmaps/server/internal/api"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/ratelimit"
	"github.com/packalares/localmaps/server/internal/regions"
	"github.com/packalares/localmaps/server/internal/telemetry"
	"github.com/packalares/localmaps/server/internal/ws"
	"github.com/packalares/localmaps/internal/geofabrik"
)

// Build-time metadata; set via -ldflags in deploy/Dockerfile.gateway.
var (
	buildVersion = "0.1.0"
	buildCommit  = "unknown"
	buildTime    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "localmaps: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	boot, err := config.LoadBoot()
	if err != nil {
		return fmt.Errorf("load boot config: %w", err)
	}
	tel := telemetry.New(os.Stderr, boot.LogLevel)
	api.SetVersion(api.VersionInfo{
		Version: buildVersion,
		Commit:  buildCommit,
		BuiltAt: buildTime,
	})

	if boot.IsWorker() {
		// In worker mode, the worker binary owns execution. We refuse
		// to start the gateway server to keep modes cleanly separated.
		tel.Logger.Info().Msg("LOCALMAPS_MODE=worker: hand off to worker binary")
		return errors.New("worker mode requested; run the worker binary")
	}

	// Open the SQLite config DB; migrations + defaults run on first boot.
	dbPath := filepath.Join(boot.DataDir, "config.db")
	store, err := config.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open config db at %s: %w", dbPath, err)
	}
	defer func() { _ = store.Close() }()

	hub := ws.NewHub()
	defer hub.Close()
	limiter := ratelimit.New(store)

	// Regions service bootstrap. The catalog client reads
	// regions.mirrorBase from settings; on failure we still boot the
	// gateway (regions endpoints will 5xx until the setting is
	// corrected).
	cacheDir := filepath.Join(boot.DataDir, "cache", "geofabrik")
	catClient, cerr := geofabrik.NewClient(
		&http.Client{Timeout: 60 * time.Second},
		store, cacheDir)
	var regionsSvc *regions.Service
	if cerr != nil {
		tel.Logger.Warn().Err(cerr).
			Msg("regions: geofabrik client unavailable; /api/regions/* will 5xx")
	} else {
		redisOpt, rerr := asynq.ParseRedisURI(boot.RedisURL)
		if rerr != nil {
			tel.Logger.Warn().Err(rerr).
				Str("redisURL", boot.RedisURL).
				Msg("regions: asynq client unavailable; /api/regions/* will 5xx")
		} else {
			asynqClient := asynq.NewClient(redisOpt)
			defer func() { _ = asynqClient.Close() }()
			regionsSvc = regions.NewService(store.DB(), catClient, asynqClient)
		}
	}

	// Build the session manager once so bootstrap + router share state.
	authMgr := api.BuildManager(store, store.DB())
	pw, err := api.BootstrapAdmin(authMgr)
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	if pw != "" {
		// Print the randomly-generated password ONCE to stdout so the
		// operator can capture it on first boot. See docs/08-security.md.
		fmt.Printf("admin password: %s\n", pw)
		tel.Logger.Info().Msg("bootstrapped admin user; password printed to stdout once")
	}

	app := fiber.New(fiber.Config{
		AppName: "localmaps",
	})
	api.Register(app, api.Deps{
		Boot:      boot,
		Store:     store,
		Telemetry: tel,
		Hub:       hub,
		Limiter:   limiter,
		Regions:   regionsSvc,
		Auth:      authMgr,
	})

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		tel.Logger.Info().
			Str("addr", boot.ListenAddr).
			Str("version", buildVersion).
			Msg("gateway listening")
		errCh <- app.Listen(boot.ListenAddr)
	}()

	select {
	case <-ctx.Done():
		tel.Logger.Info().Msg("shutdown signal; draining")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return app.ShutdownWithContext(shutCtx)
	case err := <-errCh:
		return err
	}
}
