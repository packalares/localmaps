package api

// MapLibre style-layer builders for styleHandler (see map.go).
//
// Extracted from map.go because the layer list is big (~35 layers)
// and inlining every map literal into the handler drowns the routing
// logic. Helpers return `[]any` slices in render order (bottom→top),
// which the handler concatenates into the final style document.
//
// All colour / width / zoom decisions live in this file. Adjust the
// lightPalette / darkPalette constants below to retint both themes.

// mapPalette captures every colour referenced by the vector layers,
// plus the text colour family. Fill/casing pairs are grouped per road
// class so tweaks are local. Nothing here is per-region — the palette
// depends only on the style name (light vs dark).
type mapPalette struct {
	background string

	water        string
	waterway     string
	waterOutline string

	parkFill     string
	woodFill     string
	grassFill    string
	sandFill     string
	cemeteryFill string

	landuseResidential string
	landuseCommercial  string
	landuseIndustrial  string
	landuseFarmland    string

	buildingFill    string
	buildingOutline string

	boundaryCountry string
	boundaryState   string

	// Road palette — fill on top, casing underneath.
	motorwayFill, motorwayCasing   string
	trunkFill, trunkCasing         string
	primaryFill, primaryCasing     string
	secondaryFill, secondaryCasing string
	tertiaryFill, tertiaryCasing   string
	minorFill, minorCasing         string
	serviceFill, serviceCasing     string
	pathLine                       string
	railLine                       string

	// Labels.
	textCountry       string
	textCity          string
	textTown          string
	textVillage       string
	textNeighbourhood string
	textStreet        string
	textWater         string
	textPOI           string
	textHalo          string
	textHaloWater     string

	// POI icon tints (applied as `icon-color`; requires SDF sprites).
	poiFood          string
	poiShopping      string
	poiTransit       string
	poiLodging       string
	poiServices      string
	poiHealthcare    string
	poiEntertainment string
	poiEducation     string
	poiOther         string
}

// lightPalette mirrors the Google Maps light theme (see docs/color-palette.md).
var lightPalette = mapPalette{
	background: "#f5f3ee",

	water:        "#aee0f4",
	waterway:     "#aee0f4",
	waterOutline: "#8fc5ef",

	parkFill:     "#c8e6c9",
	woodFill:     "#b7e1b2",
	grassFill:    "#c8e6c9",
	sandFill:     "#efe3c5",
	cemeteryFill: "#dae6d0",

	landuseResidential: "#efebe0",
	landuseCommercial:  "#f1e8d4",
	landuseIndustrial:  "#e8e4d8",
	landuseFarmland:    "#f0ead0",

	buildingFill:    "#e8e3d6",
	buildingOutline: "#d9d2c0",

	boundaryCountry: "#9aa0a6",
	boundaryState:   "#b0a8a0",

	motorwayFill:    "#f4f2ec",
	motorwayCasing:  "#c7c2b8",
	trunkFill:       "#f8f6f0",
	trunkCasing:     "#cfc9bd",
	primaryFill:     "#ffffff",
	primaryCasing:   "#d6d0c8",
	secondaryFill:   "#ffffff",
	secondaryCasing: "#dbd5cc",
	tertiaryFill:    "#ffffff",
	tertiaryCasing:  "#e1dcd2",
	minorFill:       "#ffffff",
	minorCasing:     "#e6e1d8",
	serviceFill:     "#fbfaf7",
	serviceCasing:   "#ebe6dd",
	pathLine:        "#b39a78",
	railLine:        "#9c978d",

	textCountry:       "#5f6368",
	textCity:          "#5f6368",
	textTown:          "#5f6368",
	textVillage:       "#6b7074",
	textNeighbourhood: "#818589",
	textStreet:        "#5f6368",
	textWater:         "#4c7a94",
	textPOI:           "#5f6368",
	textHalo:          "#ffffff",
	textHaloWater:     "#c4e0f5",

	poiFood:          "#e8833a",
	poiShopping:      "#3367d6",
	poiTransit:       "#1a73e8",
	poiLodging:       "#c2185b",
	poiServices:      "#7b3ff2",
	poiHealthcare:    "#d93025",
	poiEntertainment: "#11a683",
	poiEducation:     "#f2a100",
	poiOther:         "#5f6368",
}

// darkPalette keeps the layer set identical but retints for dark mode.
// Values drawn from docs/color-palette.md §4; water is bumped to a slightly
// more saturated navy and parks to a marginally bluer green per the spec.
var darkPalette = mapPalette{
	background: "#1a1c20",

	water:        "#24495e",
	waterway:     "#1e4160",
	waterOutline: "#0f2c40",

	parkFill:     "#1e2d26",
	woodFill:     "#1a2618",
	grassFill:    "#222d22",
	sandFill:     "#2b2a21",
	cemeteryFill: "#252b23",

	landuseResidential: "#22252b",
	landuseCommercial:  "#2b2722",
	landuseIndustrial:  "#1f2329",
	landuseFarmland:    "#232a1f",

	buildingFill:    "#2d3036",
	buildingOutline: "#40434a",

	boundaryCountry: "#8b8e94",
	boundaryState:   "#636870",

	motorwayFill:    "#6d5a42",
	motorwayCasing:  "#2a2620",
	trunkFill:       "#5a4e3e",
	trunkCasing:     "#2a2620",
	primaryFill:     "#434852",
	primaryCasing:   "#23262c",
	secondaryFill:   "#3e424a",
	secondaryCasing: "#23262c",
	tertiaryFill:    "#3a3e46",
	tertiaryCasing:  "#23262c",
	minorFill:       "#31343b",
	minorCasing:     "#23262c",
	serviceFill:     "#2c2f36",
	serviceCasing:   "#23262c",
	pathLine:        "#6a6560",
	railLine:        "#5a5a5a",

	textCountry:       "#bdc1c6",
	textCity:          "#e8eaed",
	textTown:          "#bdc1c6",
	textVillage:       "#9aa0a6",
	textNeighbourhood: "#80868b",
	textStreet:        "#bdc1c6",
	textWater:         "#8ab4f8",
	textPOI:           "#bdc1c6",
	textHalo:          "#1a1c20",
	textHaloWater:     "#24495e",

	poiFood:          "#f4a15b",
	poiShopping:      "#6b9bf0",
	poiTransit:       "#7ba7f0",
	poiLodging:       "#ec4888",
	poiServices:      "#a479f7",
	poiHealthcare:    "#f28b7b",
	poiEntertainment: "#4ecbae",
	poiEducation:     "#f9c055",
	poiOther:         "#9aa0a6",
}

// nameField is the multi-language text-field expression: English first,
// fallback to the local name. Planetiler's basemap profile exposes both
// `name:en` (OSM tag) and `name_en` (normalised) depending on source.
func nameField() []any {
	return []any{"coalesce", []any{"get", "name:en"}, []any{"get", "name_en"}, []any{"get", "name"}}
}

// interp is a tiny convenience wrapper around MapLibre's `interpolate`
// expression. stops is a flat list of zoom, value, zoom, value...
func interp(stops ...any) []any {
	return append([]any{"interpolate", []any{"linear"}, []any{"zoom"}}, stops...)
}

// landLayers emits polygon fills sitting between the background and
// water: landcover (natural), landuse (anthropogenic), park.
func landLayers(p mapPalette) []any {
	return []any{
		// Landcover — natural surfaces. One layer per subclass so each
		// gets its own tint; MapLibre paints them in order.
		map[string]any{
			"id": "landcover-wood", "type": "fill", "source": "protomaps", "source-layer": "landcover",
			"filter": []any{"in", "subclass", "wood", "forest"},
			"paint":  map[string]any{"fill-color": p.woodFill, "fill-opacity": 0.75},
		},
		map[string]any{
			"id": "landcover-grass", "type": "fill", "source": "protomaps", "source-layer": "landcover",
			"filter": []any{"in", "subclass", "grass", "meadow", "heath", "scrub"},
			"paint":  map[string]any{"fill-color": p.grassFill, "fill-opacity": 0.6},
		},
		map[string]any{
			"id": "landcover-sand", "type": "fill", "source": "protomaps", "source-layer": "landcover",
			"filter": []any{"in", "subclass", "sand", "beach", "glacier"},
			"paint":  map[string]any{"fill-color": p.sandFill, "fill-opacity": 0.7},
		},

		// Landuse — anthropogenic polygons.
		map[string]any{
			"id": "landuse-residential", "type": "fill", "source": "protomaps", "source-layer": "landuse",
			"filter": []any{"==", "class", "residential"},
			"paint":  map[string]any{"fill-color": p.landuseResidential, "fill-opacity": 0.7},
		},
		map[string]any{
			"id": "landuse-commercial", "type": "fill", "source": "protomaps", "source-layer": "landuse",
			"filter": []any{"in", "class", "commercial", "retail"},
			"paint":  map[string]any{"fill-color": p.landuseCommercial, "fill-opacity": 0.7},
		},
		map[string]any{
			"id": "landuse-industrial", "type": "fill", "source": "protomaps", "source-layer": "landuse",
			"filter": []any{"==", "class", "industrial"},
			"paint":  map[string]any{"fill-color": p.landuseIndustrial, "fill-opacity": 0.7},
		},
		map[string]any{
			"id": "landuse-cemetery", "type": "fill", "source": "protomaps", "source-layer": "landuse",
			"filter": []any{"==", "class", "cemetery"},
			"paint":  map[string]any{"fill-color": p.cemeteryFill, "fill-opacity": 0.7},
		},
		map[string]any{
			"id": "landuse-farmland", "type": "fill", "source": "protomaps", "source-layer": "landuse",
			"filter": []any{"in", "class", "farmland", "farmyard", "orchard", "vineyard"},
			"paint":  map[string]any{"fill-color": p.landuseFarmland, "fill-opacity": 0.7},
		},

		// Parks sit on top of generic landuse so they look lush.
		map[string]any{
			"id": "park", "type": "fill", "source": "protomaps", "source-layer": "park",
			"paint": map[string]any{"fill-color": p.parkFill, "fill-opacity": 0.75},
		},
	}
}

// waterLayers covers oceans/lakes (polygons) + streams/canals (lines).
func waterLayers(p mapPalette) []any {
	return []any{
		map[string]any{
			"id": "water", "type": "fill", "source": "protomaps", "source-layer": "water",
			"paint": map[string]any{"fill-color": p.water},
		},
		map[string]any{
			"id": "waterway", "type": "line", "source": "protomaps", "source-layer": "waterway",
			"minzoom": 8,
			"paint": map[string]any{
				"line-color": p.waterway,
				"line-width": interp(8, 0.4, 14, 1.5, 18, 3),
			},
		},
	}
}

// boundaryLayers renders country boundaries solid, state/province dashed.
// `admin_level` in planetiler's basemap profile is an integer tag on the
// `boundary` source-layer.
func boundaryLayers(p mapPalette) []any {
	return []any{
		map[string]any{
			"id": "boundary-country", "type": "line", "source": "protomaps", "source-layer": "boundary",
			"filter": []any{"<=", "admin_level", 2},
			"paint": map[string]any{
				"line-color":   p.boundaryCountry,
				"line-width":   interp(2, 0.6, 10, 1.8),
				"line-opacity": 0.9,
			},
		},
		map[string]any{
			"id": "boundary-state", "type": "line", "source": "protomaps", "source-layer": "boundary",
			"filter": []any{"all", []any{">", "admin_level", 2}, []any{"<=", "admin_level", 4}},
			"paint": map[string]any{
				"line-color":     p.boundaryState,
				"line-width":     interp(3, 0.4, 10, 1.2),
				"line-dasharray": []any{2, 2},
				"line-opacity":   0.7,
			},
		},
	}
}

// roadLayers emits (casing, fill) line pairs per road class. Order:
// paint minor casings first so the major fills overwrite at junctions.
// Within a class, casing goes immediately before fill because MapLibre
// draws later layers on top.
func roadLayers(p mapPalette) []any {
	// Helper: one road-class layer entry.
	layer := func(id, class string, minzoom int, casing bool, color string, widthStops ...any) map[string]any {
		l := map[string]any{
			"id": id, "type": "line", "source": "protomaps", "source-layer": "transportation",
			"filter":  []any{"==", "class", class},
			"minzoom": minzoom,
			"layout": map[string]any{
				"line-cap":  "round",
				"line-join": "round",
			},
			"paint": map[string]any{
				"line-color": color,
				"line-width": interp(widthStops...),
			},
		}
		_ = casing // flag kept for readability at call sites
		return l
	}
	return []any{
		// Rail first — thin, dashed-style, beneath roads.
		map[string]any{
			"id": "rail", "type": "line", "source": "protomaps", "source-layer": "transportation",
			"filter":  []any{"==", "class", "rail"},
			"minzoom": 10,
			"paint": map[string]any{
				"line-color":     p.railLine,
				"line-width":     interp(10, 0.5, 16, 1.5),
				"line-dasharray": []any{3, 2},
			},
		},

		// Paths / tracks / service — fine lines, no casing.
		map[string]any{
			"id": "road-path", "type": "line", "source": "protomaps", "source-layer": "transportation",
			"filter":  []any{"in", "class", "path", "track"},
			"minzoom": 14,
			"paint": map[string]any{
				"line-color":     p.pathLine,
				"line-width":     interp(14, 0.5, 18, 1.5),
				"line-dasharray": []any{2, 2},
			},
		},

		// Casings — drawn bottom up (minor first so major casings
		// overlap at junctions).
		layer("road-service-casing", "service", 14, true, p.serviceCasing, 14, 1.0, 18, 5),
		layer("road-minor-casing", "minor", 12, true, p.minorCasing, 12, 1.0, 18, 8),
		layer("road-tertiary-casing", "tertiary", 11, true, p.tertiaryCasing, 11, 1.0, 18, 12),
		layer("road-secondary-casing", "secondary", 9, true, p.secondaryCasing, 9, 0.8, 18, 16),
		layer("road-primary-casing", "primary", 7, true, p.primaryCasing, 7, 0.8, 18, 20),
		layer("road-trunk-casing", "trunk", 5, true, p.trunkCasing, 5, 0.6, 18, 22),
		layer("road-motorway-casing", "motorway", 4, true, p.motorwayCasing, 4, 0.6, 18, 24),

		// Fills — same stacking order so each road class's white/coloured
		// body sits on top of its own casing.
		layer("road-service", "service", 14, false, p.serviceFill, 14, 0.4, 18, 3),
		layer("road-minor", "minor", 12, false, p.minorFill, 12, 0.5, 18, 6),
		layer("road-tertiary", "tertiary", 11, false, p.tertiaryFill, 11, 0.6, 18, 9),
		layer("road-secondary", "secondary", 9, false, p.secondaryFill, 9, 0.5, 18, 12),
		layer("road-primary", "primary", 7, false, p.primaryFill, 7, 0.5, 18, 15),
		layer("road-trunk", "trunk", 5, false, p.trunkFill, 5, 0.3, 18, 16),
		layer("road-motorway", "motorway", 4, false, p.motorwayFill, 4, 0.3, 18, 18),
	}
}

// buildingLayer is a single polygon fill (MapLibre style-spec doesn't
// support a separate outline in one layer without fill-extrusion).
func buildingLayer(p mapPalette) []any {
	return []any{
		map[string]any{
			"id": "building", "type": "fill", "source": "protomaps", "source-layer": "building",
			"minzoom": 14,
			"paint": map[string]any{
				"fill-color":         p.buildingFill,
				"fill-outline-color": p.buildingOutline,
				"fill-opacity":       interp(14, 0, 16, 0.9),
			},
		},
	}
}

// roadNameLayers renders street name labels placed along the road lines.
func roadNameLayers(p mapPalette) []any {
	return []any{
		map[string]any{
			"id": "road-name-major", "type": "symbol", "source": "protomaps", "source-layer": "transportation_name",
			"filter":  []any{"in", "class", "motorway", "trunk", "primary", "secondary"},
			"minzoom": 11,
			"layout": map[string]any{
				"symbol-placement":    "line",
				"text-field":          nameField(),
				"text-font":           []any{"Noto Sans Regular"},
				"text-size":           interp(11, 10, 18, 14),
				"text-letter-spacing": 0.05,
			},
			"paint": map[string]any{
				"text-color":      p.textStreet,
				"text-halo-color": p.textHalo,
				"text-halo-width": 1.5,
			},
		},
		map[string]any{
			"id": "road-name-minor", "type": "symbol", "source": "protomaps", "source-layer": "transportation_name",
			"filter":  []any{"in", "class", "tertiary", "minor"},
			"minzoom": 13,
			"layout": map[string]any{
				"symbol-placement":    "line",
				"text-field":          nameField(),
				"text-font":           []any{"Noto Sans Regular"},
				"text-size":           interp(13, 10, 18, 13),
				"text-letter-spacing": 0.03,
			},
			"paint": map[string]any{
				"text-color":      p.textStreet,
				"text-halo-color": p.textHalo,
				"text-halo-width": 1.5,
			},
		},
	}
}

// waterNameLayers italic labels for rivers / lakes.
func waterNameLayers(p mapPalette) []any {
	return []any{
		map[string]any{
			"id": "water-name", "type": "symbol", "source": "protomaps", "source-layer": "water_name",
			"minzoom": 9,
			"layout": map[string]any{
				"text-field": nameField(),
				"text-font":  []any{"Noto Sans Italic"},
				"text-size":  interp(9, 10, 16, 14),
			},
			"paint": map[string]any{
				"text-color":      p.textWater,
				"text-halo-color": p.textHaloWater,
				"text-halo-width": 1.5,
			},
		},
	}
}

// placeLayers emits city / town / village / neighbourhood labels. Uses
// `class` on the `place` source-layer; `rank` biases size.
func placeLayers(p mapPalette) []any {
	return []any{
		map[string]any{
			"id": "place-country", "type": "symbol", "source": "protomaps", "source-layer": "place",
			"filter":  []any{"==", "class", "country"},
			"minzoom": 1,
			"maxzoom": 5,
			"layout": map[string]any{
				"text-field":          nameField(),
				"text-font":           []any{"Noto Sans Bold"},
				"text-size":           interp(1, 10, 5, 18),
				"text-letter-spacing": 0.12,
				"text-transform":      "uppercase",
			},
			"paint": map[string]any{
				"text-color":      p.textCountry,
				"text-halo-color": p.textHalo,
				"text-halo-width": 2,
			},
		},
		map[string]any{
			"id": "place-city", "type": "symbol", "source": "protomaps", "source-layer": "place",
			"filter":  []any{"==", "class", "city"},
			"minzoom": 3,
			"maxzoom": 12,
			"layout": map[string]any{
				"text-field": nameField(),
				"text-font":  []any{"Noto Sans Bold"},
				"text-size":  interp(3, 11, 8, 16, 12, 22),
			},
			"paint": map[string]any{
				"text-color":      p.textCity,
				"text-halo-color": p.textHalo,
				"text-halo-width": 2,
			},
		},
		map[string]any{
			"id": "place-town", "type": "symbol", "source": "protomaps", "source-layer": "place",
			"filter":  []any{"==", "class", "town"},
			"minzoom": 7,
			"maxzoom": 15,
			"layout": map[string]any{
				"text-field": nameField(),
				"text-font":  []any{"Noto Sans Regular"},
				"text-size":  interp(7, 10, 14, 15),
			},
			"paint": map[string]any{
				"text-color":      p.textTown,
				"text-halo-color": p.textHalo,
				"text-halo-width": 1.5,
			},
		},
		map[string]any{
			"id": "place-village", "type": "symbol", "source": "protomaps", "source-layer": "place",
			"filter":  []any{"==", "class", "village"},
			"minzoom": 10,
			"maxzoom": 17,
			"layout": map[string]any{
				"text-field": nameField(),
				"text-font":  []any{"Noto Sans Regular"},
				"text-size":  interp(10, 10, 16, 13),
			},
			"paint": map[string]any{
				"text-color":      p.textVillage,
				"text-halo-color": p.textHalo,
				"text-halo-width": 1.5,
			},
		},
		map[string]any{
			"id": "place-suburb", "type": "symbol", "source": "protomaps", "source-layer": "place",
			"filter":  []any{"in", "class", "suburb", "quarter"},
			"minzoom": 11,
			"layout": map[string]any{
				"text-field":          nameField(),
				"text-font":           []any{"Noto Sans Regular"},
				"text-size":           interp(11, 10, 18, 13),
				"text-letter-spacing": 0.08,
				"text-transform":      "uppercase",
			},
			"paint": map[string]any{
				"text-color":      p.textNeighbourhood,
				"text-halo-color": p.textHalo,
				"text-halo-width": 1.5,
			},
		},
		map[string]any{
			"id": "place-neighbourhood", "type": "symbol", "source": "protomaps", "source-layer": "place",
			"filter":  []any{"in", "class", "neighbourhood", "hamlet"},
			"minzoom": 12,
			"layout": map[string]any{
				"text-field":          nameField(),
				"text-font":           []any{"Noto Sans Regular"},
				"text-size":           interp(12, 9, 18, 12),
				"text-letter-spacing": 0.08,
				"text-transform":      "uppercase",
			},
			"paint": map[string]any{
				"text-color":      p.textNeighbourhood,
				"text-halo-color": p.textHalo,
				"text-halo-width": 1.5,
			},
		},
	}
}

// poiCategory groups OSM POI class/subclass values into a single toggle-able
// layer. Layer IDs (`poi-<name>`) are stable — the F3 agent uses them to
// drive icon visibility from the UI. Keep ids + slug names in sync with
// the UI's category picker.
type poiCategory struct {
	id       string // stable layer id, e.g. "poi-food"
	filterIn []any  // `in` filter values (matched against `class`)
	color    func(mapPalette) string
	icon     string // sprite name (kept for reference; glyph drives display)
	glyph    string // single-char text label rendered on top of the dot
}

// poiCategories lists every category layer we emit, in paint order.
// Each entry becomes one icon layer + one paired label layer, both
// sharing the same filter — the UI toggles them as a pair by id prefix.
// The `color` accessor resolves the per-category `icon-color` override
// from the active palette (see §6 of docs/color-palette.md).
func poiCategories() []poiCategory {
	return []poiCategory{
		{id: "poi-food", icon: "restaurant", glyph: "F", color: func(p mapPalette) string { return p.poiFood }, filterIn: []any{"in", "class", "restaurant", "fast_food", "cafe", "bar", "pub", "food_court", "ice_cream"}},
		{id: "poi-shopping", icon: "shop", glyph: "S", color: func(p mapPalette) string { return p.poiShopping }, filterIn: []any{"in", "class", "shop", "mall", "supermarket", "convenience", "marketplace", "department_store", "grocery"}},
		{id: "poi-transit", icon: "bus", glyph: "T", color: func(p mapPalette) string { return p.poiTransit }, filterIn: []any{"in", "class", "bus", "rail", "ferry", "aerialway", "airport", "subway", "tram", "tram_stop", "bus_stop"}},
		{id: "poi-lodging", icon: "lodging", glyph: "H", color: func(p mapPalette) string { return p.poiLodging }, filterIn: []any{"in", "class", "hotel", "hostel", "motel", "guest_house", "lodging"}},
		{id: "poi-services", icon: "bank", glyph: "$", color: func(p mapPalette) string { return p.poiServices }, filterIn: []any{"in", "class", "bank", "atm", "post_office", "fuel", "police", "fire_station", "townhall", "embassy"}},
		{id: "poi-healthcare", icon: "hospital", glyph: "+", color: func(p mapPalette) string { return p.poiHealthcare }, filterIn: []any{"in", "class", "hospital", "clinic", "pharmacy", "doctors", "dentist", "veterinary"}},
		{id: "poi-entertainment", icon: "attraction", glyph: "★", color: func(p mapPalette) string { return p.poiEntertainment }, filterIn: []any{"in", "class", "cinema", "theatre", "museum", "stadium", "zoo", "attraction", "arts_centre", "gallery", "nightclub"}},
		{id: "poi-education", icon: "school", glyph: "A", color: func(p mapPalette) string { return p.poiEducation }, filterIn: []any{"in", "class", "school", "university", "college", "kindergarten", "library"}},
		{id: "poi-other", icon: "park", glyph: "•", color: func(p mapPalette) string { return p.poiOther }, filterIn: []any{"in", "class", "park", "playground", "place_of_worship", "cemetery", "toilet", "drinking_water", "information"}},
	}
}

// poiLayers returns the icon layers for every poiCategory. Icon image
// picks subclass first (Maki names match OSM subclass for most types),
// falling back to class, then to the generic "marker" sprite.
//
// Per-category `icon-color` overrides come from the palette (§6 of
// docs/color-palette.md). They only apply when the sprite is shipped
// as SDF (monochrome mask); with a full-colour PNG sprite the hint is
// ignored by MapLibre, so keeping them here is forward-compatible and
// harmless on today's sprite pack.
func poiLayers(p mapPalette) []any {
	out := make([]any, 0, 2*len(poiCategories()))
	for _, c := range poiCategories() {
		// Neutral grey backdrop matching Google Maps' default POI
		// chips — a small gray dot with a thin white border. Per-
		// category colour is dropped here; the white pictogram on
		// top is the differentiator.
		out = append(out, map[string]any{
			"id": c.id + "-bg", "type": "circle", "source": "protomaps", "source-layer": "poi",
			"filter":  c.filterIn,
			"minzoom": 14,
			"paint": map[string]any{
				"circle-radius":       interp(14, 7, 18, 10),
				"circle-color":        "#5f6368",
				"circle-stroke-color": "#ffffff",
				"circle-stroke-width": 1.2,
			},
		})
		// White pictogram centred on top of the dot. The icon image
		// (`lm-poi-<key>`) is registered at runtime by the UI from
		// the chip-row SVG paths via map.addImage(..., {sdf:true}) —
		// guaranteed identical visual to the chip markers, no
		// dependency on the bundled Maki sprite.
		out = append(out, map[string]any{
			"id": c.id, "type": "symbol", "source": "protomaps", "source-layer": "poi",
			"filter":  c.filterIn,
			"minzoom": 14,
			"layout": map[string]any{
				"icon-image":            "lm-" + c.id,
				"icon-size":             interp(14, 0.55, 18, 0.75),
				"icon-allow-overlap":    true,
				"icon-ignore-placement": true,
			},
			"paint": map[string]any{
				"icon-color": "#ffffff",
			},
		})
	}
	return out
}

// poiLabelLayers returns one label layer per poiCategory, painted above
// the icons. Label id uses `<cat-id>-label` so the UI can still target
// the icon and label together via id-prefix matching.
func poiLabelLayers(p mapPalette) []any {
	out := make([]any, 0, len(poiCategories()))
	for _, c := range poiCategories() {
		out = append(out, map[string]any{
			"id": c.id + "-label", "type": "symbol", "source": "protomaps", "source-layer": "poi",
			"filter":  c.filterIn,
			"minzoom": 16,
			"layout": map[string]any{
				"text-field":     nameField(),
				"text-font":      []any{"Noto Sans Regular"},
				"text-size":      interp(16, 10, 18, 12),
				"text-offset":    []any{0, 1.1},
				"text-anchor":    "top",
				"text-max-width": 8,
				"text-optional":  true,
			},
			"paint": map[string]any{
				"text-color":      p.textPOI,
				"text-halo-color": p.textHalo,
				"text-halo-width": 1.5,
			},
		})
	}
	return out
}

// buildRegionStyle assembles the full MapLibre style doc for a region.
// Layer order (bottom→top): background → land → water → boundaries →
// roads (rail, paths, casings, fills) → buildings → water names →
// road names → place labels → POI icons → POI labels. This matches
// the stacking described in map.go's doc comment.
func buildRegionStyle(name, region string, p mapPalette) map[string]any {
	layers := []any{
		map[string]any{
			"id":    "background",
			"type":  "background",
			"paint": map[string]any{"background-color": p.background},
		},
	}
	layers = append(layers, landLayers(p)...)
	layers = append(layers, waterLayers(p)...)
	layers = append(layers, boundaryLayers(p)...)
	layers = append(layers, roadLayers(p)...)
	layers = append(layers, buildingLayer(p)...)
	layers = append(layers, waterNameLayers(p)...)
	layers = append(layers, roadNameLayers(p)...)
	layers = append(layers, placeLayers(p)...)
	layers = append(layers, poiLayers(p)...)
	layers = append(layers, poiLabelLayers(p)...)

	return map[string]any{
		"version": 8,
		"name":    "LocalMaps " + name,
		"glyphs":  "/api/glyphs/{fontstack}/{range}.pbf",
		"sprite":  "/api/sprites/default",
		"sources": map[string]any{
			"protomaps": map[string]any{
				"type":        "vector",
				"tiles":       []any{"/api/tiles/" + region + "/{z}/{x}/{y}.pbf"},
				"scheme":      "xyz",
				"minzoom":     0,
				"maxzoom":     14,
				"attribution": "© OpenStreetMap contributors",
			},
		},
		"layers": layers,
		"metadata": map[string]any{
			"localmaps:region":      region,
			"localmaps:labelColor":  p.textStreet,
			"localmaps:placeholder": false,
		},
	}
}
