package api_test

// Integration tests for /api/geocode/* + /api/pois/* handlers. The
// upstream pelias-api is mocked with an httptest.Server; the real pod
// wiring lives in deploy/ and is out of scope here.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/api"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/ratelimit"
	"github.com/packalares/localmaps/server/internal/telemetry"
	"github.com/packalares/localmaps/server/internal/ws"
)

// newFakePeliasAPI mounts a pelias-api stand-in with enough wire
// fidelity to drive every /api/geocode + /api/pois path.
func newFakePeliasAPI(t *testing.T) *httptest.Server {
	t.Helper()
	venue := `{"type":"FeatureCollection","features":[
		{"type":"Feature",
		 "geometry":{"type":"Point","coordinates":[26.1025,44.4268]},
		 "bbox":[26.1,44.4,26.2,44.5],
		 "properties":{"gid":"openstreetmap:venue:node/1","layer":"venue",
		   "source":"openstreetmap","source_id":"node/1",
		   "name":"Cafe Bucur","label":"Cafe Bucur, Bucharest, Romania",
		   "confidence":0.9,"locality":"Bucharest","country":"Romania",
		   "category":["food:cafe"]}}
	]}`
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/autocomplete", "/v1/search", "/v1/reverse", "/v1/place":
			_, _ = w.Write([]byte(venue))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// buildAppWithPelias wires the app against a pelias-api URL.
func buildAppWithPelias(t *testing.T, peliasURL string) *fiber.App {
	t.Helper()
	store, err := config.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	tel := telemetry.New(io.Discard, "info")
	hub := ws.NewHub()
	t.Cleanup(hub.Close)

	app := fiber.New()
	api.Register(app, api.Deps{
		Boot:      &config.Boot{PeliasURL: peliasURL},
		Store:     store,
		Telemetry: tel,
		Hub:       hub,
		Limiter:   ratelimit.New(store),
	})
	return app
}

// TestGeocodeSearch_HappyPath verifies search proxies to pelias and
// reshapes the FeatureCollection into the GeocodeResult envelope.
func TestGeocodeSearch_HappyPath(t *testing.T) {
	srv := newFakePeliasAPI(t)
	defer srv.Close()
	app := buildAppWithPelias(t, srv.URL)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/geocode/search?q=cafe&limit=5", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))
	results := got["results"].([]interface{})
	require.Len(t, results, 1)
	first := results[0].(map[string]interface{})
	require.Equal(t, "Cafe Bucur, Bucharest, Romania", first["label"])
	require.Equal(t, "openstreetmap:venue:node/1", first["id"])
	center := first["center"].(map[string]interface{})
	require.Equal(t, 44.4268, center["lat"])
	require.Equal(t, 26.1025, center["lon"])
}

// TestGeocodeAutocomplete_BadQueryReturns400 exercises the q-required guard.
func TestGeocodeAutocomplete_BadQueryReturns400(t *testing.T) {
	srv := newFakePeliasAPI(t)
	defer srv.Close()
	app := buildAppWithPelias(t, srv.URL)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/geocode/autocomplete", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// TestPois_Query returns the venue feature list.
func TestPois_Query(t *testing.T) {
	srv := newFakePeliasAPI(t)
	defer srv.Close()
	app := buildAppWithPelias(t, srv.URL)

	resp, err := app.Test(httptest.NewRequest(
		"GET", "/api/pois?bbox=25,44,27,45&category=restaurant&limit=10", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))
	pois := got["pois"].([]interface{})
	require.Len(t, pois, 1)
	poi := pois[0].(map[string]interface{})
	require.Equal(t, "osm", poi["source"])
}

// TestPois_Categories is always-on: no upstream required.
func TestPois_Categories(t *testing.T) {
	app := buildAppWithPelias(t, "") // empty URL keeps proxy off
	resp, err := app.Test(httptest.NewRequest("GET", "/api/pois/categories", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))
	cats := got["categories"].([]interface{})
	require.NotEmpty(t, cats)
	// Spot-check one top-level id.
	ids := map[string]bool{}
	for _, c := range cats {
		ids[c.(map[string]interface{})["id"].(string)] = true
	}
	require.True(t, ids["food"])
	require.True(t, ids["services"])
}

// TestGeocodeReverse_NotFound covers the ErrNotFound → 404 translation.
func TestGeocodeReverse_NotFound(t *testing.T) {
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer empty.Close()
	app := buildAppWithPelias(t, empty.URL)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/geocode/reverse?lat=44.4&lon=26.1", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}
