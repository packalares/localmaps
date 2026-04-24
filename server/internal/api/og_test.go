package api

// Unit tests for the OG preview handler. Black-box tests through
// Fiber's App.Test to exercise the full middleware chain. Uses a
// sync.atomic counter instead of a mock so we can also assert the
// cache-hit short-circuit.

import (
	"bytes"
	"context"
	"image"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/og"
	"github.com/packalares/localmaps/server/internal/ratelimit"
	"github.com/packalares/localmaps/server/internal/telemetry"
)

// pngHeader is the first 8 bytes of every PNG file per RFC 2083.
var pngHeader = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// newOGApp builds a minimal Fiber app wired to the OG handler with a
// temp cache directory, and returns the handler, the app, and a
// pointer to a counter that tracks how many times the renderer ran.
func newOGApp(t *testing.T, renderErr error) (*fiber.App, *ogHandler, *int32, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := config.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	// Tests fire > 10 requests for the same IP; lift the default cap.
	require.NoError(t, store.Set("rateLimit.ogPreviewPerMinutePerIP", 10000, "test"))

	tel := telemetry.New(io.Discard, "info")
	limiter := ratelimit.New(store)

	var calls int32
	h := newOGHandler(store, dir, tel)
	h.render = func(ctx context.Context, p og.Params) ([]byte, error) {
		atomic.AddInt32(&calls, 1)
		if renderErr != nil {
			return nil, renderErr
		}
		// Use the real renderer so cache contents are valid PNG bytes.
		return og.Render(ctx, p)
	}

	app := fiber.New()
	app.Use(tel.NewTracer())
	app.Get("/og/preview.png", h.ogPreviewHandler,
		limiter.PerIP("rateLimit.ogPreviewPerMinutePerIP"))
	return app, h, &calls, dir
}

// TestOGPreview_ValidatesQueryParams covers the 400 branches of parseOGQuery.
func TestOGPreview_ValidatesQueryParams(t *testing.T) {
	app, _, _, _ := newOGApp(t, nil)
	cases := []string{
		"/og/preview.png",                                 // missing lat+lon
		"/og/preview.png?lat=91&lon=0",                    // lat out of range
		"/og/preview.png?lat=0&lon=181",                   // lon out of range
		"/og/preview.png?lat=0&lon=0&zoom=99",             // zoom too high
		"/og/preview.png?lat=0&lon=0&zoom=-1",             // zoom too low
		"/og/preview.png?lat=0&lon=0&width=10",            // width too small
		"/og/preview.png?lat=0&lon=0&height=99999",        // height too big
		"/og/preview.png?lat=0&lon=0&style=neon",          // unknown style
		"/og/preview.png?lat=0&lon=0&region=../etc",       // bad region
		"/og/preview.png?lat=0&lon=0&pin=maybe",           // bad pin bool
		"/og/preview.png?lat=foo&lon=0",                   // lat not a number
	}
	for _, path := range cases {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err, path)
		require.Equalf(t, fiber.StatusBadRequest, resp.StatusCode, "path=%s", path)
	}
}

// TestOGPreview_MissRendersAndCaches asserts that a cache-miss invokes
// the renderer, writes a file to <dataDir>/cache/og/, and serves PNG.
func TestOGPreview_MissRendersAndCaches(t *testing.T) {
	app, h, calls, dir := newOGApp(t, nil)
	req := httptest.NewRequest("GET",
		"/og/preview.png?lat=44.4268&lon=26.1025&zoom=10&width=400&height=200&region=europe-romania", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	require.Contains(t, resp.Header.Get("Cache-Control"), "max-age=")
	body, _ := io.ReadAll(resp.Body)
	require.True(t, bytes.HasPrefix(body, pngHeader))

	// Verify the cache file exists.
	require.Equal(t, int32(1), atomic.LoadInt32(calls), "render should have been called once")
	// Find any .png file under <dataDir>/cache/og/.
	cacheDir := filepath.Join(dir, "cache", "og")
	entries, rerr := os.ReadDir(cacheDir)
	require.NoError(t, rerr)
	require.Len(t, entries, 1, "one cache entry expected")
	require.Equal(t, ".png", filepath.Ext(entries[0].Name()))

	// Decode the cached file as PNG for a belt-and-braces sanity check.
	cached, err := os.ReadFile(filepath.Join(cacheDir, entries[0].Name()))
	require.NoError(t, err)
	_, _, err = image.DecodeConfig(bytes.NewReader(cached))
	require.NoError(t, err)

	// Also confirm the helper agrees.
	require.True(t, ensureCacheFileExists(h.dataDir, cacheKey(og.Params{
		Center: og.LatLon{Lat: 44.4268, Lon: 26.1025}, Zoom: 10,
		Pin:    &og.LatLon{Lat: 44.4268, Lon: 26.1025},
		Size:   og.Size{W: 400, H: 200}, Region: "europe-romania",
	})))
}

// TestOGPreview_HitServesFromCacheWithoutRender is the key caching
// assertion: the second request does NOT invoke the render function.
func TestOGPreview_HitServesFromCacheWithoutRender(t *testing.T) {
	app, _, calls, _ := newOGApp(t, nil)
	path := "/og/preview.png?lat=0&lon=0&zoom=2&width=320&height=200"

	r1, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, r1.StatusCode)
	require.Equal(t, int32(1), atomic.LoadInt32(calls))

	r2, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, r2.StatusCode)
	require.Equal(t, int32(1), atomic.LoadInt32(calls),
		"cache-hit must not invoke render")

	b2, _ := io.ReadAll(r2.Body)
	require.True(t, bytes.HasPrefix(b2, pngHeader))
}

// TestOGPreview_RenderTimeout_Returns503 exercises the deadline path.
func TestOGPreview_RenderTimeout_Returns503(t *testing.T) {
	app, _, _, _ := newOGApp(t, og.ErrRenderTimeout)
	resp, err := app.Test(httptest.NewRequest("GET",
		"/og/preview.png?lat=0&lon=0", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadGateway, resp.StatusCode) // UPSTREAM_UNAVAILABLE → 502
	require.NotEmpty(t, resp.Header.Get("Retry-After"))
}

// TestCacheKey_StableForSameParams locks down the canonical key format.
func TestCacheKey_StableForSameParams(t *testing.T) {
	pin := og.LatLon{Lat: 1, Lon: 2}
	a := cacheKey(og.Params{Center: og.LatLon{Lat: 1, Lon: 2}, Zoom: 5, Pin: &pin, Size: og.Size{W: 100, H: 50}})
	b := cacheKey(og.Params{Center: og.LatLon{Lat: 1, Lon: 2}, Zoom: 5, Pin: &pin, Size: og.Size{W: 100, H: 50}})
	require.Equal(t, a, b)
	require.Len(t, a, 64)
}

// TestCacheKey_DiffersBySize verifies different sizes produce different keys.
func TestCacheKey_DiffersBySize(t *testing.T) {
	p1 := og.Params{Center: og.LatLon{Lat: 1, Lon: 2}, Zoom: 5, Size: og.Size{W: 100, H: 50}}
	p2 := og.Params{Center: og.LatLon{Lat: 1, Lon: 2}, Zoom: 5, Size: og.Size{W: 200, H: 50}}
	require.NotEqual(t, cacheKey(p1), cacheKey(p2))
}

// TestStub_Returns501 covers the fallback when there's no Store.
func TestStub_Returns501(t *testing.T) {
	stub := newOGHandlerStub()
	app := fiber.New()
	app.Get("/og/preview.png", stub)
	resp, err := app.Test(httptest.NewRequest("GET", "/og/preview.png", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotImplemented, resp.StatusCode)
}
