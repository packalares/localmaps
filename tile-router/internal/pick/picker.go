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
// public identifier (for logging / debug) and its bbox. The actual
// pmtiles handle is stored elsewhere; the picker only does geometry.
type Region struct {
	Name string
	BBox BBox
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

// Pick returns the region best covering tile (z, x, y), or (nil, false)
// when no region's bbox overlaps the tile at all (ocean / not installed
// / etc — the caller serves a 404).
//
// Algorithm:
//   1. Compute the tile's bbox.
//   2. For each region with a valid bbox, compute overlap area.
//   3. The largest overlap wins.
//   4. On tie (within `epsArea`), the smaller-region-bbox wins so a
//      tighter country extract beats an overlapping continent extract.
//
// Linear in the number of regions; fine for the <100 we expect.
func Pick(regions []Region, z, x, y int) (*Region, bool) {
	tile := TileBBox(z, x, y)
	const epsArea = 1e-12
	var best *Region
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
		if best == nil || a > bestOverlap+epsArea {
			best = r
			bestOverlap = a
			continue
		}
		// Tie: prefer the SMALLER bbox (more local detail).
		if math.Abs(a-bestOverlap) < epsArea && r.BBox.Area() < best.BBox.Area() {
			best = r
			bestOverlap = a
		}
	}
	return best, best != nil
}
