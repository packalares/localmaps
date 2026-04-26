package api

// router.go wires every path from contracts/openapi.yaml to its
// handler. If a spec path is missing from this file, that's a bug per
// docs/06-agent-rules.md R2 and R10.
//
// NOTE on Fiber v3 signatures: Get/Post/Put/Delete/Patch are
//   (path string, handler Handler, middleware ...Handler)
// — i.e. the FINAL handler is first, middleware comes after. Middleware
// is still executed before the final handler at request time (see
// App.register in fiber v3).

import (
	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/auth"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/ratelimit"
	"github.com/packalares/localmaps/server/internal/regions"
	"github.com/packalares/localmaps/server/internal/telemetry"
	"github.com/packalares/localmaps/server/internal/ws"
)

// Deps bundles everything the router needs to wire handlers and
// middleware. Constructed in cmd/localmaps/main.go.
type Deps struct {
	Boot      *config.Boot
	Store     *config.Store
	Telemetry *telemetry.Telemetry
	Hub       *ws.Hub
	Limiter   *ratelimit.Limiter
	// Regions is optional — when nil, the regions endpoints return 501.
	// The gateway bootstrap builds it once; tests may leave it nil.
	Regions *regions.Service
	// Auth is the session manager. When nil (tests), the middleware
	// falls back to a store-derived manager so admin routes still gate.
	Auth *auth.Manager
}

// Register attaches every handler from the spec to the Fiber app.
// Admin routes also get auth.Require() in their middleware chain.
func Register(app *fiber.App, d Deps) {
	// Cross-cutting middleware: tracer/metrics first so every request
	// carries a traceId and feeds the Prometheus counters.
	app.Use(d.Telemetry.NewTracer())

	// Build an auth manager when the caller didn't pass one (tests,
	// bootstrap-lite paths). All middleware below relies on it.
	mgr := d.Auth
	if mgr == nil && d.Store != nil {
		mgr = BuildManager(d.Store, d.Store.DB())
	}
	app.Use(auth.WithManager(mgr))
	// Auth.Optional reads the cookie and attaches an Identity when
	// present, but does not reject anonymous requests.
	app.Use(auth.Optional(mgr))

	// Wire the auth handlers to the real manager.
	setAuthHandlers(newAuthHandlers(mgr, d.Store))

	// --- meta (public) ---------------------------------------------
	app.Get("/api/health", healthHandler)
	app.Get("/api/ready", newReadyHandler(d.Boot))
	app.Get("/api/version", versionHandler)
	// /metrics is NOT under /api per openapi.yaml — it sits at the root.
	app.Get("/metrics", d.Telemetry.MetricsHandler())

	// --- auth ------------------------------------------------------
	// Login is public but rate-limited; logout is public (it revokes
	// the caller's cookie); /me + change-password + user mgmt require auth.
	loginLimit := d.Limiter.PerIP("auth.rateLimit.loginPerMinutePerIP")
	app.Post("/api/auth/login", authLoginHandler, loginLimit)
	app.Post("/api/auth/logout", authLogoutHandler)
	app.Get("/api/auth/me", authMeHandler)
	app.Post("/api/auth/change-password", authChangePasswordHandler, auth.Require(mgr))
	app.Get("/api/auth/users", authListUsersHandler, auth.Require(mgr, auth.RoleAdmin))
	app.Post("/api/auth/users", authCreateUserHandler, auth.Require(mgr, auth.RoleAdmin))
	app.Delete("/api/auth/users/:id", authDeleteUserHandler, auth.Require(mgr, auth.RoleAdmin))

	// --- map (public, rate-limited) --------------------------------
	tilesLimit := d.Limiter.PerIP("rateLimit.tilesPerMinutePerIP")
	app.Get("/api/tiles/metadata", tileMetadataHandler, tilesLimit)
	// Proxy vector tiles from the in-cluster pmtiles server. The
	// `:region` param selects the archive (see newTileHandler doc).
	protomapsURL := ""
	if d.Boot != nil {
		protomapsURL = d.Boot.ProtomapsURL
	}
	app.Get("/api/tiles/:region/:z/:x/:y.pbf", newTileHandler(protomapsURL), tilesLimit)
	app.Get("/api/styles/:name.json", styleHandler, tilesLimit)
	// Fiber v3's path parser uses /, -, . as segment delimiters, so `@`
	// is NOT a splittable char — a single `:name.:ext` route matches both
	// `default.json` and `default@2x.json`; the handler splits `@2x` out
	// of the name internally.
	app.Get("/api/sprites/:name.:ext", spriteHandler, tilesLimit)
	app.Get("/api/glyphs/:fontstack/:range.pbf", glyphHandler, tilesLimit)

	// --- geocode (public, rate-limited) ----------------------------
	// Wire the pelias-api proxy client from Boot (same pattern as the
	// Valhalla routing client above). A nil Boot keeps the handlers
	// behaving as 501 stubs for tests that don't wire upstreams.
	if d.Boot != nil {
		setGeocodingClientFromBoot(d.Boot.PeliasURL, d.Boot.PeliasESURL)
	} else {
		setGeocodingClientFromBoot("", "")
	}
	geoLimit := d.Limiter.PerIP("rateLimit.geocodePerMinutePerIP")
	app.Get("/api/geocode/autocomplete", geocodeAutocompleteHandler, geoLimit)
	app.Get("/api/geocode/search", geocodeSearchHandler, geoLimit)
	app.Get("/api/geocode/reverse", geocodeReverseHandler, geoLimit)

	// --- routing (public, rate-limited) ----------------------------
	// Wire the Valhalla proxy client from Boot. A nil Boot (tests that
	// construct api.Deps{} with an in-memory store but no upstreams)
	// leaves routingClient nil so handlers keep their 501 stub behaviour.
	if d.Boot != nil {
		setRoutingClientFromBoot(d.Boot.ValhallaURL)
	} else {
		setRoutingClientFromBoot("")
	}
	routeLimit := d.Limiter.PerIP("rateLimit.routePerMinutePerIP")
	app.Post("/api/route", routeHandler, routeLimit)
	app.Get("/api/route/:id/gpx", routeGPXHandler, routeLimit)
	app.Get("/api/route/:id/kml", routeKMLHandler, routeLimit)
	app.Post("/api/matrix", matrixHandler, routeLimit)
	app.Post("/api/isochrone", isochroneHandler, routeLimit)

	// --- pois (public, rate-limited) -------------------------------
	app.Get("/api/pois", poisQueryHandler, geoLimit)
	app.Get("/api/pois/categories", poisCategoriesHandler, geoLimit)
	// Pelias gids contain slashes (e.g. "openstreetmap:venue:node/123"),
	// so we need a greedy wildcard rather than a single-segment param.
	// Fiber v3 captures `+` segments as `*1`; the handler reads it.
	app.Get("/api/pois/+", poisGetHandler, geoLimit)

	// --- regions (mixed public/admin) ------------------------------
	adminLimit := d.Limiter.PerIP("rateLimit.regionsAdminPerMinute")
	rh := regionsRoutes(d.Regions)
	app.Get("/api/regions", rh.regionsListHandler)
	app.Get("/api/regions/catalog", rh.regionsCatalogHandler)
	app.Get("/api/regions/:name", rh.regionsGetHandler)
	app.Post("/api/regions", rh.regionsInstallHandler, adminLimit, auth.Require(mgr, auth.RoleAdmin))
	app.Delete("/api/regions/:name", rh.regionsDeleteHandler, adminLimit, auth.Require(mgr, auth.RoleAdmin))
	app.Post("/api/regions/:name/update", rh.regionsUpdateHandler, adminLimit, auth.Require(mgr, auth.RoleAdmin))
	app.Put("/api/regions/:name/schedule", rh.regionsScheduleHandler, adminLimit, auth.Require(mgr, auth.RoleAdmin))
	app.Post("/api/regions/:name/activate", rh.regionsActivateHandler, adminLimit, auth.Require(mgr, auth.RoleAdmin))

	// --- jobs + WS -------------------------------------------------
	setJobsStore(d.Store)
	app.Get("/api/jobs/:jobId", jobsGetHandler, auth.Require(mgr, auth.RoleAdmin))
	app.Get("/api/ws", d.Hub.Handler())

	// --- settings --------------------------------------------------
	// schema is anonymous (UI needs it to render login screen too);
	// all other settings paths require auth.
	setSettingsStore(d.Store)
	app.Get("/api/settings/schema", settingsSchemaHandler)
	app.Get("/api/settings", settingsGetHandler, auth.Require(mgr, auth.RoleAdmin))
	app.Put("/api/settings", settingsPutHandler, auth.Require(mgr, auth.RoleAdmin))
	app.Patch("/api/settings", settingsPatchHandler, auth.Require(mgr, auth.RoleAdmin))

	// --- share -----------------------------------------------------
	ogLimit := d.Limiter.PerIP("rateLimit.ogPreviewPerMinutePerIP")
	// Wire the short-link handlers against the live config.Store so the
	// package-level linksCreateHandler / linksResolveHandler below route
	// to real implementations. Passing a nil Store keeps them as 501
	// stubs, which lets tests that don't need share still boot cleanly.
	setShareHTTPFromStore(d.Store)
	app.Post("/api/links", linksCreateHandler)
	app.Get("/api/links/:code", linksResolveHandler)
	ogDataDir := ""
	if d.Boot != nil {
		ogDataDir = d.Boot.DataDir
	}
	ogH := newOGHandler(d.Store, ogDataDir, d.Telemetry)
	app.Get("/og/preview.png", ogH.ogPreviewHandler, ogLimit)
	app.Get("/embed", newEmbedHandler(d.Store))
}
