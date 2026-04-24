package api

// Handlers for the `regions` tag in contracts/openapi.yaml.
//
// The Fiber layer is intentionally thin — input parsing, error mapping,
// and response shaping — while the real work lives in
// server/internal/regions.Service. Each handler pattern-matches against
// the response schema defined in openapi.yaml.

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/auth"
	"github.com/packalares/localmaps/server/internal/regions"
)

// regionsListHandler — GET /api/regions
func (h *RegionsHTTP) regionsListHandler(c fiber.Ctx) error {
	rows, err := h.Svc.ListInstalled(c.Context())
	if err != nil {
		return apierr.Write(c, apierr.CodeInternal,
			"failed to list regions", true)
	}
	return c.JSON(fiber.Map{"regions": rows})
}

// regionsInstallHandler — POST /api/regions (admin)
func (h *RegionsHTTP) regionsInstallHandler(c fiber.Ctx) error {
	var body struct {
		Name     string  `json:"name"`
		Schedule *string `json:"schedule,omitempty"`
	}
	if err := c.Bind().JSON(&body); err != nil || body.Name == "" {
		return apierr.Write(c, apierr.CodeBadRequest,
			"request body must include a 'name' field", false)
	}
	user := "system"
	if id := auth.FromCtx(c); id != nil {
		user = id.Username
	}
	region, job, err := h.Svc.Install(c.Context(), body.Name, user)
	if err != nil {
		return mapInstallError(c, err)
	}
	if body.Schedule != nil {
		if _, err := h.Svc.SetSchedule(c.Context(), region.Name, *body.Schedule); err != nil {
			// Don't fail the install — the row is already queued.
			// Log through the request logger if set.
		}
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"region": region, "jobId": job.ID,
	})
}

// regionsCatalogHandler — GET /api/regions/catalog
func (h *RegionsHTTP) regionsCatalogHandler(c fiber.Ctx) error {
	tree, err := h.Svc.ListCatalog(c.Context())
	if err != nil {
		return apierr.Write(c, apierr.CodeUpstreamUnavailable,
			fmt.Sprintf("failed to load catalog: %v", err), true)
	}
	return c.JSON(fiber.Map{
		"catalog":   tree,
		"fetchedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

// regionsGetHandler — GET /api/regions/{name}
func (h *RegionsHTTP) regionsGetHandler(c fiber.Ctx) error {
	r, err := h.Svc.Get(c.Context(), c.Params("name"))
	if errors.Is(err, regions.ErrNotFound) {
		return apierr.Write(c, apierr.CodeRegionNotInstalled,
			"region not installed", false)
	}
	if err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	}
	return c.JSON(r)
}

// regionsDeleteHandler — DELETE /api/regions/{name} (admin)
func (h *RegionsHTTP) regionsDeleteHandler(c fiber.Ctx) error {
	user := "system"
	if id := auth.FromCtx(c); id != nil {
		user = id.Username
	}
	region, _, err := h.Svc.Delete(c.Context(), c.Params("name"), user)
	if errors.Is(err, regions.ErrNotFound) {
		return apierr.Write(c, apierr.CodeRegionNotInstalled,
			"region not installed", false)
	}
	if err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	}
	return c.JSON(fiber.Map{"region": region})
}

// regionsUpdateHandler — POST /api/regions/{name}/update (admin)
func (h *RegionsHTTP) regionsUpdateHandler(c fiber.Ctx) error {
	user := "system"
	if id := auth.FromCtx(c); id != nil {
		user = id.Username
	}
	job, err := h.Svc.Update(c.Context(), c.Params("name"), user)
	if errors.Is(err, regions.ErrNotFound) {
		return apierr.Write(c, apierr.CodeRegionNotInstalled,
			"region not installed", false)
	}
	if errors.Is(err, regions.ErrConflict) {
		return apierr.Write(c, apierr.CodeConflict,
			"region is not in a state that permits update", false)
	}
	if err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"jobId": job.ID})
}

// regionsScheduleHandler — PUT /api/regions/{name}/schedule (admin)
func (h *RegionsHTTP) regionsScheduleHandler(c fiber.Ctx) error {
	var body struct {
		Schedule string `json:"schedule"`
	}
	if err := c.Bind().JSON(&body); err != nil || body.Schedule == "" {
		return apierr.Write(c, apierr.CodeBadRequest,
			"request body must include a 'schedule' field", false)
	}
	r, err := h.Svc.SetSchedule(c.Context(), c.Params("name"), body.Schedule)
	if errors.Is(err, regions.ErrNotFound) {
		return apierr.Write(c, apierr.CodeRegionNotInstalled,
			"region not installed", false)
	}
	if errors.Is(err, regions.ErrInvalidSchedule) {
		return apierr.Write(c, apierr.CodeBadRequest,
			"schedule must be never|daily|weekly|monthly or a 5-field cron string", false)
	}
	if err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	}
	return c.JSON(r)
}

// mapInstallError converts regions.Install errors into the correct
// apierr envelope + HTTP status.
func mapInstallError(c fiber.Ctx, err error) error {
	if errors.Is(err, regions.ErrConflict) {
		return apierr.Write(c, apierr.CodeConflict,
			"region is already installed or installing", false)
	}
	// Invalid name / unknown to catalog / resolve failures.
	return apierr.Write(c, apierr.CodeInvalidRegionName, err.Error(), false)
}

// RegionsHTTP bundles the per-request handlers around a *regions.Service.
// Constructed in router.go from the bootstrap Deps; NOT shared with
// other tags.
type RegionsHTTP struct {
	Svc *regions.Service
}

// regionsRoutes returns a RegionsHTTP when svc != nil, otherwise a
// shim whose handlers return 501 — preserves Phase-1 behaviour when
// tests run without a live Service.
func regionsRoutes(svc *regions.Service) regionsRouteSet {
	if svc == nil {
		return regionsStub{}
	}
	return &RegionsHTTP{Svc: svc}
}

// regionsRouteSet is the common interface implemented by both the
// real handlers and the 501 stub. Methods map 1:1 to router entries.
type regionsRouteSet interface {
	regionsListHandler(c fiber.Ctx) error
	regionsInstallHandler(c fiber.Ctx) error
	regionsCatalogHandler(c fiber.Ctx) error
	regionsGetHandler(c fiber.Ctx) error
	regionsDeleteHandler(c fiber.Ctx) error
	regionsUpdateHandler(c fiber.Ctx) error
	regionsScheduleHandler(c fiber.Ctx) error
}

// regionsStub is the 501 fallback when no Service is wired.
type regionsStub struct{}

func (regionsStub) regionsListHandler(c fiber.Ctx) error     { return notImplemented(c) }
func (regionsStub) regionsInstallHandler(c fiber.Ctx) error  { return notImplemented(c) }
func (regionsStub) regionsCatalogHandler(c fiber.Ctx) error  { return notImplemented(c) }
func (regionsStub) regionsGetHandler(c fiber.Ctx) error      { return notImplemented(c) }
func (regionsStub) regionsDeleteHandler(c fiber.Ctx) error   { return notImplemented(c) }
func (regionsStub) regionsUpdateHandler(c fiber.Ctx) error   { return notImplemented(c) }
func (regionsStub) regionsScheduleHandler(c fiber.Ctx) error { return notImplemented(c) }
