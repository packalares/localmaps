package scheduler

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/catalog"
	"github.com/packalares/localmaps/internal/jobs"
)

// newSched builds a Scheduler wired to an in-memory DB + canned
// catalog. Defined here rather than in fixtures because it depends on
// per-test inputs (now + queue + fetcher).
func newSched(t *testing.T, db *sql.DB, fetch *multiFetcher, q *recordingQueue, now time.Time) *Scheduler {
	t.Helper()
	entries := map[string]catalog.Entry{}
	rows, err := db.Query(`SELECT name, source_url FROM regions`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var n, u string
		require.NoError(t, rows.Scan(&n, &u))
		entries[n] = catalog.Entry{Name: n, SourceURL: u}
	}
	return &Scheduler{
		DB:      db,
		Queue:   q,
		Catalog: &stubCatalog{entries: entries},
		Fetcher: fetch,
		Now:     func() time.Time { return now },
		Logger:  zerolog.Nop(),
	}
}

func TestTick_EnqueuesOnlyDueRegionsWithChangedChecksum(t *testing.T) {
	db := openSchedulerDB(t)
	now := time.Date(2026, 4, 24, 4, 0, 0, 0, time.UTC) // after 03:00.
	// 1. ready + daily + past due + changed md5 -> enqueued.
	seed(t, db, "europe-romania", "daily", "old-sum",
		"2026-04-24T03:00:00Z", "ready")
	// 2. ready + daily + past due + same md5 -> no enqueue, bump only.
	seed(t, db, "europe-monaco", "daily", "same-sum",
		"2026-04-24T03:00:00Z", "ready")
	// 3. ready + weekly + future next_update_at -> skipped.
	seed(t, db, "europe-hungary", "weekly", "any-sum",
		"2026-04-30T03:00:00Z", "ready")
	// 4. ready + never -> filtered out by WHERE clause.
	seed(t, db, "europe-italy", "never", "any-sum",
		"2026-04-24T03:00:00Z", "ready")
	// 5. state=downloading -> filtered out.
	seed(t, db, "europe-france", "daily", "old-sum",
		"2026-04-24T03:00:00Z", "downloading")

	fetch := &multiFetcher{byRegion: map[string]string{
		"europe-romania": "new-sum",
		"europe-monaco":  "same-sum",
		"europe-hungary": "any-sum",
	}}
	q := &recordingQueue{}
	s := newSched(t, db, fetch, q, now)

	out, err := s.Tick(context.Background())
	require.NoError(t, err)
	require.NoError(t, out.FirstError)
	require.Equal(t, 3, out.Checked, "romania + monaco + hungary match filter")
	require.Equal(t, 2, out.Due, "romania + monaco were due")
	require.Equal(t, 1, out.Enqueued, "only romania had changed md5")

	// Verify which region was enqueued.
	payloads := q.regions()
	require.Len(t, payloads, 1)
	require.Contains(t, payloads[0], "europe-romania")

	q.mu.Lock()
	require.Equal(t, jobs.KindRegionUpdate, q.tasks[0].Type())
	q.mu.Unlock()

	// next_update_at should be bumped for romania + monaco, untouched
	// for hungary (not due).
	require.NotEqual(t, "2026-04-24T03:00:00Z", queryNextAt(t, db, "europe-romania"))
	require.NotEqual(t, "2026-04-24T03:00:00Z", queryNextAt(t, db, "europe-monaco"))
	require.Equal(t, "2026-04-30T03:00:00Z", queryNextAt(t, db, "europe-hungary"))

	// jobs row for romania + active_job_id attached.
	var cnt int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE region = ? AND kind = ?`,
		"europe-romania", jobs.OpenAPIJobKindUpdateRegion).Scan(&cnt))
	require.Equal(t, 1, cnt)
	var active sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT active_job_id FROM regions WHERE name = ?`, "europe-romania").Scan(&active))
	require.True(t, active.Valid)
	require.NotEmpty(t, active.String)
}

func TestTick_NoOpAfterBumpIsIdempotent(t *testing.T) {
	db := openSchedulerDB(t)
	now := time.Date(2026, 4, 24, 4, 0, 0, 0, time.UTC)
	seed(t, db, "europe-monaco", "daily", "same-sum",
		"2026-04-24T03:00:00Z", "ready")

	fetch := &multiFetcher{byRegion: map[string]string{"europe-monaco": "same-sum"}}
	q := &recordingQueue{}
	s := newSched(t, db, fetch, q, now)

	_, err := s.Tick(context.Background())
	require.NoError(t, err)
	require.Empty(t, q.regions(), "up-to-date region should not enqueue")

	// Second tick at same now: next_update_at was bumped past now, so
	// region is not due.
	out2, err := s.Tick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, out2.Due)
	require.Equal(t, 0, out2.Enqueued)
}

func TestTick_CheckFailureDoesNotStallLoop(t *testing.T) {
	db := openSchedulerDB(t)
	now := time.Date(2026, 4, 24, 4, 0, 0, 0, time.UTC)
	seed(t, db, "europe-broken", "daily", "old", "", "ready")
	seed(t, db, "europe-ok", "daily", "old", "", "ready")

	fetch := &multiFetcher{
		byRegion: map[string]string{"europe-ok": "new"},
		errors:   map[string]bool{"europe-broken": true},
	}
	q := &recordingQueue{}
	s := newSched(t, db, fetch, q, now)

	out, err := s.Tick(context.Background())
	require.NoError(t, err) // Tick itself succeeds.
	require.Error(t, out.FirstError, "broken region surfaces as summary error")
	require.Equal(t, 2, out.Checked)
	require.Equal(t, 2, out.Due)
	require.Equal(t, 1, out.Enqueued, "ok region still enqueued")
}

func TestTick_NilDB(t *testing.T) {
	s := &Scheduler{Logger: zerolog.Nop()}
	_, err := s.Tick(context.Background())
	require.Error(t, err)
}

func TestStart_StopsOnContextCancel(t *testing.T) {
	db := openSchedulerDB(t)
	s := &Scheduler{
		DB:       db,
		TickCron: "* * * * *",
		Now:      func() time.Time { return time.Now().UTC() },
		Logger:   zerolog.Nop(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel")
	}
}

func TestStart_InvalidCronFallsBack(t *testing.T) {
	db := openSchedulerDB(t)
	s := &Scheduler{
		DB:       db,
		TickCron: "not a cron",
		Now:      func() time.Time { return time.Now().UTC() },
		Logger:   zerolog.Nop(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()
	cancel()
	<-done
}

func TestIsDue(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	require.True(t, isDue("", now))
	require.True(t, isDue("garbage", now))
	require.True(t, isDue("2026-04-24T11:59:00Z", now))
	require.True(t, isDue("2026-04-24T12:00:00Z", now)) // equal → due
	require.False(t, isDue("2026-04-24T12:00:01Z", now))
}
