package api

// Handler for GET /og/preview.png — defined in contracts/openapi.yaml
// under the `share` tag. Phase 5 Agent Q.
//
// Flow:
//  1. parse query params (lat, lon, zoom, pin, width, height, style,
//     region); validate against the openapi schema bounds.
//  2. compute a sha256 cache key, look under <dataDir>/cache/og/.
//  3. on miss, call og.Render with a deadline from
//     share.ogRenderTimeoutSeconds and write the result to cache.
//  4. stream the PNG with image/png + Cache-Control: public,
//     max-age=<share.ogCacheTTLSeconds>, immutable.
//
// Rate-limiting is already attached in router.go via d.Limiter.PerIP
// reading rateLimit.ogPreviewPerMinutePerIP — no duplication here.
// Auth is unauthenticated by design (bot-friendly preview).
//
// Cache helpers live in og_cache.go; query parsing lives in og_parse.go.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/og"
	"github.com/packalares/localmaps/server/internal/telemetry"
)

// Defaults mirrored from docs/07-config-schema.md so the handler keeps
// working when the settings row is missing (fresh install, in-memory
// test DB, etc.). Do not hardcode anything else.
const (
	defaultOgCacheTTLSeconds  = 604800 // 7 days
	defaultOgRenderTimeoutSec = 10
)

// ogHandler bundles the dependencies the GET /og/preview.png handler
// needs. Constructed once per process in router.go.
type ogHandler struct {
	dataDir string
	store   *config.Store
	render  func(ctx context.Context, p og.Params) ([]byte, error)
	reqs    *prometheus.CounterVec
}

// newOGHandler wires an ogHandler against the live store + telemetry.
// It registers a localmaps_og_requests_total{cache="hit|miss|error"}
// counter on the telemetry registry if one is available.
func newOGHandler(store *config.Store, dataDir string, tel *telemetry.Telemetry) *ogHandler {
	h := &ogHandler{
		dataDir: dataDir,
		store:   store,
		render:  og.Render,
	}
	if tel != nil && tel.Registry != nil {
		h.reqs = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "localmaps_og_requests_total",
			Help: "OpenGraph preview requests by cache outcome.",
		}, []string{"cache"})
		if err := tel.Registry.Register(h.reqs); err != nil {
			// If already registered (shouldn't happen, but tests share
			// processes), fall back to the collector from the error.
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				if cv, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
					h.reqs = cv
				}
			}
		}
	}
	return h
}

// ogPreviewHandler implements GET /og/preview.png.
func (h *ogHandler) ogPreviewHandler(c fiber.Ctx) error {
	params, err := parseOGQuery(c)
	if err != nil {
		h.count("error")
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	}
	ttl := h.readIntSetting("share.ogCacheTTLSeconds", defaultOgCacheTTLSeconds)
	timeout := h.readIntSetting("share.ogRenderTimeoutSeconds", defaultOgRenderTimeoutSec)
	if ttl < 0 {
		ttl = defaultOgCacheTTLSeconds
	}
	if timeout <= 0 {
		timeout = defaultOgRenderTimeoutSec
	}

	key := cacheKey(params)
	path, err := cacheFilePath(h.dataDir, key)
	if err != nil {
		h.count("error")
		return apierr.Write(c, apierr.CodeInternal, "cache path error", false)
	}

	if data, ok := readCache(path); ok {
		h.count("hit")
		return writePNG(c, data, ttl)
	}

	ctx, cancel := context.WithTimeout(c.Context(), time.Duration(timeout)*time.Second)
	defer cancel()

	data, err := h.render(ctx, params)
	if err != nil {
		return h.writeRenderError(c, err)
	}
	// Best-effort cache write — failures don't block the response.
	_ = writeCache(path, data)
	h.count("miss")
	return writePNG(c, data, ttl)
}

// writeRenderError maps a renderer error to the right apierr envelope.
// Split out of the main handler to keep it under the file size cap.
func (h *ogHandler) writeRenderError(c fiber.Ctx, err error) error {
	if errors.Is(err, og.ErrRenderTimeout) || errors.Is(err, context.DeadlineExceeded) {
		h.count("error")
		c.Set("Retry-After", "30")
		return apierr.Write(c, apierr.CodeUpstreamUnavailable,
			"render timed out; please retry", true)
	}
	if errors.Is(err, og.ErrInvalidParams) {
		h.count("error")
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	}
	h.count("error")
	return apierr.Write(c, apierr.CodeInternal, "render failed", true)
}

// count increments the OG counter if wired. Safe when h.reqs is nil
// (test harnesses that pass nil telemetry).
func (h *ogHandler) count(outcome string) {
	if h.reqs != nil {
		h.reqs.WithLabelValues(outcome).Inc()
	}
}

// readIntSetting returns the config value at key, or fallback if
// missing/invalid. Does not log — the caller is on a hot path.
func (h *ogHandler) readIntSetting(key string, fallback int) int {
	if h.store == nil {
		return fallback
	}
	v, err := h.store.GetInt(key)
	if err != nil {
		return fallback
	}
	return v
}

// writePNG sets headers + body for a successful 200 OK.
func writePNG(c fiber.Ctx, data []byte, ttl int) error {
	c.Set("Content-Type", "image/png")
	if ttl > 0 {
		c.Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", ttl))
	} else {
		c.Set("Cache-Control", "no-store")
	}
	return c.Send(data)
}

// newOGHandlerStub is the 501 fallback used when Deps.Store is nil
// (e.g. in early-boot error paths). It lets router.go unconditionally
// call newOGHandler without nil-checks.
func newOGHandlerStub() func(fiber.Ctx) error {
	return func(c fiber.Ctx) error { return notImplemented(c) }
}
