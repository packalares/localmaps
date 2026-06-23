// Routing-tiles download handler.
//
// Replaces the old per-region valhalla_build_admins / _timezones /
// _tiles / _extract chain with a one-time download of a pre-built
// world routing graph. Per-region builds were:
//   - Slow (30-80 min per country)
//   - Disconnected (each country's tar was its own graph; no border
//     crossings)
//   - Expensive at scale (a 20-country combined rebuild took hours
//     and re-ran on every install)
//
// A community-published planet tar (default: GIS-OPS' weekly build)
// is downloaded ONCE on first need, dropped at
// /data/regions/_world/valhalla_tiles.tar, and the active-region
// pointer flips to `_world`. The valhalla container's watch loop
// (deploy/templates/deployment.yaml ~line 230) picks it up within
// 5 seconds. Routing then works across the entire planet, not just
// installed regions.
//
// Trigger:
//   - chain.go's routingWork (handlers_agent_fg.go) enqueues this
//     job during the first region install whose routing stage finds
//     no world tar on disk.
//   - Operators can also enqueue manually via the `enqueue` helper
//     for refreshes (the source publishes weekly).
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

// JobKindDownloadRoutingTiles is the Asynq kind for the one-time
// download. Lives next to the other internal kinds so the handler
// file is self-contained.
const JobKindDownloadRoutingTiles = "download_routing_tiles"

// defaultRoutingTilesURL is the source we'll fetch when the operator
// hasn't pinned `routing.tilesURL` in settings. The pre-built tar
// covers the entire OSM planet — ~30-50 GB, updated weekly by the
// upstream maintainer.
//
// Operators who want a smaller or differently-tuned tar can override
// this in settings to point at their own mirror or extract. Setting
// to "" disables the download entirely (cross-region routing stays
// unavailable until a real URL is set).
const defaultRoutingTilesURL = "https://valhalla-tiles.builds.gisops.com/tiles/v2/planet-latest.valhalla.tar"

// routingDownloadHandler returns the Asynq dispatcher. Captures deps
// so the closure has DB + DataDir + Settings without globals.
func routingDownloadHandler(deps *ChainDeps, log zerolog.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return runRoutingTilesDownload(ctx, deps, log)
	}
}

// runRoutingTilesDownload streams the world routing tar from the
// configured URL into _world.new/valhalla_tiles.tar, then atomic-
// renames to _world/ and flips the `.active-region` pointer.
//
// Atomicity: we stream into a `.tmp` file first so a crash mid-
// download leaves no partially-formed tar at the canonical path.
// The rename happens only after the HTTP body is fully written.
//
// Idempotency: if the tar already exists AND is non-empty, this
// returns nil immediately. Operators wanting to force a refresh
// should `rm` the file first.
func runRoutingTilesDownload(ctx context.Context, deps *ChainDeps, log zerolog.Logger) error {
	const worldKey = "_world"

	url := settingOrDefault(deps.Settings, "routing.tilesURL", defaultRoutingTilesURL)
	if strings.TrimSpace(url) == "" {
		log.Warn().Msg("routing-download: routing.tilesURL is empty; cross-region routing will be unavailable")
		return nil
	}

	worldDir := filepath.Join(deps.DataDir, "regions", worldKey)
	finalPath := filepath.Join(worldDir, "valhalla_tiles.tar")
	if info, err := os.Stat(finalPath); err == nil && info.Size() > 0 {
		log.Info().Str("path", finalPath).Int64("bytes", info.Size()).
			Msg("routing-download: tar already in place; skipping")
		return nil
	}

	if err := os.MkdirAll(worldDir, 0o755); err != nil {
		return fmt.Errorf("routing-download: mkdir %s: %w", worldDir, err)
	}
	tmpPath := finalPath + ".tmp"
	// Pre-clean any half-downloaded tar from a previous run so we
	// always start with a fresh file. ResumeFromOffset is a future
	// optimisation; for now whole-tar each retry.
	_ = os.Remove(tmpPath)

	log.Info().Str("url", url).Str("dest", finalPath).
		Msg("routing-download: starting (this may take 10-60 minutes for ~30-50 GB)")
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("routing-download: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("routing-download: HTTP fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("routing-download: upstream returned %d %s", resp.StatusCode, resp.Status)
	}

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("routing-download: open tmp: %w", err)
	}
	n, err := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("routing-download: stream body (%d bytes written): %w", n, err)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("routing-download: close tmp: %w", closeErr)
	}
	if n == 0 {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("routing-download: upstream returned empty body")
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("routing-download: rename tmp->final: %w", err)
	}

	// Flip the watch-loop's pointer so the valhalla container picks
	// up the new tar within 5 seconds.
	pointer := filepath.Join(deps.DataDir, "regions", ".active-region")
	if err := os.WriteFile(pointer, []byte(worldKey+"\n"), 0o644); err != nil {
		return fmt.Errorf("routing-download: write pointer: %w", err)
	}

	log.Info().
		Str("path", finalPath).
		Int64("bytes", n).
		Dur("duration", time.Since(start)).
		Msg("routing-download: complete; valhalla will reload within 5s")
	return nil
}
