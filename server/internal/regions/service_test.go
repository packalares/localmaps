package regions

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/catalog"
	"github.com/packalares/localmaps/server/internal/config"
)

// mockQueue records every Enqueue call.
type mockQueue struct {
	mu    sync.Mutex
	tasks []*asynq.Task
	err   error
}

func (m *mockQueue) EnqueueContext(_ context.Context, t *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	m.tasks = append(m.tasks, t)
	return &asynq.TaskInfo{}, nil
}

func (m *mockQueue) kinds() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.tasks))
	for _, t := range m.tasks {
		out = append(out, t.Type())
	}
	return out
}

// stubCatalog returns canned entries.
type stubCatalog struct {
	entries map[string]catalog.Entry
	listErr error
}

func (s *stubCatalog) ListRegions(context.Context) ([]catalog.Entry, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]catalog.Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	return out, nil
}

func (s *stubCatalog) Resolve(_ context.Context, key string) (catalog.Entry, error) {
	e, ok := s.entries[key]
	if !ok {
		return catalog.Entry{}, errors.New("not in catalog")
	}
	return e, nil
}

func newTestService(t *testing.T, entries map[string]catalog.Entry) (*Service, *mockQueue, *sqlx.DB) {
	t.Helper()
	store, err := config.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	db := store.DB()
	q := &mockQueue{}
	cat := &stubCatalog{entries: entries}
	return NewService(db, cat, q), q, db
}

func sampleRomania() catalog.Entry {
	iso := "RO"
	parent := "europe"
	return catalog.Entry{
		Name:        "europe-romania",
		DisplayName: "Romania",
		Kind:        catalog.KindCountry,
		Parent:      &parent,
		SourceURL:   "https://download.geofabrik.de/europe/romania-latest.osm.pbf",
		ISO31661:    &iso,
	}
}

func TestInstall_FirstTime(t *testing.T) {
	svc, q, _ := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	region, job, err := svc.Install(context.Background(), "europe/romania", "alice")
	require.NoError(t, err)
	require.Equal(t, "europe-romania", region.Name)
	require.Equal(t, StateDownloading, region.State)
	require.NotEmpty(t, job.ID)
	require.Equal(t, []string{"region:install"}, q.kinds())

	// Second install should 409 when state is downloading.
	_, _, err = svc.Install(context.Background(), "europe/romania", "alice")
	require.ErrorIs(t, err, ErrConflict)
}

func TestInstall_AfterFailedIsAllowed(t *testing.T) {
	svc, q, db := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	_, _, err := svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)

	// Flip to failed and retry.
	_, err = db.Exec(`UPDATE regions SET state = ? WHERE name = ?`,
		StateFailed, "europe-romania")
	require.NoError(t, err)

	_, _, err = svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	require.Len(t, q.kinds(), 2)
}

func TestInstall_RejectsBadKey(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	_, _, err := svc.Install(context.Background(), "../etc/passwd", "alice")
	require.Error(t, err)
}

func TestInstall_ResolvesAndPopulatesRow(t *testing.T) {
	svc, _, db := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	_, _, err := svc.Install(context.Background(), "europe/romania", "alice")
	require.NoError(t, err)

	var src string
	require.NoError(t, db.Get(&src,
		`SELECT source_url FROM regions WHERE name = ?`, "europe-romania"))
	require.Contains(t, src, "romania-latest.osm.pbf")
}

func TestListInstalled_ReturnsAllRows(t *testing.T) {
	svc, _, _ := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	_, _, err := svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	rows, err := svc.ListInstalled(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Romania", rows[0].DisplayName)
}

func TestGet_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	_, err := svc.Get(context.Background(), "europe-nonexistent")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestUpdate_RequiresReadyOrFailed(t *testing.T) {
	svc, _, db := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	_, _, err := svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)

	// State is downloading — update must refuse.
	_, err = svc.Update(context.Background(), "europe-romania", "alice")
	require.ErrorIs(t, err, ErrConflict)

	// Flip to ready.
	_, err = db.Exec(`UPDATE regions SET state = ? WHERE name = ?`,
		StateReady, "europe-romania")
	require.NoError(t, err)

	job, err := svc.Update(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	require.NotEmpty(t, job.ID)
}

func TestDelete_FlipsToArchived(t *testing.T) {
	svc, q, _ := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	_, _, err := svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	q.tasks = nil // clear

	region, job, err := svc.Delete(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)
	require.Equal(t, StateArchived, region.State)
	require.NotEmpty(t, job.ID)
	require.Equal(t, []string{"region:delete"}, q.kinds())
}

func TestDelete_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	_, _, err := svc.Delete(context.Background(), "europe-xyz", "alice")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSetSchedule_AcceptsEnumAndCron(t *testing.T) {
	svc, _, _ := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	_, _, err := svc.Install(context.Background(), "europe-romania", "alice")
	require.NoError(t, err)

	// Enum presets.
	for _, sched := range []string{"never", "daily", "weekly", "monthly"} {
		r, err := svc.SetSchedule(context.Background(), "europe-romania", sched)
		require.NoError(t, err)
		require.NotNil(t, r.Schedule)
		require.Equal(t, sched, *r.Schedule)
	}

	// Cron: 5 fields.
	r, err := svc.SetSchedule(context.Background(), "europe-romania", "0 3 * * 0")
	require.NoError(t, err)
	require.Equal(t, "0 3 * * 0", *r.Schedule)

	// Invalid.
	_, err = svc.SetSchedule(context.Background(), "europe-romania", "asap")
	require.ErrorIs(t, err, ErrInvalidSchedule)
}

func TestSetSchedule_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	_, err := svc.SetSchedule(context.Background(), "europe-xyz", "daily")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestListCatalog_DelegatesToCatalog(t *testing.T) {
	svc, _, _ := newTestService(t, map[string]catalog.Entry{
		"europe-romania": sampleRomania(),
	})
	tree, err := svc.ListCatalog(context.Background())
	require.NoError(t, err)
	require.Len(t, tree, 1)
	require.Equal(t, "europe-romania", tree[0].Name)
}
