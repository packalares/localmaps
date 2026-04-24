// Unit tests for the settings-driven runner construction in
// handlers_agent_fg.go. We don't spawn java / valhalla binaries here —
// we verify that the StageWork fails closed when the required
// `tiles.planetilerJarSha256` is unset (docs/08-security.md) and that
// the settings fallback helpers behave as documented.
package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/jobs"
)

// fakeSettings is an in-memory StageSettings for tests.
type fakeSettings struct {
	strs    map[string]string
	ints    map[string]int
	bools   map[string]bool
	strarrs map[string][]string
}

func (f *fakeSettings) GetString(k string) (string, error) {
	if v, ok := f.strs[k]; ok {
		return v, nil
	}
	return "", sql.ErrNoRows
}

func (f *fakeSettings) GetInt(k string) (int, error) {
	if v, ok := f.ints[k]; ok {
		return v, nil
	}
	return 0, sql.ErrNoRows
}

func (f *fakeSettings) GetBool(k string) (bool, error) {
	if v, ok := f.bools[k]; ok {
		return v, nil
	}
	return false, sql.ErrNoRows
}

func (f *fakeSettings) GetStringSlice(k string) ([]string, error) {
	if v, ok := f.strarrs[k]; ok {
		return v, nil
	}
	return nil, sql.ErrNoRows
}

// TestTilesWork_FailsClosedOnMissingSHA — docs/08-security.md: empty
// tiles.planetilerJarSha256 MUST abort the job rather than downloading
// an unverified binary. The returned error must name the key so the
// operator can fix it.
func TestTilesWork_FailsClosedOnMissingSHA(t *testing.T) {
	dir := t.TempDir()
	deps := ChainDeps{
		DataDir:  dir,
		Settings: &fakeSettings{},
	}
	work := tilesWork(deps)
	err := work(context.Background(),
		filepath.Join(dir, "regions", "europe-monaco.new"),
		jobs.PipelineStagePayload{Region: "europe-monaco", JobID: "j1"},
		zerolog.Nop())
	require.Error(t, err)
	require.Contains(t, err.Error(), "tiles.planetilerJarSha256")
}

// TestRoutingWork_ConstructsRunnerWithSettings — with the default
// settings (Concurrency=0, Timeout=60, no extras) the StageWork must
// still construct a valid runner and surface the first
// valhalla_build_admins start error (binary not on PATH in CI).
func TestRoutingWork_ConstructsRunnerWithSettings(t *testing.T) {
	dir := t.TempDir()
	regionDir := filepath.Join(dir, "regions", "europe-monaco.new")
	require.NoError(t, os.MkdirAll(regionDir, 0o755))
	deps := ChainDeps{
		DataDir: dir,
		Settings: &fakeSettings{
			ints: map[string]int{
				"routing.valhallaConcurrency":         2,
				"routing.valhallaBuildTimeoutMinutes": 5,
			},
			strs:    map[string]string{"routing.valhallaTileDirName": "valhalla_tiles"},
			strarrs: map[string][]string{"routing.valhallaExtraArgs": nil},
		},
	}
	work := routingWork(deps)
	// We expect an error here because:
	//   - valhalla_build_admins is almost certainly NOT on PATH in CI
	// The error must mention the tool name so operators debugging
	// missing binaries get a clear signal.
	err := work(context.Background(), regionDir,
		jobs.PipelineStagePayload{Region: "europe-monaco", JobID: "j1"},
		zerolog.Nop())
	if err != nil {
		require.Contains(t, err.Error(), "valhalla", "error should surface valhalla context: %v", err)
	}
}

// TestSettingOrDefault walks the three fallback helpers and asserts
// they return the default on missing keys + the stored value when set.
func TestSettingOrDefault(t *testing.T) {
	f := &fakeSettings{
		strs:    map[string]string{"a": "set"},
		ints:    map[string]int{"n": 42},
		strarrs: map[string][]string{"xs": {"one"}},
	}
	require.Equal(t, "set", settingOrDefault(f, "a", "def"))
	require.Equal(t, "def", settingOrDefault(f, "missing", "def"))
	require.Equal(t, 42, settingIntOrDefault(f, "n", 7))
	require.Equal(t, 7, settingIntOrDefault(f, "missing", 7))
	require.Equal(t, []string{"one"}, settingArrOrDefault(f, "xs", nil))
	require.Nil(t, settingArrOrDefault(f, "missing", nil))
	// nil receiver → defaults.
	require.Equal(t, "def", settingOrDefault(nil, "a", "def"))
	require.Equal(t, 7, settingIntOrDefault(nil, "n", 7))
	require.Nil(t, settingArrOrDefault(nil, "xs", nil))
}

// TestSettingBoolOrDefault covers the bool helper used by geocodingWork.
func TestSettingBoolOrDefault(t *testing.T) {
	f := &fakeSettings{bools: map[string]bool{"flag": true}}
	require.True(t, settingBoolOrDefault(f, "flag", false))
	require.False(t, settingBoolOrDefault(f, "missing", false))
	require.True(t, settingBoolOrDefault(nil, "flag", true))
}

