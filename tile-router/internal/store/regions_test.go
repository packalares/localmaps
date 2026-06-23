package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

const realFile = "/tmp/romania.pmtiles"

// setupTestDB creates an in-memory sqlite with the regions schema
// matching what the production worker writes. We don't use a fixture
// file because the schema is simple and inlining keeps the test
// hermetic.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE regions (
			name         TEXT PRIMARY KEY,
			state        TEXT NOT NULL,
			bbox         TEXT
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// setupRegionsDir creates a fake regions tree with a symlink to the
// real Romania pmtiles. We don't copy 450 MB into a temp dir per
// test — the symlink is what the store ends up reading via
// os.Open, and os.Open follows symlinks transparently.
func setupRegionsDir(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(realFile); err != nil {
		t.Skipf("integration fixture missing (%s); skip", realFile)
	}
	root := t.TempDir()
	romDir := filepath.Join(root, "europe-romania")
	if err := os.Mkdir(romDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(realFile, filepath.Join(romDir, "map.pmtiles")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	return root
}

func TestRefresh_LoadsReadyRegions(t *testing.T) {
	db := setupTestDB(t)
	regionsDir := setupRegionsDir(t)
	_, err := db.Exec(`INSERT INTO regions (name, state) VALUES (?, ?)`, "europe-romania", "ready")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	s := New(db, regionsDir, time.Hour, nil, zerolog.Nop())
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	regions, loaded := s.Snapshot()
	if len(regions) != 1 || regions[0].Name != "europe-romania" {
		t.Fatalf("expected 1 region (europe-romania), got %+v", regions)
	}
	if _, ok := loaded["europe-romania"]; !ok {
		t.Fatal("Loaded map should contain europe-romania")
	}
	// BBox should come from the pmtiles header, not the DB column.
	r := regions[0]
	if r.BBox.MinLon < 18 || r.BBox.MaxLon > 32 {
		t.Errorf("Romania bbox from pmtiles header looks wrong: %+v", r.BBox)
	}
}

func TestRefresh_IgnoresNonReadyRegions(t *testing.T) {
	db := setupTestDB(t)
	regionsDir := setupRegionsDir(t)
	_, _ = db.Exec(`INSERT INTO regions (name, state) VALUES (?, ?)`, "europe-romania", "ready")
	_, _ = db.Exec(`INSERT INTO regions (name, state) VALUES (?, ?)`, "europe-bulgaria", "processing_tiles")
	_, _ = db.Exec(`INSERT INTO regions (name, state) VALUES (?, ?)`, "europe-poland", "failed")

	s := New(db, regionsDir, time.Hour, nil, zerolog.Nop())
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	regions, _ := s.Snapshot()
	if len(regions) != 1 || regions[0].Name != "europe-romania" {
		t.Fatalf("only europe-romania should load (others not ready); got %+v", regions)
	}
}

func TestRefresh_SkipsRegionsWithMissingFile(t *testing.T) {
	db := setupTestDB(t)
	tmp := t.TempDir() // empty — no map.pmtiles inside

	_, _ = db.Exec(`INSERT INTO regions (name, state) VALUES (?, ?)`, "europe-ghost", "ready")
	s := New(db, tmp, time.Hour, nil, zerolog.Nop())
	// Refresh shouldn't return an error — open failures are logged
	// and the region is skipped so one bad row can't shadow the rest.
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh should swallow missing-file errors; got %v", err)
	}
	regions, _ := s.Snapshot()
	if len(regions) != 0 {
		t.Fatalf("ghost region should NOT load; got %+v", regions)
	}
}

func TestRefresh_UnloadsRemovedRegions(t *testing.T) {
	db := setupTestDB(t)
	regionsDir := setupRegionsDir(t)
	_, _ = db.Exec(`INSERT INTO regions (name, state) VALUES (?, ?)`, "europe-romania", "ready")

	s := New(db, regionsDir, time.Hour, nil, zerolog.Nop())
	_ = s.Refresh(context.Background())

	regions, _ := s.Snapshot()
	if len(regions) != 1 {
		t.Fatalf("preconditions: should have 1; got %d", len(regions))
	}

	// Simulate the operator archiving the region: state flips away
	// from `ready`. Next Refresh should close the reader + drop it.
	_, _ = db.Exec(`UPDATE regions SET state = 'archived' WHERE name = ?`, "europe-romania")
	_ = s.Refresh(context.Background())

	regions, _ = s.Snapshot()
	if len(regions) != 0 {
		t.Fatalf("archived region should unload; still have %+v", regions)
	}
}

func TestSnapshot_IsStableUnderConcurrentRefresh(t *testing.T) {
	db := setupTestDB(t)
	regionsDir := setupRegionsDir(t)
	_, _ = db.Exec(`INSERT INTO regions (name, state) VALUES (?, ?)`, "europe-romania", "ready")

	s := New(db, regionsDir, time.Hour, nil, zerolog.Nop())
	_ = s.Refresh(context.Background())

	// Hammer Snapshot from N goroutines while a Refresh churns in
	// the background. If we held the wrong locks or returned the
	// internal map directly, this would race; with the snapshot copy
	// design it should be clean. -race catches it under `go test -race`.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			_ = s.Refresh(context.Background())
		}
		close(done)
	}()
	for i := 0; i < 500; i++ {
		regions, loaded := s.Snapshot()
		_ = regions
		_ = loaded
	}
	<-done
}
