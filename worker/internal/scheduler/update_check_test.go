package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/catalog"
	_ "modernc.org/sqlite"
)

type stubCatalog struct {
	entries map[string]catalog.Entry
	err     error
}

func (s *stubCatalog) ListRegions(context.Context) ([]catalog.Entry, error) {
	return nil, nil
}
func (s *stubCatalog) Resolve(_ context.Context, key string) (catalog.Entry, error) {
	if s.err != nil {
		return catalog.Entry{}, s.err
	}
	e, ok := s.entries[key]
	if !ok {
		return catalog.Entry{}, fmt.Errorf("not found: %s", key)
	}
	return e, nil
}

type stubFetcher struct {
	resolveErr error
	url        string
	md5        string
	fetchErr   error
}

func (f *stubFetcher) ResolvePbfURL(entry catalog.Entry) (string, error) {
	if f.resolveErr != nil {
		return "", f.resolveErr
	}
	if f.url != "" {
		return f.url, nil
	}
	return entry.SourceURL, nil
}
func (f *stubFetcher) FetchSHA256(_ context.Context, _ string) (string, int64, error) {
	if f.fetchErr != nil {
		return "", 0, f.fetchErr
	}
	return f.md5, 123, nil
}

func openCheckDB(t *testing.T) *sql.DB {
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
			next_update_at TEXT
		);
	`)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedRegion(t *testing.T, db *sql.DB, name, md5 string) {
	t.Helper()
	if md5 == "" {
		_, err := db.Exec(
			`INSERT INTO regions(name, display_name, source_url, state)
			 VALUES (?,?,?,?)`,
			name, name, "http://stub/"+name+".osm.pbf", "ready")
		require.NoError(t, err)
		return
	}
	_, err := db.Exec(
		`INSERT INTO regions(name, display_name, source_url, source_pbf_sha256, state)
		 VALUES (?,?,?,?,?)`,
		name, name, "http://stub/"+name+".osm.pbf", md5, "ready")
	require.NoError(t, err)
}

func TestShouldUpdate_UpToDate(t *testing.T) {
	db := openCheckDB(t)
	seedRegion(t, db, "europe-monaco", "abc123")
	uc := UpdateCheck{
		DB:      db,
		Catalog: &stubCatalog{entries: map[string]catalog.Entry{"europe-monaco": {Name: "europe-monaco", SourceURL: "http://stub/europe-monaco.osm.pbf"}}},
		Fetcher: &stubFetcher{md5: "ABC123"},
	}
	ok, reason, err := uc.ShouldUpdate(context.Background(), "europe-monaco")
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, ReasonUpToDate, reason)
}

func TestShouldUpdate_ChecksumChanged(t *testing.T) {
	db := openCheckDB(t)
	seedRegion(t, db, "europe-romania", "old-sum")
	uc := UpdateCheck{
		DB:      db,
		Catalog: &stubCatalog{entries: map[string]catalog.Entry{"europe-romania": {Name: "europe-romania", SourceURL: "http://stub/europe-romania.osm.pbf"}}},
		Fetcher: &stubFetcher{md5: "new-sum"},
	}
	ok, reason, err := uc.ShouldUpdate(context.Background(), "europe-romania")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, ReasonChecksumChanged, reason)
}

func TestShouldUpdate_MissingInstalledChecksum(t *testing.T) {
	db := openCheckDB(t)
	seedRegion(t, db, "europe-monaco", "")
	uc := UpdateCheck{
		DB:      db,
		Catalog: &stubCatalog{entries: map[string]catalog.Entry{"europe-monaco": {Name: "europe-monaco", SourceURL: "http://stub/europe-monaco.osm.pbf"}}},
		Fetcher: &stubFetcher{md5: "fresh"},
	}
	ok, reason, err := uc.ShouldUpdate(context.Background(), "europe-monaco")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, ReasonMissingChecksum, reason)
}

func TestShouldUpdate_HTTPError(t *testing.T) {
	db := openCheckDB(t)
	seedRegion(t, db, "europe-monaco", "abc")
	uc := UpdateCheck{
		DB:      db,
		Catalog: &stubCatalog{entries: map[string]catalog.Entry{"europe-monaco": {Name: "europe-monaco", SourceURL: "http://stub/europe-monaco.osm.pbf"}}},
		Fetcher: &stubFetcher{fetchErr: errors.New("connection reset")},
	}
	ok, reason, err := uc.ShouldUpdate(context.Background(), "europe-monaco")
	require.Error(t, err)
	require.False(t, ok)
	require.Equal(t, ReasonUnknown, reason)
}

func TestShouldUpdate_SidecarEmpty(t *testing.T) {
	db := openCheckDB(t)
	seedRegion(t, db, "europe-monaco", "abc")
	uc := UpdateCheck{
		DB:      db,
		Catalog: &stubCatalog{entries: map[string]catalog.Entry{"europe-monaco": {Name: "europe-monaco", SourceURL: "http://stub/europe-monaco.osm.pbf"}}},
		Fetcher: &stubFetcher{md5: ""},
	}
	ok, reason, err := uc.ShouldUpdate(context.Background(), "europe-monaco")
	require.Error(t, err)
	require.False(t, ok)
	require.Equal(t, ReasonUnknown, reason)
}

func TestShouldUpdate_CatalogError(t *testing.T) {
	db := openCheckDB(t)
	seedRegion(t, db, "europe-monaco", "abc")
	uc := UpdateCheck{
		DB:      db,
		Catalog: &stubCatalog{err: errors.New("network down")},
		Fetcher: &stubFetcher{md5: "abc"},
	}
	ok, reason, err := uc.ShouldUpdate(context.Background(), "europe-monaco")
	require.Error(t, err)
	require.False(t, ok)
	require.Equal(t, ReasonUnknown, reason)
}

func TestShouldUpdate_NilDepsErrors(t *testing.T) {
	uc := UpdateCheck{}
	_, _, err := uc.ShouldUpdate(context.Background(), "x")
	require.Error(t, err)
}

func TestShouldUpdate_UnknownRegion(t *testing.T) {
	db := openCheckDB(t)
	uc := UpdateCheck{
		DB:      db,
		Catalog: &stubCatalog{entries: map[string]catalog.Entry{}},
		Fetcher: &stubFetcher{md5: "x"},
	}
	_, _, err := uc.ShouldUpdate(context.Background(), "europe-atlantis")
	require.Error(t, err)
}
