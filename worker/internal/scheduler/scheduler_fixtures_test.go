package scheduler

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/catalog"
	_ "modernc.org/sqlite"
)

// recordingQueue records every Asynq task enqueued. Test-only stub for
// the Enqueuer interface.
type recordingQueue struct {
	mu    sync.Mutex
	tasks []*asynq.Task
}

func (q *recordingQueue) EnqueueContext(_ context.Context, t *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, t)
	return &asynq.TaskInfo{}, nil
}

func (q *recordingQueue) regions() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]string, 0, len(q.tasks))
	for _, t := range q.tasks {
		out = append(out, string(t.Payload()))
	}
	return out
}

// openSchedulerDB builds a fresh in-memory SQLite instance with the
// schema subset scheduler tests touch.
func openSchedulerDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		CREATE TABLE regions (
			name TEXT PRIMARY KEY,
			display_name TEXT,
			source_url TEXT,
			source_pbf_sha256 TEXT,
			state TEXT NOT NULL DEFAULT 'ready',
			schedule TEXT,
			next_update_at TEXT,
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
	`)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seed inserts a regions row.
func seed(t *testing.T, db *sql.DB, name, schedule, md5, nextAt, state string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO regions(name, display_name, source_url, source_pbf_sha256,
		                    state, schedule, next_update_at)
		VALUES (?,?,?,?,?,?,?)`,
		name, name, "http://stub/"+name+".osm.pbf", md5,
		state, schedule, nextAt)
	require.NoError(t, err)
}

// multiFetcher is a ChecksumFetcher that looks up per-region md5 + errors.
type multiFetcher struct {
	byRegion map[string]string
	errors   map[string]bool
}

func (m *multiFetcher) ResolvePbfURL(entry catalog.Entry) (string, error) {
	return entry.SourceURL, nil
}

func (m *multiFetcher) FetchSHA256(_ context.Context, pbfURL string) (string, int64, error) {
	for name := range m.byRegion {
		if pbfURL == "http://stub/"+name+".osm.pbf" {
			if m.errors[name] {
				return "", 0, errFakeNetwork
			}
			return m.byRegion[name], 0, nil
		}
	}
	for name := range m.errors {
		if pbfURL == "http://stub/"+name+".osm.pbf" && m.errors[name] {
			return "", 0, errFakeNetwork
		}
	}
	return "", 0, nil
}

var errFakeNetwork = &fakeErr{msg: "simulated network error"}

type fakeErr struct{ msg string }

func (f *fakeErr) Error() string { return f.msg }

func queryNextAt(t *testing.T, db *sql.DB, region string) string {
	t.Helper()
	var s sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT next_update_at FROM regions WHERE name = ?`, region).Scan(&s))
	if !s.Valid {
		return ""
	}
	return s.String
}
