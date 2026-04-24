// Package geocode will proxy `/api/geocode/*` requests to the internal
// Pelias API per docs/01-architecture.md. Phase 1 stubs the package.
package geocode

// Client is the interface the HTTP layer will call.
type Client interface {
	// Phase 2 will flesh this out — see contracts/openapi.yaml.
}
