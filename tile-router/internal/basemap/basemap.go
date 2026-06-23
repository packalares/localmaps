// Package basemap renders MVT tiles at low zoom levels from the
// embedded Natural Earth country polygons. It exists because country-
// scale pmtiles aren't enough at z=0-5: each tile spans multiple
// countries, the picker has to pick ONE, and the rendered tile only
// covers part of the visible geography. The user sees patchy gaps
// along borders ("half of Romania is missing").
//
// The basemap fills the gap by serving a SIMPLE world overview for
// z=0-5: country fills + borders rendered from the same Natural
// Earth dataset the picker already uses for point-in-polygon. Detail
// is minimal — no POIs, no roads, no labels — but every country gets
// rendered consistently across every tile, so the user sees the
// outline of "where Romania, Bulgaria, Greece, Turkey are" instead
// of gaps.
//
// Country-scale pmtiles take over at z=6+ where each tile is small
// enough to fit cleanly inside one country.
package basemap

import (
	"bytes"
	"sync"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/planar"
	"github.com/paulmach/orb/project"

	"github.com/packalares/localmaps/tile-router/internal/pick"
)

// MaxZoom is the inclusive cap on basemap tile generation. Anything
// above this falls through to the per-region pmtiles picker.
const MaxZoom uint8 = 5

// country holds everything one Natural Earth feature needs to render
// at low zoom: the polygon set for the border line + a pre-computed
// label anchor point in lat/lon.
type country struct {
	name    string // "Romania", "United States", …
	iso     string // 3-letter ISO code; falls back to ADMIN when empty
	polygon orb.MultiPolygon
	// labelAt is the centroid of the LARGEST sub-polygon, computed
	// once at load. Using the largest-polygon centroid (vs the
	// MultiPolygon's combined centroid) avoids placing the "United
	// States" label in the Pacific between Alaska and the mainland.
	labelAt orb.Point
}

// Renderer holds the parsed country polygons in a form ready for
// per-tile clipping. Construct once at boot from a pick.Atlas; the
// Renderer reuses the polygon data but reorganises it as orb
// geometries the MVT encoder understands.
type Renderer struct {
	// countries maps Natural Earth's 3-letter ISO code (or ADMIN
	// fallback) to the parsed country record. Lookup keys by the
	// same identifier the picker uses so logs / debug headers
	// cross-reference cleanly.
	countries map[string]country

	// installed is the set of country keys the operator has actually
	// installed pmtiles for. Used to tag polygon features with
	// `installed: 1` so the style can highlight them differently.
	// Mutex-protected so the regions-poll goroutine can update it
	// concurrently with HTTP request handlers reading it during
	// Render(). All accesses go through the methods below — never
	// touch the field directly.
	mu        sync.RWMutex
	installed map[string]struct{}
}

// SetInstalled atomically replaces the set of "installed" country
// keys. The next Render() call will tag matching polygon features
// with the `installed: 1` property; the style highlights them.
// Pass keys matching the Atlas's CountryForRegion lookup result
// (typically ISO_A3). Pass nil/empty to disable highlighting.
func (r *Renderer) SetInstalled(keys map[string]struct{}) {
	r.mu.Lock()
	r.installed = keys
	r.mu.Unlock()
}

// NewRenderer converts a pick.Atlas to a basemap.Renderer. Each
// CountryPolygon's `Polygons [][][2]float64` field is folded into
// one orb.MultiPolygon (outer rings first, holes filtered by the
// orb library at encode time per MVT spec).
//
// Atlas may be nil; in that case the Renderer has zero countries and
// every Render call returns an empty layer (which the MVT encoder
// turns into a valid but empty tile). The caller can use IsEmpty()
// to decide whether to bother calling Render at all.
func NewRenderer(atlas *pick.Atlas) *Renderer {
	r := &Renderer{countries: map[string]country{}}
	if atlas == nil {
		return r
	}
	for _, c := range atlas.Countries {
		key := c.ISO_A3
		if key == "" {
			key = c.Admin
		}
		// Each entry in `c.Polygons` is one RING. Natural Earth
		// stores Polygons as outer-then-holes-then-outer-then-holes
		// for MultiPolygon countries, but we don't have the boundary
		// markers separating "polygon groups". The safest read is:
		// every ring becomes its own polygon (no holes). For our
		// purpose (visual basemap fill) the missing hole subtraction
		// is invisible at z=0-5 — countries with internal holes
		// (e.g. Italy/Vatican, South Africa/Lesotho) are too small to
		// resolve at world-scale anyway.
		mp := make(orb.MultiPolygon, 0, len(c.Polygons))
		var largest orb.Polygon
		var largestArea float64
		for _, ring := range c.Polygons {
			if len(ring) < 3 {
				continue
			}
			pts := make(orb.Ring, len(ring))
			for i, p := range ring {
				pts[i] = orb.Point{p[0], p[1]}
			}
			poly := orb.Polygon{pts}
			mp = append(mp, poly)
			// Track the largest sub-polygon for label placement —
			// for archipelagos this lands the country name on the
			// mainland instead of in open water between islands.
			a := planar.Area(poly)
			if a > largestArea {
				largestArea = a
				largest = poly
			}
		}
		if len(mp) == 0 {
			continue
		}
		// Centroid of the largest sub-polygon. Falls back to the
		// MultiPolygon centroid for the (impossible-in-practice)
		// case where every sub-polygon had zero area.
		var label orb.Point
		if len(largest) > 0 {
			label, _ = planar.CentroidArea(largest)
		} else {
			label, _ = planar.CentroidArea(mp)
		}
		name := c.Name
		if name == "" {
			name = c.Admin
		}
		r.countries[key] = country{
			name:    name,
			iso:     key,
			polygon: mp,
			labelAt: label,
		}
	}
	return r
}

// IsEmpty reports whether the renderer has no polygons loaded. The
// HTTP handler checks this and falls through to a normal 404 when
// true, rather than serving an empty-but-valid MVT.
func (r *Renderer) IsEmpty() bool {
	return r == nil || len(r.countries) == 0
}

// Render returns a complete MVT tile (gzip-compressed protobuf) for
// (z, x, y) containing one Layer named "countries", with one Feature
// per polygon that intersects the tile. Geometry is clipped to the
// tile extent.
//
// Returns nil bytes when no country geometry overlaps the tile (open
// ocean). The caller can serve that as 404 just like the per-region
// path does.
func (r *Renderer) Render(z uint8, x, y uint32) ([]byte, error) {
	if r.IsEmpty() {
		return nil, nil
	}

	// orb's mvt encoder expects features in tile-space (origin at
	// the top-left of the tile, extent 0..4096). We use the maptile
	// helper to compute the tile's lat/lon bounds and let
	// `project.Mercator` handle the projection; the mvt library
	// then clips + delta-encodes for us.
	tile := maptile.New(uint32(x), uint32(y), maptile.Zoom(z))
	bound := tile.Bound() // tile bbox in lat/lon

	// Build features for any country whose bbox overlaps the tile.
	// The per-feature clip-to-tile happens inside the mvt encoder.
	//
	// CRITICAL: deep-clone each polygon before handing it to orb's
	// pipeline. `layers.ProjectToTile()` mutates point coordinates
	// in place, so reusing the shared `r.countries` polygons would
	// project the *first* request's tile-pixel coords into our
	// canonical lat/lon copy. Subsequent requests would then see
	// pre-projected coords and render almost nothing into their
	// own tile. Cloning here is O(coords-per-country) but happens
	// only for the country bboxes that intersect this tile, so
	// most countries are skipped by the prefilter.
	//
	// We emit TWO layers:
	//   - `countries` — polygon outlines, drives basemap borders +
	//     optional fill in the style.
	//   - `country_labels` — point features at the pre-computed
	//     label anchors, drives basemap text labels in the style.
	// Polygon centroids could in theory be derived client-side by
	// MapLibre via `text-field` on a polygon symbol layer, but the
	// auto-placement is poor for archipelagos (US, Indonesia, …) —
	// shipping explicit label points keeps placement consistent.
	polyFC := geojson.NewFeatureCollection()
	labelFC := geojson.NewFeatureCollection()
	for iso, c := range r.countries {
		if !intersects(c.polygon.Bound(), bound) {
			continue
		}
		poly := geojson.NewFeature(cloneMultiPolygon(c.polygon))
		poly.Properties["iso"] = iso
		poly.Properties["name"] = c.name
		r.mu.RLock()
		_, yes := r.installed[iso]
		r.mu.RUnlock()
		if yes {
			// 1 vs absent (rather than 0) keeps the wire size down
			// — most countries are not installed, so the property
			// is omitted from those features entirely.
			poly.Properties["installed"] = 1
		}
		polyFC.Append(poly)

		// Label point only if its anchor is actually inside this tile
		// (a country whose POLYGON overlaps but whose label sits in
		// a neighbouring tile wouldn't render here anyway and adding
		// it would just bloat the wire payload).
		if c.labelAt[0] >= bound.Min.X() && c.labelAt[0] <= bound.Max.X() &&
			c.labelAt[1] >= bound.Min.Y() && c.labelAt[1] <= bound.Max.Y() {
			lbl := geojson.NewFeature(orb.Point{c.labelAt[0], c.labelAt[1]})
			lbl.Properties["iso"] = iso
			lbl.Properties["name"] = c.name
			labelFC.Append(lbl)
		}
	}

	if len(polyFC.Features) == 0 {
		return nil, nil
	}

	// Assemble the MVT. Order matters at render time: MapLibre
	// composites layers bottom-up, so `countries` polygons land
	// underneath `country_labels` symbols by default.
	layers := mvt.Layers{
		mvt.NewLayer("countries", polyFC),
	}
	if len(labelFC.Features) > 0 {
		layers = append(layers, mvt.NewLayer("country_labels", labelFC))
	}

	// Project + clip to the tile's pixel coordinate space.
	layers.ProjectToTile(tile)
	layers.Clip(mvt.MapboxGLDefaultExtentBound)

	// MarshalGzipped gives us the exact wire format the protomaps /
	// pmtiles layer hands back, complete with the gzip framing — so
	// the HTTP handler can stream it straight out with
	// Content-Encoding: gzip the same as a pmtiles tile.
	return mvt.MarshalGzipped(layers)
}

// intersects is a fast bbox-vs-bbox check using orb's bound. Used as
// a prefilter before the (much more expensive) MVT clip.
func intersects(a, b orb.Bound) bool {
	return !(a.Max.X() < b.Min.X() || a.Min.X() > b.Max.X() ||
		a.Max.Y() < b.Min.Y() || a.Min.Y() > b.Max.Y())
}

// cloneMultiPolygon returns a deep copy. Necessary because orb's
// `ProjectToTile` rewrites Point values in place — without this, the
// canonical lat/lon polygons stored in `r.countries` would mutate
// into tile-pixel coords on the first request and every subsequent
// render call would see garbage (already-projected coords being
// projected again).
func cloneMultiPolygon(src orb.MultiPolygon) orb.MultiPolygon {
	dst := make(orb.MultiPolygon, len(src))
	for i, poly := range src {
		dpoly := make(orb.Polygon, len(poly))
		for j, ring := range poly {
			dring := make(orb.Ring, len(ring))
			copy(dring, ring)
			dpoly[j] = dring
		}
		dst[i] = dpoly
	}
	return dst
}

// _bytesBufferUnusedAvoidLint silences `unused import: bytes` when
// the function above doesn't reach the import. Kept here as a marker;
// remove once a second function lands.
var _ = bytes.NewBuffer

// _projectUnused keeps the project import live for the next iteration
// when we'll switch from MarshalGzipped (which projects internally)
// to a manual project + custom layer assembly. Currently unused.
var _ = project.Mercator
