// Package api implements the HTTP handlers registered on the Fiber
// router. Every route MUST correspond to a path in
// contracts/openapi.yaml; agents are forbidden from inventing endpoints
// (docs/06-agent-rules.md R2).
//
// In Phase 1 every handler is a stub that returns 501 with an
// ErrorResponse body. The router is the single source of registration.
package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
)

// notImplemented is the Phase-1 placeholder body every stub returns.
// Phase 2+ agents replace individual stubs with real handlers.
func notImplemented(c fiber.Ctx) error {
	return apierr.WriteWithStatus(c, fiber.StatusNotImplemented,
		apierr.CodeInternal, "not yet implemented", false)
}
