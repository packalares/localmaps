// Package routing will proxy `/api/route*`, `/api/matrix`, `/api/isochrone`
// requests to the internal Valhalla service per docs/01-architecture.md.
// Phase 1 stubs the package.
package routing

// Client is the interface the HTTP layer will call.
type Client interface {
	// Phase 2 will flesh this out — see contracts/openapi.yaml.
}
