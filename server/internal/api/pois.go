package api

// Handlers for the `pois` tag in contracts/openapi.yaml. Query + single
// fetch proxy to pelias-api via geocodingClient; the categories endpoint
// returns a static OSM-derived taxonomy.

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/geocoding"
)

// poiCategoryTree is the hardcoded OSM-rollup taxonomy surfaced by
// GET /api/pois/categories. Structure matches PoiCategory in
// openapi.yaml. Mutable slices are avoided — this literal is shared
// across goroutines in the response envelope.
var poiCategoryTree = []fiber.Map{
	{"id": "food", "label": "Food & Drink", "children": []fiber.Map{
		{"id": "restaurant", "label": "Restaurant"},
		{"id": "cafe", "label": "Cafe"},
		{"id": "bar", "label": "Bar"},
		{"id": "fast_food", "label": "Fast Food"},
	}},
	{"id": "shopping", "label": "Shopping", "children": []fiber.Map{
		{"id": "supermarket", "label": "Supermarket"},
		{"id": "mall", "label": "Mall"},
		{"id": "convenience", "label": "Convenience"},
	}},
	{"id": "entertainment", "label": "Entertainment", "children": []fiber.Map{
		{"id": "cinema", "label": "Cinema"},
		{"id": "theatre", "label": "Theatre"},
		{"id": "museum", "label": "Museum"},
		{"id": "park", "label": "Park"},
	}},
	{"id": "transit", "label": "Transit", "children": []fiber.Map{
		{"id": "bus_station", "label": "Bus Station"},
		{"id": "train_station", "label": "Train Station"},
		{"id": "airport", "label": "Airport"},
	}},
	{"id": "healthcare", "label": "Healthcare", "children": []fiber.Map{
		{"id": "hospital", "label": "Hospital"},
		{"id": "clinic", "label": "Clinic"},
		{"id": "pharmacy", "label": "Pharmacy"},
	}},
	{"id": "lodging", "label": "Lodging", "children": []fiber.Map{
		{"id": "hotel", "label": "Hotel"},
		{"id": "hostel", "label": "Hostel"},
	}},
	{"id": "services", "label": "Services", "children": []fiber.Map{
		{"id": "bank", "label": "Bank"},
		{"id": "atm", "label": "ATM"},
		{"id": "post_office", "label": "Post Office"},
		{"id": "fuel", "label": "Fuel"},
	}},
}

// parseBBox splits a `minLon,minLat,maxLon,maxLat` query string into
// four pointers. Returns (nil…) on malformed input so the caller can
// skip the boundary.rect filter.
func parseBBox(raw string) (*float64, *float64, *float64, *float64) {
	if raw == "" {
		return nil, nil, nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return nil, nil, nil, nil
	}
	vals := make([]float64, 4)
	for i, s := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, nil, nil, nil
		}
		vals[i] = v
	}
	return &vals[0], &vals[1], &vals[2], &vals[3]
}

// GET /api/pois
func poisQueryHandler(c fiber.Ctx) error {
	if geocodingClient == nil {
		return notImplemented(c)
	}
	minLon, minLat, maxLon, maxLat := parseBBox(c.Query("bbox"))
	results, err := geocodingClient.QueryPois(c.Context(), geocoding.PoiQueryParams{
		BBoxMinLon: minLon,
		BBoxMinLat: minLat,
		BBoxMaxLon: maxLon,
		BBoxMaxLat: maxLat,
		Text:       c.Query("q"),
		Category:   c.Query("category"),
		Size:       parseSize(c, 50, 200),
	})
	if err != nil {
		return mapGeocodingErr(c, err)
	}
	return c.JSON(fiber.Map{"pois": results, "traceId": traceIDOrEmpty(c)})
}

// GET /api/pois/{id}
func poisGetHandler(c fiber.Ctx) error {
	if geocodingClient == nil {
		return notImplemented(c)
	}
	// Route uses `+` greedy capture (pelias gids contain slashes); read
	// from the wildcard param. Fall back to `:id` for legacy callers.
	id := c.Params("+1")
	if id == "" {
		id = c.Params("id")
	}
	if strings.TrimSpace(id) == "" {
		return apierr.Write(c, apierr.CodeBadRequest, "id is required", false)
	}
	poi, err := geocodingClient.Place(c.Context(), id)
	if err != nil {
		return mapGeocodingErr(c, err)
	}
	return c.JSON(poi)
}

// GET /api/pois/categories
func poisCategoriesHandler(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"categories": poiCategoryTree})
}
