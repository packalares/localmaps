package api

// Handlers for the `share` tag in contracts/openapi.yaml.
//
// POST /api/links        → create a ShortLink and return it verbatim.
// GET  /api/links/:code  → 301 redirect to the stored URL; 404 when
//                          missing or expired. (The openapi ErrorCode
//                          enum has no LINK_EXPIRED value, so expiry
//                          surfaces as NOT_FOUND — see docs/04-data-model
//                          note: expiry is derived from the
//                          `share.shortLinkTTLDays` setting, not stored
//                          per-row.)
//
// /og/preview.png is implemented in og.go (Agent Q).
// /embed          is implemented in embed.go (Agent P).

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/shortlinks"
)

// linksStore is the narrow behaviour the create/resolve handlers need
// from shortlinks.Store. An interface keeps the handlers unit-testable
// without spinning up a real SQLite.
type linksStore interface {
	Create(ctx context.Context, rawURL string) (shortlinks.ShortLink, error)
	Resolve(ctx context.Context, code string, ttlDays int) (shortlinks.ShortLink, error)
	IncrementViews(ctx context.Context, code string) error
}

// shareSettings reads the `share.*` config keys the handlers consume.
type shareSettings interface {
	GetString(key string) (string, error)
	GetInt(key string) (int, error)
}

// ShareHTTP bundles the per-request handlers around a shortlinks.Store
// and the config.Store.
type ShareHTTP struct {
	Store    linksStore
	Settings shareSettings
}

// shareHTTP is the process-wide instance used by the package-level
// handler funcs below. router.Register() populates it via
// setShareHTTPFromStore at app boot.
var shareHTTP *ShareHTTP

// setShareHTTPFromStore wires the share handler against a live
// config.Store. nil store → handlers keep returning 501 (stub mode).
// Called from router.go Register(); kept small so tests can also swap
// in an in-memory fake via setShareHTTP().
func setShareHTTPFromStore(store *config.Store) {
	if store == nil {
		shareHTTP = nil
		return
	}
	shareHTTP = &ShareHTTP{
		Store:    shortlinks.New(store.DB()),
		Settings: store,
	}
}

// setShareHTTP lets tests (share_test.go) inject a fake ShareHTTP and
// restore the previous one via the returned closure.
func setShareHTTP(sh *ShareHTTP) func() {
	prev := shareHTTP
	shareHTTP = sh
	return func() { shareHTTP = prev }
}

// POST /api/links — wired in router.go.
func linksCreateHandler(c fiber.Ctx) error {
	if shareHTTP == nil {
		return notImplemented(c)
	}
	return shareHTTP.create(c)
}

// GET /api/links/{code} — wired in router.go.
func linksResolveHandler(c fiber.Ctx) error {
	if shareHTTP == nil {
		return notImplemented(c)
	}
	return shareHTTP.resolve(c)
}

type createLinkRequest struct {
	URL string `json:"url"`
}

// create — POST /api/links. Body {"url": "..."}. 201 → ShortLink.
//
// Security: the URL must be same-origin with `share.ogBaseURL`, or a
// root-relative path. This forecloses the open-redirect attack where a
// caller POSTs {"url":"https://evil.example"} and lures a user into
// clicking the resulting (same-origin) short URL.
func (h *ShareHTTP) create(c fiber.Ctx) error {
	var body createLinkRequest
	if err := c.Bind().JSON(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		return apierr.Write(c, apierr.CodeBadRequest,
			"request body must include a non-empty 'url'", false)
	}
	origin, _ := h.Settings.GetString("share.ogBaseURL")
	if !isSafeShareURL(body.URL, origin) {
		return apierr.Write(c, apierr.CodeBadRequest,
			"url must be same-origin with share.ogBaseURL or a relative path",
			false)
	}
	link, err := h.Store.Create(c.Context(), body.URL)
	if err != nil {
		if errors.Is(err, shortlinks.ErrCodeCollision) {
			return apierr.Write(c, apierr.CodeInternal,
				"failed to allocate a unique code; please retry", true)
		}
		return apierr.Write(c, apierr.CodeInternal,
			"failed to create short link", true)
	}
	return c.Status(fiber.StatusCreated).JSON(link)
}

// resolve — GET /api/links/{code}. 301 redirect per openapi.yaml.
func (h *ShareHTTP) resolve(c fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return apierr.Write(c, apierr.CodeNotFound, "link not found", false)
	}
	ttl, _ := h.Settings.GetInt("share.shortLinkTTLDays")
	link, err := h.Store.Resolve(c.Context(), code, ttl)
	if err != nil {
		switch {
		case errors.Is(err, shortlinks.ErrNotFound),
			errors.Is(err, shortlinks.ErrExpired):
			return apierr.Write(c, apierr.CodeNotFound, "link not found", false)
		default:
			return apierr.Write(c, apierr.CodeInternal,
				"failed to resolve short link", true)
		}
	}
	// Best-effort view bump — a failing update must not block the
	// user's redirect.
	_ = h.Store.IncrementViews(c.Context(), code)
	return c.Redirect().Status(fiber.StatusMovedPermanently).To(link.URL)
}

// isSafeShareURL returns true when target is either a root-relative
// path or an absolute URL whose scheme+host exactly match ogBaseURL.
// Protocol-relative "//host" and CRLF injection attempts are rejected.
func isSafeShareURL(target, ogBaseURL string) bool {
	t := strings.TrimSpace(target)
	if t == "" {
		return false
	}
	if strings.ContainsAny(t, "\r\n") {
		return false
	}
	// Root-relative paths pass — but guard against "//host" which is
	// protocol-relative and would escape the origin when browsers see it.
	if strings.HasPrefix(t, "/") && !strings.HasPrefix(t, "//") {
		return true
	}
	u, err := url.Parse(t)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if ogBaseURL == "" {
		return false
	}
	base, err := url.Parse(ogBaseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, base.Scheme) &&
		strings.EqualFold(u.Host, base.Host)
}
