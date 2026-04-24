// Package tiles will proxy Mapbox Vector Tile requests (`/api/tiles/*`,
// `/api/sprites/*`, `/api/glyphs/*`) to the internal go-pmtiles server
// as described in docs/01-architecture.md. Phase 1 stubs the package;
// the actual client is implemented in Phase 2.
package tiles

// Client is the interface the HTTP layer will call. Phase 2 fills in
// the concrete type that speaks to a `go-pmtiles serve` process.
type Client interface {
	// Phase 2 will flesh this out — see docs/03-contracts.md.
}
