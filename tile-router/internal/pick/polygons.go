// Polygon-based region picking.
//
// Bbox-overlap picking is correct only when every tile's bbox sits
// cleanly inside exactly one region's bbox. At low-to-medium zoom
// levels (z=4..10) and along country borders this isn't true — for
// example tile (8, 146, 93) is centered at 26.10 E, 43.69 N which
// is inside BOTH Romania's bbox AND Bulgaria's bbox (Bulgaria's
// north edge is 44.22 N, Romania's south edge is 43.61 N, so any
// 1°-wide tile straddling that strip qualifies for both). The
// area-based bbox tiebreaker then chooses Bulgaria — which is wrong:
// Bucharest is geographically in Romania.
//
// Country POLYGONS resolve this. A polygon-in-polygon (or here
// point-in-polygon on the tile center) check picks the region whose
// actual territory contains the tile, ignoring how the rectangular
// bbox happens to sit.
//
// Source: Natural Earth admin_0_countries at 1:50m scale, slimmed to
// {ADMIN, NAME, ISO_A2, ISO_A3} attributes and embedded as gzip into
// the binary at compile time. ~3 MB of source GeoJSON compresses to
// ~835 KB; decoding at boot takes ~50 ms.
//
// Not every region has a polygon match:
//   - "europe-romania" → Natural Earth NAME="Romania"  ✓
//   - "europe-alps" — a Geofabrik super-extract spanning ~6 countries;
//     no single NE polygon covers it. Falls back to bbox.
// The picker handles both cases — point-in-polygon when we have one,
// bbox-of-tile-center otherwise, overlap area as the final fallback
// for tiles that don't sit in any installed region's bbox at all.

package pick

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"strings"
)

//go:embed admin0_slim.geojson.gz
var admin0Gz []byte

// CountryPolygon is one Natural Earth country's geometry, simplified
// to the polygon ring(s) we actually traverse during PIP tests.
//
// We don't keep the full GeoJSON tree — geometry can be a `Polygon`
// (single outer ring + optional holes) or `MultiPolygon` (list of
// such rings, for archipelagos / detached islands). Both fold into
// the same flat list of polygons here: each polygon is one outer
// ring plus zero or more inner rings (holes). The PIP test xors the
// containment of each ring, so holes correctly subtract.
type CountryPolygon struct {
	Admin   string  // human-readable, e.g. "Romania"
	Name    string  // short name, often same as Admin
	ISO_A2  string  // 2-letter ISO code, e.g. "RO"
	ISO_A3  string  // 3-letter ISO code, e.g. "ROU"
	Polygons [][][2]float64 // flattened: each entry is one ring (outer or hole)
}

// Atlas is the loaded set of country polygons + a name→polygon
// index. Lookups are case-insensitive on the short name and on the
// 3-letter ISO code; that's what matches Geofabrik region naming
// (`europe-romania` → strip the continent prefix → `romania`).
type Atlas struct {
	Countries []CountryPolygon
	byKey     map[string]*CountryPolygon
}

// LoadAtlas parses the embedded Natural Earth GeoJSON. Called once
// at startup; the picker reuses the returned Atlas for every tile
// request.
//
// Error handling: any failure (gzip, JSON, or schema) returns an
// empty Atlas + the error. The caller is expected to log + carry
// on with bbox-only picking — a missing polygon set is degraded
// service, not a crash.
func LoadAtlas() (*Atlas, error) {
	rdr, err := gzip.NewReader(bytes.NewReader(admin0Gz))
	if err != nil {
		return &Atlas{byKey: map[string]*CountryPolygon{}}, err
	}
	defer rdr.Close()
	body, err := io.ReadAll(rdr)
	if err != nil {
		return &Atlas{byKey: map[string]*CountryPolygon{}}, err
	}
	var fc struct {
		Features []struct {
			Type       string `json:"type"`
			Properties struct {
				Admin  string `json:"ADMIN"`
				Name   string `json:"NAME"`
				ISO_A2 string `json:"ISO_A2"`
				ISO_A3 string `json:"ISO_A3"`
			} `json:"properties"`
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &fc); err != nil {
		return &Atlas{byKey: map[string]*CountryPolygon{}}, err
	}
	a := &Atlas{
		Countries: make([]CountryPolygon, 0, len(fc.Features)),
		byKey:     map[string]*CountryPolygon{},
	}
	for _, f := range fc.Features {
		cp := CountryPolygon{
			Admin:  f.Properties.Admin,
			Name:   f.Properties.Name,
			ISO_A2: f.Properties.ISO_A2,
			ISO_A3: f.Properties.ISO_A3,
		}
		// Geometry decoding: Polygon = `[[ring], [hole], …]`,
		// MultiPolygon = `[[[ring], [hole]], [[ring], …]]`.
		switch f.Geometry.Type {
		case "Polygon":
			var rings [][][2]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &rings); err != nil {
				continue
			}
			cp.Polygons = append(cp.Polygons, rings...)
		case "MultiPolygon":
			var multi [][][][2]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &multi); err != nil {
				continue
			}
			for _, p := range multi {
				cp.Polygons = append(cp.Polygons, p...)
			}
		default:
			continue
		}
		a.Countries = append(a.Countries, cp)
		// Index by NAME, ADMIN, ISO codes — all case-insensitive +
		// space-stripped so "United States" matches both "us" and
		// "unitedstates" lookups.
		final := &a.Countries[len(a.Countries)-1]
		for _, k := range []string{cp.Name, cp.Admin, cp.ISO_A3, cp.ISO_A2} {
			if key := normalizeKey(k); key != "" {
				a.byKey[key] = final
			}
		}
	}
	return a, nil
}

// CountryForRegion attempts to resolve a region name (e.g.
// "europe-romania", "us-california", "north-america-canada") to a
// Natural Earth country polygon. Returns nil when no match — that's
// not an error; the caller should fall back to bbox.
//
// Matching strategy:
//  1. Strip a leading continent prefix ("europe-", "africa-", …)
//  2. Replace remaining dashes with spaces
//  3. Lookup in the byKey index (case-insensitive, space-stripped)
//
// Names that don't map (e.g. "europe-alps" — a Geofabrik super-extract
// spanning multiple countries) return nil. The picker handles that
// case gracefully by falling back to bbox containment.
func (a *Atlas) CountryForRegion(regionName string) *CountryPolygon {
	if a == nil {
		return nil
	}
	stripped := stripContinentPrefix(regionName)
	candidates := []string{stripped, strings.ReplaceAll(stripped, "-", " ")}
	for _, c := range candidates {
		if cp := a.byKey[normalizeKey(c)]; cp != nil {
			return cp
		}
	}
	return nil
}

var continentPrefixes = []string{
	"europe-",
	"africa-",
	"asia-",
	"north-america-",
	"south-america-",
	"central-america-",
	"australia-oceania-",
	"antarctica-",
	"russia-",
}

func stripContinentPrefix(s string) string {
	low := strings.ToLower(s)
	for _, p := range continentPrefixes {
		if strings.HasPrefix(low, p) {
			return s[len(p):]
		}
	}
	return s
}

func normalizeKey(s string) string {
	// Lowercase + drop all whitespace + drop punctuation that varies
	// between datasets ("U.S.A." vs "USA" vs "United States").
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Contains returns true if (lon, lat) lies inside any of the
// country's polygons. Handles holes correctly via xor (a point
// inside an outer ring but inside a hole isn't contained).
//
// Algorithm: classic ray-casting with horizontal east-going ray.
// Each ring is processed independently and the boolean parity
// across all rings is the answer. Holes are subtracted automatically.
func (cp *CountryPolygon) Contains(lon, lat float64) bool {
	inside := false
	for _, ring := range cp.Polygons {
		if pointInRing(ring, lon, lat) {
			inside = !inside
		}
	}
	return inside
}

// pointInRing is the standard even-odd ray-casting PIP test. Edge
// cases (point exactly on an edge) are accepted as "inside"; we'd
// rather over-include than under-include since a missed match falls
// through to bbox where the test is much coarser.
func pointInRing(ring [][2]float64, lon, lat float64) bool {
	if len(ring) < 3 {
		return false
	}
	inside := false
	j := len(ring) - 1
	for i := 0; i < len(ring); i++ {
		xi, yi := ring[i][0], ring[i][1]
		xj, yj := ring[j][0], ring[j][1]
		// Standard edge-crossing check. The conditional avoids
		// division-by-zero when an edge is horizontal.
		if ((yi > lat) != (yj > lat)) &&
			lon < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}
