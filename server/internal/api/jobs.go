package api

// Handlers for the `jobs` tag in contracts/openapi.yaml. The WS
// handler is wired from router.go to the ws.Hub directly.

import "github.com/gofiber/fiber/v3"

// GET /api/jobs/{jobId}
func jobsGetHandler(c fiber.Ctx) error { return notImplemented(c) }
