// Package pipeline — valhalla_config_test covers the generator that
// turns per-region RegionPaths + runtime config into a valhalla.json
// payload. Tests deliberately do NOT invoke any subprocess; they only
// exercise the structured JSON output.
package pipeline

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTestPaths() RegionPaths {
	return RegionPaths{
		PbfPath:    "/data/regions/europe-romania.new/source.osm.pbf",
		TileDir:    "/data/regions/europe-romania.new/valhalla_tiles",
		TarPath:    "/data/regions/europe-romania.new/valhalla_tiles.tar",
		AdminDB:    "/data/regions/europe-romania.new/valhalla_admin.sqlite",
		TimezoneDB: "/data/regions/europe-romania.new/valhalla_timezones.sqlite",
	}
}

func TestGenerateConfigRoundTrip(t *testing.T) {
	b, err := GenerateConfig("europe-romania", validTestPaths(),
		ValhallaRuntimeConfig{Concurrency: 2, BuildTimeoutMin: 60})
	require.NoError(t, err)

	// Unmarshal back into a generic map to prove the JSON is well-formed
	// and carries the mjolnir keys valhalla_build_* tools require.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(b, &raw))

	mj, ok := raw["mjolnir"].(map[string]any)
	require.True(t, ok, "mjolnir block missing")

	for _, k := range []string{"tile_dir", "tile_extract", "concurrency",
		"timezone", "admin", "logging"} {
		_, present := mj[k]
		assert.Truef(t, present, "mjolnir.%s missing", k)
	}
	assert.Equal(t, validTestPaths().TileDir, mj["tile_dir"])
	assert.Equal(t, validTestPaths().TarPath, mj["tile_extract"])
	assert.EqualValues(t, 2, mj["concurrency"])
}

func TestGenerateConfigTyped(t *testing.T) {
	b, err := GenerateConfig("eu", validTestPaths(),
		ValhallaRuntimeConfig{Concurrency: 4, BuildTimeoutMin: 30})
	require.NoError(t, err)

	var cfg BuildConfig
	require.NoError(t, json.Unmarshal(b, &cfg))
	assert.Equal(t, 4, cfg.Mjolnir.Concurrency)
	assert.Equal(t, "std_out", cfg.Mjolnir.Logging.Type)
	assert.Empty(t, cfg.Mjolnir.TransitDir)
}

func TestGenerateConfigTrailingNewline(t *testing.T) {
	b, err := GenerateConfig("eu", validTestPaths(),
		ValhallaRuntimeConfig{Concurrency: 1})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(b), "\n"),
		"expected trailing newline for POSIX friendliness")
}

func TestGenerateConfigValidation(t *testing.T) {
	good := validTestPaths()
	t.Run("empty region", func(t *testing.T) {
		_, err := GenerateConfig("", good,
			ValhallaRuntimeConfig{Concurrency: 1})
		require.Error(t, err)
	})
	t.Run("zero concurrency", func(t *testing.T) {
		_, err := GenerateConfig("eu", good,
			ValhallaRuntimeConfig{Concurrency: 0})
		require.Error(t, err)
	})
	t.Run("missing PbfPath", func(t *testing.T) {
		p := good
		p.PbfPath = ""
		_, err := GenerateConfig("eu", p,
			ValhallaRuntimeConfig{Concurrency: 1})
		require.ErrorContains(t, err, "PbfPath")
	})
	t.Run("missing TileDir", func(t *testing.T) {
		p := good
		p.TileDir = ""
		_, err := GenerateConfig("eu", p,
			ValhallaRuntimeConfig{Concurrency: 1})
		require.ErrorContains(t, err, "TileDir")
	})
	t.Run("missing TarPath", func(t *testing.T) {
		p := good
		p.TarPath = ""
		_, err := GenerateConfig("eu", p,
			ValhallaRuntimeConfig{Concurrency: 1})
		require.ErrorContains(t, err, "TarPath")
	})
	t.Run("missing AdminDB", func(t *testing.T) {
		p := good
		p.AdminDB = ""
		_, err := GenerateConfig("eu", p,
			ValhallaRuntimeConfig{Concurrency: 1})
		require.ErrorContains(t, err, "AdminDB")
	})
	t.Run("missing TimezoneDB", func(t *testing.T) {
		p := good
		p.TimezoneDB = ""
		_, err := GenerateConfig("eu", p,
			ValhallaRuntimeConfig{Concurrency: 1})
		require.ErrorContains(t, err, "TimezoneDB")
	})
}

func TestNewValhallaRuntimeConfig(t *testing.T) {
	rt := NewValhallaRuntimeConfig(8, 120, []string{"--verbose"})
	assert.Equal(t, 8, rt.Concurrency)
	assert.Equal(t, 120, rt.BuildTimeoutMin)
	assert.Equal(t, []string{"--verbose"}, rt.ExtraArgs)
}
