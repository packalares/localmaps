// Package regions exposes shared helpers that both the server and
// worker rely on. The canonical region key is the hyphenated form of a
// Geofabrik path ("europe/romania" becomes "europe-romania"). The
// canonical form is what we store in the regions table (primary key),
// what we put in URLs, and what we write into paths under /data.
//
// See docs/04-data-model.md for the definition.
package regions

import (
	"errors"
	"regexp"
	"strings"
)

// Errors returned by the helpers in this package.
var (
	// ErrEmptyKey is returned when the input is blank.
	ErrEmptyKey = errors.New("regions: empty key")
	// ErrAbsoluteKey is returned for leading '/' (would be an absolute path).
	ErrAbsoluteKey = errors.New("regions: absolute path not allowed")
	// ErrPathTraversal is returned when the input contains ".." segments.
	ErrPathTraversal = errors.New("regions: path traversal not allowed")
	// ErrInvalidChars is returned when the input has characters outside
	// [a-z0-9-/].
	ErrInvalidChars = errors.New("regions: invalid characters in key")
)

// keyPattern matches one or more [a-z0-9-/] characters. Uppercase is
// forbidden so that the canonical form is stable. We intentionally do
// NOT allow underscores — Geofabrik's keys do not use them and we want
// one canonical shape.
var keyPattern = regexp.MustCompile(`^[a-z0-9/\-]+$`)

// NormaliseKey accepts either a Geofabrik-style slash-separated path
// (e.g. "europe/romania") or the already-hyphenated canonical key
// ("europe-romania") and returns the canonical key. Nested paths with
// multiple slashes collapse to hyphens:
//
//	"europe"                         -> "europe"
//	"europe/romania"                 -> "europe-romania"
//	"europe/germany/baden-wuerttemberg" -> "europe-germany-baden-wuerttemberg"
//	"europe-romania"                 -> "europe-romania"
//
// Rejects empty strings, absolute paths, ".." components, and anything
// with characters outside [a-z0-9-/].
func NormaliseKey(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", ErrEmptyKey
	}
	if strings.HasPrefix(s, "/") {
		return "", ErrAbsoluteKey
	}
	// Reject ".." anywhere as a segment — it's never a valid region.
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return "", ErrPathTraversal
		}
		if seg == "" {
			// Empty segment means a double slash or trailing slash.
			return "", ErrInvalidChars
		}
	}
	if !keyPattern.MatchString(s) {
		return "", ErrInvalidChars
	}
	// Replace slashes with hyphens. Hyphen is already in Geofabrik's
	// subregion names (e.g. baden-wuerttemberg), which is why we don't
	// use a different separator.
	return strings.ReplaceAll(s, "/", "-"), nil
}

// IsCanonical reports whether s is already the canonical hyphenated
// form (no slashes, only [a-z0-9-]). It does NOT validate that the key
// corresponds to a real Geofabrik region.
func IsCanonical(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

// GeofabrikPath reverses NormaliseKey for the common
// continent/country case:
//
//	"europe"          -> "europe"
//	"europe-romania"  -> "europe/romania"
//
// Because hyphens ALSO appear inside Geofabrik subregion names (e.g.
// "baden-wuerttemberg", "czech-republic", "north-america"), the first
// hyphen — and only the first — is converted to a slash when the
// input starts with one of the known continent prefixes. Everything
// after is treated as the country-or-subregion id exactly as Geofabrik
// publishes it, including any internal hyphens.
//
// Multi-level subregions (e.g. "europe/germany/baden-wuerttemberg")
// cannot be recovered from the canonical key alone; callers that need
// them must go through Client.Resolve which consults the catalog.
// GeofabrikPath returns the best-effort continent/rest string for
// those inputs, which is still the correct -latest.pbf URL for
// country-level extracts.
//
// Rejects empty, non-canonical, and inputs that do not match any known
// continent prefix and have no hyphen.
func GeofabrikPath(canonical string) (string, error) {
	c := strings.TrimSpace(canonical)
	if c == "" {
		return "", ErrEmptyKey
	}
	if !IsCanonical(c) {
		return "", ErrInvalidChars
	}
	// Bare continent (or any single-segment key) is valid as-is.
	if !strings.Contains(c, "-") {
		return c, nil
	}
	for _, continent := range GeofabrikContinents {
		prefix := continent + "-"
		if strings.HasPrefix(c, prefix) {
			rest := strings.TrimPrefix(c, prefix)
			return continent + "/" + rest, nil
		}
		if c == continent {
			return c, nil
		}
	}
	// Not a known continent prefix. This is either a sub-continent slug
	// we treat as its own root (e.g. "south-america") — already handled
	// above — or malformed input. Return an error so callers fall back
	// to catalog lookup.
	return "", ErrInvalidChars
}

// GeofabrikContinents is the list of top-level continent slugs
// Geofabrik publishes (from https://download.geofabrik.de/). These
// double-hyphenated slugs (e.g. "australia-oceania", "north-america")
// are treated as single logical continents by the reverse mapper.
var GeofabrikContinents = []string{
	"africa",
	"antarctica",
	"asia",
	"australia-oceania",
	"central-america",
	"europe",
	"north-america",
	"russia",
	"south-america",
}
