// Package pipeline — valhalla build configuration.
//
// GenerateConfig emits a minimal valhalla.json with only the fields the
// build tools care about. The upstream schema is huge; we intentionally
// touch only the subset used by `valhalla_build_admins`,
// `valhalla_build_timezones`, `valhalla_build_tiles`, and
// `valhalla_build_extract`. Anything else stays at valhalla's defaults.
//
// Authority: docs/02-stack.md lists valhalla_build_tiles as the routing
// build tool. docs/04-data-model.md defines the filesystem layout under
// `/data/regions/<name>/valhalla_tiles/` and the .tar extract.
package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
)

// RegionPaths carries the absolute, pre-sanitised paths for a single
// region's valhalla build. Callers are responsible for validating that
// these stay under `/data/regions/<name>/` — the runner only reads them.
//
// TODO(primary): pelias.go used to declare a different RegionPaths with
// {Root, PbfPath, RegionKey} fields. Agent H's TODO acknowledges this
// is Agent G's canonical struct; at phase gate the primary folds any
// extra fields pelias needs (Root, RegionKey) into this type.
type RegionPaths struct {
	// PbfPath is the absolute path to the downloaded .osm.pbf extract
	// that admins/tiles will ingest.
	PbfPath string
	// TileDir is the output directory for routing tile files. Written
	// by valhalla_build_tiles.
	TileDir string
	// TarPath is the output tar archive produced by
	// valhalla_build_extract for memory-mapped serving.
	TarPath string
	// AdminDB is the sqlite path produced by valhalla_build_admins.
	AdminDB string
	// TimezoneDB is the sqlite path produced by valhalla_build_timezones.
	TimezoneDB string
	// Root is the per-region dir, e.g. /data/regions/europe-romania[.new].
	// Preserved for pelias.go compatibility.
	Root string
	// RegionKey is the hyphenated canonical key (e.g. "europe-romania").
	// Preserved for pelias.go compatibility.
	RegionKey string
}

// BuildConfig is the subset of valhalla.json we set. Field names match
// upstream exactly (snake_case JSON keys) so the generated file drops
// straight into `--config`.
type BuildConfig struct {
	Mjolnir MjolnirConfig `json:"mjolnir"`
}

// MjolnirConfig mirrors the `mjolnir` block of valhalla.json. Only keys
// we care about are represented.
type MjolnirConfig struct {
	TileDir     string        `json:"tile_dir"`
	TileExtract string        `json:"tile_extract"`
	TransitDir  string        `json:"transit_dir"`
	Concurrency int           `json:"concurrency"`
	Timezone    string        `json:"timezone"`
	Admin       string        `json:"admin"`
	Logging     LoggingConfig `json:"logging"`
}

// LoggingConfig configures the valhalla logger for the build tools.
// "std_out" streams to stderr which we line-parse for progress.
type LoggingConfig struct {
	Type string `json:"type"`
}

// ValhallaRuntimeConfig is the intersection of settings the runner
// reads — kept separate from the broader pipeline Config so tests can
// inject values without wiring the whole gateway config store.
//
// All fields have hardcoded fallbacks in NewValhallaRunner when the
// gateway settings store has no routing.valhalla* keys yet (see
// NEEDED notes in the agent report — these keys are not yet in
// docs/07-config-schema.md).
type ValhallaRuntimeConfig struct {
	Concurrency     int
	BuildTimeoutMin int
	ExtraArgs       []string
}

// NewValhallaRuntimeConfig constructs a runtime-config value for use by
// callers outside the package (e.g. the worker main). Fields can also
// be set directly; this helper is purely a convenience.
func NewValhallaRuntimeConfig(concurrency, buildTimeoutMin int, extraArgs []string) ValhallaRuntimeConfig {
	return ValhallaRuntimeConfig{
		Concurrency:     concurrency,
		BuildTimeoutMin: buildTimeoutMin,
		ExtraArgs:       extraArgs,
	}
}

// GenerateConfig returns the JSON bytes of valhalla.json for a given
// region build. The result is deterministic for identical inputs so
// tests can compare against a golden file.
//
// Validation: every path must be non-empty; concurrency must be ≥ 1.
// The function does NOT do filesystem sanitisation — callers must
// have already validated the paths.
func GenerateConfig(region string, paths RegionPaths, rt ValhallaRuntimeConfig) ([]byte, error) {
	if region == "" {
		return nil, errors.New("valhalla: empty region")
	}
	if err := validateValhallaPaths(paths); err != nil {
		return nil, err
	}
	if rt.Concurrency < 1 {
		return nil, fmt.Errorf("valhalla: concurrency must be >= 1, got %d", rt.Concurrency)
	}

	cfg := BuildConfig{
		Mjolnir: MjolnirConfig{
			TileDir:     paths.TileDir,
			TileExtract: paths.TarPath,
			TransitDir:  "",
			Concurrency: rt.Concurrency,
			Timezone:    paths.TimezoneDB,
			Admin:       paths.AdminDB,
			Logging: LoggingConfig{
				Type: "std_out",
			},
		},
	}
	// MarshalIndent so the file is diff-friendly and stable.
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("valhalla: marshal config: %w", err)
	}
	// Trailing newline matches POSIX convention and tidies golden files.
	return append(b, '\n'), nil
}

// validateValhallaPaths enforces that every path required by the
// four-step build is present. Name is unique to avoid collision with
// any future validatePaths in neighbouring stage files.
func validateValhallaPaths(p RegionPaths) error {
	switch {
	case p.PbfPath == "":
		return errors.New("valhalla: RegionPaths.PbfPath empty")
	case p.TileDir == "":
		return errors.New("valhalla: RegionPaths.TileDir empty")
	case p.TarPath == "":
		return errors.New("valhalla: RegionPaths.TarPath empty")
	case p.AdminDB == "":
		return errors.New("valhalla: RegionPaths.AdminDB empty")
	case p.TimezoneDB == "":
		return errors.New("valhalla: RegionPaths.TimezoneDB empty")
	}
	return nil
}
