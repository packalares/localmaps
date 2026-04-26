package peliasindex

import (
	"testing"

	"github.com/paulmach/osm"
	"github.com/stretchr/testify/require"
)

// tags is a tiny helper to build an osm.Tags list from a map literal
// without worrying about ordering.
func tags(m map[string]string) osm.Tags {
	out := make(osm.Tags, 0, len(m))
	for k, v := range m {
		out = append(out, osm.Tag{Key: k, Value: v})
	}
	return out
}

func TestNodeDoc_IndexesPOI(t *testing.T) {
	n := &osm.Node{
		ID:   42,
		Lat:  44.4268,
		Lon:  26.1025,
		Tags: tags(map[string]string{"name": "Cafe Bucur", "amenity": "cafe", "addr:city": "Bucharest"}),
	}
	d, ok := nodeDoc(n, "europe-romania")
	require.True(t, ok)
	require.Equal(t, "node/42", d.SourceID)
	require.Equal(t, "venue", d.Layer)
	require.Equal(t, "Cafe Bucur", d.Name.Default)
	require.Equal(t, 44.4268, d.CenterPoint.Lat)
	require.Equal(t, 26.1025, d.CenterPoint.Lon)
	require.Equal(t, []string{"amenity:cafe"}, d.Category)
	// region is tagged into addendum.osm so per-region purges can key
	// off it via _delete_by_query.
	require.NotNil(t, d.Addendum)
	require.Equal(t, "europe-romania", d.Addendum["osm"]["region"])
	// `locality` / `country` fields were removed per strict schema —
	// pelias derives admin hierarchy from `parent.*` only. city/country
	// tags are now dropped silently.
}

func TestNodeDoc_IndexesPlace(t *testing.T) {
	n := &osm.Node{
		ID:   7,
		Lat:  44.43,
		Lon:  26.10,
		Tags: tags(map[string]string{"name": "Bucharest", "place": "city"}),
	}
	d, ok := nodeDoc(n, "europe-romania")
	require.True(t, ok)
	require.Equal(t, "locality", d.Layer)
	require.Equal(t, "Bucharest", d.Name.Default)
	require.Empty(t, d.Category)
}

func TestNodeDoc_SkipsUnnamedPOI(t *testing.T) {
	n := &osm.Node{
		ID:   13,
		Tags: tags(map[string]string{"amenity": "bench"}),
	}
	_, ok := nodeDoc(n, "europe-romania")
	require.False(t, ok)
}

func TestNodeDoc_SkipsHamlets(t *testing.T) {
	n := &osm.Node{
		ID:   13,
		Tags: tags(map[string]string{"name": "Tiny Spot", "place": "hamlet"}),
	}
	_, ok := nodeDoc(n, "europe-romania")
	require.False(t, ok)
}

func TestWayDoc_IndexesNamedStreet(t *testing.T) {
	w := &osm.Way{
		ID:    99,
		Tags:  tags(map[string]string{"name": "Strada Lipscani", "highway": "residential"}),
		Nodes: osm.WayNodes{{ID: 1, Lat: 44.4, Lon: 26.1}},
	}
	d, ok := wayDoc(w, "europe-romania")
	require.True(t, ok)
	require.Equal(t, "street", d.Layer)
	require.Equal(t, "way/99", d.SourceID)
	require.Equal(t, "Strada Lipscani", d.Name.Default)
}

func TestWayDoc_VenueWithAddress(t *testing.T) {
	w := &osm.Way{
		ID: 101,
		Tags: tags(map[string]string{
			"name": "Central Market", "amenity": "marketplace",
			"addr:street": "Strada Lipscani", "addr:housenumber": "12",
		}),
		Nodes: osm.WayNodes{{ID: 1, Lat: 44.4, Lon: 26.1}},
	}
	d, ok := wayDoc(w, "europe-romania")
	require.True(t, ok)
	require.Equal(t, "venue", d.Layer)
	require.Equal(t, []string{"amenity:marketplace"}, d.Category)
	require.Equal(t, "Strada Lipscani", d.AddressPart["street"])
	require.Equal(t, "12", d.AddressPart["number"])
}

func TestWayDoc_SkipsUnnamed(t *testing.T) {
	w := &osm.Way{
		ID:    200,
		Tags:  tags(map[string]string{"highway": "residential"}),
		Nodes: osm.WayNodes{{ID: 1, Lat: 44.4, Lon: 26.1}},
	}
	_, ok := wayDoc(w, "europe-romania")
	require.False(t, ok)
}

func TestNodeDoc_POIAttachesAddendum(t *testing.T) {
	n := &osm.Node{
		ID:  77,
		Lat: 44.4268, Lon: 26.1025,
		Tags: tags(map[string]string{
			"name":           "Cafe Bucur",
			"amenity":        "restaurant",
			"opening_hours":  "Mo-Su 08:00-22:00",
			"phone":          "+40 21 123 4567",
			"website":        "https://cafe-bucur.example",
			"email":          "hi@cafe-bucur.example",
			"wheelchair":     "yes",
			"cuisine":        "romanian",
			"brand":          "Bucur",
			"takeaway":       "yes",
			"outdoor_seating": "yes",
			// Not in the allowlist; must be skipped.
			"fixme": "check hours",
		}),
	}
	d, ok := nodeDoc(n, "europe-romania")
	require.True(t, ok)
	require.NotNil(t, d.Addendum)
	osmAdd, ok := d.Addendum["osm"]
	require.True(t, ok, "addendum must carry an `osm` namespace")
	require.Equal(t, "Mo-Su 08:00-22:00", osmAdd["opening_hours"])
	require.Equal(t, "+40 21 123 4567", osmAdd["phone"])
	require.Equal(t, "https://cafe-bucur.example", osmAdd["website"])
	require.Equal(t, "hi@cafe-bucur.example", osmAdd["email"])
	require.Equal(t, "yes", osmAdd["wheelchair"])
	require.Equal(t, "romanian", osmAdd["cuisine"])
	require.Equal(t, "Bucur", osmAdd["brand"])
	require.Equal(t, "yes", osmAdd["takeaway"])
	require.Equal(t, "yes", osmAdd["outdoor_seating"])
	_, hasFixme := osmAdd["fixme"]
	require.False(t, hasFixme, "non-allowlisted keys must not leak")
}

func TestNodeDoc_POIWithoutEnrichmentCarriesOnlyRegion(t *testing.T) {
	n := &osm.Node{
		ID: 7, Lat: 44.4, Lon: 26.1,
		Tags: tags(map[string]string{"name": "Plain Cafe", "amenity": "cafe"}),
	}
	d, ok := nodeDoc(n, "europe-romania")
	require.True(t, ok)
	// No enrichment tags → addendum carries just the region key so
	// per-region purges still match this doc.
	require.NotNil(t, d.Addendum)
	require.Equal(t, "europe-romania", d.Addendum["osm"]["region"])
	_, hasOpeningHours := d.Addendum["osm"]["opening_hours"]
	require.False(t, hasOpeningHours)
}

func TestNodeDoc_PlaceCarriesRegionInAddendum(t *testing.T) {
	// Places (city/town/village) must NOT carry enrichment-style
	// addendum keys — addendum enrichment is a POI-only concern. They
	// MUST still carry the region tag so per-region purges work.
	n := &osm.Node{
		ID: 9, Lat: 44.43, Lon: 26.10,
		Tags: tags(map[string]string{
			"name": "Bucharest", "place": "city",
			"opening_hours": "24/7",
		}),
	}
	d, ok := nodeDoc(n, "europe-romania")
	require.True(t, ok)
	require.Equal(t, "locality", d.Layer)
	require.NotNil(t, d.Addendum)
	require.Equal(t, "europe-romania", d.Addendum["osm"]["region"])
	_, hasOpeningHours := d.Addendum["osm"]["opening_hours"]
	require.False(t, hasOpeningHours,
		"localities must not surface enrichment-only tags through addendum")
}

func TestExtractAddendum_ReturnsNilWhenEmpty(t *testing.T) {
	require.Nil(t, extractAddendum(tags(map[string]string{"name": "X"})))
}

func TestNodeDoc_IndexesAddress(t *testing.T) {
	// A pure address node (no amenity, no place, no name) carrying
	// addr:street + addr:housenumber must surface as layer=address so
	// reverse-geocode can return "Strada Lipscani 12" verbatim.
	n := &osm.Node{
		ID:  500,
		Lat: 44.4268, Lon: 26.1025,
		Tags: tags(map[string]string{
			"addr:street":      "Strada Lipscani",
			"addr:housenumber": "12",
			"addr:postcode":    "030167",
			"addr:city":        "Bucharest",
		}),
	}
	d, ok := nodeDoc(n, "europe-romania")
	require.True(t, ok)
	require.Equal(t, "address", d.Layer)
	require.Equal(t, "node/500", d.SourceID)
	require.Equal(t, "Strada Lipscani 12", d.Name.Default)
	require.Equal(t, "Strada Lipscani 12", d.Phrase.Default)
	require.Equal(t, "12", d.AddressPart["number"])
	require.Equal(t, "Strada Lipscani", d.AddressPart["street"])
	require.Equal(t, "030167", d.AddressPart["zip"])
	require.NotNil(t, d.Addendum)
	require.Equal(t, "europe-romania", d.Addendum["osm"]["region"])
	require.Empty(t, d.Category)
}

func TestNodeDoc_AddressSkippedWithoutHousenumber(t *testing.T) {
	// addr:street alone is just a building shape, not a useful address;
	// the housenumber is the discriminator that turns it into a real
	// reverse-geocode target.
	n := &osm.Node{
		ID: 501, Lat: 44.4, Lon: 26.1,
		Tags: tags(map[string]string{"addr:street": "Strada Lipscani"}),
	}
	_, ok := nodeDoc(n, "europe-romania")
	require.False(t, ok)
}

func TestNodeDoc_AddressSkippedWithoutStreet(t *testing.T) {
	// And a housenumber without a street name is unlabel-able too.
	n := &osm.Node{
		ID: 502, Lat: 44.4, Lon: 26.1,
		Tags: tags(map[string]string{"addr:housenumber": "12"}),
	}
	_, ok := nodeDoc(n, "europe-romania")
	require.False(t, ok)
}

func TestNodeDoc_VenueWithAddressDoesNotEmitDuplicate(t *testing.T) {
	// A POI that also carries addr:* tags must emit ONLY the venue doc
	// — the address branch is for pure-address features.
	n := &osm.Node{
		ID:  503,
		Lat: 44.4268, Lon: 26.1025,
		Tags: tags(map[string]string{
			"name":             "Cafe Bucur",
			"amenity":          "cafe",
			"addr:street":      "Strada Lipscani",
			"addr:housenumber": "12",
		}),
	}
	d, ok := nodeDoc(n, "europe-romania")
	require.True(t, ok)
	require.Equal(t, "venue", d.Layer)
	require.Equal(t, "Cafe Bucur", d.Name.Default)
	// The venue doc still carries the address parts so search by
	// address still resolves to the cafe.
	require.Equal(t, "12", d.AddressPart["number"])
	require.Equal(t, "Strada Lipscani", d.AddressPart["street"])
}

func TestWayDoc_IndexesBuildingAddress(t *testing.T) {
	// A building footprint with addr:street + addr:housenumber but no
	// name and no POI tag should surface as layer=address.
	w := &osm.Way{
		ID: 600,
		Tags: tags(map[string]string{
			"building":         "yes",
			"addr:street":      "Strada Lipscani",
			"addr:housenumber": "14",
			"addr:postcode":    "030167",
		}),
		Nodes: osm.WayNodes{{ID: 1, Lat: 44.4, Lon: 26.1}},
	}
	d, ok := wayDoc(w, "europe-romania")
	require.True(t, ok)
	require.Equal(t, "address", d.Layer)
	require.Equal(t, "way/600", d.SourceID)
	require.Equal(t, "Strada Lipscani 14", d.Name.Default)
	require.Equal(t, "14", d.AddressPart["number"])
	require.Equal(t, "Strada Lipscani", d.AddressPart["street"])
	require.Equal(t, "030167", d.AddressPart["zip"])
}

func TestWayDoc_VenueAttachesAddendum(t *testing.T) {
	w := &osm.Way{
		ID: 555,
		Tags: tags(map[string]string{
			"name": "Central Market", "amenity": "marketplace",
			"opening_hours": "Mo-Sa 06:00-18:00",
			"website":       "https://market.example",
		}),
		Nodes: osm.WayNodes{{ID: 1, Lat: 44.4, Lon: 26.1}},
	}
	d, ok := wayDoc(w, "europe-romania")
	require.True(t, ok)
	require.NotNil(t, d.Addendum)
	require.Equal(t, "Mo-Sa 06:00-18:00", d.Addendum["osm"]["opening_hours"])
	require.Equal(t, "https://market.example", d.Addendum["osm"]["website"])
}
