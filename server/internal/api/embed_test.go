package api_test

import (
	"encoding/json"
	"io"
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

// embedAppWithOrigins boots the Fiber app with a named override for the
// `share.embedAllowedOrigins` setting. Returns the app and the underlying
// store in case the test needs to mutate it further.
func embedAppWithOrigins(t *testing.T, origins []string) *fiber.App {
	t.Helper()
	store, err := config.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	if origins != nil {
		require.NoError(t, store.Set("share.embedAllowedOrigins", origins, "test"))
	}

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

func TestEmbed_ValidParams_ReturnsHTMLWithSecurityHeaders(t *testing.T) {
	app := embedAppWithOrigins(t, nil) // default wildcard
	req := httptest.NewRequest("GET",
		"/embed?lat=44.43&lon=26.10&zoom=12&pin=44.43,26.10:Bucharest&style=dark",
		nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	csp := resp.Header.Get("Content-Security-Policy")
	require.NotEmpty(t, csp, "CSP header must be set")
	require.Contains(t, csp, "frame-ancestors *")
	require.NotContains(t, csp, "frame-ancestors 'none'")
	require.Equal(t, "", resp.Header.Get("X-Frame-Options"),
		"must NOT set X-Frame-Options when frame-ancestors covers it")
	require.Equal(t, "no-referrer", resp.Header.Get("Referrer-Policy"))
	require.Contains(t, resp.Header.Get("Permissions-Policy"), "geolocation=(self)")
	require.Empty(t, resp.Header.Values("Set-Cookie"))
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
}

func TestEmbed_InvalidLat_Returns400Envelope(t *testing.T) {
	app := embedAppWithOrigins(t, nil)
	resp, err := app.Test(httptest.NewRequest("GET", "/embed?lat=999", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	b, _ := io.ReadAll(resp.Body)
	var body map[string]any
	require.NoError(t, json.Unmarshal(b, &body))
	require.Contains(t, body, "error")
	require.Contains(t, body, "traceId")
	errObj := body["error"].(map[string]any)
	require.Equal(t, "BAD_REQUEST", errObj["code"])
	require.Contains(t, errObj["message"], "lat")
}

func TestEmbed_InvalidLon_Returns400(t *testing.T) {
	app := embedAppWithOrigins(t, nil)
	resp, err := app.Test(httptest.NewRequest("GET", "/embed?lon=-999", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestEmbed_InvalidZoom_Returns400(t *testing.T) {
	app := embedAppWithOrigins(t, nil)
	resp, err := app.Test(httptest.NewRequest("GET", "/embed?zoom=50", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestEmbed_InvalidStyle_Returns400(t *testing.T) {
	app := embedAppWithOrigins(t, nil)
	resp, err := app.Test(httptest.NewRequest("GET", "/embed?style=neon", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestEmbed_InvalidPin_Returns400(t *testing.T) {
	app := embedAppWithOrigins(t, nil)
	// Single scalar, not "lat,lon".
	resp, err := app.Test(httptest.NewRequest("GET", "/embed?pin=garbage", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestEmbed_InvalidRegion_Returns400(t *testing.T) {
	app := embedAppWithOrigins(t, nil)
	resp, err := app.Test(httptest.NewRequest("GET", "/embed?region=EUROPE/Romania", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestEmbed_AllowedOrigins_ProducesNamedAncestors(t *testing.T) {
	app := embedAppWithOrigins(t, []string{
		"https://example.com",
		"https://docs.example.com",
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/embed", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	csp := resp.Header.Get("Content-Security-Policy")
	require.Contains(t, csp,
		"frame-ancestors https://example.com https://docs.example.com")
	require.NotContains(t, csp, "frame-ancestors *")
	// Still no X-Frame-Options per the modern-browser guidance.
	require.Equal(t, "", resp.Header.Get("X-Frame-Options"))
}

func TestEmbed_EmptyOrigins_ProducesNoneAncestor(t *testing.T) {
	app := embedAppWithOrigins(t, []string{})
	resp, err := app.Test(httptest.NewRequest("GET", "/embed", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	csp := resp.Header.Get("Content-Security-Policy")
	require.Contains(t, csp, "frame-ancestors 'none'")
}

func TestEmbed_UIOriginEnv_Redirects(t *testing.T) {
	t.Setenv("LOCALMAPS_UI_ORIGIN", "https://maps.example.com/")
	app := embedAppWithOrigins(t, nil)
	resp, err := app.Test(httptest.NewRequest("GET",
		"/embed?lat=1&lon=2&zoom=3", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusFound, resp.StatusCode)
	loc := resp.Header.Get("Location")
	require.True(t, strings.HasPrefix(loc, "https://maps.example.com/embed?"),
		"redirect target was %q", loc)
	require.Contains(t, loc, "lat=1")
	require.Contains(t, loc, "lon=2")
	require.Contains(t, loc, "zoom=3")
	// Security headers still applied on the redirect response.
	require.NotEmpty(t, resp.Header.Get("Content-Security-Policy"))
	require.Empty(t, resp.Header.Values("Set-Cookie"))
}

func TestEmbed_InlineShell_RefreshesToUI(t *testing.T) {
	app := embedAppWithOrigins(t, nil)
	resp, err := app.Test(httptest.NewRequest("GET",
		"/embed?lat=10&lon=20&zoom=5", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	require.Contains(t, body, "<meta http-equiv=\"refresh\"")
	require.Contains(t, body, "/embed?")
	require.Contains(t, body, "lat=10")
}
