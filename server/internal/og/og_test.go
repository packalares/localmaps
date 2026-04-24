package og_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/og"
)

// pngSignature is the first 8 bytes of every PNG file per RFC 2083.
var pngSignature = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// TestRender_ProducesValidPNG is the minimum acceptance test — the
// renderer must emit parseable PNG bytes of the requested size.
func TestRender_ProducesValidPNG(t *testing.T) {
	ctx := context.Background()
	pin := og.LatLon{Lat: 44.4268, Lon: 26.1025}
	params := og.Params{
		Center: og.LatLon{Lat: 44.4268, Lon: 26.1025},
		Zoom:   12,
		Pin:    &pin,
		Style:  "light",
		Region: "europe-romania",
		Size:   og.Size{W: 600, H: 315}, // smaller for faster test
	}
	data, err := og.Render(ctx, params)
	require.NoError(t, err)
	require.True(t, len(data) > len(pngSignature))
	require.Equal(t, pngSignature, data[:len(pngSignature)])

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "png", format)
	require.Equal(t, 600, cfg.Width)
	require.Equal(t, 315, cfg.Height)
}

// TestRender_Defaults verifies zero-valued Size falls back to 1200x630.
func TestRender_Defaults(t *testing.T) {
	ctx := context.Background()
	data, err := og.Render(ctx, og.Params{
		Center: og.LatLon{Lat: 0, Lon: 0},
		Zoom:   2,
	})
	require.NoError(t, err)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, og.DefaultWidth, cfg.Width)
	require.Equal(t, og.DefaultHeight, cfg.Height)
}

// TestRender_Dark verifies the dark style path doesn't crash + still
// emits PNG bytes.
func TestRender_Dark(t *testing.T) {
	ctx := context.Background()
	data, err := og.Render(ctx, og.Params{
		Center: og.LatLon{Lat: 10, Lon: 10}, Zoom: 8, Style: "dark",
		Size: og.Size{W: 400, H: 200},
	})
	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 400, img.Bounds().Dx())
}

// TestRender_ValidatesParams ensures out-of-range inputs are rejected
// rather than producing a malformed image.
func TestRender_ValidatesParams(t *testing.T) {
	cases := []struct {
		name string
		p    og.Params
	}{
		{"latTooHigh", og.Params{Center: og.LatLon{Lat: 91, Lon: 0}}},
		{"latTooLow", og.Params{Center: og.LatLon{Lat: -91, Lon: 0}}},
		{"lonTooHigh", og.Params{Center: og.LatLon{Lat: 0, Lon: 181}}},
		{"lonTooLow", og.Params{Center: og.LatLon{Lat: 0, Lon: -181}}},
		{"zoomTooHigh", og.Params{Center: og.LatLon{Lat: 0, Lon: 0}, Zoom: 99}},
		{"zoomNegative", og.Params{Center: og.LatLon{Lat: 0, Lon: 0}, Zoom: -1}},
		{"latNaN", og.Params{Center: og.LatLon{Lat: math.NaN(), Lon: 0}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := og.Render(context.Background(), c.p)
			require.ErrorIs(t, err, og.ErrInvalidParams)
		})
	}
}

// TestRender_RespectsCancelledContext ensures the renderer bails early
// when the caller aborts.
func TestRender_RespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := og.Render(ctx, og.Params{
		Center: og.LatLon{Lat: 0, Lon: 0}, Zoom: 2,
	})
	require.Error(t, err)
}

// TestRender_DeadlineExpiredMaps returns the sentinel when the context
// hit a deadline rather than an explicit cancel.
func TestRender_DeadlineExpired(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(),
		time.Now().Add(-time.Second))
	defer cancel()
	_, err := og.Render(ctx, og.Params{
		Center: og.LatLon{Lat: 0, Lon: 0}, Zoom: 2,
	})
	require.ErrorIs(t, err, og.ErrRenderTimeout)
}

// TestRegionPMTilesPath verifies the safepath contract: legitimate
// keys return a joined path, malicious ones are rejected.
func TestRegionPMTilesPath(t *testing.T) {
	got, err := og.RegionPMTilesPath("/data", "europe-romania")
	require.NoError(t, err)
	require.Equal(t, "/data/regions/europe-romania/map.pmtiles", got)

	_, err = og.RegionPMTilesPath("/data", "../../etc/passwd")
	require.Error(t, err)

	_, err = og.RegionPMTilesPath("", "x")
	require.Error(t, err)
	_, err = og.RegionPMTilesPath("/data", "")
	require.Error(t, err)
}

// TestDiskPMTilesSource_Stub ensures the placeholder implementation
// never returns an image today (so Render falls back to Option B) and
// propagates context cancellation.
func TestDiskPMTilesSource_Stub(t *testing.T) {
	src := &og.DiskPMTilesSource{DataDir: "/data", Region: "europe-monaco"}
	_, err := src.Raster(context.Background(), og.LatLon{}, 5, og.Size{W: 10, H: 10})
	require.ErrorIs(t, err, og.ErrUnavailable)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = src.Raster(ctx, og.LatLon{}, 5, og.Size{W: 10, H: 10})
	require.Error(t, err)
}

// stubSource returns a solid-colour image for the requested size so we
// can verify the Renderer uses it when provided.
type stubSource struct{ called bool }

func (s *stubSource) Raster(_ context.Context, _ og.LatLon, _ int, sz og.Size) (*image.RGBA, error) {
	s.called = true
	return image.NewRGBA(image.Rect(0, 0, sz.W, sz.H)), nil
}

func TestRenderer_UsesInjectedSource(t *testing.T) {
	src := &stubSource{}
	r := &og.Renderer{Source: src}
	_, err := r.Render(context.Background(), og.Params{
		Center: og.LatLon{Lat: 0, Lon: 0}, Zoom: 2,
		Size: og.Size{W: 200, H: 200},
	})
	require.NoError(t, err)
	require.True(t, src.called, "injected TileSource should have been consulted")
}
