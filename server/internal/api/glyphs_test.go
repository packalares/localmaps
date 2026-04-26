package api_test

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// TestGlyphs_ServesEmbeddedPBF exercises the three standard fontstacks
// MapLibre requests when rendering text against our style. Each request
// must return a real protobuf payload with the glyphs.proto `stacks`
// tag (0x0a) as the first byte.
func TestGlyphs_ServesEmbeddedPBF(t *testing.T) {
	app := buildApp(t)

	cases := []string{
		"/api/glyphs/Noto%20Sans%20Regular/0-255.pbf",
		"/api/glyphs/Noto%20Sans%20Bold/0-255.pbf",
		"/api/glyphs/Noto%20Sans%20Italic/256-511.pbf",
		// Comma-separated fallback — Arial is not embedded, Noto is.
		"/api/glyphs/Arial%20Unicode%20MS%20Regular,Noto%20Sans%20Regular/0-255.pbf",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", path, nil))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.Equal(t, "application/x-protobuf", resp.Header.Get("Content-Type"))
			require.Contains(t, resp.Header.Get("Cache-Control"), "immutable")
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(body), 1000, "PBF unexpectedly small")
			require.Equal(t, byte(0x0a), body[0], "first byte should be protobuf stacks tag")
		})
	}
}

// TestGlyphs_UnknownFont_404 ensures non-embedded fonts / malformed
// ranges produce 404 so MapLibre can fall back silently.
func TestGlyphs_UnknownFont_404(t *testing.T) {
	app := buildApp(t)
	for _, path := range []string{
		"/api/glyphs/Missing%20Font/0-255.pbf",
		"/api/glyphs/Noto%20Sans%20Regular/bogus.pbf",
	} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusNotFound, resp.StatusCode, "path %s", path)
	}
}
