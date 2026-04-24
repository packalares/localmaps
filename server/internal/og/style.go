package og

import "image/color"

// palette is the minimal visual theme applied to the PNG. Values are
// hand-picked to read well on Discord, Slack and Twitter embed cards
// regardless of the client's own dark/light mode.
type palette struct {
	bg       color.RGBA // backdrop
	grid     color.RGBA // graticule lines
	ink      color.RGBA // text and watermark
	pinBody  color.RGBA // marker pin fill
	pinRing  color.RGBA // marker pin outline
	pinCore  color.RGBA // marker pin inner dot
	accent   color.RGBA // brand accent (borders, corner tab)
}

// paletteForStyle picks a palette keyed by the map.style setting. Only
// the values documented in docs/07-config-schema.md are honoured —
// "light", "dark", "auto" (treated as light for the rendered PNG since
// we can't read the user agent's color-scheme from a server).
func paletteForStyle(style string) palette {
	switch style {
	case "dark":
		return palette{
			bg:      color.RGBA{R: 0x0F, G: 0x17, B: 0x23, A: 0xFF},
			grid:    color.RGBA{R: 0x1E, G: 0x29, B: 0x3B, A: 0xFF},
			ink:     color.RGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF},
			pinBody: color.RGBA{R: 0x38, G: 0xBD, B: 0xF8, A: 0xFF},
			pinRing: color.RGBA{R: 0x0F, G: 0x17, B: 0x23, A: 0xFF},
			pinCore: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
			accent:  color.RGBA{R: 0x0E, G: 0xA5, B: 0xE9, A: 0xFF},
		}
	default: // "light" | "auto" | unknown — fall through to the neutral palette
		return palette{
			bg:      color.RGBA{R: 0xF3, G: 0xF6, B: 0xFA, A: 0xFF},
			grid:    color.RGBA{R: 0xD9, G: 0xE1, B: 0xEC, A: 0xFF},
			ink:     color.RGBA{R: 0x1F, G: 0x2A, B: 0x3A, A: 0xFF},
			pinBody: color.RGBA{R: 0x0E, G: 0xA5, B: 0xE9, A: 0xFF},
			pinRing: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
			pinCore: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
			accent:  color.RGBA{R: 0x0E, G: 0xA5, B: 0xE9, A: 0xFF},
		}
	}
}

// KnownStyles returns the enum values the renderer understands.
// Callers validating query params should reject anything else.
func KnownStyles() []string { return []string{"light", "dark", "auto"} }
