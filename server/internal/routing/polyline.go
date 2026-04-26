package routing

// DecodePolyline6 decodes a Google-style encoded polyline using
// precision-6 (1e-6 deg), which is what Valhalla emits. Unknown
// or truncated input returns whatever points were decoded before
// the error plus an `ok=false`.
//
// Reference: https://valhalla.github.io/valhalla/decoding/
//
// The caller receives a flat []LatLon (lat,lon pairs).
func DecodePolyline6(encoded string) ([]LatLon, bool) {
	const factor = 1e6
	var (
		lat, lon int64
		out      = make([]LatLon, 0, len(encoded)/4)
	)
	i := 0
	for i < len(encoded) {
		dlat, n := decodeVarint(encoded, i)
		if n == 0 {
			return out, false
		}
		i += n
		dlon, n := decodeVarint(encoded, i)
		if n == 0 {
			return out, false
		}
		i += n
		lat += dlat
		lon += dlon
		out = append(out, LatLon{
			Lat: float64(lat) / factor,
			Lon: float64(lon) / factor,
		})
	}
	return out, true
}

// decodeVarint reads one signed varint from s starting at off.
// Returns (value, bytesRead). bytesRead is 0 on malformed input.
func decodeVarint(s string, off int) (int64, int) {
	var (
		shift uint
		result int64
	)
	for j := off; j < len(s); j++ {
		b := int64(s[j]) - 63
		if b < 0 {
			return 0, 0
		}
		result |= (b & 0x1f) << shift
		if b < 0x20 {
			// Final chunk. ZigZag-decode.
			n := j - off + 1
			if result&1 != 0 {
				return ^(result >> 1), n
			}
			return result >> 1, n
		}
		shift += 5
		if shift > 63 {
			return 0, 0
		}
	}
	return 0, 0
}
