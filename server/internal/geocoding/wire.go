// Package geocoding — wire.go holds the pelias-api response types and
// the HTTP plumbing used by client.go. Split out so client.go stays
// under the 250-line cap.
package geocoding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// peliasFC is the subset of pelias-api's GeoJSON FeatureCollection that
// we read. Pelias wraps FeatureCollection with an additional
// "bbox":[...] and "geocoding":{...} envelope we ignore.
type peliasFC struct {
	Features []peliasFeature `json:"features"`
}

// peliasFeature is one entry of the FeatureCollection. We flatten
// geometry.coordinates[0..1] into LatLon at read time.
type peliasFeature struct {
	Type       string          `json:"type"`
	Geometry   peliasGeometry  `json:"geometry"`
	Properties peliasProps     `json:"properties"`
	BBox       []float64       `json:"bbox,omitempty"`
}

type peliasGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// peliasProps is the subset of pelias feature properties we surface.
// Pelias uses snake_case; additional keys we don't model are ignored.
type peliasProps struct {
	ID         string          `json:"id"`
	GID        string          `json:"gid"`
	Layer      string          `json:"layer"`
	Source     string          `json:"source"`
	SourceID   string          `json:"source_id"`
	Name       string          `json:"name"`
	Label      string          `json:"label"`
	Confidence float64         `json:"confidence"`
	Housenumber string         `json:"housenumber,omitempty"`
	Street     string          `json:"street,omitempty"`
	Postalcode string          `json:"postalcode,omitempty"`
	Locality   string          `json:"locality,omitempty"`
	Region     string          `json:"region,omitempty"`
	Country    string          `json:"country,omitempty"`
	Category   []string        `json:"category,omitempty"`
	Addendum   json.RawMessage `json:"addendum,omitempty"`
}

// flattenGeocode converts pelias features → GeocodeResult slice.
func flattenGeocode(fs []peliasFeature) []GeocodeResult {
	out := make([]GeocodeResult, 0, len(fs))
	for _, f := range fs {
		center := extractCenter(f)
		label := f.Properties.Label
		if label == "" {
			label = f.Properties.Name
		}
		id := f.Properties.GID
		if id == "" {
			id = f.Properties.ID
		}
		r := GeocodeResult{
			ID:         id,
			Label:      label,
			Center:     center,
			Confidence: f.Properties.Confidence,
			Address:    extractAddress(f.Properties),
		}
		if cat := primaryCategory(f.Properties.Category); cat != "" {
			c := cat
			r.Category = &c
		}
		if len(f.BBox) == 4 {
			r.BBox = append([]float64(nil), f.BBox...)
		}
		out = append(out, r)
	}
	return out
}

// flattenPoi converts one pelias venue feature → Poi.
func flattenPoi(f peliasFeature) Poi {
	label := f.Properties.Label
	if label == "" {
		label = f.Properties.Name
	}
	id := f.Properties.GID
	if id == "" {
		id = f.Properties.ID
	}
	p := Poi{
		ID:     id,
		Label:  label,
		Center: extractCenter(f),
		Source: "osm",
	}
	if cat := primaryCategory(f.Properties.Category); cat != "" {
		c := cat
		p.Category = &c
	}
	if tags := extractAddress(f.Properties); len(tags) > 0 {
		p.Tags = tags
	}
	if osmAdd := extractOSMAddendum(f.Properties.Addendum); osmAdd != nil {
		p.Hours = osmAdd["opening_hours"]
		p.Phone = osmAdd["phone"]
		p.Website = osmAdd["website"]
		p.Wheelchair = osmAdd["wheelchair"]
		p.Cuisine = osmAdd["cuisine"]
		p.Brand = osmAdd["brand"]
		p.Email = osmAdd["email"]
	}
	return p
}

// extractOSMAddendum decodes feature.properties.addendum and pulls the
// `osm` namespace out. Pelias emits addendum as an object shaped
// `{ "<source>": {...} }`; some importers stringify the inner block,
// so we accept either raw object or JSON-encoded string. Returns nil
// when the addendum is missing / unparseable.
func extractOSMAddendum(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	osmRaw, ok := envelope["osm"]
	if !ok || len(osmRaw) == 0 {
		return nil
	}
	// Case 1: inner block is an object.
	var obj map[string]string
	if err := json.Unmarshal(osmRaw, &obj); err == nil {
		return obj
	}
	// Case 2: pelias stored it as a JSON-encoded string (some
	// importers do this to fit the ES keyword mapping).
	var s string
	if err := json.Unmarshal(osmRaw, &s); err == nil && s != "" {
		_ = json.Unmarshal([]byte(s), &obj)
		return obj
	}
	return nil
}

// extractCenter pulls LatLon out of a GeoJSON Point geometry. Pelias
// always emits [lon, lat] order per GeoJSON spec.
func extractCenter(f peliasFeature) LatLon {
	if len(f.Geometry.Coordinates) >= 2 {
		return LatLon{Lon: f.Geometry.Coordinates[0], Lat: f.Geometry.Coordinates[1]}
	}
	return LatLon{}
}

// extractAddress assembles the pelias address_parts-equivalent map from
// the flat properties pelias emits. Keys follow pelias's own naming so
// the UI can key off a stable vocabulary (housenumber, street, locality,
// region, country, postalcode).
func extractAddress(p peliasProps) map[string]string {
	m := map[string]string{}
	if p.Housenumber != "" {
		m["housenumber"] = p.Housenumber
	}
	if p.Street != "" {
		m["street"] = p.Street
	}
	if p.Postalcode != "" {
		m["postalcode"] = p.Postalcode
	}
	if p.Locality != "" {
		m["locality"] = p.Locality
	}
	if p.Region != "" {
		m["region"] = p.Region
	}
	if p.Country != "" {
		m["country"] = p.Country
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func primaryCategory(cats []string) string {
	if len(cats) == 0 {
		return ""
	}
	return cats[0]
}

// getFC issues GET baseURL+path?q and decodes the response into a
// peliasFC. 4xx → ErrUpstreamBadRequest, 5xx / network → ErrUpstreamUnavailable.
func (c *Client) getFC(ctx context.Context, path string, q url.Values) (*peliasFC, error) {
	raw, err := c.getRaw(ctx, path, q)
	if err != nil {
		return nil, err
	}
	var fc peliasFC
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("%w: decoding pelias response: %v", ErrUpstreamUnavailable, err)
	}
	return &fc, nil
}

// getRaw returns the raw body. Exported only for tests that want to
// inspect the request path (not the response). Stays package-private.
func (c *Client) getRaw(ctx context.Context, path string, q url.Values) ([]byte, error) {
	reqCtx, cancel := contextWithTimeout(ctx, c.timeout)
	defer cancel()
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: building request: %v", ErrUpstreamUnavailable, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: reading response: %v", ErrUpstreamUnavailable, err)
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrUpstreamBadRequest,
			resp.StatusCode, truncate(string(body), 200))
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrUpstreamUnavailable,
			resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// contextWithTimeout wraps context.WithTimeout so request deadlines are
// applied in a single spot.
func contextWithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

// esHit is one entry of pelias-es's `hits.hits[]` array. We decode the
// stored `_source` directly into the same peliasFeature/peliasProps
// shapes used for pelias-api responses so the rest of the client can
// keep its single flatten path.
type esHit struct {
	Source struct {
		Source     string          `json:"source"`
		SourceID   string          `json:"source_id"`
		Layer      string          `json:"layer"`
		Center     struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"center_point"`
		Name struct {
			Default string `json:"default"`
		} `json:"name"`
		Category    []string                     `json:"category,omitempty"`
		AddressPart map[string]string            `json:"address_parts,omitempty"`
		Addendum    map[string]map[string]string `json:"addendum,omitempty"`
	} `json:"_source"`
}

// esResponse is the subset of pelias-es's `_search` body we read.
type esResponse struct {
	Hits struct {
		Hits []esHit `json:"hits"`
	} `json:"hits"`
}

// queryPoisDirectES runs the bbox-only branch of QueryPois against the
// pelias-es index directly. /v1/search rejects an empty / "*" text and
// /v1/place doesn't accept boundary filters, so this is the only way
// to honour a true "show me every venue in this rect" request.
//
// The query is tightly scoped: layer=venue + geo_bounding_box on
// center_point. The function reshapes hits into the same Poi struct
// the pelias-api path returns so the API surface stays uniform.
func (c *Client) queryPoisDirectES(ctx context.Context, p PoiQueryParams) ([]Poi, error) {
	if c.esURL == "" {
		// No ES URL configured — surface as bad-request rather than
		// silently returning empty so callers get a useful error.
		return nil, fmt.Errorf("%w: bbox-only POI query needs pelias-es URL", ErrUpstreamBadRequest)
	}
	if p.BBoxMinLon == nil || p.BBoxMinLat == nil || p.BBoxMaxLon == nil || p.BBoxMaxLat == nil {
		return nil, fmt.Errorf("%w: bbox-only POI query needs all four boundary.rect.* params", ErrUpstreamBadRequest)
	}
	size := p.Size
	if size <= 0 {
		size = 50
	}
	filters := []map[string]any{
		{"term": map[string]any{"layer": "venue"}},
		{"geo_bounding_box": map[string]any{
			"center_point": map[string]any{
				"top_left": map[string]any{
					"lat": *p.BBoxMaxLat,
					"lon": *p.BBoxMinLon,
				},
				"bottom_right": map[string]any{
					"lat": *p.BBoxMinLat,
					"lon": *p.BBoxMaxLon,
				},
			},
		}},
	}
	if cat := strings.TrimSpace(p.Category); cat != "" {
		// Map UI chip categories to OSM `<key>:<value>` patterns.
		// Pelias indexes `category` as a keyword array (e.g.
		// "amenity:fast_food", "shop:supermarket"). For each chip we
		// build a `bool.should` of `prefix` + `term` clauses.
		shoulds := buildCategoryShoulds(cat)
		if len(shoulds) > 0 {
			filters = append(filters, map[string]any{
				"bool": map[string]any{
					"should":               shoulds,
					"minimum_should_match": 1,
				},
			})
		}
	}
	body := map[string]any{
		"size": size,
		"query": map[string]any{
			"bool": map[string]any{"filter": filters},
		},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%w: encoding ES request: %v", ErrUpstreamUnavailable, err)
	}
	reqCtx, cancel := contextWithTimeout(ctx, c.timeout)
	defer cancel()
	u := strings.TrimRight(c.esURL, "/") + "/pelias/_search"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("%w: building ES request: %v", ErrUpstreamUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: reading ES response: %v", ErrUpstreamUnavailable, err)
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, fmt.Errorf("%w: ES status %d: %s", ErrUpstreamBadRequest,
			resp.StatusCode, truncate(string(respBody), 200))
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: ES status %d: %s", ErrUpstreamUnavailable,
			resp.StatusCode, truncate(string(respBody), 200))
	}
	var parsed esResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("%w: decoding ES response: %v", ErrUpstreamUnavailable, err)
	}
	out := make([]Poi, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		out = append(out, flattenESHit(h))
	}
	return out, nil
}

// flattenESHit reshapes an _source row into the same Poi the pelias-api
// path emits. The two paths share the same Poi schema so callers don't
// have to branch on which path produced the result.
func flattenESHit(h esHit) Poi {
	gid := h.Source.Source + ":" + h.Source.Layer + ":" + h.Source.SourceID
	p := Poi{
		ID:     gid,
		Label:  h.Source.Name.Default,
		Center: LatLon{Lat: h.Source.Center.Lat, Lon: h.Source.Center.Lon},
		Source: "osm",
	}
	if cat := primaryCategory(h.Source.Category); cat != "" {
		c := cat
		p.Category = &c
	}
	if len(h.Source.AddressPart) > 0 {
		p.Tags = make(map[string]string, len(h.Source.AddressPart))
		for k, v := range h.Source.AddressPart {
			p.Tags[k] = v
		}
	}
	if osmAdd := h.Source.Addendum["osm"]; osmAdd != nil {
		p.Hours = osmAdd["opening_hours"]
		p.Phone = osmAdd["phone"]
		p.Website = osmAdd["website"]
		p.Wheelchair = osmAdd["wheelchair"]
		p.Cuisine = osmAdd["cuisine"]
		p.Brand = osmAdd["brand"]
		p.Email = osmAdd["email"]
	}
	return p
}

// chipCategories maps each top-bar POI chip to the OSM `<key>:<value>`
// patterns we want to surface. Items ending in `:` are matched as
// prefixes (e.g. `shop:` → any shop). Exact `<key>:<value>` items are
// matched verbatim. The mapping is intentionally narrow to avoid noise.
var chipCategories = map[string][]string{
	"food": {
		"amenity:restaurant", "amenity:cafe", "amenity:fast_food",
		"amenity:bar", "amenity:pub", "amenity:food_court",
		"amenity:ice_cream", "amenity:biergarten",
		"shop:bakery", "shop:coffee", "shop:pastry",
	},
	"shopping": {
		"shop:", // prefix — any shop
		"amenity:marketplace",
	},
	"transit": {
		"public_transport:", "railway:station", "railway:halt",
		"amenity:bus_station", "highway:bus_stop",
		"aeroway:aerodrome", "aeroway:terminal",
	},
	"lodging": {
		"tourism:hotel", "tourism:hostel", "tourism:motel",
		"tourism:guest_house", "tourism:apartment", "tourism:chalet",
	},
	"services": {
		"amenity:bank", "amenity:atm", "amenity:post_office",
		"amenity:post_box", "amenity:fuel", "amenity:police",
		"amenity:fire_station", "amenity:townhall", "amenity:embassy",
	},
	"healthcare": {
		"amenity:pharmacy", "amenity:hospital", "amenity:clinic",
		"amenity:doctors", "amenity:dentist", "amenity:veterinary",
	},
	"entertainment": {
		"amenity:cinema", "amenity:theatre", "amenity:nightclub",
		"amenity:arts_centre",
		"tourism:museum", "tourism:attraction", "tourism:gallery",
		"tourism:zoo", "tourism:viewpoint",
		"leisure:stadium", "leisure:sports_centre",
	},
	"education": {
		"amenity:school", "amenity:university", "amenity:college",
		"amenity:library", "amenity:kindergarten", "amenity:language_school",
	},
	"other": {
		"leisure:park", "leisure:playground", "leisure:dog_park",
		"amenity:place_of_worship", "amenity:toilets",
		"amenity:drinking_water", "tourism:information",
	},
}

// buildCategoryShoulds returns a slice of ES query clauses that, OR'd,
// match any document whose `category` keyword starts with one of the
// configured patterns for the given chip.
func buildCategoryShoulds(chip string) []map[string]any {
	patterns, ok := chipCategories[chip]
	if !ok {
		// Unknown chip — try the chip name as a literal prefix so future
		// additions don't 400 the user.
		return []map[string]any{
			{"prefix": map[string]any{"category": chip}},
		}
	}
	out := make([]map[string]any, 0, len(patterns))
	for _, pat := range patterns {
		if strings.HasSuffix(pat, ":") {
			out = append(out, map[string]any{
				"prefix": map[string]any{"category": pat},
			})
		} else {
			out = append(out, map[string]any{
				"term": map[string]any{"category": pat},
			})
		}
	}
	return out
}
