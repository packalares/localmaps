package install

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/catalog"
	"github.com/packalares/localmaps/internal/jobs"

	_ "modernc.org/sqlite"
)

// fakeQueue records every enqueued task.
type fakeQueue struct {
	mu    sync.Mutex
	tasks []*asynq.Task
}

func (q *fakeQueue) EnqueueContext(_ context.Context, t *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, t)
	return &asynq.TaskInfo{}, nil
}

func (q *fakeQueue) types() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]string, 0, len(q.tasks))
	for _, t := range q.tasks {
		out = append(out, t.Type())
	}
	return out
}

// stubCatalog returns canned entries.
type stubCatalog struct {
	byKey map[string]catalog.Entry
}

func (s *stubCatalog) ListRegions(context.Context) ([]catalog.Entry, error) {
	return nil, nil
}
func (s *stubCatalog) Resolve(_ context.Context, key string) (catalog.Entry, error) {
	e, ok := s.byKey[key]
	if !ok {
		return catalog.Entry{}, errors.New("not found")
	}
	return e, nil
}

// openTestDB returns a sqlite DB with the subset of schema the handler
// touches.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	// Keep the connection alive for the whole test.
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		CREATE TABLE regions (
			name TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			parent TEXT,
			source_url TEXT NOT NULL,
			source_pbf_sha256 TEXT,
			source_pbf_bytes INTEGER,
			bbox TEXT,
			state TEXT NOT NULL,
			state_detail TEXT,
			last_error TEXT,
			installed_at TEXT,
			last_updated_at TEXT,
			next_update_at TEXT,
			schedule TEXT,
			disk_bytes INTEGER,
			active_job_id TEXT
		);
		CREATE TABLE jobs (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			region TEXT,
			state TEXT NOT NULL,
			progress REAL,
			message TEXT,
			started_at TEXT,
			finished_at TEXT,
			error TEXT,
			created_by TEXT,
			parent_job_id TEXT
		);
		INSERT INTO regions (name, display_name, source_url, state)
			VALUES ('europe-monaco', 'Monaco',
			        'http://placeholder/europe/monaco-latest.osm.pbf', 'downloading');
		INSERT INTO jobs (id, kind, state) VALUES ('job-1', 'download_pbf', 'queued');
	`)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// servePbf returns an httptest server that delivers the given bytes at
// /europe/monaco-latest.osm.pbf.
func servePbf(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/europe/monaco-latest.osm.pbf",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", itoa(len(body)))
			_, _ = w.Write(body)
		})
	return httptest.NewServer(mux)
}

// itoa is small to avoid pulling in strconv in this test file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestDownload_WritesFileAndReturnsMD5(t *testing.T) {
	// Tiny fixture: 16 bytes of "dummy planet pbf".
	fixture, err := os.ReadFile(findFixture(t))
	require.NoError(t, err)

	srv := servePbf(t, fixture)
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "source.osm.pbf")

	md5sum, size, err := Download(context.Background(), http.DefaultClient,
		srv.URL+"/europe/monaco-latest.osm.pbf", dest)
	require.NoError(t, err)
	require.NotEmpty(t, md5sum)
	require.EqualValues(t, len(fixture), size)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, fixture, got)
}

func TestHandler_OrchestratesInstall(t *testing.T) {
	fixture, err := os.ReadFile(findFixture(t))
	require.NoError(t, err)

	srv := servePbf(t, fixture)
	defer srv.Close()

	db := openTestDB(t)
	q := &fakeQueue{}
	dataDir := t.TempDir()

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
	handler := NewHandler(deps, zerolog.Nop())

	payload := []byte(`{"region":"europe-monaco","jobId":"job-1","triggeredBy":"alice"}`)
	task := asynq.NewTask(jobs.KindRegionInstall, payload)
	err = handler(context.Background(), task)
	require.NoError(t, err)

	// Next stage should have been enqueued.
	require.Equal(t, []string{jobs.KindPipelineTiles}, q.types())

	// PBF on disk under <dataDir>/regions/europe-monaco.new/source.osm.pbf
	onDisk := filepath.Join(dataDir, "regions", "europe-monaco.new", "source.osm.pbf")
	got, err := os.ReadFile(onDisk)
	require.NoError(t, err)
	require.Equal(t, fixture, got)

	// source_pbf_sha256 + bytes written to regions row.
	var sum string
	var sz int64
	err = db.QueryRow(
		`SELECT source_pbf_sha256, source_pbf_bytes FROM regions WHERE name = ?`,
		"europe-monaco").Scan(&sum, &sz)
	require.NoError(t, err)
	require.NotEmpty(t, sum)
	require.EqualValues(t, len(fixture), sz)
}

func TestHandler_FailureMarksRegionFailed(t *testing.T) {
	db := openTestDB(t)
	q := &fakeQueue{}
	// Catalog deliberately missing the region -> Resolve fails.
	deps := Deps{
		DB:      db,
		DataDir: t.TempDir(),
		Catalog: &stubCatalog{byKey: map[string]catalog.Entry{}},
		Queue:   q,
		HTTP:    http.DefaultClient,
	}
	handler := NewHandler(deps, zerolog.Nop())

	payload := []byte(`{"region":"europe-monaco","jobId":"job-1"}`)
	err := handler(context.Background(), asynq.NewTask(jobs.KindRegionInstall, payload))
	require.Error(t, err)

	var state, lastErr string
	err = db.QueryRow(`SELECT state, COALESCE(last_error, '')
	                   FROM regions WHERE name = ?`,
		"europe-monaco").Scan(&state, &lastErr)
	require.NoError(t, err)
	require.Equal(t, "failed", state)
	require.Contains(t, lastErr, "resolve catalog")

	var jobState, jobErr string
	err = db.QueryRow(`SELECT state, COALESCE(error, '')
	                   FROM jobs WHERE id = ?`, "job-1").Scan(&jobState, &jobErr)
	require.NoError(t, err)
	require.Equal(t, "failed", jobState)
}

func TestHandler_RejectsMalformedPayload(t *testing.T) {
	handler := NewHandler(Deps{}, zerolog.Nop())
	err := handler(context.Background(),
		asynq.NewTask(jobs.KindRegionInstall, []byte("not json")))
	require.Error(t, err)
}

// findFixture returns a path to the Monaco-ish fixture. The primary
// keeps this under test/fixtures/ at the project root, so we walk up
// until we find it.
func findFixture(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"../../../test/fixtures/monaco.osm.pbf",
		"../../../../test/fixtures/monaco.osm.pbf",
		"../../test/fixtures/monaco.osm.pbf",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Fallback: write a tiny inline fixture and return its temp path.
	t.Helper()
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "monaco.osm.pbf")
	// 512 bytes of pseudo-random data is enough for the orchestration.
	body := make([]byte, 512)
	for i := range body {
		body[i] = byte(i * 37)
	}
	require.NoError(t, os.WriteFile(dst, body, 0o644))
	return dst
}

// Unused but keeps the package deps honest.
var _ = io.Copy
