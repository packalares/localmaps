// Package catalog holds the wire types for the Geofabrik region
// catalog. It lives under the shared `internal/` root so that both the
// gateway (which proxies the catalog to the UI) and the worker (which
// fetches the upstream JSON) can reference the same structs without
// crossing Go's internal-package boundary.
//
// Field names and JSON tags MUST match the RegionCatalogEntry schema in
// contracts/openapi.yaml exactly.
package catalog

import "context"

// Kind is the enum used by RegionCatalogEntry.kind in openapi.yaml.
type Kind string

const (
	// KindContinent is the top-level Geofabrik folder (no parent).
	KindContinent Kind = "continent"
	// KindCountry is a country-level extract beneath a continent.
	KindCountry Kind = "country"
	// KindSubregion is a state/province/county beneath a country.
	KindSubregion Kind = "subregion"
)

// Entry mirrors the RegionCatalogEntry schema in contracts/openapi.yaml.
type Entry struct {
	Name                string   `json:"name"`
	DisplayName         string   `json:"displayName"`
	Kind                Kind     `json:"kind"`
	Parent              *string  `json:"parent,omitempty"`
	SourceURL           string   `json:"sourceUrl"`
	SourcePbfBytes      *int64   `json:"sourcePbfBytes,omitempty"`
	ISO31661            *string  `json:"iso3166_1,omitempty"`
	EstimatedBuildBytes *int64   `json:"estimatedBuildBytes,omitempty"`
	Children            []Entry  `json:"children,omitempty"`
}

// Reader is the narrow interface the gateway uses to talk to whichever
// concrete catalog implementation is wired in. The worker's geofabrik
// package implements it against the upstream HTTP endpoint.
type Reader interface {
	// ListRegions returns (possibly cached) catalog tree.
	ListRegions(ctx context.Context) ([]Entry, error)
	// Resolve returns a single entry for the given canonical region key.
	Resolve(ctx context.Context, canonicalKey string) (Entry, error)
}
