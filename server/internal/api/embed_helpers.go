package api

// Helpers for the GET /embed handler in embed.go. Factored out so each
// file stays under the 250-line cap from docs/06-agent-rules.md.

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/config"
)

// allowedEmbedStyles is the small enum of styles the gateway will accept on
// the query string. Matches the three named styles served by /api/styles/.
var allowedEmbedStyles = map[string]struct{}{
	"light": {},
	"dark":  {},
	"print": {},
}

// validatePin parses "lat,lon[:label]" and ensures the coordinates are in
// WGS84 bounds. The label is free-form UTF-8 up to 120 chars; stripped of
// control characters so it's safe to render inline.
func validatePin(raw string) error {
	coord, label, _ := strings.Cut(raw, ":")
	parts := strings.Split(coord, ",")
	if len(parts) != 2 {
		return fmt.Errorf("pin must be 'lat,lon' or 'lat,lon:label'")
	}
	lat, errLat := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lon, errLon := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errLat != nil || errLon != nil {
		return fmt.Errorf("pin coordinates must be numbers")
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return fmt.Errorf("pin coordinates must be within WGS84 bounds")
	}
	if len(label) > 120 {
		return fmt.Errorf("pin label must be <=120 characters")
	}
	for _, r := range label {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("pin label must not contain control characters")
		}
	}
	return nil
}

// isCanonicalRegion tests the string against the same canonical-key shape
// the UI enforces (`region-subregion`). We only accept lowercase a-z0-9
// separated by single hyphens, and the total length is bounded to keep
// URLs compact.
func isCanonicalRegion(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	prevHyphen := true
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			prevHyphen = false
		case r == '-':
			if prevHyphen {
				return false
			}
			prevHyphen = true
		default:
			return false
		}
	}
	return !prevHyphen
}

// readEmbedAllowedOrigins returns the configured origins, defaulting to the
// wildcard list when the setting is missing (matches docs/07).
func readEmbedAllowedOrigins(store *config.Store) []string {
	var origins []string
	if store != nil {
		if err := store.Get("share.embedAllowedOrigins", &origins); err == nil {
			return origins
		}
	}
	return []string{"*"}
}

// applyEmbedSecurityHeaders writes the CSP + companion headers described in
// docs/08-security.md. `frame-ancestors` is built from the provided origins;
// a wildcard list yields `frame-ancestors *` and omits X-Frame-Options.
func applyEmbedSecurityHeaders(c fiber.Ctx, origins []string) {
	ancestors := buildFrameAncestors(origins)
	csp := strings.Join([]string{
		"default-src 'self'",
		"img-src 'self' data: blob:",
		"style-src 'self' 'unsafe-inline'",
		"script-src 'self' 'unsafe-inline'",
		"worker-src 'self' blob:",
		"connect-src 'self' ws: wss:",
		"font-src 'self' data:",
		"frame-ancestors " + ancestors,
	}, "; ")
	c.Set(fiber.HeaderContentSecurityPolicy, csp)
	c.Set(fiber.HeaderReferrerPolicy, "no-referrer")
	c.Set("Permissions-Policy", "geolocation=(self)")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
}

// buildFrameAncestors turns the settings array into a CSP ancestor list. A
// single `*` entry collapses to a wildcard; otherwise each origin is
// rendered verbatim. Entries are never quoted — CSP origins are bare URLs.
func buildFrameAncestors(origins []string) string {
	if len(origins) == 0 {
		return "'none'"
	}
	for _, o := range origins {
		if strings.TrimSpace(o) == "*" {
			return "*"
		}
	}
	cleaned := make([]string, 0, len(origins))
	for _, o := range origins {
		trimmed := strings.TrimSpace(o)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) == 0 {
		return "'none'"
	}
	return strings.Join(cleaned, " ")
}

// buildEmbedQuery re-encodes the validated params into a query string. We
// prefer rebuilding over forwarding `c.Context().QueryArgs()` so any
// un-validated fragment (stray keys, malformed encodings) is dropped.
func buildEmbedQuery(p *EmbedValidatedParams) string {
	q := url.Values{}
	if p.Lat != 0 || p.Lon != 0 || p.Zoom != 0 {
		q.Set("lat", strconv.FormatFloat(p.Lat, 'f', -1, 64))
		q.Set("lon", strconv.FormatFloat(p.Lon, 'f', -1, 64))
	}
	if p.Zoom != 0 {
		q.Set("zoom", strconv.FormatFloat(p.Zoom, 'f', -1, 64))
	}
	if p.Pin != "" {
		q.Set("pin", p.Pin)
	}
	if p.Style != "" {
		q.Set("style", p.Style)
	}
	if p.Region != "" {
		q.Set("region", p.Region)
	}
	return q.Encode()
}

// renderEmbedShell emits the minimal HTML served when no separate UI origin
// is configured. The shell self-refreshes into the bundled /embed route so
// the same gateway binary can host both the API and the UI export.
func renderEmbedShell(query string) string {
	path := "/embed"
	if query != "" {
		path = path + "?" + query
	}
	escaped := htmlEscape(path)
	return `<!DOCTYPE html><html lang="en"><head>` +
		`<meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<meta http-equiv="refresh" content="0;url=` + escaped + `">` +
		`<title>LocalMaps embed</title></head><body>` +
		`<p>Loading map… <a href="` + escaped + `">Open map</a></p>` +
		`</body></html>`
}

// htmlEscape escapes the characters that matter inside an attribute + text.
// We deliberately avoid pulling in html/template for a one-line shell.
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}
