package api

// Handlers for the `geocode` tag in contracts/openapi.yaml. Each is a
// thin adapter between the Fiber request cycle and
// server/internal/geocoding.Client (which speaks to pelias-api).

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/geocoding"
)

// geocodingClient is the process-wide pelias-api proxy. router.Register
// populates it via setGeocodingClientFromBoot; left nil, handlers keep
// their 501 stub behaviour so tests wiring api.Deps{} without a Boot
// still boot cleanly.
var geocodingClient *geocoding.Client

// setGeocodingClientFromBoot constructs a Client from the Boot config.
// Called once from router.Register; passes a nil Client when Boot is
// nil (phase-1 tests) so stubs stay 501. The pelias-es URL is wired
// through to power the bbox-only POI fallback path in geocoding.QueryPois.
func setGeocodingClientFromBoot(peliasURL, peliasESURL string) {
	if strings.TrimSpace(peliasURL) == "" {
		geocodingClient = nil
		return
	}
	geocodingClient = geocoding.NewClient(peliasURL)
	if strings.TrimSpace(peliasESURL) != "" {
		geocodingClient.SetESURL(peliasESURL)
	}
}

// mapGeocodingErr translates a geocoding.Client error into the apierr
// envelope. 4xx-from-upstream → BAD_REQUEST; anything else →
// UPSTREAM_UNAVAILABLE (retryable).
func mapGeocodingErr(c fiber.Ctx, err error) error {
	fmt.Fprintf(os.Stderr, "[geocode-err] upstream=%s err=%v\n",
		func() string {
			if geocodingClient == nil {
				return "<nil>"
			}
			return geocodingClient.BaseURL()
		}(), err)
	switch {
	case errors.Is(err, geocoding.ErrUpstreamBadRequest):
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	case errors.Is(err, geocoding.ErrNotFound):
		return apierr.Write(c, apierr.CodeNotFound, err.Error(), false)
	case errors.Is(err, geocoding.ErrUpstreamUnavailable):
		return apierr.Write(c, apierr.CodeUpstreamUnavailable,
			"geocoding upstream unavailable", true)
	default:
		return apierr.Write(c, apierr.CodeInternal, err.Error(), true)
	}
}

// parseFocus pulls the optional focus.lat / focus.lon query pair. Both
// must be present + numeric; any other combination returns nil, nil so
// we simply omit the upstream knob.
func parseFocus(c fiber.Ctx) (*float64, *float64) {
	latStr := c.Query("focus.lat")
	lonStr := c.Query("focus.lon")
	if latStr == "" || lonStr == "" {
		return nil, nil
	}
	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)
	if err1 != nil || err2 != nil {
		return nil, nil
	}
	return &lat, &lon
}

// parseSize reads `limit` (the OpenAPI knob) and clamps it to sane
// bounds. Zero means "use pelias-api's default".
func parseSize(c fiber.Ctx, def, maxV int) int {
	v, err := strconv.Atoi(c.Query("limit"))
	if err != nil || v <= 0 {
		return def
	}
	if v > maxV {
		return maxV
	}
	return v
}

// GET /api/geocode/autocomplete
func geocodeAutocompleteHandler(c fiber.Ctx) error {
	if geocodingClient == nil {
		return notImplemented(c)
	}
	text := strings.TrimSpace(c.Query("q"))
	if text == "" {
		return apierr.Write(c, apierr.CodeBadRequest, "q is required", false)
	}
	lat, lon := parseFocus(c)
	results, err := geocodingClient.Autocomplete(c.Context(), geocoding.AutocompleteParams{
		Text:     text,
		FocusLat: lat,
		FocusLon: lon,
		Size:     parseSize(c, 10, 50),
		Lang:     c.Query("lang"),
	})
	if err != nil {
		return mapGeocodingErr(c, err)
	}
	return c.JSON(fiber.Map{"results": results, "traceId": traceIDOrEmpty(c)})
}

// GET /api/geocode/search
func geocodeSearchHandler(c fiber.Ctx) error {
	if geocodingClient == nil {
		return notImplemented(c)
	}
	text := strings.TrimSpace(c.Query("q"))
	if text == "" {
		return apierr.Write(c, apierr.CodeBadRequest, "q is required", false)
	}
	lat, lon := parseFocus(c)
	results, err := geocodingClient.Search(c.Context(), geocoding.SearchParams{
		Text:     text,
		FocusLat: lat,
		FocusLon: lon,
		Size:     parseSize(c, 10, 50),
	})
	if err != nil {
		return mapGeocodingErr(c, err)
	}
	return c.JSON(fiber.Map{"results": results, "traceId": traceIDOrEmpty(c)})
}

// GET /api/geocode/reverse
func geocodeReverseHandler(c fiber.Ctx) error {
	if geocodingClient == nil {
		return notImplemented(c)
	}
	lat, err1 := strconv.ParseFloat(c.Query("lat"), 64)
	lon, err2 := strconv.ParseFloat(c.Query("lon"), 64)
	if err1 != nil || err2 != nil {
		return apierr.Write(c, apierr.CodeBadRequest,
			"lat and lon query params are required and must be numeric", false)
	}
	result, err := geocodingClient.Reverse(c.Context(), geocoding.ReverseParams{Lat: lat, Lon: lon})
	if err != nil {
		return mapGeocodingErr(c, err)
	}
	return c.JSON(fiber.Map{"result": result, "traceId": traceIDOrEmpty(c)})
}
