// Package peliasindex — doc.go holds the OSM-tag → pelias-document
// translation rules. Split from peliasindex.go so the build loop stays
// readable.
package peliasindex

import (
	"fmt"

	"github.com/paulmach/osm"
)

// doc is the minimal pelias-es document shape. Field names match
// pelias's mapping so pelias-api can read the docs back without a
// schema translation layer.
type doc struct {
	// Note: the real pelias schema declares `"dynamic": "strict"` so any
	// extra top-level keys cause bulk failures. We OMIT `gid` (already
	// carried as the bulk _id) and any ad-hoc keys; pelias-api builds its
	// own gid from `<source>:<layer>:<source_id>` at read time.
	Source      string      `json:"source"`
	SourceID    string      `json:"source_id"`
	Layer       string      `json:"layer"`
	CenterPoint centerPoint `json:"center_point"`
	Name        nameBlock   `json:"name"`
	// Phrase mirrors Name. Pelias-api's autocomplete/search queries
	// match against `phrase.*` (analysed with peliasPhrase) for the
	// non-edge-ngrammed paths; populating it from the same string keeps
	// the two query paths in sync. See pelias/api → query/text_parser.
	Phrase      nameBlock         `json:"phrase"`
	Category    []string          `json:"category,omitempty"`
	// AddressPart keys must match the strict schema: name|unit|number|
	// street|cross_street|zip. See pelias/schema → mappings/document.
	AddressPart map[string]string `json:"address_parts,omitempty"`
	// Addendum carries source-namespaced POI metadata (opening hours,
	// phone, website, …). Pelias convention: `addendum.<source> = {...}`
	// — we namespace under "osm" so future importers can coexist.
	Addendum map[string]map[string]string `json:"addendum,omitempty"`
}

type centerPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type nameBlock struct {
	Default string `json:"default"`
}

// poiTagKeys is the set of OSM tag keys whose presence marks a node as
// a POI worth indexing. Matches the filter listed in the agent brief.
var poiTagKeys = []string{
	"amenity", "shop", "tourism", "leisure", "historic", "office",
}

// placeTagValues is the set of `place=` values we treat as named
// localities (cities, towns, etc.). Everything else (hamlet, isolated
// dwelling, farm) is noisy so we skip it.
var placeTagValues = map[string]bool{
	"city":          true,
	"town":          true,
	"village":       true,
	"suburb":        true,
	"neighbourhood": true,
}

// nodeDoc returns the pelias document for n, or (doc{}, false) when n
// doesn't meet any indexing rule. All named POIs become layer=venue;
// named places become layer=locality.
func nodeDoc(n *osm.Node, region string) (doc, bool) {
	if n == nil {
		return doc{}, false
	}
	name := n.Tags.Find("name")
	// POI check first — more specific than place=*. Requires a `name`
	// tag because anonymous amenities aren't useful in search.
	if name != "" {
		if cat, ok := poiCategory(n.Tags); ok {
			d := docBuilder(fmt.Sprintf("node/%d", n.ID), "venue", name, n.Lat, n.Lon, region, cat, addressParts(n.Tags))
			attachAddendum(&d, n.Tags)
			return d, true
		}
		if pl := n.Tags.Find("place"); pl != "" && placeTagValues[pl] {
			return docBuilder(fmt.Sprintf("node/%d", n.ID), "locality", name, n.Lat, n.Lon, region, nil, nil), true
		}
	}
	// Address node: has addr:housenumber + addr:street, but is neither
	// a POI nor a named place. Emit a layer=address doc so reverse-
	// geocode returns "Strada Lipscani 12" instead of the nearest POI.
	if d, ok := addressDoc(fmt.Sprintf("node/%d", n.ID), n.Tags, n.Lat, n.Lon, region); ok {
		return d, true
	}
	return doc{}, false
}

// wayDoc returns the pelias document for w, or (doc{}, false) when w
// doesn't meet a rule. Ways don't carry coordinates themselves; we
// anchor on the first node's Ref via the osm PBF's dense-node lat/lon
// only when the PBF decoder has populated Nodes[i].Lat/Lon. For a
// streaming-scan PBF that's not populated, so wayDoc returns false
// when the lat/lon is zero (see tests for the valid case).
func wayDoc(w *osm.Way, region string) (doc, bool) {
	if w == nil || len(w.Nodes) == 0 {
		return doc{}, false
	}
	name := w.Tags.Find("name")
	if name != "" {
		// Named streets — layer=street. `highway=*` covers roads, paths,
		// cycleways, etc.
		if hw := w.Tags.Find("highway"); hw != "" {
			lat, lon, ok := firstNodeLatLon(w)
			if !ok {
				return doc{}, false
			}
			return docBuilder(fmt.Sprintf("way/%d", w.ID), "street", name, lat, lon, region, nil, nil), true
		}
		// Buildings / areas carrying a POI-like tag.
		if cat, ok := poiCategory(w.Tags); ok {
			lat, lon, latOk := firstNodeLatLon(w)
			if !latOk {
				return doc{}, false
			}
			d := docBuilder(fmt.Sprintf("way/%d", w.ID), "venue", name, lat, lon, region, cat, addressParts(w.Tags))
			attachAddendum(&d, w.Tags)
			return d, true
		}
	}
	// Building/area carrying only addr:* tags (no name, no POI tag) —
	// emit layer=address so reverse-geocode resolves to a real street
	// number rather than the nearest POI.
	lat, lon, latOk := firstNodeLatLon(w)
	if !latOk {
		return doc{}, false
	}
	if d, ok := addressDoc(fmt.Sprintf("way/%d", w.ID), w.Tags, lat, lon, region); ok {
		return d, true
	}
	return doc{}, false
}

// poiCategory scans tags for a POI-like key (amenity, shop, …) and
// returns a "<key>:<value>" label. Matches pelias's own category
// convention (see categories/openstreetmap.json upstream).
func poiCategory(tags osm.Tags) ([]string, bool) {
	for _, k := range poiTagKeys {
		if v := tags.Find(k); v != "" {
			return []string{k + ":" + v}, true
		}
	}
	return nil, false
}

// docBuilder materialises a doc with the pelias gid convention
// "openstreetmap:<layer>:<source_id>". The region is stored under
// `addendum.osm.region` so per-region purges (delete-by-query) can key
// off it; the strict pelias schema rejects unknown root fields, but the
// `addendum` field is mapped as `dynamic:true` so nested keys are free.
func docBuilder(sourceID, layer, name string, lat, lon float64, region string, category []string, address map[string]string) doc {
	d := doc{
		Source:      "openstreetmap",
		SourceID:    sourceID,
		Layer:       layer,
		CenterPoint: centerPoint{Lat: lat, Lon: lon},
		Name:        nameBlock{Default: name},
		Phrase:      nameBlock{Default: name},
	}
	if len(category) > 0 {
		d.Category = category
	}
	if len(address) > 0 {
		d.AddressPart = address
	}
	if region != "" {
		if d.Addendum == nil {
			d.Addendum = map[string]map[string]string{}
		}
		osmMap := d.Addendum["osm"]
		if osmMap == nil {
			osmMap = map[string]string{}
		}
		osmMap["region"] = region
		d.Addendum["osm"] = osmMap
	}
	return d
}

// addressParts pulls the addr:* subset pelias cares about into a flat
// map keyed by pelias's field names.
func addressParts(tags osm.Tags) map[string]string {
	out := map[string]string{}
	// Schema is strict: only name|unit|number|street|cross_street|zip.
	if v := tags.Find("addr:housenumber"); v != "" {
		out["number"] = v
	}
	if v := tags.Find("addr:street"); v != "" {
		out["street"] = v
	}
	if v := tags.Find("addr:postcode"); v != "" {
		out["zip"] = v
	}
	if v := tags.Find("addr:unit"); v != "" {
		out["unit"] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// addressDoc builds a layer=address pelias doc from a feature's addr:*
// tags. Returns (doc{}, false) when the feature lacks the discriminating
// addr:housenumber tag — without it the doc is just a building shape,
// not an address. Pelias's reverse path treats layer=address as the
// preferred match, so this branch is what makes "Strada Lipscani 12"
// appear instead of the nearest POI.
func addressDoc(sourceID string, t osm.Tags, lat, lon float64, region string) (doc, bool) {
	number := t.Find("addr:housenumber")
	if number == "" {
		return doc{}, false
	}
	street := t.Find("addr:street")
	if street == "" {
		return doc{}, false
	}
	// Pelias's address layer convention is "<street> <number>" — same
	// shape pelias-openaddresses imports use; lets the autocomplete /
	// reverse paths surface a Google-style label.
	name := street + " " + number
	parts := addressParts(t)
	return docBuilder(sourceID, "address", name, lat, lon, region, nil, parts), true
}

// addendumTagKeys is the set of OSM tag keys we surface to pelias as
// POI-detail metadata. These feed the POI detail card in the UI; any
// key not listed here is silently dropped to keep the doc compact.
var addendumTagKeys = []string{
	"opening_hours",
	"phone",
	"website",
	"email",
	"wheelchair",
	"wheelchair:description",
	"cuisine",
	"brand",
	"operator",
	"internet_access",
	"takeaway",
	"outdoor_seating",
}

// extractAddendum pulls the subset of enrichment tags listed in
// addendumTagKeys from tags. Returns nil when none of the keys are
// present so callers can skip the field entirely (keeps docs tidy for
// POIs with no extra metadata).
func extractAddendum(tags osm.Tags) map[string]any {
	var out map[string]any
	for _, k := range addendumTagKeys {
		if v := tags.Find(k); v != "" {
			if out == nil {
				out = map[string]any{}
			}
			out[k] = v
		}
	}
	return out
}

// attachAddendum copies extractAddendum(tags) into d.Addendum["osm"]
// when there is anything to attach. No-op for POIs without enrichment
// tags so the indexed doc stays minimal. Preserves any pre-existing
// keys (e.g. `region` seeded by docBuilder).
func attachAddendum(d *doc, tags osm.Tags) {
	add := extractAddendum(tags)
	if len(add) == 0 {
		return
	}
	if d.Addendum == nil {
		d.Addendum = map[string]map[string]string{}
	}
	osmMap := d.Addendum["osm"]
	if osmMap == nil {
		osmMap = make(map[string]string, len(add))
	}
	for k, v := range add {
		if s, ok := v.(string); ok {
			osmMap[k] = s
		}
	}
	d.Addendum["osm"] = osmMap
}

// firstNodeLatLon returns the first way-node's lat/lon if the PBF
// decoder has populated them (i.e. nodes came from dense-node blocks
// that included tags). Pure streaming scans don't populate this — the
// production path swallows those ways silently.
func firstNodeLatLon(w *osm.Way) (float64, float64, bool) {
	if len(w.Nodes) == 0 {
		return 0, 0, false
	}
	n := w.Nodes[0]
	if n.Lat == 0 && n.Lon == 0 {
		return 0, 0, false
	}
	return n.Lat, n.Lon, true
}

