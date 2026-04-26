package api

// Handlers for the `map` tag in contracts/openapi.yaml.

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/glyphs"
	"github.com/packalares/localmaps/server/internal/sprites"
)

// protomapsClient is a dedicated short-timeout HTTP client for
// upstream tile fetches. Tile requests are latency-sensitive: a stale
// upstream shouldn't pile up connections on the gateway.
var protomapsClient = &http.Client{Timeout: 3 * time.Second}

// newTileHandler proxies GET /api/tiles/{region}/{z}/{x}/{y}.pbf to the
// in-cluster pmtiles server. protomaps (pmtiles serve) exposes XYZ
// endpoints at `<base>/<archive>/{z}/{x}/{y}.mvt`, where the archive
// name is the pmtiles filename without extension. Regions live on disk
// at `<dataDir>/regions/<region>/map.pmtiles`, so the archive name per
// region is `<region>/map`.
//
// base should be the upstream root (e.g. `http://protomaps:8000`),
// trailing slash stripped. An empty base falls back to the default.
func newTileHandler(base string) fiber.Handler {
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "http://protomaps:8000"
	}
	return func(c fiber.Ctx) error {
		region := c.Params("region")
		z := c.Params("z")
		x := c.Params("x")
		y := c.Params("y")
		if region == "" || z == "" || x == "" || y == "" {
			return c.SendStatus(fiber.StatusBadRequest)
		}
		url := base + "/" + region + "/map/" + z + "/" + x + "/" + y + ".mvt"
		req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, url, nil)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		resp, err := protomapsClient.Do(req)
		if err != nil {
			return c.SendStatus(fiber.StatusBadGateway)
		}
		defer resp.Body.Close()

		// Preserve protomaps' status (notably 404 for missing tiles and
		// 204 for empty tiles) so the client can react appropriately.
		c.Status(resp.StatusCode)
		c.Set("Content-Type", "application/vnd.mapbox-vector-tile")
		if cc := resp.Header.Get("Cache-Control"); cc != "" {
			c.Set("Cache-Control", cc)
		} else {
			c.Set("Cache-Control", "public, max-age=300")
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.SendStatus(fiber.StatusBadGateway)
		}
		return c.Send(body)
	}
}

// GET /api/tiles/metadata
func tileMetadataHandler(c fiber.Ctx) error { return notImplemented(c) }

// GET /api/styles/{name}.json — returns a MapLibre style document.
//
// Without a region the response is a bare background + metadata so
// MapLibre can mount cleanly before tiles land. When `?region=<slug>`
// is supplied, the style expands into the full basemap built by
// buildRegionStyle (see mapstyle.go): landcover, landuse, water,
// boundaries, roads (per-class casing + fill pairs), buildings, water
// names, road names, place labels, POI icons, and POI labels.
// Layer source-layer names follow the planetiler `basemap` profile
// schema (water, landcover, landuse, transportation, transportation_name,
// building, boundary, park, place, poi, water_name).
//
// The two canonical names are `light` and `dark`; any other name
// falls back to light. `region` + `lang` query params are accepted
// but currently only `region` influences the output (selects which
// tile source to reference when present).
func styleHandler(c fiber.Ctx) error {
	name := c.Params("name")
	if name != "light" && name != "dark" {
		name = "light"
	}
	region := c.Query("region")

	palette := lightPalette
	if name == "dark" {
		palette = darkPalette
	}

	var style map[string]any
	if region == "" {
		// Placeholder style — just the background so MapLibre mounts
		// without errors before a region is installed. Glyphs + sprites
		// are intentionally omitted because there are no text/symbol
		// layers that would trigger the fetch.
		style = map[string]any{
			"version": 8,
			"name":    "LocalMaps " + name,
			"sources": map[string]any{},
			"layers": []any{
				map[string]any{
					"id":    "background",
					"type":  "background",
					"paint": map[string]any{"background-color": palette.background},
				},
			},
			"metadata": map[string]any{
				"localmaps:region":      region,
				"localmaps:labelColor":  palette.textStreet,
				"localmaps:placeholder": true,
			},
		}
	} else {
		style = buildRegionStyle(name, region, palette)
	}

	c.Set("Cache-Control", "public, max-age=60")
	return c.JSON(style)
}

// GET /api/sprites/{name}.{ext} and /api/sprites/{name}@{density}.{ext}
//
// MapLibre fetches sprite atlases as two files each — a JSON index +
// PNG image — optionally at @2x density for high-DPI displays. We serve
// both variants of the pre-built Maki atlas (see internal/sprites).
//
// Fiber's path parser treats only /, -, . as segment delimiters, so
// the `@density` fragment has to be split manually from the :name param
// rather than relying on `:name@:density.:ext` syntax (which would swallow
// the `@` into the name).
func spriteHandler(c fiber.Ctx) error {
	raw := c.Params("name") // e.g. "default" or "default@2x"
	ext := c.Params("ext")  // "json" or "png"
	if ext != "json" && ext != "png" {
		return c.SendStatus(fiber.StatusNotFound)
	}
	name := raw
	density := ""
	if i := strings.LastIndexByte(raw, '@'); i >= 0 {
		name = raw[:i]
		density = raw[i+1:]
	}
	sp, ok := sprites.Lookup(name, density)
	if !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	if ext == "json" {
		c.Set("Content-Type", "application/json")
		return c.Send(sp.JSON)
	}
	c.Set("Content-Type", "image/png")
	return c.Send(sp.PNG)
}

// GET /api/glyphs/{fontstack}/{range}.pbf
//
// Serves pre-built MapLibre glyph PBFs from the embedded `glyphs` package.
// MapLibre fetches one PBF per 256-codepoint Unicode range as it encounters
// text in the style; each response is immutable and heavily cached.
//
// The `{fontstack}` param is URL-escaped by MapLibre (spaces become %20)
// and may be a comma-separated fallback list. We unescape once and let
// glyphs.Lookup pick the first matching font. Unknown fonts / ranges
// return 404 so MapLibre can fall back gracefully.
func glyphHandler(c fiber.Ctx) error {
	fontstack, err := url.PathUnescape(c.Params("fontstack"))
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	rng := c.Params("range") // e.g. "0-255"
	data, ok := glyphs.Lookup(fontstack, rng)
	if !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	c.Set("Content-Type", "application/x-protobuf")
	c.Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.Send(data)
}
