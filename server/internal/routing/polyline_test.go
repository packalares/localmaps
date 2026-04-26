package routing

import (
	"math"
	"testing"
)

func TestDecodePolyline6_Empty(t *testing.T) {
	got, ok := DecodePolyline6("")
	if !ok {
		t.Fatalf("empty string should decode to zero points with ok=true")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 points, got %d", len(got))
	}
}

func TestDecodePolyline6_Malformed(t *testing.T) {
	// A single low-ASCII byte (value < 63) triggers the b<0 branch.
	got, ok := DecodePolyline6(" ")
	if ok {
		t.Fatalf("expected ok=false for malformed input, got %v", got)
	}
}

// Encode a series of points with Valhalla's precision-6 algorithm and
// verify round-trip.
func TestDecodePolyline6_RoundTrip(t *testing.T) {
	points := []LatLon{
		{Lat: 44.4268, Lon: 26.1025},
		{Lat: 44.4270, Lon: 26.1030},
		{Lat: 44.4272, Lon: 26.1029},
	}
	encoded := encodePolyline6(points)
	got, ok := DecodePolyline6(encoded)
	if !ok {
		t.Fatalf("round-trip decode failed")
	}
	if len(got) != len(points) {
		t.Fatalf("len mismatch: want %d got %d", len(points), len(got))
	}
	for i, p := range got {
		if math.Abs(p.Lat-points[i].Lat) > 1e-6 ||
			math.Abs(p.Lon-points[i].Lon) > 1e-6 {
			t.Errorf("point %d mismatch: want %v got %v", i, points[i], p)
		}
	}
}

// encodePolyline6 is the inverse of DecodePolyline6, used only by tests
// to avoid depending on a vendor-specific encoded fixture.
func encodePolyline6(points []LatLon) string {
	const factor = 1e6
	var (
		prevLat, prevLon int64
		out              []byte
	)
	for _, p := range points {
		lat := int64(math.Round(p.Lat * factor))
		lon := int64(math.Round(p.Lon * factor))
		out = appendVarint(out, lat-prevLat)
		out = appendVarint(out, lon-prevLon)
		prevLat, prevLon = lat, lon
	}
	return string(out)
}

func appendVarint(dst []byte, v int64) []byte {
	u := uint64(v << 1)
	if v < 0 {
		u = ^uint64(v<<1)
	}
	for u >= 0x20 {
		dst = append(dst, byte((0x20|(u&0x1f))+63))
		u >>= 5
	}
	dst = append(dst, byte(u+63))
	return dst
}
