// Command worker runs the Asynq consumer that builds region artifacts.
// It is a SEPARATE binary from the gateway so deployments can scale
// them independently (see docs/01-architecture.md).
//
// Wiring is split across:
//   - this file — boot + Asynq server glue + legacy-placeholder router
//   - handlers_agent_e.go — KindRegionInstall / KindRegionUpdate +
//     scheduler construction
//   - handlers_agent_fg.go — pipeline:tiles (planetiler) +
//     pipeline:routing (valhalla)
//   - handlers_agent_h.go — pipeline:geocoding (pelias)
//   - chain.go / swap.go — stage-state + atomic swap glue
//   - stage_progress.go — Asynq/DB/WS progress reporter plumbing
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v11"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

// JobKind mirrors contracts/openapi.yaml components/schemas/JobKind.
// Keep every value in sync.
type JobKind string

const (
	JobKindDownloadPBF     JobKind = "download_pbf"
	JobKindBuildPMTiles    JobKind = "build_pmtiles"
	JobKindBuildValhalla   JobKind = "build_valhalla"
	JobKindBuildPelias     JobKind = "build_pelias"
	JobKindBuildOverture   JobKind = "build_overture"
	JobKindSwapRegion      JobKind = "swap_region"
	JobKindUpdateRegion    JobKind = "update_region"
	JobKindArchiveRegion   JobKind = "archive_region"
)

// AllJobKinds returns every declared kind, used to wire the mux.
func AllJobKinds() []JobKind {
	return []JobKind{
		JobKindDownloadPBF,
		JobKindBuildPMTiles,
		JobKindBuildValhalla,
		JobKindBuildPelias,
		JobKindBuildOverture,
		JobKindSwapRegion,
		JobKindUpdateRegion,
		JobKindArchiveRegion,
	}
}

// Boot is the worker-only subset of LocalMaps env vars. It mirrors
// docs/07-config-schema.md; sysadmins set LOCALMAPS_MODE=worker on the
// same binary (or this separate one).
type Boot struct {
	RedisURL    string `env:"LOCALMAPS_REDIS_URL"   envDefault:"redis://redis:6379/0"`
	LogLevel    string `env:"LOCALMAPS_LOG_LEVEL"   envDefault:"info"`
	Concurrency int    `env:"LOCALMAPS_WORKER_CONCURRENCY" envDefault:"4"`
	// DataDir mirrors LOCALMAPS_DATA_DIR (docs/07-config-schema.md). The
	// pipeline handler derives region paths off this root.
	DataDir string `env:"LOCALMAPS_DATA_DIR" envDefault:"/data"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "worker: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var b Boot
	if err := env.Parse(&b); err != nil {
		return fmt.Errorf("parse env: %w", err)
	}

	lvl, perr := zerolog.ParseLevel(b.LogLevel)
	if perr != nil {
		lvl = zerolog.InfoLevel
	}
	log := zerolog.New(os.Stderr).Level(lvl).With().
		Timestamp().
		Str("service", "localmaps-worker").
		Logger()

	redisOpt, err := asynq.ParseRedisURI(b.RedisURL)
	if err != nil {
		return fmt.Errorf("parse redis uri %q: %w", b.RedisURL, err)
	}

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: b.Concurrency,
		Logger:      asynqZerolog{log: log},
	})

	mux := asynq.NewServeMux()
	// Legacy openapi-kind placeholders stay registered so Asynq doesn't
	// drop tasks if a stale client enqueues one of the string-enum kinds
	// (download_pbf / build_pmtiles / ...). They log + return an error
	// so the queue retries or moves them to the dead-letter queue.
	for _, kind := range AllJobKinds() {
		k := kind
		mux.HandleFunc(string(k), makePlaceholderHandler(k, log))
	}
	// Agent E + pipeline F/G/H + swap + Agent N's scheduler. See
	// registerAgentEHandlers below. The returned *scheduler.Scheduler
	// may be nil in test / degraded configurations; we nil-check before
	// starting its tick loop.
	sch := registerAgentEHandlers(mux, b, log)

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if sch != nil {
		go sch.Start(ctx)
	} else {
		log.Warn().Msg("scheduler not started (missing deps); region updates will not run automatically")
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().
			Str("redis", b.RedisURL).
			Int("concurrency", b.Concurrency).
			Msg("worker starting")
		errCh <- srv.Run(mux)
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("shutdown signal; stopping worker")
		srv.Shutdown()
		return nil
	case err := <-errCh:
		return err
	}
}

// makePlaceholderHandler returns an Asynq HandlerFunc that logs the
// invocation and returns "not yet implemented". Phase 2 replaces these
// with actual pipeline implementations.
func makePlaceholderHandler(kind JobKind, log zerolog.Logger) func(context.Context, *asynq.Task) error {
	return func(_ context.Context, t *asynq.Task) error {
		log.Warn().
			Str("kind", string(kind)).
			Str("taskID", t.Type()).
			Int("payloadBytes", len(t.Payload())).
			Msg("job handler not yet implemented")
		return errors.New("not yet implemented")
	}
}

// asynqZerolog adapts zerolog.Logger to the asynq.Logger interface.
type asynqZerolog struct {
	log zerolog.Logger
}

func (a asynqZerolog) Debug(args ...interface{}) { a.log.Debug().Msgf("%v", args) }
func (a asynqZerolog) Info(args ...interface{})  { a.log.Info().Msgf("%v", args) }
func (a asynqZerolog) Warn(args ...interface{})  { a.log.Warn().Msgf("%v", args) }
func (a asynqZerolog) Error(args ...interface{}) { a.log.Error().Msgf("%v", args) }
func (a asynqZerolog) Fatal(args ...interface{}) { a.log.Fatal().Msgf("%v", args) }
