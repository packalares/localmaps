package api_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// Sprite serving is a thin wrapper over an embedded atlas. Exercise the
// four canonical URLs MapLibre issues for a style with
// `"sprite": "/api/sprites/default"`:
//
//	/api/sprites/default.json
//	/api/sprites/default.png
//	/api/sprites/default@2x.json
//	/api/sprites/default@2x.png
func TestSprites_ServesDefaultAtlas(t *testing.T) {
	app := buildApp(t)

	cases := []struct {
		path, contentType string
		wantJSON          bool
	}{
		{"/api/sprites/default.json", "application/json", true},
		{"/api/sprites/default.png", "image/png", false},
		{"/api/sprites/default@2x.json", "application/json", true},
		{"/api/sprites/default@2x.png", "image/png", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.Equal(t, tc.contentType, resp.Header.Get("Content-Type"))
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NotEmpty(t, body)
			if tc.wantJSON {
				var m map[string]map[string]any
				require.NoError(t, json.Unmarshal(body, &m))
				require.GreaterOrEqual(t, len(m), 50, "want >=50 icons")
				// Spot-check one well-known Maki icon.
				air, ok := m["airport"]
				require.True(t, ok, "airport icon missing")
				require.Contains(t, air, "x")
				require.Contains(t, air, "y")
				require.Contains(t, air, "width")
				require.Contains(t, air, "pixelRatio")
			} else {
				// PNG magic: 89 50 4E 47 0D 0A 1A 0A
				require.GreaterOrEqual(t, len(body), 8)
				magic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
				require.Equal(t, magic, body[:8], "PNG magic mismatch")
			}
		})
	}
}

func TestSprites_UnknownName_404(t *testing.T) {
	app := buildApp(t)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/sprites/nonsense.json", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}
