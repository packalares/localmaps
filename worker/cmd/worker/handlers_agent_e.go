// Agent E — region orchestration handlers. Kept in a sibling file so
// future agents can rebase main.go cleanly.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	_ "modernc.org/sqlite"

	"github.com/packalares/localmaps/internal/jobs"
	"github.com/packalares/localmaps/internal/geofabrik"
	"github.com/packalares/localmaps/worker/internal/install"
	"github.com/packalares/localmaps/worker/internal/scheduler"
)

// registerAgentEHandlers wires the Asynq kinds owned by Agent E and
// returns a *scheduler.Scheduler that the caller is expected to Start
// in a goroutine. Only KindRegionDelete + KindPipelinePOI remain
// "not yet implemented" placeholders; KindRegionInstall, the pipeline
// chain, swap, and KindRegionUpdate (Agent N) are wired end-to-end.
//
// The returned *Scheduler may be nil if its dependencies (DB, catalog,
// queue) could not be constructed; callers must nil-check before
// Start().
func registerAgentEHandlers(mux *asynq.ServeMux, b Boot, log zerolog.Logger) *scheduler.Scheduler {
	db, err := openConfigDB(b.DataDir)
	if err != nil {
		log.Warn().Err(err).Msg("agent-e: open config db; install handler will fail fast")
	}
	gfc, err := newGeofabrikFromSettings(db, b.DataDir)
	if err != nil {
		log.Warn().Err(err).Msg("agent-e: build geofabrik client")
	}
	// Asynq client sharing the same redis as the server. We reuse the
	// server's Asynq.Client package to enqueue downstream stages.
	redisOpt, perr := asynq.ParseRedisURI(b.RedisURL)
	var queue *asynq.Client
	if perr == nil {
		queue = asynq.NewClient(redisOpt)
	}

	installDeps := install.Deps{
		DB:      db,
		DataDir: b.DataDir,
		Catalog: gfc,
		Queue:   queue,
		HTTP:    &http.Client{Timeout: 30 * time.Minute},
	}
	mux.HandleFunc(jobs.KindRegionInstall, install.NewHandler(installDeps, log))
	mux.HandleFunc(jobs.KindRegionUpdate, install.NewUpdateHandler(installDeps, log))

	// region:delete — wipe disk + purge pelias docs tagged with the
	// region. The settings reader here is the same SQLite shim built
	// below for the pipeline chain; it may be nil in degraded test
	// configurations, in which case the handler falls back to the
	// schema defaults (purge=true, ES URL = http://pelias-es:9200).
	var deleteSettings install.PeliasURLReader
	if db != nil {
		xdb := sqlx.NewDb(db, "sqlite")
		s := &sqlxSettings{db: xdb}
		deleteSettings = install.AdaptSettings(s.GetString, s.GetBool)
	}
	mux.HandleFunc(jobs.KindRegionDelete,
		install.NewDeleteHandler(install.DeleteDeps{
			Deps:     installDeps,
			Settings: deleteSettings,
		}, log))

	// Shared collaborators for the pipeline chain. Settings is the live
	// SQLite-backed reader every StageWork uses to construct its runner
	// at job time (R3 — no hardcoded config); it may be nil in degraded
	// test configurations, in which case StageWorks fall back to
	// conservative defaults with clear log lines.
	var settings *sqlxSettings
	if db != nil {
		settings = &sqlxSettings{db: sqlx.NewDb(db, "sqlite")}
	}
	chain := ChainDeps{DB: db, Queue: queue, DataDir: b.DataDir, Settings: settings}

	// Pipeline stages — F (tiles) → G (routing) → H (geocoding) → swap.
	// Each stageHandler wrapper advances regions.state + chains on
	// success; the inner StageWork is the per-agent implementation.
	// The per-kind factories close over `chain` so each StageWork has
	// direct access to Settings / WsPub without stashing them globally.
	mux.HandleFunc(jobs.KindPipelineTiles,
		stageHandler(chain, "tiles", "processing_tiles",
			jobs.KindPipelineRouting, tilesWork(chain), log))
	mux.HandleFunc(jobs.KindPipelineRouting,
		stageHandler(chain, "routing", "processing_routing",
			jobs.KindPipelineGeocoding, routingWork(chain), log))
	mux.HandleFunc(jobs.KindPipelineGeocoding,
		stageHandler(chain, "geocoding", "processing_geocoding",
			jobs.KindRegionSwap, geocodingWork(chain), log))
	mux.HandleFunc(jobs.KindRegionSwap, swapHandler(chain, runSwap, log))

	// Still-pending kinds. Keep wired so Asynq surfaces them.
	for _, kind := range []string{
		jobs.KindPipelinePOI,
	} {
		k := kind
		mux.HandleFunc(k, func(_ context.Context, t *asynq.Task) error {
			log.Warn().Str("kind", k).Int("payloadBytes", len(t.Payload())).
				Msg("agent-e: handler not yet implemented")
			return fmt.Errorf("%s: not yet implemented", k)
		})
	}

	// Build the scheduler. If any of the collaborators is missing
	// (tests / misconfigured env), we return nil so main() can skip
	// starting it. The scheduler itself is safe against a nil Queue.
	if db == nil || gfc == nil {
		log.Warn().Msg("agent-e: scheduler not started (missing DB or catalog)")
		return nil
	}
	tickCron := defaultTickCron
	if db != nil {
		xdb := sqlx.NewDb(db, "sqlite")
		if v, err := (&sqlxSettings{db: xdb}).GetString("regions.updateCheckCron"); err == nil && v != "" {
			tickCron = v
		}
	}
	return &scheduler.Scheduler{
		DB:       db,
		Queue:    queue,
		Catalog:  gfc,
		Fetcher:  gfc,
		TickCron: tickCron,
		Logger:   log.With().Str("module", "scheduler").Logger(),
	}
}

// defaultTickCron mirrors scheduler.defaultTickCron; kept here so that
// registerAgentEHandlers doesn't import unexported package state.
const defaultTickCron = "0 3 * * *"

// openConfigDB opens the SQLite config DB shared with the gateway.
// Returns (nil, nil) if dataDir is empty so tests can inject.
func openConfigDB(dataDir string) (*sql.DB, error) {
	if dataDir == "" {
		return nil, errors.New("dataDir empty")
	}
	p := filepath.Join(dataDir, "config.db")
	// Match server/internal/config/store.go connection string.
	dsn := "file:" + p + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	return sql.Open("sqlite", dsn)
}

// newGeofabrikFromSettings reads regions.mirrorBase out of the SQLite
// config.db and constructs a geofabrik.Client. Worker-side: we read
// directly via sqlx since the worker does not depend on the server's
// config package (server/internal/config is not exported).
func newGeofabrikFromSettings(db *sql.DB, dataDir string) (*geofabrik.Client, error) {
	if db == nil {
		return geofabrik.NewClientWithBase(nil,
			"https://download.geofabrik.de",
			filepath.Join(dataDir, "cache", "geofabrik")), nil
	}
	xdb := sqlx.NewDb(db, "sqlite")
	return geofabrik.NewClient(nil, &sqlxSettings{db: xdb},
		filepath.Join(dataDir, "cache", "geofabrik"))
}

// sqlxSettings is a tiny SettingsReader implementation backed by the
// raw settings table. We can't import server/internal/config from the
// worker (Go internal rules); this little shim is the price.
//
// Every getter treats a missing row as a sentinel error — callers fall
// back to schema defaults (docs/07-config-schema.md) so the worker
// keeps running while the primary keeps the defaults seeded. R3: the
// values themselves are NEVER hardcoded here.
type sqlxSettings struct{ db *sqlx.DB }

func (s *sqlxSettings) GetString(key string) (string, error) {
	if s == nil || s.db == nil {
		return "", sql.ErrNoRows
	}
	var raw string
	if err := s.db.Get(&raw, `SELECT value FROM settings WHERE key = ?`, key); err != nil {
		return "", err
	}
	// settings.value is JSON-encoded; a string is "\"foo\"".
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1], nil
	}
	return raw, nil
}

func (s *sqlxSettings) GetInt(key string) (int, error) {
	if s == nil || s.db == nil {
		return 0, sql.ErrNoRows
	}
	var raw string
	if err := s.db.Get(&raw, `SELECT value FROM settings WHERE key = ?`, key); err != nil {
		return 0, err
	}
	var n int
	_, err := fmt.Sscanf(raw, "%d", &n)
	return n, err
}

// GetBool decodes a JSON-encoded boolean value.
func (s *sqlxSettings) GetBool(key string) (bool, error) {
	if s == nil || s.db == nil {
		return false, sql.ErrNoRows
	}
	var raw string
	if err := s.db.Get(&raw, `SELECT value FROM settings WHERE key = ?`, key); err != nil {
		return false, err
	}
	var v bool
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return false, err
	}
	return v, nil
}

// GetStringSlice decodes a JSON-encoded array of strings.
func (s *sqlxSettings) GetStringSlice(key string) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, sql.ErrNoRows
	}
	var raw string
	if err := s.db.Get(&raw, `SELECT value FROM settings WHERE key = ?`, key); err != nil {
		return nil, err
	}
	var v []string
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, err
	}
	return v, nil
}
