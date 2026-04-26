package routing

import (
	"encoding/xml"
	"strings"
	"testing"
)

var sampleRoute = CachedRoute{
	ID:             "test-route-1",
	Mode:           "auto",
	TimeSeconds:    123,
	DistanceMeters: 4567,
	Shape: []LatLon{
		{Lat: 44.4268, Lon: 26.1025},
		{Lat: 44.4270, Lon: 26.1030},
		{Lat: 44.4272, Lon: 26.1029},
	},
}

func TestFormatGPX_WellFormed(t *testing.T) {
	out := FormatGPX(sampleRoute)
	if len(out) == 0 {
		t.Fatal("empty output")
	}
	// Must parse as XML.
	var anyNode xmlNode
	if err := xml.Unmarshal(out, &anyNode); err != nil {
		t.Fatalf("GPX did not parse as XML: %v\n%s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Error("missing XML declaration")
	}
	if !strings.Contains(s, `<gpx`) || !strings.Contains(s, `version="1.1"`) {
		t.Error("missing gpx root with version 1.1")
	}
	if !strings.Contains(s, `xmlns="http://www.topografix.com/GPX/1/1"`) {
		t.Error("missing GPX 1.1 namespace")
	}
	// One <trkpt> per shape point.
	if got := strings.Count(s, "<trkpt "); got != len(sampleRoute.Shape) {
		t.Errorf("trkpt count: want %d got %d", len(sampleRoute.Shape), got)
	}
	// Coordinates must appear as lat/lon attributes.
	if !strings.Contains(s, `lat="44.4268" lon="26.1025"`) {
		t.Error("first point attributes missing")
	}
}

func TestFormatKML_WellFormed(t *testing.T) {
	out := FormatKML(sampleRoute)
	if len(out) == 0 {
		t.Fatal("empty output")
	}
	var anyNode xmlNode
	if err := xml.Unmarshal(out, &anyNode); err != nil {
		t.Fatalf("KML did not parse as XML: %v\n%s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, `<kml xmlns="http://www.opengis.net/kml/2.2">`) {
		t.Error("missing kml 2.2 namespace")
	}
	if !strings.Contains(s, `<LineString>`) || !strings.Contains(s, `</LineString>`) {
		t.Error("missing LineString element")
	}
	// KML coordinates: lon,lat,alt — reversed relative to GPX.
	if !strings.Contains(s, "26.1025,44.4268,0") {
		t.Error("first coord (lon,lat,alt) missing")
	}
	if !strings.Contains(s, "<coordinates>") || !strings.Contains(s, "</coordinates>") {
		t.Error("missing <coordinates> block")
	}
}

func TestFormatGPX_EscapesNameAttribute(t *testing.T) {
	r := sampleRoute
	r.ID = `<script>alert("x")</script>`
	out := string(FormatGPX(r))
	if strings.Contains(out, `<script>`) {
		t.Error("GPX did not escape injected <script> tag")
	}
	if !strings.Contains(out, `&lt;script&gt;`) {
		t.Error("GPX did not HTML-escape angle brackets in name")
	}
}

func TestFormatCoord_TrimsTrailingZeros(t *testing.T) {
	cases := map[float64]string{
		44.4268:    "44.4268",
		44.000000:  "44",
		-0.123000:  "-0.123",
		26.100500:  "26.1005",
	}
	for in, want := range cases {
		if got := formatCoord(in); got != want {
			t.Errorf("formatCoord(%v): want %q got %q", in, want, got)
		}
	}
}

// xmlNode is a generic unmarshal target — we only care that parsing
// succeeds, not about the tree structure.
type xmlNode struct {
	XMLName xml.Name
	Content string     `xml:",chardata"`
	Nodes   []xmlNode  `xml:",any"`
}
