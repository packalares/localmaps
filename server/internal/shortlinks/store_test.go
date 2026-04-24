package shortlinks_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/packalares/localmaps/server/internal/shortlinks"
)

// newStore opens a fresh in-memory SQLite DB with the short_links table
// declared in docs/04-data-model.md and wraps it in a Store.
func newStore(t *testing.T, opts ...shortlinks.Option) (*shortlinks.Store, *sqlx.DB) {
	t.Helper()
	raw, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	raw.SetMaxOpenConns(1) // in-memory DBs are per-connection
	t.Cleanup(func() { _ = raw.Close() })

	db := sqlx.NewDb(raw, "sqlite")
	_, err = db.Exec(`CREATE TABLE short_links (
		code         TEXT PRIMARY KEY,
		url          TEXT NOT NULL,
		created_at   TEXT NOT NULL,
		last_hit_at  TEXT,
		hit_count    INTEGER DEFAULT 0
	)`)
	require.NoError(t, err)
	return shortlinks.New(db, opts...), db
}

func TestStore_Create_HappyPath(t *testing.T) {
	s, _ := newStore(t,
		shortlinks.WithGenerator(fixedGen("ABC1234")),
	)
	link, err := s.Create(context.Background(), "/#12/45.0/25.0")
	require.NoError(t, err)
	require.Equal(t, "ABC1234", link.Code)
	require.Equal(t, "/#12/45.0/25.0", link.URL)
	require.NotZero(t, link.CreatedAt)
	require.Equal(t, int64(0), link.HitCount)
	require.Nil(t, link.LastHitAt)
}

func TestStore_Create_RetriesOnCollision(t *testing.T) {
	// Force the first two generations to a fixed colliding code; the
	// third succeeds with a different one.
	gen := sequenceGen("COLLIDE", "COLLIDE", "UNIQUE_")
	s, _ := newStore(t, shortlinks.WithGenerator(gen))
	ctx := context.Background()

	// Seed the collision row.
	_, err := s.Create(ctx, "/first")
	require.NoError(t, err)

	link, err := s.Create(ctx, "/second")
	require.NoError(t, err)
	require.Equal(t, "UNIQUE_", link.Code)
}

func TestStore_Create_ExhaustsRetries(t *testing.T) {
	// Always return the same code → five retries all collide after the
	// seed row is in place.
	s, _ := newStore(t, shortlinks.WithGenerator(fixedGen("SAMECDE")))
	ctx := context.Background()
	_, err := s.Create(ctx, "/seed")
	require.NoError(t, err)

	_, err = s.Create(ctx, "/again")
	require.ErrorIs(t, err, shortlinks.ErrCodeCollision)
}

func TestStore_Resolve_NotFound(t *testing.T) {
	s, _ := newStore(t)
	_, err := s.Resolve(context.Background(), "MISSING", 365)
	require.ErrorIs(t, err, shortlinks.ErrNotFound)
}

func TestStore_Resolve_Expired(t *testing.T) {
	// Insert a row 10 days ago, then resolve with a 7-day TTL.
	past := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)

	s, db := newStore(t)
	_, err := db.Exec(
		`INSERT INTO short_links (code, url, created_at, hit_count)
		 VALUES (?, ?, ?, 0)`,
		"OLDCODE", "/old", past.Format(time.RFC3339Nano))
	require.NoError(t, err)

	// Swap the clock AFTER seeding so the Create path isn't affected.
	s = shortlinks.New(db, shortlinks.WithClock(func() time.Time { return now }))

	_, err = s.Resolve(context.Background(), "OLDCODE", 7)
	require.ErrorIs(t, err, shortlinks.ErrExpired)

	// A TTL of 0 disables expiry — same row resolves cleanly.
	link, err := s.Resolve(context.Background(), "OLDCODE", 0)
	require.NoError(t, err)
	require.Equal(t, "/old", link.URL)
}

func TestStore_Resolve_HappyPath(t *testing.T) {
	s, _ := newStore(t, shortlinks.WithGenerator(fixedGen("LIVECDE")))
	ctx := context.Background()
	_, err := s.Create(ctx, "/live")
	require.NoError(t, err)

	link, err := s.Resolve(ctx, "LIVECDE", 365)
	require.NoError(t, err)
	require.Equal(t, "/live", link.URL)
}

func TestStore_IncrementViews(t *testing.T) {
	s, _ := newStore(t, shortlinks.WithGenerator(fixedGen("HITCDE_")))
	ctx := context.Background()
	_, err := s.Create(ctx, "/hit")
	require.NoError(t, err)

	require.NoError(t, s.IncrementViews(ctx, "HITCDE_"))
	require.NoError(t, s.IncrementViews(ctx, "HITCDE_"))

	link, err := s.Resolve(ctx, "HITCDE_", 365)
	require.NoError(t, err)
	require.Equal(t, int64(2), link.HitCount)
	require.NotNil(t, link.LastHitAt)

	// Missing row — no error.
	require.NoError(t, s.IncrementViews(ctx, "NOTHERE"))
}

func TestStore_Cleanup(t *testing.T) {
	// Seed three rows: two older than the TTL, one fresh.
	fixedNow := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	s, db := newStore(t,
		shortlinks.WithClock(func() time.Time { return fixedNow }),
	)
	insert := func(code string, created time.Time) {
		_, err := db.Exec(
			`INSERT INTO short_links (code, url, created_at, hit_count)
			 VALUES (?, ?, ?, 0)`,
			code, "/"+code, created.Format(time.RFC3339Nano))
		require.NoError(t, err)
	}
	insert("OLD1", fixedNow.Add(-30*24*time.Hour))
	insert("OLD2", fixedNow.Add(-10*24*time.Hour))
	insert("FRESH", fixedNow.Add(-1*time.Hour))

	// TTL of 7 days → OLD1 + OLD2 evicted, FRESH kept.
	n, err := s.Cleanup(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, int64(2), n)

	var remaining int
	require.NoError(t, db.Get(&remaining,
		`SELECT COUNT(*) FROM short_links`))
	require.Equal(t, 1, remaining)

	// TTL <= 0 disables cleanup entirely.
	n, err = s.Cleanup(context.Background(), 0)
	require.NoError(t, err)
	require.Zero(t, n)
}

// --- test helpers --------------------------------------------------

// fixedGen returns a generator that always emits the same code.
func fixedGen(code string) func() string { return func() string { return code } }

// sequenceGen returns a generator that walks through seq, repeating the
// last element once exhausted (so tests don't run off the end).
func sequenceGen(seq ...string) func() string {
	i := 0
	return func() string {
		c := seq[i]
		if i < len(seq)-1 {
			i++
		}
		return c
	}
}
