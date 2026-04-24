package og

import (
	"image"
	"image/color"
	"math"
)

// paintGraticule draws a faint lat/lon grid on the backdrop. This is
// the Phase-5 Option-B stand-in for real tiles; the density depends on
// zoom so that a small-scale view gets a coarser grid and a city view
// gets a fine one. The grid intentionally looks diagrammatic, not
// photorealistic — it reads as "a map view" in thumbnail form without
// pretending to be actual cartography.
//
// It also paints a simple compass-rose tick at the viewport centre so
// that a link-preview card shows *something* positional even when the
// center has no pin.
func paintGraticule(img *image.RGBA, center LatLon, zoom int, pal palette) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Grid spacing in pixels shrinks as zoom grows. The coefficients
	// are empirical — they keep 8–16 lines visible across zooms 0-22.
	spacing := gridSpacingPx(w, zoom)
	if spacing < 8 {
		spacing = 8
	}
	drawGridLines(img, spacing, pal.grid)

	// Centre crosshair — one horizontal + one vertical line in the
	// accent colour, slightly thicker than grid, drawn last so it
	// overlays the lines.
	cx, cy := w/2, h/2
	drawThickLine(img, 0, cy, w, cy, 1, tintLine(pal.accent, 0.35))
	drawThickLine(img, cx, 0, cx, h, 1, tintLine(pal.accent, 0.35))

	// Decorative rose ring around the centre — two concentric circles
	// so the card feels intentional even before the pin is drawn.
	drawCircle(img, cx, cy, 22, pal.accent)
	drawCircle(img, cx, cy, 10, tintLine(pal.accent, 0.6))

	// The center coordinate isn't used by the graticule geometry in
	// Option B, but we silence the unused warning and keep the
	// signature stable for the Option-A swap.
	_ = center
}

// gridSpacingPx maps zoom to grid line spacing in pixels. Coarser at
// low zoom, finer at high zoom. Clamped sensibly.
func gridSpacingPx(width, zoom int) int {
	// Fit ~10–20 lines across the canvas at zoom 12.
	base := width / 12
	// Every zoom step halves or doubles the spacing.
	shift := zoom - 12
	for shift > 0 && base > 12 {
		base /= 2
		shift--
	}
	for shift < 0 && base < 240 {
		base *= 2
		shift++
	}
	return base
}

// drawGridLines strokes evenly-spaced horizontal + vertical lines.
func drawGridLines(img *image.RGBA, spacing int, c color.RGBA) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	for x := spacing; x < w; x += spacing {
		drawThickLine(img, x, 0, x, h, 1, c)
	}
	for y := spacing; y < h; y += spacing {
		drawThickLine(img, 0, y, w, y, 1, c)
	}
}

// drawThickLine is a Bresenham line routine with optional thickness
// (extra passes along the perpendicular). Endpoints are inclusive.
func drawThickLine(img *image.RGBA, x0, y0, x1, y1, thickness int, c color.RGBA) {
	if thickness < 1 {
		thickness = 1
	}
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx + dy
	x, y := x0, y0
	for {
		for t := 0; t < thickness; t++ {
			img.SetRGBA(x, y+t, c)
			img.SetRGBA(x+t, y, c)
		}
		if x == x1 && y == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
	}
}

// drawCircle strokes a 1-pixel-thick circle using the midpoint algorithm.
func drawCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	x, y, e := r, 0, 1-r
	for x >= y {
		plotOctants(img, cx, cy, x, y, c)
		y++
		if e < 0 {
			e += 2*y + 1
		} else {
			x--
			e += 2*(y-x) + 1
		}
	}
}

// drawFilledCircle fills a disk by scanning the interior of the circle.
// Used for marker bodies where anti-aliasing isn't worth the dep weight.
func drawFilledCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(cx+dx, cy+dy, c)
			}
		}
	}
}

func plotOctants(img *image.RGBA, cx, cy, x, y int, c color.RGBA) {
	img.SetRGBA(cx+x, cy+y, c)
	img.SetRGBA(cx-x, cy+y, c)
	img.SetRGBA(cx+x, cy-y, c)
	img.SetRGBA(cx-x, cy-y, c)
	img.SetRGBA(cx+y, cy+x, c)
	img.SetRGBA(cx-y, cy+x, c)
	img.SetRGBA(cx+y, cy-x, c)
	img.SetRGBA(cx-y, cy-x, c)
}

// tintLine scales RGB towards black (factor<1) for more subtle strokes.
func tintLine(c color.RGBA, f float64) color.RGBA {
	f = math.Max(0, math.Min(1, f))
	return color.RGBA{
		R: uint8(float64(c.R) * f),
		G: uint8(float64(c.G) * f),
		B: uint8(float64(c.B) * f),
		A: c.A,
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
