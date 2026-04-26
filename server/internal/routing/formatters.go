package routing

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// FormatGPX renders a CachedRoute as a minimal GPX 1.1 XML document.
// The file contains one <trk> with one <trkseg> of <trkpt> elements
// (one per shape point). This is the 80% export — turn-by-turn
// waypoints and metadata are deliberately not emitted to keep the
// output stable for differ-driven tests.
func FormatGPX(cr CachedRoute) []byte {
	var sb strings.Builder
	sb.Grow(len(cr.Shape)*48 + 256)
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(`<gpx version="1.1" creator="localmaps" xmlns="http://www.topografix.com/GPX/1/1">`)
	sb.WriteString("\n  <trk>\n    <name>")
	sb.WriteString(xmlEscape("localmaps route " + cr.ID))
	sb.WriteString("</name>\n    <trkseg>\n")
	for _, p := range cr.Shape {
		sb.WriteString(`      <trkpt lat="`)
		sb.WriteString(formatCoord(p.Lat))
		sb.WriteString(`" lon="`)
		sb.WriteString(formatCoord(p.Lon))
		sb.WriteString(`"></trkpt>`)
		sb.WriteString("\n")
	}
	sb.WriteString("    </trkseg>\n  </trk>\n</gpx>\n")
	return []byte(sb.String())
}

// FormatKML renders a CachedRoute as a KML 2.2 document containing a
// single Placemark/LineString of the route's shape. Coordinates are
// emitted lon,lat per KML spec.
func FormatKML(cr CachedRoute) []byte {
	var sb strings.Builder
	sb.Grow(len(cr.Shape)*24 + 256)
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(`<kml xmlns="http://www.opengis.net/kml/2.2">`)
	sb.WriteString("\n  <Document>\n    <name>")
	sb.WriteString(xmlEscape("localmaps route " + cr.ID))
	sb.WriteString("</name>\n    <Placemark>\n      <name>Route</name>\n      <LineString>\n        <tessellate>1</tessellate>\n        <coordinates>\n")
	for _, p := range cr.Shape {
		sb.WriteString("          ")
		sb.WriteString(formatCoord(p.Lon))
		sb.WriteString(",")
		sb.WriteString(formatCoord(p.Lat))
		sb.WriteString(",0\n")
	}
	sb.WriteString("        </coordinates>\n      </LineString>\n    </Placemark>\n  </Document>\n</kml>\n")
	return []byte(sb.String())
}

// formatCoord renders a coordinate with up to 6 decimal places and no
// trailing zeros — matches what most GPX/KML consumers expect.
func formatCoord(v float64) string {
	s := strconv.FormatFloat(v, 'f', 6, 64)
	// Trim trailing zeros, leaving at least one digit after the dot.
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// xmlEscape uses encoding/xml's EscapeText to be safe for attributes
// and cdata alike. It's not a hot path, so the allocation is fine.
func xmlEscape(s string) string {
	var sb strings.Builder
	_ = xml.EscapeText(&sb, []byte(s))
	return sb.String()
}
