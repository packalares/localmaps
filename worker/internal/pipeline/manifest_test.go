package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 4, 24, 10, 11, 12, 0, time.UTC)
	in := TilesSection{
		SourceURL:            "https://download.geofabrik.de/europe/romania-latest.osm.pbf",
		SourceSHA256:         "abc123",
		SourceBytes:          100,
		BuiltAt:              ts,
		BuildDurationSeconds: 123.45,
		Tool:                 "planetiler",
		ToolVersion:          "v0.7.0",
		OutputFile:           "map.pmtiles",
		OutputBytes:          1_234_567,
		OutputTileCount:      42,
	}
	require.NoError(t, UpdateTilesSection(dir, "europe-romania", in))

	m, err := ReadManifest(dir)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, "europe-romania", m.Region)
	require.Equal(t, ManifestVersion, m.Version)
	require.NotNil(t, m.Tiles)
	require.Equal(t, in.Tool, m.Tiles.Tool)
	require.Equal(t, in.OutputBytes, m.Tiles.OutputBytes)
	require.True(t, in.BuiltAt.Equal(m.Tiles.BuiltAt))
}

func TestReadManifestAbsent(t *testing.T) {
	dir := t.TempDir()
	m, err := ReadManifest(dir)
	require.NoError(t, err)
	require.Nil(t, m)
}

// TestManifestPreservesUnknownSections simulates Agents G/H writing
// routing/geocoding sections that F's tiles update must NOT clobber.
func TestManifestPreservesUnknownSections(t *testing.T) {
	dir := t.TempDir()
	// Simulate a manifest.json already written by an earlier stage.
	earlier := map[string]any{
		"region":  "europe-romania",
		"version": ManifestVersion,
		"routing": map[string]any{
			"tool":    "valhalla",
			"tileDir": "valhalla_tiles",
		},
		"geocoding": map[string]any{
			"tool":    "pelias",
			"indexed": 10000,
		},
	}
	raw, err := json.MarshalIndent(earlier, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFileName), raw, 0o644))

	// Now F updates tiles.
	require.NoError(t, UpdateTilesSection(dir, "europe-romania", TilesSection{
		Tool:       "planetiler",
		OutputFile: "map.pmtiles",
		BuiltAt:    time.Now().UTC(),
	}))

	// Re-read and assert routing + geocoding survived.
	data, err := os.ReadFile(filepath.Join(dir, ManifestFileName))
	require.NoError(t, err)
	var round map[string]any
	require.NoError(t, json.Unmarshal(data, &round))
	require.Contains(t, round, "routing")
	require.Contains(t, round, "geocoding")
	require.Contains(t, round, "tiles")

	routing := round["routing"].(map[string]any)
	require.Equal(t, "valhalla", routing["tool"])
}

func TestUpdateRoutingAndGeocodingSections(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 4, 24, 1, 2, 3, 0, time.UTC)
	require.NoError(t, UpdateTilesSection(dir, "europe-monaco", TilesSection{
		Tool: "planetiler", OutputFile: "map.pmtiles", BuiltAt: ts,
	}))
	require.NoError(t, UpdateRoutingSection(dir, "europe-monaco", RoutingSection{
		Tool: "valhalla_build_tiles", TileDir: "valhalla_tiles", BuiltAt: ts,
	}))
	require.NoError(t, UpdateGeocodingSection(dir, "europe-monaco", GeocodingSection{
		Tool: "pelias-openstreetmap", IndexName: "pelias-europe-monaco-20260424", BuiltAt: ts,
	}))

	m, err := ReadManifest(dir)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, "europe-monaco", m.Region)
	require.NotNil(t, m.Tiles)
	require.NotNil(t, m.Routing)
	require.NotNil(t, m.Geocoding)
	require.Equal(t, "map.pmtiles", m.Tiles.OutputFile)
	require.Equal(t, "valhalla_tiles", m.Routing.TileDir)
	require.Equal(t, "pelias-europe-monaco-20260424", m.Geocoding.IndexName)
}

func TestWriteManifestAtomic(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{Region: "r", Version: ManifestVersion,
		Tiles: &TilesSection{Tool: "planetiler", OutputFile: "map.pmtiles"}}
	require.NoError(t, WriteManifest(dir, m))
	// No stray .manifest.*.json left over.
	ents, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range ents {
		require.NotContains(t, e.Name(), ".manifest.", "no temp file leftover")
	}
}

// TestConcurrentAppendsDifferentSectionsRace exercises a realistic race:
// two stages run UpdateTilesSection serially (our public API does NOT
// claim cross-stage concurrency safety — Primary's orchestrator
// serialises per region). We still make sure individual calls don't
// corrupt the file, even with file-system interleaving.
func TestManifestWriteParallelSafe(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := UpdateTilesSection(dir, "r", TilesSection{
				Tool: "planetiler", OutputBytes: int64(i), BuiltAt: time.Now().UTC()})
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()
	// Whichever won last must still be valid JSON.
	m, err := ReadManifest(dir)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.NotNil(t, m.Tiles)
	require.Equal(t, "planetiler", m.Tiles.Tool)
}
