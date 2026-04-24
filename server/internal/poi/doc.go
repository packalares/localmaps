// Package poi will answer `/api/pois*` queries by running DuckDB
// against per-region Overture parquet files (and OSM fallback) as
// described in docs/01-architecture.md. Phase 1 stubs the package.
package poi

// Client is the interface the HTTP layer will call.
type Client interface {
	// Phase 2 will flesh this out — see contracts/openapi.yaml.
}
