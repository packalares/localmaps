package regions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/catalog"
)

func TestActivate_WritesFileAndSetting(t *testing.T) {
	tmp := t.TempDir()
	svc, _, db := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	svc.WithDataDir(tmp)

	// Install + flip to ready (Activate refuses anything else).
	_, _, err := svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE regions SET state = ? WHERE name = ?`,
		StateReady, "europe-romania")
	require.NoError(t, err)

	region, err := svc.Activate(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	require.Equal(t, "europe-romania", region.Name)

	// Settings row mirrors the choice.
	var raw string
	require.NoError(t, db.Get(&raw,
		`SELECT value FROM settings WHERE key = ?`, activeRegionSettingKey))
	require.Equal(t, `"europe-romania"`, raw)

	// Pointer file is at <DataDir>/regions/.active-region with the
	// canonical key, no newline.
	body, err := os.ReadFile(filepath.Join(tmp, "regions", ActiveRegionFileName))
	require.NoError(t, err)
	require.Equal(t, "europe-romania", string(body))

	// ReadActiveRegionFile mirrors the disk content.
	got, err := ReadActiveRegionFile(tmp)
	require.NoError(t, err)
	require.Equal(t, "europe-romania", got)
}

func TestActivate_RequiresReadyState(t *testing.T) {
	svc, _, _ := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	svc.WithDataDir(t.TempDir())

	// Right after install state is "downloading".
	_, _, err := svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	_, err = svc.Activate(context.Background(), "europe-romania", "alice")
	require.ErrorIs(t, err, ErrConflict)
}

func TestActivate_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	svc.WithDataDir(t.TempDir())
	_, err := svc.Activate(context.Background(), "europe-xyz", "alice")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestReadActiveRegionFile_MissingIsEmpty(t *testing.T) {
	tmp := t.TempDir()
	got, err := ReadActiveRegionFile(tmp)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func TestActivate_NoDataDirSkipsFile(t *testing.T) {
	svc, _, db := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	// dataDir intentionally left empty.
	_, _, err := svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE regions SET state = ? WHERE name = ?`,
		StateReady, "europe-romania")
	require.NoError(t, err)

	_, err = svc.Activate(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
}
