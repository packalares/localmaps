package api

// Handlers for the `map` tag in contracts/openapi.yaml.

import (
	"github.com/gofiber/fiber/v3"
)

// GET /api/tiles/{z}/{x}/{y}.pbf
func tileHandler(c fiber.Ctx) error { return notImplemented(c) }

// GET /api/tiles/metadata
func tileMetadataHandler(c fiber.Ctx) error { return notImplemented(c) }

// GET /api/styles/{name}.json — returns a minimal valid MapLibre style.
//
// Until a region's pmtiles land, the style is a plain background +
// label placeholder so MapLibre can mount cleanly instead of erroring
// at 501. Once a region is installed, the gateway swaps in a source
// pointing at `/api/tiles/{region}/map.pmtiles`.
//
// The two canonical names are `light` and `dark`; any other name
// falls back to light. `region` + `lang` query params are accepted
// but currently only `region` influences the output (selects which
// pmtiles to reference when present).
func styleHandler(c fiber.Ctx) error {
	name := c.Params("name")
	if name != "light" && name != "dark" {
		name = "light"
	}
	region := c.Query("region")

	bg := "#f8f6f2" // warm light background
	labelColor := "#4a4a4a"
	if name == "dark" {
		bg = "#17181b"
		labelColor = "#c8cbd1"
	}

	// Glyphs + sprites handlers are still stubs (501). Omit `glyphs`
	// and `sprite` entries from the style — MapLibre only fetches them
	// lazily when rendering text or symbol layers, and our minimal
	// background-only style has neither. Add them back when the
	// handlers are real (needed once tile data lands).
	style := map[string]any{
		"version": 8,
		"name":    "LocalMaps " + name,
		"sources": map[string]any{},
		"layers": []any{
			map[string]any{
				"id":    "background",
				"type":  "background",
				"paint": map[string]any{"background-color": bg},
			},
		},
		"metadata": map[string]any{
			"localmaps:region":      region,
			"localmaps:labelColor":  labelColor,
			"localmaps:placeholder": region == "",
		},
	}

	// If a region is supplied, add a pmtiles source + a minimal
	// layer stack so MapLibre shows something useful once tiles
	// land. When no pmtiles exist yet the source URL resolves to a
	// 404 — MapLibre handles that gracefully by rendering background.
	if region != "" {
		style["sources"] = map[string]any{
			"protomaps": map[string]any{
				"type":        "vector",
				"url":         "pmtiles:///api/tiles/" + region + "/map.pmtiles",
				"attribution": "© OpenStreetMap contributors, Overture Maps",
			},
		}
	}

	c.Set("Cache-Control", "public, max-age=60")
	return c.JSON(style)
}

// GET /api/sprites/{name}@{density}.{ext}
func spriteHandler(c fiber.Ctx) error { return notImplemented(c) }

// GET /api/glyphs/{fontstack}/{range}.pbf
func glyphHandler(c fiber.Ctx) error { return notImplemented(c) }
