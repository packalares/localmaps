package api

// Handlers for the `routing` tag in contracts/openapi.yaml. Everything
// here is a thin adapter between the Fiber request/response cycle and
// a *routing.Client (which speaks to the Valhalla HTTP API).

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/routing"
)

// routingClient is the process-wide Valhalla proxy. router.Register()
// populates it via setRoutingClientFromBoot at boot time. Left as nil,
// the handlers below keep their 501 stub behaviour — which means unit
// tests that wire an api.Deps{} without a Boot still boot cleanly.
var routingClient *routing.Client

// setRoutingClientFromBoot constructs a Client from the Boot config.
// Called once from router.Register(); passes a nil Client when Boot is
// nil (phase-1 tests) so stubs stay 501.
func setRoutingClientFromBoot(valhallaURL string) {
	if strings.TrimSpace(valhallaURL) == "" {
		routingClient = nil
		return
	}
	routingClient = routing.NewClient(valhallaURL)
}

// traceIDOrEmpty pulls the trace id out of Fiber locals (set by the
// telemetry middleware).
func traceIDOrEmpty(c fiber.Ctx) string {
	if v := c.Locals("traceId"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// mapUpstreamErr translates a routing.Client error into the apierr
// envelope. 4xx-from-upstream → BAD_REQUEST (we sent a bad body);
// anything else → UPSTREAM_UNAVAILABLE (retryable).
func mapUpstreamErr(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, routing.ErrUpstreamBadRequest):
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	case errors.Is(err, routing.ErrUpstreamUnavailable):
		return apierr.Write(c, apierr.CodeUpstreamUnavailable,
			"routing upstream unavailable", true)
	default:
		return apierr.Write(c, apierr.CodeInternal, err.Error(), true)
	}
}

// POST /api/route
func routeHandler(c fiber.Ctx) error {
	if routingClient == nil {
		return notImplemented(c)
	}
	var body routing.RouteRequest
	if err := c.Bind().JSON(&body); err != nil {
		return apierr.Write(c, apierr.CodeBadRequest,
			"invalid request body: "+err.Error(), false)
	}
	if len(body.Locations) < 2 {
		return apierr.Write(c, apierr.CodeBadRequest,
			"locations must contain at least 2 entries", false)
	}
	if !routing.ValidMode(string(body.Mode)) {
		return apierr.Write(c, apierr.CodeBadRequest,
			"mode must be one of auto|bicycle|pedestrian|truck", false)
	}
	resp, err := routingClient.Route(c.Context(), body, traceIDOrEmpty(c))
	if err != nil {
		return mapUpstreamErr(c, err)
	}
	return c.JSON(resp)
}

// GET /api/route/{id}/gpx
func routeGPXHandler(c fiber.Ctx) error {
	if routingClient == nil {
		return notImplemented(c)
	}
	id := c.Params("id")
	cr, ok := routingClient.LookupRoute(id)
	if !ok {
		return apierr.Write(c, apierr.CodeNotFound,
			"route id not found; route cache may have expired", false)
	}
	c.Set(fiber.HeaderContentType, "application/gpx+xml")
	return c.Send(routing.FormatGPX(cr))
}

// GET /api/route/{id}/kml
func routeKMLHandler(c fiber.Ctx) error {
	if routingClient == nil {
		return notImplemented(c)
	}
	id := c.Params("id")
	cr, ok := routingClient.LookupRoute(id)
	if !ok {
		return apierr.Write(c, apierr.CodeNotFound,
			"route id not found; route cache may have expired", false)
	}
	c.Set(fiber.HeaderContentType, "application/vnd.google-earth.kml+xml")
	return c.Send(routing.FormatKML(cr))
}

// POST /api/matrix
func matrixHandler(c fiber.Ctx) error {
	if routingClient == nil {
		return notImplemented(c)
	}
	var body routing.MatrixRequest
	if err := c.Bind().JSON(&body); err != nil {
		return apierr.Write(c, apierr.CodeBadRequest,
			"invalid request body: "+err.Error(), false)
	}
	if len(body.Sources) == 0 || len(body.Targets) == 0 {
		return apierr.Write(c, apierr.CodeBadRequest,
			"sources and targets must be non-empty", false)
	}
	if !routing.ValidMode(string(body.Mode)) {
		return apierr.Write(c, apierr.CodeBadRequest,
			"mode must be one of auto|bicycle|pedestrian|truck", false)
	}
	resp, err := routingClient.Matrix(c.Context(), body, traceIDOrEmpty(c))
	if err != nil {
		return mapUpstreamErr(c, err)
	}
	return c.JSON(resp)
}

// POST /api/isochrone
func isochroneHandler(c fiber.Ctx) error {
	if routingClient == nil {
		return notImplemented(c)
	}
	var body routing.IsochroneRequest
	if err := c.Bind().JSON(&body); err != nil {
		return apierr.Write(c, apierr.CodeBadRequest,
			"invalid request body: "+err.Error(), false)
	}
	if len(body.ContoursSeconds) == 0 {
		return apierr.Write(c, apierr.CodeBadRequest,
			"contoursSeconds must be non-empty", false)
	}
	raw, err := routingClient.Isochrone(c.Context(), body)
	if err != nil {
		return mapUpstreamErr(c, err)
	}
	// Forward Valhalla's GeoJSON verbatim — it already matches our
	// IsochroneResponse contract.
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send([]byte(raw))
}

