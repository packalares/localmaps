package api

// Query-string parsing + validation for GET /og/preview.png. Split out
// of og.go to respect the agent rules' 250-line-per-file cap and to
// keep the handler body focused on flow control.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/og"
)

// parseOGQuery decodes and validates the query string against the
// bounds in contracts/openapi.yaml. `region` is an optional LocalMaps
// extension (not in openapi today; used as caption only) — we validate
// it shape-wise via the shared region-key rules before passing down.
func parseOGQuery(c fiber.Ctx) (og.Params, error) {
	q := func(k string) string { return strings.TrimSpace(c.Query(k)) }

	lat, err := strconv.ParseFloat(q("lat"), 64)
	if err != nil || lat < -90 || lat > 90 {
		return og.Params{}, fmt.Errorf("lat must be a number in [-90, 90]")
	}
	lon, err := strconv.ParseFloat(q("lon"), 64)
	if err != nil || lon < -180 || lon > 180 {
		return og.Params{}, fmt.Errorf("lon must be a number in [-180, 180]")
	}
	zoom, err := parseIntWithDefault(q("zoom"), 12, 0, 20, "zoom")
	if err != nil {
		return og.Params{}, err
	}
	pinWanted := true
	if s := q("pin"); s != "" {
		b, perr := strconv.ParseBool(s)
		if perr != nil {
			return og.Params{}, fmt.Errorf("pin must be a boolean")
		}
		pinWanted = b
	}
	width, err := parseIntWithDefault(q("width"), og.DefaultWidth, 320, 2048, "width")
	if err != nil {
		return og.Params{}, err
	}
	height, err := parseIntWithDefault(q("height"), og.DefaultHeight, 200, 2048, "height")
	if err != nil {
		return og.Params{}, err
	}
	style := q("style")
	if style != "" && !isKnownStyle(style) {
		return og.Params{}, fmt.Errorf("style must be one of: light, dark, auto")
	}
	region := q("region")
	if region != "" && !isSafeRegionToken(region) {
		return og.Params{}, fmt.Errorf("region contains invalid characters")
	}

	params := og.Params{
		Center: og.LatLon{Lat: lat, Lon: lon},
		Zoom:   zoom,
		Style:  style,
		Region: region,
		Size:   og.Size{W: width, H: height},
	}
	if pinWanted {
		params.Pin = &og.LatLon{Lat: lat, Lon: lon}
	}
	return params, nil
}

// parseIntWithDefault parses a bounded integer query param, falling
// back to def when the caller omits it, and returning a 400-friendly
// error message naming the offending field.
func parseIntWithDefault(raw string, def, lo, hi int, name string) (int, error) {
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < lo || v > hi {
		return 0, fmt.Errorf("%s must be an integer in [%d, %d]", name, lo, hi)
	}
	return v, nil
}

// isKnownStyle mirrors docs/07 map.style enum + the og package's own
// KnownStyles list. Kept in one place.
func isKnownStyle(s string) bool {
	for _, v := range og.KnownStyles() {
		if v == s {
			return true
		}
	}
	return false
}

// isSafeRegionToken accepts [a-z0-9-] canonical keys (see
// internal/regions/key.go). Mirrors the public route rule rather than
// importing the validator (keep the dependency graph narrow).
func isSafeRegionToken(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
