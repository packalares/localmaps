package api_test

// Integration tests for POST /api/route, /api/matrix, /api/isochrone,
// and the GPX/KML exports. Valhalla is mocked with an httptest.Server —
// the real pod wiring happens in deploy/ and is out of scope here.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/api"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/ratelimit"
	"github.com/packalares/localmaps/server/internal/telemetry"
	"github.com/packalares/localmaps/server/internal/ws"
)

// encodePolyline6 is a tiny inline reimplementation of Valhalla's
// precision-6 polyline encoder — used only here to build a known-good
// response body for the Valhalla mock. Mirrors the production decoder
// in server/internal/routing/polyline.go.
func encodePolyline6(pts [][2]float64) string {
	const factor = 1e6
	var (
		prevLat, prevLon int64
		out              []byte
	)
	for _, p := range pts {
		lat := int64(math.Round(p[0] * factor))
		lon := int64(math.Round(p[1] * factor))
		out = appendVarint(out, lat-prevLat)
		out = appendVarint(out, lon-prevLon)
		prevLat, prevLon = lat, lon
	}
	return string(out)
}

func appendVarint(dst []byte, v int64) []byte {
	u := uint64(v << 1)
	if v < 0 {
		u = ^uint64(v << 1)
	}
	for u >= 0x20 {
		dst = append(dst, byte((0x20|(u&0x1f))+63))
		u >>= 5
	}
	dst = append(dst, byte(u+63))
	return dst
}

// newFakeValhalla returns an httptest.Server that speaks the subset of
// the Valhalla HTTP API the Client exercises.
func newFakeValhalla(t *testing.T, capture map[string][]byte) *httptest.Server {
	t.Helper()
	encodedShape := encodePolyline6([][2]float64{
		{44.4268, 26.1025},
		{44.4270, 26.1030},
	})
	routeBody := fmt.Sprintf(`{
		"trip":{
			"units":"kilometers",
			"summary":{"time":120,"length":0.5},
			"locations":[{"lat":44.4268,"lon":26.1025},{"lat":44.4270,"lon":26.1030}],
			"legs":[{
				"summary":{"time":120,"length":0.5},
				"shape":%q,
				"maneuvers":[{
					"instruction":"Head east on Main Street",
					"begin_shape_index":0,"end_shape_index":1,
					"length":0.5,"time":120,"type":1,
					"street_names":["Main Street"]
				}]
			}]
		}
	}`, encodedShape)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if capture != nil {
			capture[r.URL.Path] = body
		}
		switch r.URL.Path {
		case "/route":
			_, _ = w.Write([]byte(routeBody))
		case "/isochrone":
			_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
		case "/sources_to_targets":
			_, _ = w.Write([]byte(`{"units":"kilometers","sources_to_targets":[[{"time":60,"distance":0.5}]]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// buildAppWithValhalla is the routing-aware cousin of buildApp: it
// wires the api.Deps with a Boot pointing at the given Valhalla URL.
func buildAppWithValhalla(t *testing.T, valhallaURL string) *fiber.App {
	t.Helper()
	store, err := config.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	tel := telemetry.New(io.Discard, "info")
	hub := ws.NewHub()
	t.Cleanup(hub.Close)

	app := fiber.New()
	api.Register(app, api.Deps{
		Boot:      &config.Boot{ValhallaURL: valhallaURL},
		Store:     store,
		Telemetry: tel,
		Hub:       hub,
		Limiter:   ratelimit.New(store),
	})
	return app
}

// TestRoute_PostHappyPath is the "synthetic test" from the brief: fire
// POST /api/route through Fiber, with Valhalla mocked via httptest, and
// verify the response carries a non-empty `routes` array with the
// OpenAPI-declared schema.
func TestRoute_PostHappyPath(t *testing.T) {
	captured := map[string][]byte{}
	srv := newFakeValhalla(t, captured)
	defer srv.Close()

	app := buildAppWithValhalla(t, srv.URL)

	body, _ := json.Marshal(map[string]interface{}{
		"locations": []map[string]float64{
			{"lat": 44.4268, "lon": 26.1025},
			{"lat": 44.4270, "lon": 26.1030},
		},
		"mode": "auto",
	})
	req := httptest.NewRequest("POST", "/api/route", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))

	routes, ok := got["routes"].([]interface{})
	require.True(t, ok, "routes array must be present")
	require.Len(t, routes, 1, "expected a single route")

	route := routes[0].(map[string]interface{})
	require.NotEmpty(t, route["id"], "route id must be a non-empty opaque string")

	legs, ok := route["legs"].([]interface{})
	require.True(t, ok, "legs must be present")
	require.Len(t, legs, 1)

	leg := legs[0].(map[string]interface{})
	geom := leg["geometry"].(map[string]interface{})
	require.Contains(t, geom, "polyline")
	require.NotEmpty(t, geom["polyline"], "polyline must be forwarded from Valhalla")

	// distanceMeters (0.5 km) must be converted to meters.
	summary := route["summary"].(map[string]interface{})
	require.Equal(t, 500.0, summary["distanceMeters"])
	require.Equal(t, 120.0, summary["timeSeconds"])

	// Verify we translated mode → costing on the way out.
	var sent map[string]interface{}
	require.NoError(t, json.Unmarshal(captured["/route"], &sent))
	require.Equal(t, "auto", sent["costing"])
}

// TestRoute_GPXAndKMLExport exercises the id→cache→formatter flow.
func TestRoute_GPXAndKMLExport(t *testing.T) {
	srv := newFakeValhalla(t, nil)
	defer srv.Close()
	app := buildAppWithValhalla(t, srv.URL)

	body, _ := json.Marshal(map[string]interface{}{
		"locations": []map[string]float64{
			{"lat": 44.4268, "lon": 26.1025},
			{"lat": 44.4270, "lon": 26.1030},
		},
		"mode": "auto",
	})
	req := httptest.NewRequest("POST", "/api/route", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var out map[string]interface{}
	raw, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(raw, &out))
	id := out["routes"].([]interface{})[0].(map[string]interface{})["id"].(string)

	// GPX.
	gpxResp, err := app.Test(httptest.NewRequest("GET", "/api/route/"+id+"/gpx", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, gpxResp.StatusCode)
	require.Equal(t, "application/gpx+xml", gpxResp.Header.Get("Content-Type"))
	gpxBody, _ := io.ReadAll(gpxResp.Body)
	require.True(t, strings.Contains(string(gpxBody), "<gpx"))
	require.True(t, strings.Contains(string(gpxBody), "<trkpt "))

	// KML.
	kmlResp, err := app.Test(httptest.NewRequest("GET", "/api/route/"+id+"/kml", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, kmlResp.StatusCode)
	require.Equal(t, "application/vnd.google-earth.kml+xml", kmlResp.Header.Get("Content-Type"))
	kmlBody, _ := io.ReadAll(kmlResp.Body)
	require.True(t, strings.Contains(string(kmlBody), "<kml"))
	require.True(t, strings.Contains(string(kmlBody), "<coordinates>"))

	// Unknown id → 404.
	missResp, err := app.Test(httptest.NewRequest("GET", "/api/route/does-not-exist/gpx", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, missResp.StatusCode)
}

// TestIsochrone_ForwardsGeoJSON confirms we pass through Valhalla's
// body verbatim to satisfy the UI's GeoJSON FeatureCollection schema.
func TestIsochrone_ForwardsGeoJSON(t *testing.T) {
	srv := newFakeValhalla(t, nil)
	defer srv.Close()
	app := buildAppWithValhalla(t, srv.URL)

	body, _ := json.Marshal(map[string]interface{}{
		"origin":          map[string]float64{"lat": 44.43, "lon": 26.10},
		"mode":            "pedestrian",
		"contoursSeconds": []int{600},
	})
	req := httptest.NewRequest("POST", "/api/isochrone", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &parsed))
	require.Equal(t, "FeatureCollection", parsed["type"])
}

// TestMatrix_ConvertsUnits verifies the km→m conversion path.
func TestMatrix_ConvertsUnits(t *testing.T) {
	srv := newFakeValhalla(t, nil)
	defer srv.Close()
	app := buildAppWithValhalla(t, srv.URL)

	body, _ := json.Marshal(map[string]interface{}{
		"sources": []map[string]float64{{"lat": 44.4, "lon": 26.1}},
		"targets": []map[string]float64{{"lat": 44.5, "lon": 26.2}},
		"mode":    "auto",
	})
	req := httptest.NewRequest("POST", "/api/matrix", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &out))
	matrix := out["matrix"].([]interface{})
	require.Len(t, matrix, 1)
	row := matrix[0].([]interface{})
	require.Len(t, row, 1)
	cell := row[0].(map[string]interface{})
	require.Equal(t, 500.0, cell["distanceMeters"])
	require.Equal(t, 60.0, cell["timeSeconds"])
}

// TestRoute_BadInputReturns400 exercises the validation short-circuit
// before the upstream is contacted.
func TestRoute_BadInputReturns400(t *testing.T) {
	// Point the Valhalla URL at an unreachable port so any request that
	// escapes the validation guard fails loudly.
	app := buildAppWithValhalla(t, "http://127.0.0.1:1")

	body, _ := json.Marshal(map[string]interface{}{
		"locations": []map[string]float64{{"lat": 1, "lon": 2}}, // only 1 point
		"mode":      "auto",
	})
	req := httptest.NewRequest("POST", "/api/route", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
