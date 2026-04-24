// Package og renders OpenGraph preview PNGs for shared map states.
//
// Phase 5 scope (Agent Q): ship a pure-Go compositor that produces a
// 1200x630 (or caller-sized) PNG summarising lat/lon/zoom/region/pin.
// A full MapLibre-compatible vector rasteriser in Go is out of scope
// for this phase — the project charter requires only that "when the
// link is pasted in Discord/Slack the preview renders". The renderer
// is therefore Option B from the Phase 5 spec: a palette-tinted
// backdrop with a grid, a centred marker pin, coordinate/region
// captions, and a watermark. All pieces are stdlib.
//
// TileSource is kept as an interface so a follow-up phase can plug in
// a real pmtiles-backed raster compositor (Option A) without touching
// the HTTP handler or the public Render signature.
package og

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"bytes"
	"math"
)

// Defaults for values the spec leaves optional. Every one of these is
// also bounded by contracts/openapi.yaml validation in the handler.
const (
	DefaultWidth  = 1200
	DefaultHeight = 630
	DefaultZoom   = 12
	MinZoom       = 0
	MaxZoom       = 22
)

// Errors returned by Render.
var (
	// ErrInvalidParams signals Render received out-of-range input.
	// The HTTP handler validates first, so this is belt-and-braces.
	ErrInvalidParams = errors.New("og: invalid parameters")
	// ErrRenderTimeout is the standard error when the parent context
	// fires before rendering completes.
	ErrRenderTimeout = errors.New("og: render timed out")
)

// LatLon is a single geographic coordinate.
type LatLon struct {
	Lat float64
	Lon float64
}

// Size is pixel dimensions of the rendered image.
type Size struct {
	W int
	H int
}

// Params is the full input to Render.
//
// Pin is optional — when nil no marker is drawn. Region is an optional
// canonical region key (e.g. "europe-romania") used purely for the
// caption; it is NOT used to load any filesystem data in Option B, but
// a future Option A raster compositor would use it via TileSource.
type Params struct {
	Center LatLon
	Zoom   int
	Pin    *LatLon
	Style  string
	Region string
	Size   Size
}

// TileSource abstracts the source of raster pixels for a lat/lon/zoom
// window. Phase 5 ships no implementation; Render falls back to the
// Option-B backdrop when Source is nil or returns ErrUnavailable.
//
// Future phases can implement this against go-pmtiles.
type TileSource interface {
	// Raster returns an RGBA image of exactly size for the given
	// bounding window, or an error. Implementations MUST honour ctx.
	Raster(ctx context.Context, center LatLon, zoom int, size Size) (*image.RGBA, error)
}

// Renderer bundles the optional tile source so tests can inject one.
type Renderer struct {
	Source TileSource
}

// New returns a Renderer with no tile source — uses the built-in
// palette backdrop. Callers in production construct this once and
// share across requests.
func New() *Renderer { return &Renderer{} }

// Render produces PNG bytes for the given params. It is the single
// public entry point; the HTTP handler in server/internal/api does not
// call any other symbol from this package except this one, Params, and
// LatLon.
func Render(ctx context.Context, p Params) ([]byte, error) {
	return New().Render(ctx, p)
}

// Render is the method form — tests that want to inject a custom
// TileSource construct a Renderer directly.
func (r *Renderer) Render(ctx context.Context, p Params) ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, ctxErr(err)
	}
	size := p.resolvedSize()
	palette := paletteForStyle(p.Style)

	// Try the injected raster source first (future Option A). Any
	// failure silently falls back to the palette backdrop so that a
	// broken source never 500s a link preview.
	var canvas *image.RGBA
	if r.Source != nil {
		if img, err := r.Source.Raster(ctx, p.Center, p.Zoom, size); err == nil && img != nil {
			canvas = img
		}
	}
	if canvas == nil {
		canvas = paintBackground(size, palette)
		paintGraticule(canvas, p.Center, p.Zoom, palette)
	}
	if err := ctx.Err(); err != nil {
		return nil, ctxErr(err)
	}
	if p.Pin != nil {
		drawPin(canvas, size, palette)
	}
	drawCaptions(canvas, p, palette)
	drawWatermark(canvas, palette)

	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// validate enforces the contracts/openapi.yaml query-param bounds.
// The HTTP handler has already done this, but a direct library caller
// may not.
func (p Params) validate() error {
	if math.IsNaN(p.Center.Lat) || math.IsNaN(p.Center.Lon) {
		return fmt.Errorf("%w: NaN coordinate", ErrInvalidParams)
	}
	if p.Center.Lat < -90 || p.Center.Lat > 90 {
		return fmt.Errorf("%w: lat out of range", ErrInvalidParams)
	}
	if p.Center.Lon < -180 || p.Center.Lon > 180 {
		return fmt.Errorf("%w: lon out of range", ErrInvalidParams)
	}
	if p.Zoom < MinZoom || p.Zoom > MaxZoom {
		return fmt.Errorf("%w: zoom out of range", ErrInvalidParams)
	}
	if p.Pin != nil {
		if p.Pin.Lat < -90 || p.Pin.Lat > 90 || p.Pin.Lon < -180 || p.Pin.Lon > 180 {
			return fmt.Errorf("%w: pin out of range", ErrInvalidParams)
		}
	}
	return nil
}

// resolvedSize picks defaults for zero fields without mutating Params.
func (p Params) resolvedSize() Size {
	w, h := p.Size.W, p.Size.H
	if w <= 0 {
		w = DefaultWidth
	}
	if h <= 0 {
		h = DefaultHeight
	}
	return Size{W: w, H: h}
}

// paintBackground fills the canvas with the style palette's background.
func paintBackground(s Size, pal palette) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, s.W, s.H))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: pal.bg}, image.Point{}, draw.Src)
	return img
}

// ctxErr normalises context errors into the package-level sentinel.
func ctxErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrRenderTimeout
	}
	return err
}

// _ keeps image/color imported even when callers pass nil Pin, etc.
var _ = color.RGBA{}
