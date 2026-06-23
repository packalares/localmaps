// Package store reads installed regions from sqlite and keeps a
// pmtiles.Reader open for each one that's `ready`. On a polling
// interval the store diffs DB ↔ in-memory: new ready regions get
// opened, removed ones get closed, no-ops are skipped.
//
// Consumers (the picker and the HTTP handlers) call Snapshot() to
// get an immutable view of the currently-loaded set. Snapshot is
// cheap — it returns a copy of an internal slice + map, both of
// which are rebuilt only when the diff actually changes something.
// Tile reads can then proceed lock-free.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/tile-router/internal/pick"
	"github.com/packalares/localmaps/tile-router/internal/pmtiles"
)

// Loaded is one region with an open pmtiles handle. The picker uses
// the embedded pick.Region for geometry; the handler uses Reader to
// pull tile bytes.
type Loaded struct {
	pick.Region
	Reader *pmtiles.Reader
}

// Store is the in-memory view of installed regions. Safe for
// concurrent Snapshot/Run.
type Store struct {
	db           *sql.DB
	regionsDir   string        // e.g. /data/regions; each ready region's pmtiles lives at <regionsDir>/<name>/map.pmtiles
	pollInterval time.Duration // how often to refresh from sqlite
	atlas        *pick.Atlas   // Natural Earth country polygons; nil = polygon picking disabled
	log          zerolog.Logger

	mu     sync.RWMutex
	loaded map[string]*Loaded // by region name
}

// New constructs a Store but does NOT refresh — call Refresh() once
// at boot to populate, then Run() to keep it fresh.
//
// `atlas` is the Natural Earth country polygon set; pass nil to
// disable polygon picking entirely (bbox-only). When non-nil, each
// region whose name resolves to a country (e.g. `europe-romania` →
// "Romania") gets that polygon attached at load time so the picker
// can do point-in-polygon tests.
func New(db *sql.DB, regionsDir string, pollInterval time.Duration, atlas *pick.Atlas, log zerolog.Logger) *Store {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	return &Store{
		db:           db,
		regionsDir:   regionsDir,
		pollInterval: pollInterval,
		atlas:        atlas,
		log:          log,
		loaded:       map[string]*Loaded{},
	}
}

// Snapshot returns a slice of currently-loaded regions for the picker
// and a name→Loaded map for the tile handler. Both are stable
// snapshots — safe to use without holding any lock, even if Refresh
// fires concurrently.
func (s *Store) Snapshot() ([]pick.Region, map[string]*Loaded) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	regions := make([]pick.Region, 0, len(s.loaded))
	loaded := make(map[string]*Loaded, len(s.loaded))
	for _, l := range s.loaded {
		regions = append(regions, l.Region)
		loaded[l.Name] = l
	}
	return regions, loaded
}

// Refresh queries sqlite for the current `ready` set and reconciles
// the in-memory map with it: opens new readers, closes removed ones,
// leaves unchanged ones alone. Errors on individual regions are
// logged and skipped — one bad pmtiles file does NOT take the whole
// router down.
func (s *Store) Refresh(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM regions WHERE state = 'ready' ORDER BY name`)
	if err != nil {
		return fmt.Errorf("query regions: %w", err)
	}
	defer rows.Close()

	wantNames := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan region: %w", err)
		}
		wantNames[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate regions: %w", err)
	}

	// Diff against the current loaded set. We hold the write lock for
	// the smallest window that lets us swap maps; the actual file
	// opens happen OUTSIDE the lock so a slow disk I/O can't block
	// in-flight Snapshot() calls. Same for closes.
	s.mu.RLock()
	toOpen := []string{}
	for name := range wantNames {
		if _, alreadyOpen := s.loaded[name]; !alreadyOpen {
			toOpen = append(toOpen, name)
		}
	}
	toClose := []*Loaded{}
	for name, l := range s.loaded {
		if _, stillWanted := wantNames[name]; !stillWanted {
			toClose = append(toClose, l)
		}
	}
	s.mu.RUnlock()

	// Open new ones (slow I/O — outside the lock).
	newlyLoaded := map[string]*Loaded{}
	for _, name := range toOpen {
		path := filepath.Join(s.regionsDir, name, "map.pmtiles")
		rdr, err := pmtiles.Open(path)
		if err != nil {
			s.log.Warn().Err(err).Str("region", name).Str("path", path).
				Msg("skip region: pmtiles open failed")
			continue
		}
		minLon, minLat, maxLon, maxLat := rdr.BBox()
		// Resolve the region name to a Natural Earth country polygon
		// if possible — "europe-romania" → "Romania". Regions that
		// don't map (e.g. "europe-alps", multi-country super-extract)
		// get nil here and the picker falls back to bbox-of-tile-center.
		var poly *pick.CountryPolygon
		if s.atlas != nil {
			poly = s.atlas.CountryForRegion(name)
			if poly != nil {
				s.log.Debug().Str("region", name).
					Str("country", poly.Admin).
					Msg("attached country polygon")
			}
		}
		newlyLoaded[name] = &Loaded{
			Region: pick.Region{
				Name:    name,
				Polygon: poly,
				BBox: pick.BBox{
					MinLon: minLon, MinLat: minLat,
					MaxLon: maxLon, MaxLat: maxLat,
				},
			},
			Reader: rdr,
		}
		s.log.Info().Str("region", name).
			Float64("minLon", minLon).Float64("minLat", minLat).
			Float64("maxLon", maxLon).Float64("maxLat", maxLat).
			Msg("region loaded")
	}

	// Swap into the live map under the write lock. We compute the new
	// state outside the lock and apply it atomically.
	if len(toOpen) > 0 || len(toClose) > 0 {
		s.mu.Lock()
		for _, l := range toClose {
			delete(s.loaded, l.Name)
		}
		for name, l := range newlyLoaded {
			s.loaded[name] = l
		}
		s.mu.Unlock()
	}

	// Close removed readers AFTER the lock so live tile reads against
	// them can drain. Worst case: a tile read that captured the old
	// pointer before the diff finishes succeeds; the next request
	// goes through the new set. Acceptable.
	for _, l := range toClose {
		if err := l.Reader.Close(); err != nil {
			s.log.Warn().Err(err).Str("region", l.Name).
				Msg("close removed region: error (continuing)")
		}
		s.log.Info().Str("region", l.Name).Msg("region unloaded")
	}
	return nil
}

// Run blocks until ctx is cancelled, calling Refresh on the polling
// interval. Errors from Refresh are logged but don't stop the loop —
// transient DB issues shouldn't kill the router.
func (s *Store) Run(ctx context.Context) {
	t := time.NewTicker(s.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("regions store shutting down")
			s.closeAll()
			return
		case <-t.C:
			if err := s.Refresh(ctx); err != nil {
				s.log.Warn().Err(err).Msg("refresh failed (will retry)")
			}
		}
	}
}

// closeAll releases every loaded reader. Called on shutdown.
func (s *Store) closeAll() {
	s.mu.Lock()
	loaded := s.loaded
	s.loaded = map[string]*Loaded{}
	s.mu.Unlock()
	for _, l := range loaded {
		_ = l.Reader.Close()
	}
}
