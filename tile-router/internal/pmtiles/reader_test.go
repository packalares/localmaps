package pmtiles

import (
	"math"
	"os"
	"testing"
)

// realFile is the integration target: a real Romania pmtiles file copied
// down from the pod for verification. Skipped when the file isn't there
// so CI / fresh clones don't fail. The path matches what the manual
// dev workflow uses; if you don't have it, run:
//
//   sshpass -p $PASS scp -P 2144 root@10.8.0.2:/.../romania/map.pmtiles /tmp/romania.pmtiles
const realFile = "/tmp/romania.pmtiles"

func TestOpen_RealRomania_HeaderMatches(t *testing.T) {
	if _, err := os.Stat(realFile); err != nil {
		t.Skipf("integration fixture missing (%s); skip", realFile)
	}
	r, err := Open(realFile)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()

	minLon, minLat, maxLon, maxLat := r.BBox()
	t.Logf("Romania pmtiles bbox: [%.4f, %.4f, %.4f, %.4f]",
		minLon, minLat, maxLon, maxLat)
	// Romania sits roughly between 20-30E and 43-49N. If the header
	// scaling is wrong (forgot E7, byte order, …) the values fall
	// outside any plausible range.
	if !inRange(minLon, 18, 22) || !inRange(maxLon, 28, 32) {
		t.Fatalf("Romania lon bbox out of plausible range: %f..%f", minLon, maxLon)
	}
	if !inRange(minLat, 42, 45) || !inRange(maxLat, 47, 49) {
		t.Fatalf("Romania lat bbox out of plausible range: %f..%f", minLat, maxLat)
	}

	minZ, maxZ := r.ZoomRange()
	t.Logf("zoom range: %d..%d", minZ, maxZ)
	if maxZ < 10 || maxZ > 16 {
		t.Errorf("unusual maxZoom %d; OpenMapTiles country builds are typically 14", maxZ)
	}
}

func TestReadTile_BucharestZ10_ReturnsBytes(t *testing.T) {
	if _, err := os.Stat(realFile); err != nil {
		t.Skipf("integration fixture missing (%s); skip", realFile)
	}
	r, err := Open(realFile)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()

	// Bucharest at z=10. lon/lat → slippy tile coords.
	z := uint8(10)
	x, y := lonLatToSlippy(26.10, 44.43, z)
	t.Logf("Bucharest z=%d tile = (%d, %d)", z, x, y)

	tile, err := r.ReadTile(z, x, y)
	if err != nil {
		t.Fatalf("ReadTile: %v", err)
	}
	if len(tile) == 0 {
		t.Fatal("Bucharest tile came back empty — expected data")
	}
	if len(tile) > 10*1024*1024 {
		t.Errorf("tile suspiciously large: %d bytes", len(tile))
	}
	t.Logf("Bucharest z=%d tile: %d bytes", z, len(tile))
	// First two bytes of a gzip stream are 0x1f 0x8b. Vector tiles
	// in pmtiles are gzip-compressed by default.
	if tile[0] == 0x1f && tile[1] == 0x8b {
		t.Log("payload is gzip-compressed (expected)")
	}
}

func TestReadTile_OutOfRegion_ReturnsNil(t *testing.T) {
	if _, err := os.Stat(realFile); err != nil {
		t.Skipf("integration fixture missing (%s); skip", realFile)
	}
	r, err := Open(realFile)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()

	// Atlantic Ocean — well outside Romania's bbox. PMTiles should
	// return no tile (a missing key in the directory). The wrapper
	// returns (nil, nil) for that case so callers can 404 cleanly.
	z := uint8(10)
	x, y := lonLatToSlippy(-40, 30, z)
	tile, err := r.ReadTile(z, x, y)
	if err != nil {
		t.Fatalf("unexpected error for ocean tile: %v", err)
	}
	if len(tile) != 0 {
		t.Errorf("ocean tile returned %d bytes; expected nil", len(tile))
	}
}

func TestReadTile_OutOfZoomRange_ReturnsNil(t *testing.T) {
	if _, err := os.Stat(realFile); err != nil {
		t.Skipf("integration fixture missing (%s); skip", realFile)
	}
	r, err := Open(realFile)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()

	// PMTiles built for OpenMapTiles typically max out at z=14. z=20
	// is way past that and should silently return nil rather than
	// reading nonsense bytes from the directory.
	tile, err := r.ReadTile(20, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error for over-zoom: %v", err)
	}
	if len(tile) != 0 {
		t.Errorf("over-zoom tile returned %d bytes; expected nil", len(tile))
	}
}

// ---- helpers ----------------------------------------------------------

func lonLatToSlippy(lon, lat float64, z uint8) (uint32, uint32) {
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

func inRange(v, lo, hi float64) bool {
	return v >= lo && v <= hi
}
