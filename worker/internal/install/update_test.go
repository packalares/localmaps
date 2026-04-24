package install

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/catalog"
	"github.com/packalares/localmaps/internal/jobs"
)

func TestUpdateHandler_OrchestratesUpdateIntoDotNewDir(t *testing.T) {
	fixture, err := os.ReadFile(findFixture(t))
	require.NoError(t, err)

	srv := servePbf(t, fixture)
	defer srv.Close()

	db := openTestDB(t)
	// Pretend the region is already ready with an older md5.
	_, err = db.Exec(
		`UPDATE regions SET state = 'ready', source_pbf_sha256 = 'old-sum'
		 WHERE name = ?`, "europe-monaco")
	require.NoError(t, err)

	q := &fakeQueue{}
	dataDir := t.TempDir()
	// Simulate a live <region>/ directory existing before the update.
	liveDir := filepath.Join(dataDir, "regions", "europe-monaco")
	require.NoError(t, os.MkdirAll(liveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(liveDir, "map.pmtiles"),
		[]byte("old-live-tiles"), 0o644))

	entry := catalog.Entry{
		Name:        "europe-monaco",
		DisplayName: "Monaco",
		SourceURL:   srv.URL + "/europe/monaco-latest.osm.pbf",
	}
	deps := Deps{
		DB:      db,
		DataDir: dataDir,
		Catalog: &stubCatalog{byKey: map[string]catalog.Entry{"europe-monaco": entry}},
		Queue:   q,
		HTTP:    http.DefaultClient,
	}
	handler := NewUpdateHandler(deps, zerolog.Nop())

	payload := []byte(`{"region":"europe-monaco","jobId":"job-1","triggeredBy":"scheduler"}`)
	task := asynq.NewTask(jobs.KindRegionUpdate, payload)
	require.NoError(t, handler(context.Background(), task))

	// Pipeline chain kicked off.
	require.Equal(t, []string{jobs.KindPipelineTiles}, q.types())

	// PBF on disk under <dataDir>/regions/europe-monaco.new/source.osm.pbf
	newPath := filepath.Join(dataDir, "regions", "europe-monaco.new", "source.osm.pbf")
	got, err := os.ReadFile(newPath)
	require.NoError(t, err)
	require.Equal(t, fixture, got)

	// Live directory is untouched by the update handler — swap.go is
	// responsible for promoting .new → live at the end of the chain.
	live, err := os.ReadFile(filepath.Join(liveDir, "map.pmtiles"))
	require.NoError(t, err)
	require.Equal(t, []byte("old-live-tiles"), live)

	// Region row transitioned to 'updating' (not 'downloading').
	var state string
	err = db.QueryRow(`SELECT state FROM regions WHERE name = ?`, "europe-monaco").Scan(&state)
	require.NoError(t, err)
	require.Equal(t, "updating", state)

	// source_pbf_sha256 + bytes persisted with the fresh values.
	var sum string
	var sz int64
	err = db.QueryRow(
		`SELECT source_pbf_sha256, source_pbf_bytes FROM regions WHERE name = ?`,
		"europe-monaco").Scan(&sum, &sz)
	require.NoError(t, err)
	require.NotEmpty(t, sum)
	require.NotEqual(t, "old-sum", sum, "md5 should have been rewritten")
	require.EqualValues(t, len(fixture), sz)
}

func TestUpdateHandler_FailureMarksRegionFailed(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(
		`UPDATE regions SET state = 'ready' WHERE name = ?`, "europe-monaco")
	require.NoError(t, err)

	q := &fakeQueue{}
	deps := Deps{
		DB:      db,
		DataDir: t.TempDir(),
		Catalog: &stubCatalog{byKey: map[string]catalog.Entry{}}, // Resolve errors
		Queue:   q,
		HTTP:    http.DefaultClient,
	}
	handler := NewUpdateHandler(deps, zerolog.Nop())

	err = handler(context.Background(),
		asynq.NewTask(jobs.KindRegionUpdate,
			[]byte(`{"region":"europe-monaco","jobId":"job-1"}`)))
	require.Error(t, err)

	var state, lastErr string
	err = db.QueryRow(`SELECT state, COALESCE(last_error, '') FROM regions WHERE name = ?`,
		"europe-monaco").Scan(&state, &lastErr)
	require.NoError(t, err)
	require.Equal(t, "failed", state)
	require.Contains(t, lastErr, "resolve catalog")
	require.Empty(t, q.types(), "no tasks enqueued on failure")
}

func TestUpdateHandler_RejectsMalformedPayload(t *testing.T) {
	handler := NewUpdateHandler(Deps{}, zerolog.Nop())
	err := handler(context.Background(),
		asynq.NewTask(jobs.KindRegionUpdate, []byte("{")))
	require.Error(t, err)
}

func TestUpdateHandler_EmptyRegion(t *testing.T) {
	handler := NewUpdateHandler(Deps{}, zerolog.Nop())
	err := handler(context.Background(),
		asynq.NewTask(jobs.KindRegionUpdate, []byte(`{"region":""}`)))
	require.Error(t, err)
}
