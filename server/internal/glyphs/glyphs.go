// Package glyphs serves pre-built MapLibre glyph PBFs embedded in the
// binary. The files under fonts/ are sourced from openmaptiles/fonts
// (gh-pages branch), which compiles Noto Sans TTFs into per-range PBFs
// using fontnik. See NOTICE.md in this directory for attribution.
//
// MapLibre's style `glyphs` URL template expands to 256-codepoint
// ranges: 0-255, 256-511, … 65280-65535. Each range is a pre-compiled
// PBF of SDF glyphs. The handler wires `Lookup(fontstack, range)` to
// the GET /api/glyphs/{fontstack}/{range}.pbf endpoint.
package glyphs

import (
	"embed"
	"io/fs"
	"regexp"
	"strings"
)

// Embed the whole fonts tree. The directive uses `all:` so hidden files
// (dotfiles starting with `.` or `_`) are included — harmless for PBFs
// but keeps the embedded layout byte-for-byte identical to disk. The
// bare `fonts` path recursively embeds every file; subdirectory names
// with spaces ("Noto Sans Regular") work with no special quoting in
// the directive because Go embed treats `fonts` as a literal path.
//
//go:embed all:fonts
var fontsFS embed.FS

// rangeRE validates the `{range}` path param. MapLibre always requests
// `START-END` where both sides are unsigned ints and END = START+255
// (0-255, 256-511, …). We only enforce the shape here; any caller
// probing with a bogus range gets 404 from the `fs.ReadFile` miss.
var rangeRE = regexp.MustCompile(`^\d+-\d+$`)

// Lookup returns the PBF bytes for (fontstack, rangeStr). fontstack is
// the raw path segment MapLibre sent, which may be a comma-separated
// list of fallbacks like "Noto Sans Regular,Arial Unicode MS Regular".
// We try each font in order and return the first that matches an
// embedded set. Returns (nil, false) if no font in the stack is
// available or the range is malformed / missing.
func Lookup(fontstack, rangeStr string) ([]byte, bool) {
	if !rangeRE.MatchString(rangeStr) {
		return nil, false
	}
	for _, name := range splitFontstack(fontstack) {
		data, err := fs.ReadFile(fontsFS, "fonts/"+name+"/"+rangeStr+".pbf")
		if err == nil {
			return data, true
		}
	}
	return nil, false
}

// splitFontstack splits on commas and trims each entry. MapLibre may
// url-decode the stack before we see it (handler does PathUnescape),
// so incoming spaces are literal. Empty entries are skipped.
func splitFontstack(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Fonts returns the sorted list of embedded font names. Useful for
// `/api/fonts` style introspection endpoints and for tests.
func Fonts() []string {
	entries, err := fs.ReadDir(fontsFS, "fonts")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}
