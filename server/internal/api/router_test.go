package api_test

import (
	"encoding/json"
	"io"
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

func buildApp(t *testing.T) *fiber.App {
	t.Helper()
	store, err := config.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	tel := telemetry.New(io.Discard, "info")
	hub := ws.NewHub()
	t.Cleanup(hub.Close)

	app := fiber.New()
	api.Register(app, api.Deps{
		Boot:      &config.Boot{},
		Store:     store,
		Telemetry: tel,
		Hub:       hub,
		Limiter:   ratelimit.New(store),
	})
	return app
}

func TestHealth(t *testing.T) {
	app := buildApp(t)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/health", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	require.Equal(t, true, m["ok"])
}

func TestVersion(t *testing.T) {
	app := buildApp(t)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/version", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	require.Contains(t, m, "version")
	require.Contains(t, m, "commit")
	require.Contains(t, m, "builtAt")
}

func TestStubsReturn501WithEnvelope(t *testing.T) {
	app := buildApp(t)

	// Sample a stub path from every tag.
	publicStubs := []struct {
		method, path string
	}{
		{"GET", "/api/tiles/metadata"},
		// /api/geocode/* + /api/pois (search) are now real handlers;
		// when the test app is built with an empty Boot they keep their
		// 501 fallback, so they stay useful here to prove the stub-path
		// still works (geocodingClient == nil).
		{"GET", "/api/geocode/autocomplete?q=x"},
		{"POST", "/api/route"},
		{"GET", "/api/regions"},
		{"GET", "/api/regions/catalog"},
		// GET /api/jobs/{id} is a real handler now (reads from sqlite).
		// GET /api/settings/schema is a real handler now (Phase 6 Agent S).
		// POST /api/links is a real handler now (Phase 5 Agent R); tests
		// for its 201 / 400 / 404 branches live in share_test.go.
	}
	for _, s := range publicStubs {
		req := httptest.NewRequest(s.method, s.path, nil)
		resp, err := app.Test(req)
		require.NoErrorf(t, err, "%s %s", s.method, s.path)
		require.Equalf(t, fiber.StatusNotImplemented, resp.StatusCode,
			"%s %s expected 501", s.method, s.path)

		b, _ := io.ReadAll(resp.Body)
		var m map[string]any
		require.NoErrorf(t, json.Unmarshal(b, &m), "%s %s", s.method, s.path)
		require.Contains(t, m, "error")
		require.Contains(t, m, "traceId")
		errObj := m["error"].(map[string]any)
		require.Equal(t, "INTERNAL", errObj["code"])
		require.Equal(t, "not yet implemented", errObj["message"])
	}
}

func TestAdminStubs_RequireAuth(t *testing.T) {
	app := buildApp(t)

	// Admin routes without the session cookie must return 401.
	cases := []struct{ method, path string }{
		{"POST", "/api/regions"},
		{"DELETE", "/api/regions/europe-romania"},
		{"POST", "/api/regions/europe-romania/update"},
		{"PUT", "/api/regions/europe-romania/schedule"},
		{"GET", "/api/settings"},
		{"PUT", "/api/settings"},
		{"PATCH", "/api/settings"},
	}
	for _, c := range cases {
		resp, err := app.Test(httptest.NewRequest(c.method, c.path, nil))
		require.NoError(t, err)
		require.Equalf(t, fiber.StatusUnauthorized, resp.StatusCode,
			"%s %s", c.method, c.path)
	}
}

func TestMetrics_Exposed(t *testing.T) {
	app := buildApp(t)
	// Hit a stub to generate a request counter sample.
	_, _ = app.Test(httptest.NewRequest("GET", "/api/regions", nil))
	resp, err := app.Test(httptest.NewRequest("GET", "/metrics", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "localmaps_http_requests_total")
}

func TestWS_NoUpgrade_Rejected(t *testing.T) {
	app := buildApp(t)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/ws", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
