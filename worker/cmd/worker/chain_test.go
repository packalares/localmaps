// Chain integration test. Drives a full region install through the
// F → G → H → swap chain against an in-memory SQLite DB and a
// recording mock Asynq queue. Fake StageWork + SwapWork functions
// stand in for the real subprocess runners — we're verifying the glue
// here, not the runners.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/packalares/localmaps/internal/jobs"
)

// recordingQueue is a ChainEnqueuer that captures every enqueued task.
type recordingQueue struct {
	tasks []*asynq.Task
}

func (r *recordingQueue) EnqueueContext(_ context.Context, t *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	r.tasks = append(r.tasks, t)
	return &asynq.TaskInfo{ID: "mock-" + t.Type()}, nil
}

// openTestDB builds an in-memory SQLite with the minimal regions + jobs
// schema we exercise in the chain. It uses the exact DDL shape from
// docs/04-data-model.md (subset).
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	for _, ddl := range []string{
		`CREATE TABLE regions (
			name TEXT PRIMARY KEY, display_name TEXT, parent TEXT,
			source_url TEXT, source_pbf_sha256 TEXT, source_pbf_bytes INTEGER,
			bbox TEXT, state TEXT NOT NULL, state_detail TEXT, last_error TEXT,
			installed_at TEXT, last_updated_at TEXT, next_update_at TEXT,
			schedule TEXT, disk_bytes INTEGER, active_job_id TEXT
		)`,
		`CREATE TABLE jobs (
			id TEXT PRIMARY KEY, kind TEXT, region TEXT,
			state TEXT NOT NULL, progress REAL, message TEXT, error TEXT,
			started_at TEXT, finished_at TEXT, payload TEXT
		)`,
	} {
		_, err := db.Exec(ddl)
		require.NoError(t, err)
	}
	return db
}

// seedRegion creates a regions + jobs row pair in state=downloading,
// ready for the pipeline chain to take over.
func seedRegion(t *testing.T, db *sql.DB, region, jobID string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO regions (name, display_name, state, state_detail, source_url, active_job_id)
		VALUES (?, ?, 'downloading', 'source downloaded', 'https://example/x.osm.pbf', ?)`,
		region, region, jobID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO jobs (id, kind, region, state, progress) VALUES (?, 'region_install', ?, 'running', 0.0)`,
		jobID, region)
	require.NoError(t, err)
}

// stagePayload returns a PipelineStagePayload task for the given kind.
func stagePayload(t *testing.T, kind, region, jobID string) *asynq.Task {
	t.Helper()
	b, err := json.Marshal(jobs.PipelineStagePayload{Region: region, JobID: jobID, ParentJobID: jobID})
	require.NoError(t, err)
	return asynq.NewTask(kind, b)
}

func TestPipelineChain_HappyPath(t *testing.T) {
	ctx := context.Background()
	log := zerolog.Nop()

	dir := t.TempDir()
	regionDir := filepath.Join(dir, "regions", "europe-monaco.new")
	require.NoError(t, os.MkdirAll(regionDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(regionDir, "source.osm.pbf"),
		[]byte("stub pbf"), 0o644))

	db := openTestDB(t)
	seedRegion(t, db, "europe-monaco", "job-1")

	q := &recordingQueue{}
	deps := ChainDeps{DB: db, Queue: q, DataDir: dir}

	// Fake StageWork that succeeds instantly so we're testing chain
	// glue, not runners.
	ok := func(context.Context, string, jobs.PipelineStagePayload, zerolog.Logger) error { return nil }

	// --- tiles ---
	tilesH := stageHandler(deps, "tiles", "processing_tiles",
		jobs.KindPipelineRouting, ok, log)
	require.NoError(t, tilesH(ctx, stagePayload(t, jobs.KindPipelineTiles, "europe-monaco", "job-1")))
	require.Len(t, q.tasks, 1)
	require.Equal(t, jobs.KindPipelineRouting, q.tasks[0].Type())
	require.Equal(t, "processing_tiles", queryState(t, db, "europe-monaco"))

	// --- routing ---
	routingH := stageHandler(deps, "routing", "processing_routing",
		jobs.KindPipelineGeocoding, ok, log)
	require.NoError(t, routingH(ctx, stagePayload(t, jobs.KindPipelineRouting, "europe-monaco", "job-1")))
	require.Len(t, q.tasks, 2)
	require.Equal(t, jobs.KindPipelineGeocoding, q.tasks[1].Type())
	require.Equal(t, "processing_routing", queryState(t, db, "europe-monaco"))

	// --- geocoding → enqueues swap (RegionSwapPayload shape) ---
	geoH := stageHandler(deps, "geocoding", "processing_geocoding",
		jobs.KindRegionSwap, ok, log)
	require.NoError(t, geoH(ctx, stagePayload(t, jobs.KindPipelineGeocoding, "europe-monaco", "job-1")))
	require.Len(t, q.tasks, 3)
	require.Equal(t, jobs.KindRegionSwap, q.tasks[2].Type())
	var swapP jobs.RegionSwapPayload
	require.NoError(t, json.Unmarshal(q.tasks[2].Payload(), &swapP))
	require.Equal(t, "europe-monaco", swapP.Region)
	require.Equal(t, "job-1", swapP.JobID)
	require.Equal(t, "processing_geocoding", queryState(t, db, "europe-monaco"))

	// --- swap ---
	swapH := swapHandler(deps, runSwap, log)
	require.NoError(t, swapH(ctx, q.tasks[2]))
	require.Equal(t, "ready", queryState(t, db, "europe-monaco"))
	// <region>.new was renamed to <region>
	_, err := os.Stat(filepath.Join(dir, "regions", "europe-monaco"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "regions", "europe-monaco.new"))
	require.True(t, os.IsNotExist(err), "expected .new dir to be gone after swap")

	// jobs row closes as succeeded.
	var jobState string
	require.NoError(t, db.QueryRow(`SELECT state FROM jobs WHERE id = ?`, "job-1").Scan(&jobState))
	require.Equal(t, "succeeded", jobState)
}

func TestPipelineChain_StageFailureMarksFailed(t *testing.T) {
	ctx := context.Background()
	log := zerolog.Nop()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "regions", "europe-monaco.new"), 0o755))
	db := openTestDB(t)
	seedRegion(t, db, "europe-monaco", "job-2")
	q := &recordingQueue{}
	deps := ChainDeps{DB: db, Queue: q, DataDir: dir}

	boom := func(context.Context, string, jobs.PipelineStagePayload, zerolog.Logger) error {
		return asErr("planetiler exited 1")
	}
	h := stageHandler(deps, "tiles", "processing_tiles",
		jobs.KindPipelineRouting, boom, log)
	err := h(ctx, stagePayload(t, jobs.KindPipelineTiles, "europe-monaco", "job-2"))
	require.Error(t, err)

	// Region + job flipped to failed; last_error preserved.
	require.Equal(t, "failed", queryState(t, db, "europe-monaco"))
	var lastErr, jobState, jobErr sql.NullString
	require.NoError(t, db.QueryRow(`SELECT last_error FROM regions WHERE name = ?`, "europe-monaco").Scan(&lastErr))
	require.Contains(t, lastErr.String, "planetiler exited 1")
	require.NoError(t, db.QueryRow(`SELECT state, error FROM jobs WHERE id = ?`, "job-2").Scan(&jobState, &jobErr))
	require.Equal(t, "failed", jobState.String)
	require.Contains(t, jobErr.String, "planetiler exited 1")

	// No chain-forward after failure.
	require.Empty(t, q.tasks)
}

func TestSwap_PreservesExistingOnFailure(t *testing.T) {
	ctx := context.Background()
	log := zerolog.Nop()
	dir := t.TempDir()
	// <region> exists from prior install; <region>.new does NOT → swap fails.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "regions", "europe-monaco"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "regions", "europe-monaco", "map.pmtiles"),
		[]byte("existing"), 0o644))
	db := openTestDB(t)
	seedRegion(t, db, "europe-monaco", "job-3")
	deps := ChainDeps{DB: db, DataDir: dir}
	h := swapHandler(deps, runSwap, log)
	b, _ := json.Marshal(jobs.RegionSwapPayload{Region: "europe-monaco", JobID: "job-3"})
	err := h(ctx, asynq.NewTask(jobs.KindRegionSwap, b))
	require.Error(t, err)
	// Existing region is intact.
	data, err := os.ReadFile(filepath.Join(dir, "regions", "europe-monaco", "map.pmtiles"))
	require.NoError(t, err)
	require.Equal(t, "existing", string(data))
}

func queryState(t *testing.T, db *sql.DB, region string) string {
	t.Helper()
	var s string
	require.NoError(t, db.QueryRow(`SELECT state FROM regions WHERE name = ?`, region).Scan(&s))
	return s
}

type strErr string

func (s strErr) Error() string { return string(s) }

func asErr(msg string) error { return strErr(msg) }
