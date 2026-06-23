// Command tile-router serves multi-region tiles to the LocalMaps UI.
//
// One process per pod, listens on $PORT (default 8000 to drop-in
// replace the previous protomaps container). Reads the sqlite at
// $LOCALMAPS_CONFIG_DB (default /data/config.db), polls every
// $LOCALMAPS_TILE_ROUTER_POLL seconds (default 5) for state=ready
// regions, opens their pmtiles, and serves tiles by bbox-picking.
//
// Lifecycle:
//
//   boot → Refresh() once (so the first /tile request is fast) →
//   start ticker → start HTTP server →
//   SIGTERM/SIGINT → stop ticker → close every pmtiles handle →
//   shutdown HTTP gracefully → exit
//
// Logging is zerolog at info level by default; flip to debug via
// LOCALMAPS_LOG_LEVEL. The format matches the gateway's so log
// aggregators can ingest both with one parser.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"

	"github.com/packalares/localmaps/tile-router/internal/basemap"
	"github.com/packalares/localmaps/tile-router/internal/pick"
	"github.com/packalares/localmaps/tile-router/internal/route"
	"github.com/packalares/localmaps/tile-router/internal/store"
)

// Boot is the env-var surface. Matches the convention of the gateway
// + worker binaries — same `LOCALMAPS_*` prefix, default values that
// align with the helm chart's configmap.
type Boot struct {
	ListenAddr   string        `env:"LOCALMAPS_TILE_ROUTER_ADDR"     envDefault:":8000"`
	ConfigDB     string        `env:"LOCALMAPS_CONFIG_DB"            envDefault:"/data/config.db"`
	RegionsDir   string        `env:"LOCALMAPS_REGIONS_DIR"          envDefault:"/data/regions"`
	PollInterval time.Duration `env:"LOCALMAPS_TILE_ROUTER_POLL"     envDefault:"5s"`
	LogLevel     string        `env:"LOCALMAPS_LOG_LEVEL"            envDefault:"info"`
	Attribution  string        `env:"LOCALMAPS_TILE_ATTRIBUTION"     envDefault:"© OpenStreetMap contributors"`
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "tile-router: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var b Boot
	if err := env.Parse(&b); err != nil {
		return fmt.Errorf("parse env: %w", err)
	}

	lvl, lvlErr := zerolog.ParseLevel(b.LogLevel)
	if lvlErr != nil {
		lvl = zerolog.InfoLevel
	}
	log := zerolog.New(os.Stderr).Level(lvl).With().
		Timestamp().
		Str("service", "localmaps-tile-router").
		Logger()
	log.Info().
		Str("addr", b.ListenAddr).
		Str("db", b.ConfigDB).
		Str("regions", b.RegionsDir).
		Dur("poll", b.PollInterval).
		Msg("tile-router booting")

	// Sqlite read-only handle. The tile-router never writes; opening
	// read-only protects against accidental WAL contention with the
	// gateway/worker writes happening in parallel.
	dsn := fmt.Sprintf("file:%s?mode=ro", b.ConfigDB)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite %s: %w", b.ConfigDB, err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Load the country-polygon atlas. Embedded gzip → ~50 ms decode.
	// On failure we log + continue with nil — the picker degrades to
	// bbox-of-tile-center which is still correct for non-border tiles.
	atlas, atlasErr := pick.LoadAtlas()
	if atlasErr != nil {
		log.Warn().Err(atlasErr).Msg("loading country polygons failed; falling back to bbox-only picking")
	} else {
		log.Info().Int("countries", len(atlas.Countries)).
			Msg("country polygons loaded")
	}

	s := store.New(db, b.RegionsDir, b.PollInterval, atlas, log)
	// First refresh BEFORE we accept HTTP traffic so the first /tile
	// request hits a populated store. Subsequent refreshes run on the
	// poll loop.
	if err := s.Refresh(ctx); err != nil {
		log.Warn().Err(err).Msg("initial refresh failed (will retry on poll)")
	}
	go s.Run(ctx)

	// Build the world-overview basemap renderer from the same Atlas.
	// Cheap: just reorganises polygon data into orb.MultiPolygon
	// without re-parsing the GeoJSON. Memory cost is one extra copy
	// of each country's coordinates (~6 MB).
	bm := basemap.NewRenderer(atlas)
	if bm.IsEmpty() {
		log.Warn().Msg("basemap renderer is empty; low-zoom tiles will 404 outside installed regions")
	} else {
		log.Info().Msg("basemap renderer ready (low-zoom fallback enabled)")
	}

	mux := http.NewServeMux()
	(&route.Handlers{
		Store:       s,
		Basemap:     bm,
		Attribution: b.Attribution,
		Log:         log,
	}).Register(mux)

	srv := &http.Server{
		Addr:         b.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("addr", b.ListenAddr).Msg("tile-router listening")
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn().Err(err).Msg("HTTP shutdown error")
		}
		// store.Run watches ctx and will close every pmtiles handle on its way out.
		return nil
	case err := <-errCh:
		return err
	}
}
