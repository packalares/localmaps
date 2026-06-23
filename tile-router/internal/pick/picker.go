// Package pick implements tile-to-region selection.
//
// Given a Web-Mercator tile coordinate (z, x, y) and a set of installed
// regions, this package picks WHICH region's pmtiles file should serve
// that tile. The choice is purely geometric — the region whose bbox
// covers the most of the tile's footprint wins. Ties (overlap area
// equal to within float epsilon) are broken by smaller-region-first
// because tighter bboxes mean denser local detail (a country-scope
// extract beats a continent-scope extract over the same point).
//
// Coordinate conventions:
//
//   - BBox is [minLon, minLat, maxLon, maxLat] in WGS84 degrees,
//     matching the JSON shape stored in `regions.bbox` AND the
//     `bounds` field of a pmtiles header.
//   - Tile coordinates follow the Web-Mercator slippy-map convention
//     (z = zoom, x = column from west, y = row from north).
//
// This file is pure-math: no DB, no filesystem, no HTTP. Trivially
// unit-testable; trivially deterministic.
package pick

import "math"

// BBox is the WGS84 lat/lon bounding box of a region.
// Order matches the pmtiles header convention.
type BBox struct {
	MinLon, MinLat, MaxLon, MaxLat float64
}

// IsValid returns true when the box has positive area in both axes.
// Degenerate bboxes (e.g. all zeros — a region whose installer didn't
// populate bbox correctly) are excluded from picking so a single bad
// row can't shadow a real one.
func (b BBox) IsValid() bool {
	return b.MaxLon > b.MinLon && b.MaxLat > b.MinLat
}

// Area returns the bbox's degree² area. Used as the tiebreaker
// (smaller area = more specific = wins on tie).
func (b BBox) Area() float64 {
	return (b.MaxLon - b.MinLon) * (b.MaxLat - b.MinLat)
}

// Overlap returns the intersection bbox of two boxes, or a zero-area
// BBox when they don't intersect (caller can check via `.IsValid()`
// or `.Area() == 0`).
func (b BBox) Overlap(other BBox) BBox {
	return BBox{
		MinLon: math.Max(b.MinLon, other.MinLon),
		MinLat: math.Max(b.MinLat, other.MinLat),
		MaxLon: math.Min(b.MaxLon, other.MaxLon),
		MaxLat: math.Min(b.MaxLat, other.MaxLat),
	}
}

// Region is the minimum a picker needs to know about a region — its
// public identifier (for logging / debug), its bbox, and an optional
// reference to a Natural Earth country polygon when one matched at
// load time. The actual pmtiles handle is stored elsewhere; the
// picker only does geometry.
//
// Polygon may be nil — that's the normal case for regions that don't
// map cleanly to a single country (e.g. `europe-alps` spans CH/AT/IT/
// FR/SI/DE). The picker handles nil gracefully by falling back to
// bbox-of-tile-center, then to overlap area.
type Region struct {
	Name    string
	BBox    BBox
	Polygon *CountryPolygon
}

// TileBBox converts a Web-Mercator slippy tile coordinate (z, x, y)
// to its WGS84 lat/lon bbox.
//
// Returns the box of the tile's footprint on the globe — used to ask
// "which region's bbox best covers this tile?". z must be >=0; x and y
// are clipped silently for out-of-range inputs because that's better
// than returning an invalid box that pollutes downstream picks.
func TileBBox(z, x, y int) BBox {
	if z < 0 {
		z = 0
	}
	n := 1 << uint(z) // 2^z tiles per axis
	if x < 0 {
		x = 0
	} else if x >= n {
		x = n - 1
	}
	if y < 0 {
		y = 0
	} else if y >= n {
		y = n - 1
	}
	// Slippy → Mercator → lon/lat. Standard formulas, see
	// https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames.
	lon0 := tileX2Lon(x, n)
	lon1 := tileX2Lon(x+1, n)
	lat0 := tileY2Lat(y+1, n) // y+1 first because lat decreases as y increases
	lat1 := tileY2Lat(y, n)
	return BBox{MinLon: lon0, MinLat: lat0, MaxLon: lon1, MaxLat: lat1}
}

func tileX2Lon(x, n int) float64 {
	return float64(x)/float64(n)*360.0 - 180.0
}

func tileY2Lat(y, n int) float64 {
	r := math.Pi - 2*math.Pi*float64(y)/float64(n)
	return math.Atan(math.Sinh(r)) * 180.0 / math.Pi
}

// Pick returns the region best covering tile (z, x, y), or (nil,
// false) when no region's geometry covers the tile at all.
//
// Three-tier strategy in priority order:
//
//   1. POLYGON containment of the tile center. A region with a Natural
//      Earth country polygon attached wins if its territory actually
//      contains the tile center point. This is the accurate path —
//      Bucharest tiles correctly pick Romania even though Bulgaria's
//      bbox overlaps the same area. When multiple polygons contain
//      the same point (border zones) the smaller-bbox region wins.
//
//   2. BBOX containment of the tile center, restricted to regions
//      that don't have a polygon. Covers regions like `europe-alps`
//      that span multiple countries — Natural Earth's per-country
//      polygons don't capture them, but their bbox does.
//
//   3. OVERLAP area as the final fallback. Only fires when the tile
//      center is OUTSIDE every region's bbox but the tile itself
//      still overlaps something (low-zoom tiles that straddle
//      ocean + a coastal region). Same logic as the original
//      bbox-only picker.
//
// Within each tier, ties are broken by SMALLER bbox area (more local
// detail wins over a continent-scale super-extract).
//
// Linear in the number of regions; fine for the <100 we expect.
func Pick(regions []Region, z, x, y int) (*Region, bool) {
	tile := TileBBox(z, x, y)
	cLon := (tile.MinLon + tile.MaxLon) / 2
	cLat := (tile.MinLat + tile.MaxLat) / 2

	// Tier 1: polygon-in-tile-center.
	var polyBest *Region
	for i := range regions {
		r := &regions[i]
		if r.Polygon == nil || !r.BBox.IsValid() {
			continue
		}
		// Fast prefilter: skip the PIP test entirely when the tile
		// center isn't even in the bbox. PIP is ~50× more expensive
		// than a bbox check for large polygons.
		if !inBBox(r.BBox, cLon, cLat) {
			continue
		}
		if !r.Polygon.Contains(cLon, cLat) {
			continue
		}
		if polyBest == nil || r.BBox.Area() < polyBest.BBox.Area() {
			polyBest = r
		}
	}
	if polyBest != nil {
		return polyBest, true
	}

	// Tier 2: bbox-in-tile-center for regions without polygons. Each
	// candidate must NOT have a polygon (so we don't double-count
	// against tier 1's rejections — a region that has a polygon and
	// failed tier 1 is genuinely the wrong choice geometrically).
	var bboxBest *Region
	for i := range regions {
		r := &regions[i]
		if r.Polygon != nil || !r.BBox.IsValid() {
			continue
		}
		if !inBBox(r.BBox, cLon, cLat) {
			continue
		}
		if bboxBest == nil || r.BBox.Area() < bboxBest.BBox.Area() {
			bboxBest = r
		}
	}
	if bboxBest != nil {
		return bboxBest, true
	}

	// Tier 3: overlap area. Last resort for low-zoom tiles whose
	// center sits in the ocean but the tile still grazes a coastal
	// region.
	const epsArea = 1e-12
	var overlapBest *Region
	var bestOverlap float64
	for i := range regions {
		r := &regions[i]
		if !r.BBox.IsValid() {
			continue
		}
		ov := r.BBox.Overlap(tile)
		if !ov.IsValid() {
			continue
		}
		a := ov.Area()
		if a <= 0 {
			continue
		}
		if overlapBest == nil || a > bestOverlap+epsArea {
			overlapBest = r
			bestOverlap = a
			continue
		}
		if math.Abs(a-bestOverlap) < epsArea && r.BBox.Area() < overlapBest.BBox.Area() {
			overlapBest = r
			bestOverlap = a
		}
	}
	return overlapBest, overlapBest != nil
}

// inBBox is a fast bbox-contains-point check. Inlined inline-able by
// the Go compiler; sits in the hot path of every tile request.
func inBBox(b BBox, lon, lat float64) bool {
	return lon >= b.MinLon && lon <= b.MaxLon &&
		lat >= b.MinLat && lat <= b.MaxLat
}
