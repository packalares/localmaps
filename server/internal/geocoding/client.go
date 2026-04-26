// Package geocoding proxies /api/geocode/* and /api/pois/* requests to
// the in-pod pelias-api container (docs/01-architecture.md) and
// reshapes the responses into the GeocodeResult / Poi schemas defined
// in contracts/openapi.yaml.
//
// The package is intentionally side-effect-free: router.Register builds
// a Client at boot from Boot.PeliasURL and calls it per request. Tests
// mount an httptest.Server and inject the URL.
//
// Pelias-api returns GeoJSON FeatureCollection with a `features[]` array
// of `{type:"Feature", geometry:{coordinates:[lon,lat]}, properties:{…}}`.
// We flatten each feature to a GeocodeResult / Poi record.
package geocoding

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LatLon mirrors the OpenAPI LatLon schema. Kept local to avoid a
// server/internal/api ↔ geocoding import cycle.
type LatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// GeocodeResult mirrors the GeocodeResult OpenAPI schema.
type GeocodeResult struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Category   *string           `json:"category,omitempty"`
	Address    map[string]string `json:"address,omitempty"`
	Center     LatLon            `json:"center"`
	BBox       []float64         `json:"bbox,omitempty"`
	Confidence float64           `json:"confidence"`
	Region     *string           `json:"region,omitempty"`
}

// Poi mirrors the Poi OpenAPI schema. `Source` is always "osm" because
// the in-process indexer only feeds OSM docs into pelias-es.
//
// The Hours/Phone/Website/… fields surface high-value OSM enrichment
// tags the indexer stashes under `addendum.osm` (see worker's
// peliasindex.extractAddendum). They are optional — empty when the
// underlying POI has no such tag — and drive the POI detail card in
// the UI.
type Poi struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Category   *string           `json:"category,omitempty"`
	Center     LatLon            `json:"center"`
	Tags       map[string]string `json:"tags,omitempty"`
	Source     string            `json:"source"`
	Region     *string           `json:"region,omitempty"`
	Hours      string            `json:"hours,omitempty"`
	Phone      string            `json:"phone,omitempty"`
	Website    string            `json:"website,omitempty"`
	Wheelchair string            `json:"wheelchair,omitempty"`
	Cuisine    string            `json:"cuisine,omitempty"`
	Brand      string            `json:"brand,omitempty"`
	Email      string            `json:"email,omitempty"`
}

// Errors surfaced to callers so handlers can pick the right apierr code.
var (
	// ErrUpstreamUnavailable wraps network / 5xx / decode failures.
	ErrUpstreamUnavailable = errors.New("pelias upstream unavailable")
	// ErrUpstreamBadRequest wraps 4xx responses (malformed query, etc).
	ErrUpstreamBadRequest = errors.New("pelias rejected the request")
	// ErrNotFound is returned when a /place lookup produces no features.
	ErrNotFound = errors.New("pelias feature not found")
)

// Client talks HTTP to a pelias-api server. esURL is the optional
// pelias-es base URL used for the bbox-only POI query path (see
// QueryPois) — pelias-api's /v1/search rejects a `text=*` wildcard, so
// the truly-empty query falls back to a direct ES `_search` call.
type Client struct {
	baseURL string
	esURL   string
	http    *http.Client
	timeout time.Duration
}

// NewClient builds a Client pointing at baseURL (trailing slash stripped).
// Pass an empty string to fall back to http://pelias-api:4000.
func NewClient(baseURL string) *Client {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if u == "" {
		u = "http://pelias-api:4000"
	}
	return &Client{
		baseURL: u,
		esURL:   "http://pelias-es:9200",
		http:    &http.Client{Timeout: 20 * time.Second},
		timeout: 10 * time.Second,
	}
}

// SetESURL overrides the pelias-es endpoint used for the bbox-only POI
// fallback. Whitespace + trailing slashes are stripped; an empty
// argument is ignored (keeps the default). Boot wires this from
// LOCALMAPS_PELIAS_ES_URL via the api router.
func (c *Client) SetESURL(esURL string) {
	u := strings.TrimRight(strings.TrimSpace(esURL), "/")
	if u == "" {
		return
	}
	c.esURL = u
}

// BaseURL returns the normalized pelias-api URL (used by tests + logs).
func (c *Client) BaseURL() string { return c.baseURL }

// ESURL returns the normalised pelias-es URL (used by tests).
func (c *Client) ESURL() string { return c.esURL }

// AutocompleteParams matches the /api/geocode/autocomplete OpenAPI
// parameters. The `focus.{lat,lon}` fields are optional (nil ⇒ omit).
type AutocompleteParams struct {
	Text     string
	FocusLat *float64
	FocusLon *float64
	Size     int
	Lang     string
}

// SearchParams matches /api/geocode/search.
type SearchParams struct {
	Text     string
	FocusLat *float64
	FocusLon *float64
	Size     int
}

// ReverseParams matches /api/geocode/reverse.
type ReverseParams struct {
	Lat float64
	Lon float64
}

// PoiQueryParams matches /api/pois. Any zero field is omitted from the
// upstream request. Pelias treats bbox via four boundary.rect.* knobs.
type PoiQueryParams struct {
	BBoxMinLon *float64
	BBoxMinLat *float64
	BBoxMaxLon *float64
	BBoxMaxLat *float64
	Text       string
	Category   string
	Size       int
}

// Autocomplete proxies req to pelias /v1/autocomplete and flattens the
// FeatureCollection body to a slice of GeocodeResult.
func (c *Client) Autocomplete(ctx context.Context, p AutocompleteParams) ([]GeocodeResult, error) {
	q := url.Values{}
	q.Set("text", p.Text)
	if p.FocusLat != nil && p.FocusLon != nil {
		q.Set("focus.point.lat", strconv.FormatFloat(*p.FocusLat, 'f', -1, 64))
		q.Set("focus.point.lon", strconv.FormatFloat(*p.FocusLon, 'f', -1, 64))
	}
	if p.Size > 0 {
		q.Set("size", strconv.Itoa(p.Size))
	}
	if p.Lang != "" {
		q.Set("lang", p.Lang)
	}
	fc, err := c.getFC(ctx, "/v1/autocomplete", q)
	if err != nil {
		return nil, err
	}
	return flattenGeocode(fc.Features), nil
}

// Search proxies req to pelias /v1/search.
func (c *Client) Search(ctx context.Context, p SearchParams) ([]GeocodeResult, error) {
	q := url.Values{}
	q.Set("text", p.Text)
	if p.FocusLat != nil && p.FocusLon != nil {
		q.Set("focus.point.lat", strconv.FormatFloat(*p.FocusLat, 'f', -1, 64))
		q.Set("focus.point.lon", strconv.FormatFloat(*p.FocusLon, 'f', -1, 64))
	}
	if p.Size > 0 {
		q.Set("size", strconv.Itoa(p.Size))
	}
	fc, err := c.getFC(ctx, "/v1/search", q)
	if err != nil {
		return nil, err
	}
	return flattenGeocode(fc.Features), nil
}

// Reverse proxies req to pelias /v1/reverse and returns the nearest
// feature. Returns ErrNotFound when the upstream response has no
// features.
func (c *Client) Reverse(ctx context.Context, p ReverseParams) (GeocodeResult, error) {
	q := url.Values{}
	q.Set("point.lat", strconv.FormatFloat(p.Lat, 'f', -1, 64))
	q.Set("point.lon", strconv.FormatFloat(p.Lon, 'f', -1, 64))
	// Prefer real street addresses over the nearest POI / street.
	// Pelias scans the listed layers in order and returns the closest
	// match, so a building with `addr:housenumber` wins over a fast-food
	// joint sitting two metres away.
	q.Set("layers", "address,venue,street")
	fc, err := c.getFC(ctx, "/v1/reverse", q)
	if err != nil {
		return GeocodeResult{}, err
	}
	results := flattenGeocode(fc.Features)
	if len(results) == 0 {
		return GeocodeResult{}, ErrNotFound
	}
	return results[0], nil
}

// QueryPois proxies req to pelias /v1/search with a category filter
// and boundary rect, then filters to venue-layer features.
//
// Pelias's /v1/search rejects `text=*` (the old wildcard idea triggers
// a 400). To accept the three real shapes the UI sends:
//
//   - text supplied                       → forward verbatim.
//   - text empty, category supplied       → use the category as the
//     text (pelias matches docs whose category contains the token,
//     scored against the same path the categories filter uses).
//   - text empty, category empty (bbox    → /v1/search has no
//     only)                                workable wildcard, so fall
//     back to a direct pelias-es _search
//     POST that filters by layer=venue +
//     geo_bounding_box on center_point.
func (c *Client) QueryPois(ctx context.Context, p PoiQueryParams) ([]Poi, error) {
	text := strings.TrimSpace(p.Text)
	category := strings.TrimSpace(p.Category)

	// Pelias-api requires libpostal for any text query. Without that
	// sidecar (which we don't deploy — too heavy) /v1/search returns 400
	// for all category-chip queries. Bypass pelias-api entirely for the
	// chip use case: when there's no free-text query, query ES directly.
	// This keeps text-search (autocomplete + free-text) on pelias-api
	// where the analyzer chain still works.
	if text == "" {
		return c.queryPoisDirectES(ctx, p)
	}

	// Bbox-only branch — no usable text token available. Pelias's
	// /v1/search rejects an empty / "*" text; /v1/place doesn't accept
	// boundary filters either. Fall through to direct ES.
	if text == "" && category == "" {
		return c.queryPoisDirectES(ctx, p)
	}

	q := url.Values{}
	if text == "" {
		// text empty, category supplied — synthesise text from the
		// category token. Pelias treats this as a normal query against
		// the indexed category strings; combined with the categories
		// filter below it yields the same result set as a true
		// "show me everything in this category" intent.
		text = category
	}
	q.Set("text", text)
	q.Set("layers", "venue")
	if category != "" {
		q.Set("categories", category)
	}
	if p.BBoxMinLon != nil && p.BBoxMinLat != nil && p.BBoxMaxLon != nil && p.BBoxMaxLat != nil {
		q.Set("boundary.rect.min_lon", strconv.FormatFloat(*p.BBoxMinLon, 'f', -1, 64))
		q.Set("boundary.rect.min_lat", strconv.FormatFloat(*p.BBoxMinLat, 'f', -1, 64))
		q.Set("boundary.rect.max_lon", strconv.FormatFloat(*p.BBoxMaxLon, 'f', -1, 64))
		q.Set("boundary.rect.max_lat", strconv.FormatFloat(*p.BBoxMaxLat, 'f', -1, 64))
	}
	if p.Size > 0 {
		q.Set("size", strconv.Itoa(p.Size))
	}
	fc, err := c.getFC(ctx, "/v1/search", q)
	if err != nil {
		return nil, err
	}
	out := make([]Poi, 0, len(fc.Features))
	for _, f := range fc.Features {
		if strings.EqualFold(f.Properties.Layer, "venue") {
			out = append(out, flattenPoi(f))
		}
	}
	return out, nil
}

// Place proxies req to pelias /v1/place?ids=<id> and returns the first
// feature reshaped as a Poi. ErrNotFound when the response is empty.
func (c *Client) Place(ctx context.Context, id string) (Poi, error) {
	q := url.Values{}
	q.Set("ids", id)
	fc, err := c.getFC(ctx, "/v1/place", q)
	if err != nil {
		return Poi{}, err
	}
	if len(fc.Features) == 0 {
		return Poi{}, ErrNotFound
	}
	return flattenPoi(fc.Features[0]), nil
}
