package geocoding_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/geocoding"
)

// newAddendumPelias mounts a pelias-api fake that returns one venue
// feature carrying an `addendum.osm` metadata block. Used by Place /
// flattenPoi tests to confirm the enrichment fields are surfaced on
// the Poi struct.
func newAddendumPelias(t *testing.T, addendum string) *httptest.Server {
	t.Helper()
	body := `{"type":"FeatureCollection","features":[{
		"type":"Feature",
		"geometry":{"type":"Point","coordinates":[26.1025,44.4268]},
		"properties":{
			"id":"node/42",
			"gid":"openstreetmap:venue:node/42",
			"layer":"venue",
			"source":"openstreetmap",
			"source_id":"node/42",
			"name":"Cafe Bucur",
			"label":"Cafe Bucur, Bucharest",
			"category":["amenity:restaurant"],
			"addendum":` + addendum + `
		}
	}]}`
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
}

// TestPlace_SurfacesOSMAddendum covers the common case — pelias-api
// returns addendum as an object `{ "osm": {...} }` — and makes sure
// flattenPoi maps each expected key onto the Poi struct.
func TestPlace_SurfacesOSMAddendum(t *testing.T) {
	addendum := `{"osm":{
		"opening_hours":"Mo-Su 08:00-22:00",
		"phone":"+40 21 123 4567",
		"website":"https://cafe-bucur.example",
		"email":"hi@cafe-bucur.example",
		"wheelchair":"yes",
		"cuisine":"romanian",
		"brand":"Bucur"
	}}`
	srv := newAddendumPelias(t, addendum)
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	p, err := c.Place(context.Background(), "openstreetmap:venue:node/42")
	require.NoError(t, err)
	require.Equal(t, "openstreetmap:venue:node/42", p.ID)
	require.Equal(t, "Mo-Su 08:00-22:00", p.Hours)
	require.Equal(t, "+40 21 123 4567", p.Phone)
	require.Equal(t, "https://cafe-bucur.example", p.Website)
	require.Equal(t, "hi@cafe-bucur.example", p.Email)
	require.Equal(t, "yes", p.Wheelchair)
	require.Equal(t, "romanian", p.Cuisine)
	require.Equal(t, "Bucur", p.Brand)
	require.NotNil(t, p.Category)
	require.Equal(t, "amenity:restaurant", *p.Category)
}

// TestPlace_AddendumStringifiedOSMBlock covers the alternate shape
// `addendum.osm = "<json-string>"` some pelias importers emit to fit
// the ES keyword mapping. flattenPoi must parse the inner JSON.
func TestPlace_AddendumStringifiedOSMBlock(t *testing.T) {
	// The inner JSON must be escaped as a JSON string literal.
	inner := `"{\"opening_hours\":\"24/7\",\"phone\":\"+1\"}"`
	srv := newAddendumPelias(t, `{"osm":`+inner+`}`)
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	p, err := c.Place(context.Background(), "openstreetmap:venue:node/42")
	require.NoError(t, err)
	require.Equal(t, "24/7", p.Hours)
	require.Equal(t, "+1", p.Phone)
}

// TestPlace_NoAddendumLeavesFieldsZero — feature without an addendum
// block must not populate Hours/Phone/… (they stay at zero value so
// `omitempty` keeps the JSON response clean).
func TestPlace_NoAddendumLeavesFieldsZero(t *testing.T) {
	body := `{"type":"FeatureCollection","features":[{
		"type":"Feature",
		"geometry":{"type":"Point","coordinates":[1,2]},
		"properties":{"gid":"x","layer":"venue","name":"X","label":"X"}
	}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	p, err := c.Place(context.Background(), "x")
	require.NoError(t, err)
	require.Empty(t, p.Hours)
	require.Empty(t, p.Phone)
	require.Empty(t, p.Website)
	require.Empty(t, p.Email)
	require.Empty(t, p.Wheelchair)
	require.Empty(t, p.Cuisine)
	require.Empty(t, p.Brand)
}
