package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneratePeliasJSON_GoldenMinimal(t *testing.T) {
	t.Parallel()
	cfg := ImportConfig{
		Region:    "europe-romania",
		PbfPath:   "/data/source.osm.pbf",
		ESHost:    "pelias-es",
		ESPort:    9200,
		IndexName: "pelias-europe-romania-20260424",
		Languages: []string{"en"},
	}

	got, err := GeneratePeliasJSON(cfg)
	require.NoError(t, err)

	goldenPath := filepath.Join("testdata", "pelias", "pelias.minimal.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(goldenPath, got, 0o644))
	}
	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "golden file missing; run with UPDATE_GOLDEN=1")
	require.Equal(t, string(want), string(got))
}

func TestGeneratePeliasJSON_ValidJSON_AndFields(t *testing.T) {
	t.Parallel()
	cfg := ImportConfig{
		Region:    "europe-romania",
		PbfPath:   "/data/source.osm.pbf",
		ESHost:    "pelias-es",
		ESPort:    9200,
		IndexName: "pelias-europe-romania-20260424",
		Languages: []string{"ro", "en"},
	}
	b, err := GeneratePeliasJSON(cfg)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(b, &decoded), "output must be valid JSON")

	// esclient hosts shape
	es := decoded["esclient"].(map[string]any)
	require.Equal(t, false, es["keepAlive"], "keepAlive must be false to avoid idle-conn hangs")
	hosts := es["hosts"].([]any)
	require.Len(t, hosts, 1)
	host := hosts[0].(map[string]any)
	require.Equal(t, "pelias-es", host["host"])
	require.EqualValues(t, 9200, host["port"])
	require.Equal(t, "http", host["protocol"])

	// imports.openstreetmap wiring
	imports := decoded["imports"].(map[string]any)
	osm := imports["openstreetmap"].(map[string]any)
	require.Equal(t, "/data", osm["datapath"])
	files := osm["import"].([]any)
	require.Equal(t, "source.osm.pbf", files[0].(map[string]any)["filename"])

	// api.indexName propagated
	api := decoded["api"].(map[string]any)
	require.Equal(t, "pelias-europe-romania-20260424", api["indexName"])

	// Languages sorted
	langs := api["languages"].([]any)
	require.Equal(t, []any{"en", "ro"}, langs)

	// Polylines disabled by default
	_, hasPoly := imports["polylines"]
	require.False(t, hasPoly)
}

func TestGeneratePeliasJSON_PolylinesEnabled(t *testing.T) {
	t.Parallel()
	cfg := ImportConfig{
		Region:           "europe-romania",
		PbfPath:          "/data/source.osm.pbf",
		ESHost:           "pelias-es",
		ESPort:           9200,
		IndexName:        "pelias-europe-romania-20260424",
		Languages:        []string{"en"},
		PolylinesEnabled: true,
	}
	b, err := GeneratePeliasJSON(cfg)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(b, &decoded))
	imports := decoded["imports"].(map[string]any)
	poly := imports["polylines"].(map[string]any)
	files := poly["files"].([]any)
	require.Equal(t, "europe-romania.polylines", files[0])
}

func TestGeneratePeliasJSON_DefaultsLanguages(t *testing.T) {
	t.Parallel()
	cfg := ImportConfig{
		Region:    "europe-romania",
		PbfPath:   "/data/source.osm.pbf",
		ESHost:    "pelias-es",
		ESPort:    9200,
		IndexName: "pelias-europe-romania-20260424",
	}
	b, err := GeneratePeliasJSON(cfg)
	require.NoError(t, err)
	require.Contains(t, string(b), `"en"`)
}

func TestGeneratePeliasJSON_Validation(t *testing.T) {
	t.Parallel()
	cases := map[string]ImportConfig{
		"empty region": {PbfPath: "/data/x.pbf", ESHost: "h", ESPort: 9200, IndexName: "i"},
		"empty pbf":    {Region: "r", ESHost: "h", ESPort: 9200, IndexName: "i"},
		"empty host":   {Region: "r", PbfPath: "/p", ESPort: 9200, IndexName: "i"},
		"bad port":     {Region: "r", PbfPath: "/p", ESHost: "h", ESPort: 0, IndexName: "i"},
		"empty index":  {Region: "r", PbfPath: "/p", ESHost: "h", ESPort: 9200},
	}
	for name, cfg := range cases {
		cfg := cfg
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := GeneratePeliasJSON(cfg)
			require.Error(t, err)
		})
	}
}

func TestGeneratePeliasJSON_NoUpstreamKeyDrift(t *testing.T) {
	// Guard: the only top-level keys we may emit are those documented
	// in the spec. If somebody adds a key, this test fails until the
	// doc is updated too.
	t.Parallel()
	b, err := GeneratePeliasJSON(ImportConfig{
		Region: "r", PbfPath: "/data/r.pbf", ESHost: "h", ESPort: 9200, IndexName: "i",
	})
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	allowed := map[string]struct{}{
		"logger": {}, "esclient": {}, "acceptLanguage": {},
		"api": {}, "imports": {},
	}
	for k := range m {
		_, ok := allowed[k]
		require.Truef(t, ok, "unexpected top-level key %q (not in allow-list: %s)",
			k, strings.Join(keys(allowed), ","))
	}
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
