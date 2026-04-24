package geofabrik

// decode.go isolates the Geofabrik index-v1.json wire format decoding
// from the Client plumbing in client.go. Splitting keeps each file
// under the 250-line limit described in docs/06-agent-rules.md.

import (
	"encoding/json"
	"fmt"
	"strings"
)

// rawFeatureCollection is just enough of index-v1.json to walk it.
type rawFeatureCollection struct {
	Type     string       `json:"type"`
	Features []rawFeature `json:"features"`
}

type rawFeature struct {
	Type       string      `json:"type"`
	Properties rawFeatProp `json:"properties"`
}

type rawFeatProp struct {
	ID        string   `json:"id"`
	Parent    string   `json:"parent"`
	Name      string   `json:"name"`
	ISO3166_1 []string `json:"iso3166-1:alpha2"`
	URLs      rawURLs  `json:"urls"`
	// Geofabrik doesn't publish byte sizes in the catalog; left nil.
}

type rawURLs struct {
	PBF string `json:"pbf"`
	MD5 string `json:"md5"`
}

// decodeCatalog parses index-v1.json and returns the catalog as a tree
// of CatalogEntry. Entry names are full-path canonical keys built by
// walking up the Geofabrik parent chain (so a Baden-Württemberg
// feature whose upstream id is "germany/baden-wuerttemberg" and whose
// parent is "germany" becomes "europe-germany-baden-wuerttemberg").
// A parent of "" means continent.
func decodeCatalog(body []byte) ([]CatalogEntry, error) {
	var fc rawFeatureCollection
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, err
	}
	if fc.Type != "FeatureCollection" {
		return nil, fmt.Errorf("unexpected type %q", fc.Type)
	}
	type rawNode struct {
		id     string // upstream id (may contain '/')
		parent string // upstream parent id ("" for continents)
		feat   rawFeature
	}
	raw := make(map[string]*rawNode, len(fc.Features))
	upstreamOrder := make([]string, 0, len(fc.Features))
	for _, f := range fc.Features {
		if f.Properties.ID == "" {
			continue
		}
		raw[f.Properties.ID] = &rawNode{
			id:     f.Properties.ID,
			parent: f.Properties.Parent,
			feat:   f,
		}
		upstreamOrder = append(upstreamOrder, f.Properties.ID)
	}
	canonCache := make(map[string]string, len(raw))
	var canonOf func(id string) string
	canonOf = func(id string) string {
		if c, ok := canonCache[id]; ok {
			return c
		}
		n, ok := raw[id]
		if !ok {
			return strings.ReplaceAll(id, "/", "-")
		}
		localPart := strings.ReplaceAll(n.id, "/", "-")
		if n.parent != "" && strings.HasPrefix(n.id, n.parent+"/") {
			localPart = strings.ReplaceAll(
				strings.TrimPrefix(n.id, n.parent+"/"), "/", "-")
		}
		var c string
		if n.parent == "" {
			c = localPart
		} else {
			c = canonOf(n.parent) + "-" + localPart
		}
		canonCache[id] = c
		return c
	}
	byCanon := make(map[string]*CatalogEntry, len(raw))
	canonOrder := make([]string, 0, len(raw))
	for _, id := range upstreamOrder {
		n := raw[id]
		canon := canonOf(id)
		kind := KindContinent
		if n.parent != "" {
			if parent, ok := raw[n.parent]; ok && parent.parent != "" {
				kind = KindSubregion
			} else {
				kind = KindCountry
			}
		}
		entry := &CatalogEntry{
			Name:        canon,
			DisplayName: n.feat.Properties.Name,
			Kind:        kind,
			SourceURL:   n.feat.Properties.URLs.PBF,
		}
		if n.parent != "" {
			p := canonOf(n.parent)
			entry.Parent = &p
		}
		if len(n.feat.Properties.ISO3166_1) == 1 {
			iso := n.feat.Properties.ISO3166_1[0]
			entry.ISO31661 = &iso
		}
		byCanon[canon] = entry
		canonOrder = append(canonOrder, canon)
	}
	childrenOf := make(map[string][]string, len(byCanon))
	for _, canon := range canonOrder {
		e := byCanon[canon]
		if e.Parent == nil {
			continue
		}
		childrenOf[*e.Parent] = append(childrenOf[*e.Parent], canon)
	}
	var build func(canon string) CatalogEntry
	build = func(canon string) CatalogEntry {
		e := *byCanon[canon]
		kids := childrenOf[canon]
		if len(kids) > 0 {
			e.Children = make([]CatalogEntry, 0, len(kids))
			for _, kid := range kids {
				e.Children = append(e.Children, build(kid))
			}
		}
		return e
	}
	var roots []CatalogEntry
	for _, canon := range canonOrder {
		e := byCanon[canon]
		if e.Parent == nil {
			roots = append(roots, build(canon))
		}
	}
	return roots, nil
}
