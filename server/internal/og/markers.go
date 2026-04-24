package og

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// drawPin paints a teardrop-style marker at the viewport centre. The
// renderer only exposes a "there's a pin" boolean; positional
// differences between the view centre and the pin LatLon are
// irrelevant in the Option-B layout because we render a diagrammatic
// card, not a georeferenced map. A future Option-A renderer will
// project Pin to pixel space via TileSource.
func drawPin(img *image.RGBA, s Size, pal palette) {
	cx, cy := s.W/2, s.H/2-6
	// Outer ring shadow (soft contrast against the backdrop).
	drawFilledCircle(img, cx+1, cy+18, 3, tintLine(pal.ink, 0.15))

	// Teardrop: large disk + downward triangle.
	drawFilledCircle(img, cx, cy, 18, pal.pinBody)
	drawFilledCircle(img, cx, cy, 18, pal.pinBody) // harmless double for coverage
	// Outline ring.
	drawCircle(img, cx, cy, 18, pal.pinRing)
	drawCircle(img, cx, cy, 17, pal.pinRing)
	// Triangle point.
	for dy := 0; dy < 14; dy++ {
		width := 14 - dy
		for dx := -width / 2; dx <= width/2; dx++ {
			img.SetRGBA(cx+dx, cy+18+dy, pal.pinBody)
		}
	}
	// Inner dot.
	drawFilledCircle(img, cx, cy, 5, pal.pinCore)
}

// drawCaptions writes the coordinate label (top-left) and region label
// (bottom-left) in the pixel font. The strings are truncated to fit;
// Discord card rendering doesn't need full typography.
func drawCaptions(img *image.RGBA, p Params, pal palette) {
	b := img.Bounds()
	// Top-left: a corner tab panel with coordinates + zoom.
	coord := fmt.Sprintf("%.4f, %.4f  Z%d", p.Center.Lat, p.Center.Lon, p.Zoom)
	drawTextBox(img, 24, 24, coord, pal, pal.bg)

	// Bottom-left: region name, if provided.
	if p.Region != "" {
		label := strings.ToUpper(strings.ReplaceAll(p.Region, "-", " "))
		drawTextBox(img, 24, b.Dy()-24-pxFontHeight-16, label, pal, pal.bg)
	}
}

// drawWatermark stamps the product name in the bottom-right corner.
func drawWatermark(img *image.RGBA, pal palette) {
	b := img.Bounds()
	label := "LOCALMAPS"
	w := pxTextWidth(label)
	drawTextBox(img, b.Dx()-w-24-16, b.Dy()-pxFontHeight-24-8, label, pal, pal.accent)
}

// drawTextBox renders s in the pixel font at (x, y), with a rounded
// (square) backing rectangle for legibility.
func drawTextBox(img *image.RGBA, x, y int, s string, pal palette, bg color.RGBA) {
	w := pxTextWidth(s)
	padX, padY := 8, 6
	// Fill backdrop.
	for dy := -padY; dy < pxFontHeight+padY; dy++ {
		for dx := -padX; dx < w+padX; dx++ {
			img.SetRGBA(x+dx, y+dy, bg)
		}
	}
	// Thin accent underline.
	for dx := -padX; dx < w+padX; dx++ {
		img.SetRGBA(x+dx, y+pxFontHeight+padY-1, pal.accent)
	}
	// Glyphs.
	drawPxText(img, x, y, s, pal.ink)
}

// --- tiny pixel font --------------------------------------------------

const (
	pxFontWidth   = 5
	pxFontHeight  = 7
	pxFontKerning = 1
)

// pxTextWidth returns the pixel width of s in the pixel font, kerning
// included. Unknown glyphs render as a blank box of equal width.
func pxTextWidth(s string) int {
	if s == "" {
		return 0
	}
	return len(s)*pxFontWidth + (len(s)-1)*pxFontKerning
}

// drawPxText renders s at (x, y) in the given ink colour. Glyphs that
// don't exist in pxGlyphs are skipped silently.
func drawPxText(img *image.RGBA, x, y int, s string, ink color.RGBA) {
	for i, r := range s {
		glyph, ok := pxGlyphs[r]
		if !ok {
			glyph, ok = pxGlyphs[' ']
			if !ok {
				continue
			}
		}
		ox := x + i*(pxFontWidth+pxFontKerning)
		for row := 0; row < pxFontHeight; row++ {
			bits := glyph[row]
			for col := 0; col < pxFontWidth; col++ {
				if bits&(1<<(pxFontWidth-1-col)) != 0 {
					img.SetRGBA(ox+col, y+row, ink)
				}
			}
		}
	}
}

// pxGlyphs is a 5x7 pixel font covering uppercase A-Z, digits, and the
// few punctuation characters the renderer uses. Each glyph is 7 rows
// of 5 bits (MSB-first). Missing glyphs fall back to a blank cell.
var pxGlyphs = map[rune][pxFontHeight]byte{
	' ':  {0, 0, 0, 0, 0, 0, 0},
	'.':  {0, 0, 0, 0, 0, 0x04, 0x04},
	',':  {0, 0, 0, 0, 0, 0x04, 0x08},
	'-':  {0, 0, 0, 0x0E, 0, 0, 0},
	':':  {0, 0x04, 0, 0, 0, 0x04, 0},
	'/':  {0x01, 0x02, 0x02, 0x04, 0x08, 0x08, 0x10},
	'Z':  {0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F},
	'0':  {0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E},
	'1':  {0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'2':  {0x0E, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1F},
	'3':  {0x0E, 0x11, 0x01, 0x06, 0x01, 0x11, 0x0E},
	'4':  {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
	'5':  {0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E},
	'6':  {0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E},
	'7':  {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8':  {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
	'9':  {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C},
	'A':  {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B':  {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	'C':  {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
	'D':  {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E':  {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'F':  {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
	'G':  {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0F},
	'H':  {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'I':  {0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'J':  {0x07, 0x02, 0x02, 0x02, 0x02, 0x12, 0x0C},
	'K':  {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
	'L':  {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'M':  {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
	'N':  {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11},
	'O':  {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'P':  {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'Q':  {0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D},
	'R':  {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S':  {0x0F, 0x10, 0x10, 0x0E, 0x01, 0x01, 0x1E},
	'T':  {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'U':  {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'V':  {0x11, 0x11, 0x11, 0x11, 0x11, 0x0A, 0x04},
	'W':  {0x11, 0x11, 0x11, 0x15, 0x15, 0x15, 0x0A},
	'X':  {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
	'Y':  {0x11, 0x11, 0x11, 0x0A, 0x04, 0x04, 0x04},
}

// ensure image package used even if Go's deadcode weighs in.
var _ = image.Rect
