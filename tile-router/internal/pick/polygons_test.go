package pick

import (
	"math"
	"testing"
)

// TestLoadAtlas_Boot smoke-tests the embedded Natural Earth dataset:
// it should parse, contain a sane number of countries, and resolve
// the regions the user has installed.
func TestLoadAtlas_Boot(t *testing.T) {
	a, err := LoadAtlas()
	if err != nil {
		t.Fatalf("LoadAtlas: %v", err)
	}
	if len(a.Countries) < 200 {
		t.Errorf("expected ~242 countries, got %d", len(a.Countries))
	}
	// Sanity: the regions installed on the test pod should all
	// resolve to a polygon — except `europe-alps` which spans 6
	// countries and is not a single NE polygon.
	for _, region := range []struct {
		name      string
		shouldMap bool
	}{
		{"europe-romania", true},
		{"europe-bulgaria", true},
		{"europe-greece", true},
		{"europe-turkey", true},
		{"europe-albania", true},
		{"europe-alps", false}, // multi-country super-extract
	} {
		got := a.CountryForRegion(region.name)
		if region.shouldMap && got == nil {
			t.Errorf("%s should resolve to a polygon", region.name)
		}
		if !region.shouldMap && got != nil {
			t.Errorf("%s should NOT resolve (got %s) — multi-country", region.name, got.Admin)
		}
	}
}

// TestPick_BordersCorrectlyResolved is the regression test for the
// actual bug the user hit. At z=8 each tile spans ~1.4° lon × 0.9°
// lat, big enough to straddle country bboxes — Bulgaria's bbox
// extends north into the Romanian strip, Greece's east into Turkish
// waters, etc. With polygon picking each capital tile should land
// in its true country.
func TestPick_BordersCorrectlyResolved(t *testing.T) {
	a, err := LoadAtlas()
	if err != nil {
		t.Fatalf("LoadAtlas: %v", err)
	}

	regions := []Region{
		{Name: "europe-romania", BBox: BBox{20.2418, 43.6125, 30.2790, 48.2695}, Polygon: a.CountryForRegion("europe-romania")},
		{Name: "europe-bulgaria", BBox: BBox{22.3571, 41.2349, 29.1882, 44.2152}, Polygon: a.CountryForRegion("europe-bulgaria")},
		{Name: "europe-greece", BBox: BBox{18.9706, 34.5911, 29.6568, 41.7495}, Polygon: a.CountryForRegion("europe-greece")},
		{Name: "europe-turkey", BBox: BBox{25.5240, 35.7170, 44.8599, 43.0748}, Polygon: a.CountryForRegion("europe-turkey")},
		{Name: "europe-albania", BBox: BBox{18.9000, 39.6300, 21.0600, 42.6600}, Polygon: a.CountryForRegion("europe-albania")},
		// Alps has no polygon — bbox-only.
		{Name: "europe-alps", BBox: BBox{4.842, 42.698, 16.762, 48.401}, Polygon: nil},
	}

	// Coordinates derived from each capital's lat/lon via slippy
	// math so the test stays correct as zoom levels change.
	cases := []struct {
		label string
		lon, lat float64
		z    int
		want string
	}{
		{"Bucharest z=8", 26.10, 44.43, 8, "europe-romania"},
		{"Sofia z=8", 23.32, 42.70, 8, "europe-bulgaria"},
		{"Athens z=8", 23.73, 37.98, 8, "europe-greece"},
		{"Ankara z=8", 32.85, 39.93, 8, "europe-turkey"},
		{"Tirana z=10", 19.82, 41.33, 10, "europe-albania"},
		// Also exercise mid-zoom where a tile is ~0.7° wide and
		// border ambiguity is still possible.
		{"Bucharest z=10", 26.10, 44.43, 10, "europe-romania"},
		{"Sofia z=10", 23.32, 42.70, 10, "europe-bulgaria"},
	}
	for _, c := range cases {
		x, y := lonLatToTileF(c.lon, c.lat, c.z)
		got, ok := Pick(regions, c.z, int(x), int(y))
		if !ok {
			t.Errorf("%s (tile %d/%d/%d): no region picked", c.label, c.z, x, y)
			continue
		}
		if got.Name != c.want {
			t.Errorf("%s (tile %d/%d/%d): got %s want %s", c.label, c.z, x, y, got.Name, c.want)
		}
	}
}

// TestPick_AlpsStillWorksViaBBox checks that the bbox fallback fires
// for regions without a polygon. The Alps super-extract spans 6
// countries; no Natural Earth polygon matches. The picker should
// still serve tiles in the Swiss/Austrian Alps from the alps pmtiles
// via the bbox-of-tile-center fallback.
//
// We use Zurich (8.541E, 47.376N) — that's inside the Alps bbox AND
// inside Switzerland's NE polygon. If we DON'T install a Switzerland
// region, the Alps region (no polygon) should win via tier 2.
func TestPick_AlpsStillWorksViaBBox(t *testing.T) {
	a, _ := LoadAtlas()
	regions := []Region{
		{Name: "europe-alps", BBox: BBox{4.842, 42.698, 16.762, 48.401}, Polygon: nil},
		// Distractor: Romania is far from Zurich and has a polygon
		// that doesn't contain Zurich. Should NOT be picked.
		{Name: "europe-romania", BBox: BBox{20.2418, 43.6125, 30.2790, 48.2695}, Polygon: a.CountryForRegion("europe-romania")},
	}
	z, x, y := lonLatToTile(8.541, 47.376, 10)
	got, ok := Pick(regions, z, x, y)
	if !ok || got.Name != "europe-alps" {
		t.Fatalf("Zurich should pick europe-alps via bbox tier; got %v ok=%v", got, ok)
	}
}

// TestPolygon_Contains is a minimal sanity check on the polygon
// containment math — make sure Bucharest is in Romania's polygon
// and Sofia is in Bulgaria's, NOT vice versa.
func TestPolygon_Contains(t *testing.T) {
	a, err := LoadAtlas()
	if err != nil {
		t.Fatalf("LoadAtlas: %v", err)
	}
	ro := a.CountryForRegion("europe-romania")
	bg := a.CountryForRegion("europe-bulgaria")
	gr := a.CountryForRegion("europe-greece")
	if ro == nil || bg == nil || gr == nil {
		t.Fatalf("missing polygons: ro=%v bg=%v gr=%v", ro != nil, bg != nil, gr != nil)
	}
	if !ro.Contains(26.10, 44.43) {
		t.Errorf("Bucharest (26.10, 44.43) should be inside Romania")
	}
	if bg.Contains(26.10, 44.43) {
		t.Errorf("Bucharest should NOT be inside Bulgaria — that was the bug")
	}
	if !bg.Contains(23.32, 42.70) {
		t.Errorf("Sofia (23.32, 42.70) should be inside Bulgaria")
	}
	if ro.Contains(23.32, 42.70) {
		t.Errorf("Sofia should NOT be inside Romania")
	}
	if !gr.Contains(23.73, 37.98) {
		t.Errorf("Athens (23.73, 37.98) should be inside Greece")
	}
}

// helper — matches the picker test's slippy converter
func lonLatToTileF(lon, lat float64, z int) (uint32, uint32) {
	n := 1 << uint(z)
	x := int(math.Floor((lon + 180) / 360 * float64(n)))
	latRad := lat * math.Pi / 180
	y := int(math.Floor((1 - math.Log(math.Tan(latRad)+1/math.Cos(latRad))/math.Pi) / 2 * float64(n)))
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
	return uint32(x), uint32(y)
}
