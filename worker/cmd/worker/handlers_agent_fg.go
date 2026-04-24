// handlers_agent_fg.go — real StageWork funcs for pipeline:tiles and
// pipeline:routing. Split out of main.go so main.go stays under the
// 250-line cap and each file has a single responsibility.
//
// These wire the existing planetiler + valhalla runners in
// worker/internal/pipeline to the stageHandler chain (chain.go). Every
// knob is read live from settings (docs/07-config-schema.md) — R3: no
// hardcoded values.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/jobs"
	"github.com/packalares/localmaps/worker/internal/pipeline"
)

// tilesWork returns the real `pipeline:tiles` StageWork: it downloads
// the planetiler jar (SHA-pinned) on first run, then shells out to
// java -jar planetiler.jar … to produce <regionDir>/map.pmtiles.
//
// The factory closes over ChainDeps so the StageWork can read
// tiles.planetiler* settings live at job time (R3). Missing settings
// fall back to docs/07-config-schema.md defaults — except the SHA,
// which is required per docs/08-security.md (no unverified downloads).
func tilesWork(deps ChainDeps) StageWork {
	return func(ctx context.Context, regionDir string, p jobs.PipelineStagePayload, log zerolog.Logger) error {
		pbfPath := filepath.Join(regionDir, "source.osm.pbf")
		outPath := filepath.Join(regionDir, "map.pmtiles")
		if err := os.MkdirAll(regionDir, 0o755); err != nil {
			return fmt.Errorf("mkdir region: %w", err)
		}

		jarURL := settingOrDefault(deps.Settings, "tiles.planetilerJarURL",
			"https://github.com/onthegomap/planetiler/releases/download/v0.8.2/planetiler.jar")
		jarSHA := settingOrDefault(deps.Settings, "tiles.planetilerJarSha256", "")
		if jarSHA == "" {
			return errors.New("tiles.planetilerJarSha256 unset — set the pinned SHA-256 in settings (docs/08-security.md fail-closed)")
		}
		memMB := settingIntOrDefault(deps.Settings, "tiles.planetilerMemoryMB", 4096)
		extra := settingArrOrDefault(deps.Settings, "tiles.planetilerExtraArgs", nil)
		maxMin := settingIntOrDefault(deps.Settings, "tiles.planetilerMaxDurationMinutes", 240)
		allowed := settingArrOrDefault(deps.Settings, "security.allowedEgressHosts",
			[]string{"download.geofabrik.de", "github.com", "objects.githubusercontent.com"})

		cache := &pipeline.JarCache{
			DestDir: filepath.Join(deps.DataDir, "cache", "planetiler"),
			Client:  &http.Client{Timeout: 15 * time.Minute},
			Allowed: allowed,
		}
		jarPath, err := cache.Ensure(ctx, jarURL, jarSHA)
		if err != nil {
			return fmt.Errorf("ensure planetiler jar: %w", err)
		}
		runner, err := pipeline.NewPlanetilerRunner(pipeline.PlanetilerConfig{
			JarPath:     jarPath,
			MemoryMB:    memMB,
			ExtraArgs:   extra,
			MaxDuration: time.Duration(maxMin) * time.Minute,
		}, log)
		if err != nil {
			return fmt.Errorf("planetiler runner: %w", err)
		}
		progress := newStageProgress(ctx, deps, p.JobID, log)
		start := time.Now()
		if err := runner.Run(ctx, pbfPath, outPath, progress); err != nil {
			return fmt.Errorf("planetiler run: %w", err)
		}
		info, err := os.Stat(outPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", outPath, err)
		}
		log.Info().Str("out", outPath).Int64("bytes", info.Size()).
			Dur("duration", time.Since(start)).Msg("pipeline:tiles complete")
		return pipeline.UpdateTilesSection(regionDir, p.Region, pipeline.TilesSection{
			BuiltAt:              time.Now().UTC(),
			BuildDurationSeconds: time.Since(start).Seconds(),
			Tool:                 "planetiler",
			ToolVersion:          jarSHA[:8],
			OutputFile:           "map.pmtiles",
			OutputBytes:          info.Size(),
		})
	}
}

// routingWork returns the real `pipeline:routing` StageWork. It invokes
// the four-step valhalla build chain (admins → timezones → tiles →
// extract) against the region's pbf. The binaries must be on PATH in
// the worker image; absence is surfaced via the runner's start error.
func routingWork(deps ChainDeps) StageWork {
	return func(ctx context.Context, regionDir string, p jobs.PipelineStagePayload, log zerolog.Logger) error {
		tileDirName := settingOrDefault(deps.Settings, "routing.valhallaTileDirName", "valhalla_tiles")
		paths := pipeline.RegionPaths{
			Root:       regionDir,
			RegionKey:  p.Region,
			PbfPath:    filepath.Join(regionDir, "source.osm.pbf"),
			TileDir:    filepath.Join(regionDir, tileDirName),
			TarPath:    filepath.Join(regionDir, tileDirName+".tar"),
			AdminDB:    filepath.Join(regionDir, "valhalla_admin.sqlite"),
			TimezoneDB: filepath.Join(regionDir, "valhalla_timezones.sqlite"),
		}
		if err := os.MkdirAll(paths.TileDir, 0o755); err != nil {
			return fmt.Errorf("mkdir tiledir: %w", err)
		}
		cfg := pipeline.NewValhallaRuntimeConfig(
			settingIntOrDefault(deps.Settings, "routing.valhallaConcurrency", 0),
			settingIntOrDefault(deps.Settings, "routing.valhallaBuildTimeoutMinutes", 60),
			settingArrOrDefault(deps.Settings, "routing.valhallaExtraArgs", nil),
		)
		runner := pipeline.NewValhallaRunner(log, cfg)
		progress := newStageProgress(ctx, deps, p.JobID, log)
		start := time.Now()
		if err := runner.Run(ctx, p.Region, paths, progress); err != nil {
			return fmt.Errorf("valhalla run: %w", err)
		}
		log.Info().Str("tileDir", paths.TileDir).
			Dur("duration", time.Since(start)).Msg("pipeline:routing complete")
		return pipeline.UpdateRoutingSection(regionDir, p.Region, pipeline.RoutingSection{
			BuiltAt:              time.Now().UTC(),
			BuildDurationSeconds: time.Since(start).Seconds(),
			Tool:                 "valhalla_build_tiles",
			TileDir:              paths.TileDir,
			TarPath:              paths.TarPath,
			AdminDB:              paths.AdminDB,
			TimezoneDB:           paths.TimezoneDB,
		})
	}
}

// settingOrDefault reads a string setting with a fallback. The default
// value mirrors docs/07-config-schema.md so a degraded worker (missing
// DB or settings row) still behaves predictably. Fallback is taken on
// any error — callers are expected to log once if it matters.
func settingOrDefault(s StageSettings, key, def string) string {
	if s == nil {
		return def
	}
	if v, err := s.GetString(key); err == nil && v != "" {
		return v
	}
	return def
}

// settingIntOrDefault mirrors settingOrDefault for int keys.
func settingIntOrDefault(s StageSettings, key string, def int) int {
	if s == nil {
		return def
	}
	if v, err := s.GetInt(key); err == nil {
		return v
	}
	return def
}

// settingArrOrDefault mirrors settingOrDefault for []string keys.
func settingArrOrDefault(s StageSettings, key string, def []string) []string {
	if s == nil {
		return def
	}
	if v, err := s.GetStringSlice(key); err == nil {
		return v
	}
	return def
}
