package api

// Handlers for the `geocode` tag in contracts/openapi.yaml.

import "github.com/gofiber/fiber/v3"

// GET /api/geocode/autocomplete
func geocodeAutocompleteHandler(c fiber.Ctx) error { return notImplemented(c) }

// GET /api/geocode/search
func geocodeSearchHandler(c fiber.Ctx) error { return notImplemented(c) }

// GET /api/geocode/reverse
func geocodeReverseHandler(c fiber.Ctx) error { return notImplemented(c) }
