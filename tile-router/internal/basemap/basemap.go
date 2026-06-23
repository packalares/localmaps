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

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/project"

	"github.com/packalares/localmaps/tile-router/internal/pick"
)

// MaxZoom is the inclusive cap on basemap tile generation. Anything
// above this falls through to the per-region pmtiles picker.
const MaxZoom uint8 = 5

// Renderer holds the parsed country polygons in a form ready for
// per-tile clipping. Construct once at boot from a pick.Atlas; the
// Renderer reuses the polygon data but reorganises it as orb
// geometries the MVT encoder understands.
type Renderer struct {
	// countries maps Natural Earth's 3-letter ISO code to the
	// orb.MultiPolygon assembled at load time. Lookup goes by ISO
	// rather than name because the picker also keys by ISO and we
	// want consistent identification across both code paths.
	countries map[string]orb.MultiPolygon
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
	r := &Renderer{countries: map[string]orb.MultiPolygon{}}
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
		for _, ring := range c.Polygons {
			if len(ring) < 3 {
				continue
			}
			pts := make(orb.Ring, len(ring))
			for i, p := range ring {
				pts[i] = orb.Point{p[0], p[1]}
			}
			mp = append(mp, orb.Polygon{pts})
		}
		if len(mp) > 0 {
			r.countries[key] = mp
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
	fc := geojson.NewFeatureCollection()
	for iso, mp := range r.countries {
		if !intersects(mp.Bound(), bound) {
			continue
		}
		f := geojson.NewFeature(cloneMultiPolygon(mp))
		f.Properties["iso"] = iso
		fc.Append(f)
	}

	if len(fc.Features) == 0 {
		return nil, nil
	}

	// Assemble the single-layer MVT.
	layers := mvt.Layers{
		mvt.NewLayer("countries", fc),
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
