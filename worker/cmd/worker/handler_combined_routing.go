// Combined-routing build handler.
//
// Cross-country routing — the user wanted "Bucharest → Sofia in one
// directions request" — works at the Valhalla layer if (and only if)
// the routing tile graph was built from BOTH countries' PBFs. Per-
// region tile sets are isolated graphs that don't know about each
// other; Valhalla's solver gives up when start and destination land
// in different sets.
//
// This handler builds ONE Valhalla tile set from EVERY installed
// region's PBF, dropping the output at
// `<dataDir>/regions/_combined/valhalla_tiles.tar`. The valhalla
// container's watch-loop (see deploy/templates/deployment.yaml around
// line 230) reads `.active-region`; flipping that pointer to
// `_combined` causes it to load the cross-country tar and start
// answering cross-border directions.
//
// Triggering:
//   - The handler is registered on a new Asynq kind: `build_combined_routing`.
//   - chain.go's `finishSwap` enqueues this job whenever a region
//     transitions to `ready`. So every successful single-region
//     build is followed by a combined rebuild that picks up the
//     new region.
//   - The job is idempotent — a no-op when fewer than 2 regions are
//     ready (single-region routing already works for those).
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/worker/internal/pipeline"
)

// JobKindBuildCombinedRouting is the Asynq kind for combined builds.
// Mirrors the per-stage kinds declared above in main.go; kept here so
// the handler file is self-contained.
const JobKindBuildCombinedRouting = "build_combined_routing"

// combinedRoutingHandler returns a wrapper Asynq can dispatch to. The
// closure captures the shared deps so we can re-use the same config
// resolution + logging surface as the per-region handlers.
func combinedRoutingHandler(deps *ChainDeps, log zerolog.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return runCombinedRoutingBuild(ctx, deps, log)
	}
}

// runCombinedRoutingBuild walks every region marked `ready`, collects
// its PBF path, and feeds the list to ValhallaRunner.RunMulti.
// Output goes to a synthetic region directory `_combined` so the
// existing valhalla container picks it up via .active-region.
//
// Race notes:
//   - The job's Asynq concurrency is 1 (worker.LOCALMAPS_WORKER_CONCURRENCY=1
//     by default); a fresh combined build won't start while another is
//     running.
//   - If a per-region build is also in flight when this kicks off,
//     valhalla_build_tiles uses *only* the PBFs explicitly passed —
//     the in-progress region won't accidentally show up half-built.
func runCombinedRoutingBuild(ctx context.Context, deps *ChainDeps, log zerolog.Logger) error {
	const combinedKey = "_combined"

	// List ready regions and their on-disk PBF paths. The picker
	// only considers regions whose state has settled to `ready` —
	// in-flight installs are skipped this round.
	pbfs, regions, err := readyRegionPBFs(ctx, deps)
	if err != nil {
		return fmt.Errorf("combined-routing: list ready regions: %w", err)
	}
	if len(pbfs) < 2 {
		// Less than two regions installed → per-region routing is
		// already sufficient. Drop a marker file so the valhalla
		// container falls back to the single-region tar via the
		// existing watch-loop's "find biggest tar" path.
		log.Info().Int("ready", len(pbfs)).
			Msg("combined-routing: <2 ready regions; skipping (single-region routing still works)")
		return nil
	}

	regionDir := filepath.Join(deps.DataDir, "regions", combinedKey+".new")
	if err := os.RemoveAll(regionDir); err != nil {
		return fmt.Errorf("combined-routing: clean staging: %w", err)
	}
	if err := os.MkdirAll(regionDir, 0o755); err != nil {
		return fmt.Errorf("combined-routing: mkdir staging: %w", err)
	}
	paths := pipeline.RegionPaths{
		PbfPath:    pbfs[0], // unused by RunMulti but validator wants a real file
		TileDir:    filepath.Join(regionDir, "valhalla_tiles"),
		TarPath:    filepath.Join(regionDir, "valhalla_tiles.tar"),
		AdminDB:    filepath.Join(regionDir, "valhalla_admin.sqlite"),
		TimezoneDB: filepath.Join(regionDir, "valhalla_timezones.sqlite"),
	}
	if err := os.MkdirAll(paths.TileDir, 0o755); err != nil {
		return fmt.Errorf("combined-routing: mkdir tiledir: %w", err)
	}
	cfg := pipeline.NewValhallaRuntimeConfig(
		settingIntOrDefault(deps.Settings, "routing.valhallaConcurrency", 0),
		// Combined builds can take many hours — bump the timeout
		// well past the single-region default. Operators can pin
		// via settings if needed.
		settingIntOrDefault(deps.Settings, "routing.valhallaCombinedBuildTimeoutMinutes", 360),
		settingArrOrDefault(deps.Settings, "routing.valhallaExtraArgs", nil),
	)
	runner := pipeline.NewValhallaRunner(log, cfg)
	start := time.Now()
	log.Info().Strs("regions", regions).Int("pbfs", len(pbfs)).
		Str("output", paths.TarPath).
		Msg("combined-routing: starting build")
	// Synthetic region name lands in the generated valhalla config's
	// metadata + log lines so an operator can grep for it.
	if err := runner.RunMulti(ctx, combinedKey, paths, pbfs, nil); err != nil {
		return fmt.Errorf("combined-routing: run: %w", err)
	}

	// Atomic-ish swap: rename .new → final, then update the
	// `.active-region` pointer. The watch-loop polls every 5 s so
	// the new tar takes effect within a few seconds of the rename.
	finalDir := filepath.Join(deps.DataDir, "regions", combinedKey)
	if err := os.RemoveAll(finalDir); err != nil {
		return fmt.Errorf("combined-routing: clean final: %w", err)
	}
	if err := os.Rename(regionDir, finalDir); err != nil {
		return fmt.Errorf("combined-routing: swap final: %w", err)
	}
	// Pointer write — the watch-loop reads this file every poll.
	pointer := filepath.Join(deps.DataDir, "regions", ".active-region")
	if err := os.WriteFile(pointer, []byte(combinedKey+"\n"), 0o644); err != nil {
		return fmt.Errorf("combined-routing: write pointer: %w", err)
	}
	log.Info().Strs("regions", regions).
		Dur("duration", time.Since(start)).
		Str("pointer", pointer).
		Msg("combined-routing: build complete; valhalla will reload within 5s")
	return nil
}

// readyRegionPBFs enumerates regions that are fully installed and
// returns parallel slices of PBF paths + region names. The names are
// only used for logging; the PBF paths are the actual input to the
// valhalla build.
func readyRegionPBFs(ctx context.Context, deps *ChainDeps) ([]string, []string, error) {
	rows, err := deps.DB.QueryContext(ctx,
		`SELECT name FROM regions WHERE state = 'ready' ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, nil, err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	pbfs := make([]string, 0, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		p := filepath.Join(deps.DataDir, "regions", n, "source.osm.pbf")
		if _, err := os.Stat(p); err == nil {
			pbfs = append(pbfs, p)
			out = append(out, n)
		}
	}
	return pbfs, out, nil
}
