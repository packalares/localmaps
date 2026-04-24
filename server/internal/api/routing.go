package api

// Handlers for the `routing` tag in contracts/openapi.yaml.

import "github.com/gofiber/fiber/v3"

// POST /api/route
func routeHandler(c fiber.Ctx) error { return notImplemented(c) }

// GET /api/route/{id}/gpx
func routeGPXHandler(c fiber.Ctx) error { return notImplemented(c) }

// GET /api/route/{id}/kml
func routeKMLHandler(c fiber.Ctx) error { return notImplemented(c) }

// POST /api/matrix
func matrixHandler(c fiber.Ctx) error { return notImplemented(c) }

// POST /api/isochrone
func isochroneHandler(c fiber.Ctx) error { return notImplemented(c) }
