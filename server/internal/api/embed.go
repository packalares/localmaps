package api

// Handler for GET /embed (contracts/openapi.yaml, tag "share").
//
// Phase-5 Agent P implementation:
//   1. Validates query params (lat/lon/zoom/pin/style/region).
//   2. Emits the third-party-embed security headers from docs/08-security.md:
//      - Content-Security-Policy with dynamic `frame-ancestors` built from
//        `share.embedAllowedOrigins` (docs/07-config-schema.md).
//      - Referrer-Policy: no-referrer.
//      - Permissions-Policy: geolocation=(self).
//      - NO X-Frame-Options (modern browsers prefer frame-ancestors).
//      - NO Set-Cookie.
//   3. If LOCALMAPS_UI_ORIGIN is set, redirects to the Next.js /embed route
//      preserving the validated query string. Otherwise serves a tiny inline
//      HTML shell that points back at the same origin's `/embed` UI bundle.
//
// The helpers (validators, CSP builder, HTML shell) live in
// embed_helpers.go so each file stays under the per-file size cap.

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/config"
)

// EmbedValidatedParams is the parsed + validated query-string state. Kept
// package-private because only the handler consumes it; exposed here for
// tests in the same package.
type EmbedValidatedParams struct {
	Lat    float64
	Lon    float64
	Zoom   float64
	Pin    string // raw "lat,lon[:label]" (validated)
	Style  string // "" when unset — caller falls back to settings default
	Region string // canonical hyphen key or "" when unset
}

// newEmbedHandler builds the GET /embed handler, closing over the config
// store so each request can read `share.embedAllowedOrigins` live without
// re-wiring on settings changes.
func newEmbedHandler(store *config.Store) fiber.Handler {
	return func(c fiber.Ctx) error {
		p, err := parseEmbedParams(c)
		if err != nil {
			return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
		}

		origins := readEmbedAllowedOrigins(store)
		applyEmbedSecurityHeaders(c, origins)

		uiOrigin := strings.TrimRight(os.Getenv("LOCALMAPS_UI_ORIGIN"), "/")
		target := buildEmbedQuery(p)
		if uiOrigin != "" {
			return c.Redirect().Status(fiber.StatusFound).
				To(uiOrigin + "/embed?" + target)
		}

		c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
		return c.SendString(renderEmbedShell(target))
	}
}

// parseEmbedParams pulls the accepted query parameters and validates each
// one. Returns a user-safe error message on the first failure.
func parseEmbedParams(c fiber.Ctx) (*EmbedValidatedParams, error) {
	p := &EmbedValidatedParams{}

	if raw := c.Query("lat"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < -90 || v > 90 {
			return nil, fmt.Errorf("lat must be a number in [-90, 90]")
		}
		p.Lat = v
	}
	if raw := c.Query("lon"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < -180 || v > 180 {
			return nil, fmt.Errorf("lon must be a number in [-180, 180]")
		}
		p.Lon = v
	}
	if raw := c.Query("zoom"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < 0 || v > 22 {
			return nil, fmt.Errorf("zoom must be a number in [0, 22]")
		}
		p.Zoom = v
	}
	if raw := c.Query("pin"); raw != "" {
		if err := validatePin(raw); err != nil {
			return nil, err
		}
		p.Pin = raw
	}
	if raw := c.Query("style"); raw != "" {
		if _, ok := allowedEmbedStyles[raw]; !ok {
			return nil, fmt.Errorf("style must be one of light, dark, print")
		}
		p.Style = raw
	}
	if raw := c.Query("region"); raw != "" {
		if !isCanonicalRegion(raw) {
			return nil, fmt.Errorf("region must be a canonical hyphenated key")
		}
		p.Region = raw
	}
	return p, nil
}
