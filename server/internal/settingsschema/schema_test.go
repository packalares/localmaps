package settingsschema_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/settingsschema"
)

func TestBuildSchema_EveryDefaultHasNode(t *testing.T) {
	nodes := settingsschema.BuildSchema(config.Defaults())
	byKey := settingsschema.ByKey(nodes)

	for _, d := range config.Defaults() {
		if d.Key == "schema.version" {
			continue
		}
		n, ok := byKey[d.Key]
		require.Truef(t, ok, "missing schema node for %q", d.Key)
		require.Equal(t, d.Key, n.Key)
		require.NotEmpty(t, n.UIGroup, "node %q has no UIGroup", d.Key)
		require.NotEmpty(t, string(n.Type), "node %q has no Type", d.Key)
	}
}

func TestBuildSchema_EnumsApplied(t *testing.T) {
	nodes := settingsschema.BuildSchema(config.Defaults())
	byKey := settingsschema.ByKey(nodes)

	style := byKey["map.style"]
	require.Equal(t, settingsschema.TypeEnum, style.Type)
	require.ElementsMatch(t,
		[]any{"light", "dark", "auto"}, style.Enum)

	// Regions default-schedule is still an enum we can sanity-check.
	sched := byKey["regions.defaultSchedule"]
	require.Equal(t, settingsschema.TypeEnum, sched.Type)
	require.Len(t, sched.Enum, 4)
}

func TestBuildSchema_RangesApplied(t *testing.T) {
	nodes := settingsschema.BuildSchema(config.Defaults())
	byKey := settingsschema.ByKey(nodes)

	n := byKey["map.maxZoom"]
	require.Equal(t, settingsschema.TypeInteger, n.Type)
	require.NotNil(t, n.Min)
	require.NotNil(t, n.Max)
	require.Equal(t, float64(0), *n.Min)
	require.Equal(t, float64(19), *n.Max)
}

func TestBuildSchema_UIGroupIsTopLevelKey(t *testing.T) {
	nodes := settingsschema.BuildSchema(config.Defaults())
	byKey := settingsschema.ByKey(nodes)
	require.Equal(t, "routing", byKey["routing.truck.heightMeters"].UIGroup)
	require.Equal(t, "map", byKey["map.defaultCenter"].UIGroup)
}

func TestBuildSchema_IsStable(t *testing.T) {
	a := settingsschema.BuildSchema(config.Defaults())
	b := settingsschema.BuildSchema(config.Defaults())
	require.Equal(t, a, b)
	for i := 1; i < len(a); i++ {
		require.LessOrEqual(t, a[i-1].Key, a[i].Key, "not sorted")
	}
}

func TestBuildSchema_SchemaVersionExcluded(t *testing.T) {
	// schema.version is not present in Defaults() anyway, but we guard
	// against a future accidental introduction.
	defs := append([]config.Default{{Key: "schema.version", Value: 1}}, config.Defaults()...)
	nodes := settingsschema.BuildSchema(defs)
	byKey := settingsschema.ByKey(nodes)
	_, ok := byKey["schema.version"]
	require.False(t, ok)
}

func TestValidateAnnotations_NoDrift(t *testing.T) {
	drift := settingsschema.ValidateAnnotations(config.Defaults())
	require.Empty(t, drift, "annotations reference keys not in defaults: %v", drift)
}

func TestValidateValue_AllBranches(t *testing.T) {
	nodes := settingsschema.BuildSchema(config.Defaults())
	byKey := settingsschema.ByKey(nodes)

	require.NoError(t, settingsschema.ValidateValue(byKey["map.style"], "dark"))
	require.Error(t, settingsschema.ValidateValue(byKey["map.style"], "neon"))

	require.NoError(t, settingsschema.ValidateValue(byKey["map.maxZoom"], 14))
	require.Error(t, settingsschema.ValidateValue(byKey["map.maxZoom"], 99))
	require.Error(t, settingsschema.ValidateValue(byKey["map.maxZoom"], 1.5))
	require.Error(t, settingsschema.ValidateValue(byKey["map.maxZoom"], "fourteen"))

	require.NoError(t, settingsschema.ValidateValue(byKey["routing.truck.heightMeters"], 3.5))
	require.Error(t, settingsschema.ValidateValue(byKey["routing.truck.heightMeters"], 50.0))

	require.NoError(t, settingsschema.ValidateValue(byKey["map.showBuildings3D"], true))
	require.Error(t, settingsschema.ValidateValue(byKey["map.showBuildings3D"], "true"))

	require.NoError(t, settingsschema.ValidateValue(byKey["pois.sources"], []any{"overture", "osm"}))
	require.Error(t, settingsschema.ValidateValue(byKey["pois.sources"], []any{"overture", 1}))

	require.NoError(t, settingsschema.ValidateValue(byKey["map.defaultCenter"],
		map[string]any{"lat": 0.0, "lon": 0.0, "zoom": 2.0}))
	require.Error(t, settingsschema.ValidateValue(byKey["map.defaultCenter"], "nope"))

	require.NoError(t, settingsschema.ValidateValue(byKey["ui.brandColor"], "#0ea5e9"))
	require.Error(t, settingsschema.ValidateValue(byKey["ui.brandColor"], "sky"))
}

func TestTree_AndFlatten_Roundtrip(t *testing.T) {
	flat := map[string]any{
		"map.style":          "dark",
		"map.maxZoom":        14,
		"routing.avoidTolls": true,
		"map.defaultCenter":  map[string]any{"lat": 1.0, "lon": 2.0, "zoom": 3.0},
	}
	tree := settingsschema.Tree(flat)
	require.IsType(t, map[string]any{}, tree["map"])
	got := settingsschema.Flatten(tree)
	require.Equal(t, flat, got)
}
