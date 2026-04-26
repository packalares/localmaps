package install

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/jobs"
)

// stubSettings is a tiny in-memory PeliasURLReader for tests.
type stubSettings struct {
	strings map[string]string
	bools   map[string]bool
}

func (s *stubSettings) GetString(key string) (string, error) {
	if v, ok := s.strings[key]; ok {
		return v, nil
	}
	return "", sql.ErrNoRows
}

func (s *stubSettings) GetBool(key string) (bool, error) {
	if v, ok := s.bools[key]; ok {
		return v, nil
	}
	return false, sql.ErrNoRows
}

// TestDeleteHandler_WipesDirsAndPurgesPelias is the happy path: live
// dir + .new dir on disk, ES reachable, delete-by-query honoured,
// jobs/regions row finalised.
func TestDeleteHandler_WipesDirsAndPurgesPelias(t *testing.T) {
	db := openTestDB(t)

	// Stash the region as archived (server's mutations.Delete already
	// did this transition before enqueueing the worker task).
	_, err := db.Exec(
		`UPDATE regions SET state = 'archived', active_job_id = 'job-del'
		 WHERE name = 'europe-monaco'`)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO jobs (id, kind, state) VALUES ('job-del', 'archive_region', 'queued')`)
	require.NoError(t, err)

	dataDir := t.TempDir()
	liveDir := filepath.Join(dataDir, "regions", "europe-monaco")
	require.NoError(t, os.MkdirAll(liveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(liveDir, "map.pmtiles"),
		[]byte("live"), 0o644))
	newDir := filepath.Join(dataDir, "regions", "europe-monaco.new")
	require.NoError(t, os.MkdirAll(newDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(newDir, "source.osm.pbf"),
		[]byte("partial"), 0o644))

	var purgePath string
	var purgeBody []byte
	es := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		purgePath = r.URL.Path
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		purgeBody = buf
		_, _ = w.Write([]byte(`{"deleted":7,"failures":[]}`))
	}))
	defer es.Close()

	deps := DeleteDeps{
		Deps: Deps{
			DB:      db,
			DataDir: dataDir,
			HTTP:    http.DefaultClient,
		},
		Settings: &stubSettings{
			strings: map[string]string{"search.peliasElasticUrl": es.URL},
			bools:   map[string]bool{"regions.deletePurgesPelias": true},
		},
	}
	handler := NewDeleteHandler(deps, zerolog.Nop())
	payload := []byte(`{"region":"europe-monaco","jobId":"job-del"}`)
	require.NoError(t, handler(context.Background(),
		asynq.NewTask(jobs.KindRegionDelete, payload)))

	require.NoDirExists(t, liveDir)
	require.NoDirExists(t, newDir)

	require.Equal(t, "/pelias/_delete_by_query", purgePath)
	require.Contains(t, string(purgeBody), `"europe-monaco"`)
	require.Contains(t, string(purgeBody), `"addendum.osm.region"`)

	var jobState string
	err = db.QueryRow(`SELECT state FROM jobs WHERE id = ?`, "job-del").Scan(&jobState)
	require.NoError(t, err)
	require.Equal(t, "succeeded", jobState)

	var activeJob sql.NullString
	err = db.QueryRow(`SELECT active_job_id FROM regions WHERE name = ?`,
		"europe-monaco").Scan(&activeJob)
	require.NoError(t, err)
	require.False(t, activeJob.Valid, "active_job_id must be cleared")
}

// TestDeleteHandler_SkipsPurgeWhenSettingFalse covers the
// `regions.deletePurgesPelias=false` opt-out path.
func TestDeleteHandler_SkipsPurgeWhenSettingFalse(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(
		`UPDATE regions SET state = 'archived' WHERE name = 'europe-monaco'`)
	require.NoError(t, err)

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(
		filepath.Join(dataDir, "regions", "europe-monaco"), 0o755))

	var hits int
	es := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"deleted":0}`))
	}))
	defer es.Close()

	deps := DeleteDeps{
		Deps: Deps{DB: db, DataDir: dataDir, HTTP: http.DefaultClient},
		Settings: &stubSettings{
			strings: map[string]string{"search.peliasElasticUrl": es.URL},
			bools:   map[string]bool{"regions.deletePurgesPelias": false},
		},
	}
	handler := NewDeleteHandler(deps, zerolog.Nop())
	require.NoError(t, handler(context.Background(),
		asynq.NewTask(jobs.KindRegionDelete,
			[]byte(`{"region":"europe-monaco","jobId":"job-1"}`))))
	require.Equal(t, 0, hits, "ES must not be hit when purge setting is false")
}

// TestDeleteHandler_IgnoresMissingDirs covers the scheduler-driven
// case where a delete races a never-installed region.
func TestDeleteHandler_IgnoresMissingDirs(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(
		`UPDATE regions SET state = 'archived' WHERE name = 'europe-monaco'`)
	require.NoError(t, err)

	es := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"deleted":0}`))
	}))
	defer es.Close()
	deps := DeleteDeps{
		Deps: Deps{DB: db, DataDir: t.TempDir(), HTTP: http.DefaultClient},
		Settings: &stubSettings{
			strings: map[string]string{"search.peliasElasticUrl": es.URL},
			bools:   map[string]bool{"regions.deletePurgesPelias": true},
		},
	}
	handler := NewDeleteHandler(deps, zerolog.Nop())
	require.NoError(t, handler(context.Background(),
		asynq.NewTask(jobs.KindRegionDelete,
			[]byte(`{"region":"europe-monaco","jobId":"job-1"}`))))
}

func TestDeleteHandler_RejectsMalformedPayload(t *testing.T) {
	handler := NewDeleteHandler(DeleteDeps{}, zerolog.Nop())
	require.Error(t, handler(context.Background(),
		asynq.NewTask(jobs.KindRegionDelete, []byte("{"))))
}

func TestDeleteHandler_EmptyRegion(t *testing.T) {
	handler := NewDeleteHandler(DeleteDeps{}, zerolog.Nop())
	require.Error(t, handler(context.Background(),
		asynq.NewTask(jobs.KindRegionDelete, []byte(`{"region":""}`))))
}
