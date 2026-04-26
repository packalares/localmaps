package geocoding_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/geocoding"
)

// newFakePelias mounts a pelias-api fake. Each path returns a canned
// FeatureCollection; the capture map lets tests assert the forwarded
// query string.
func newFakePelias(t *testing.T, capture map[string]string) *httptest.Server {
	t.Helper()
	sample := map[string]interface{}{
		"type": "FeatureCollection",
		"features": []map[string]interface{}{
			{
				"type":     "Feature",
				"geometry": map[string]interface{}{"type": "Point", "coordinates": []float64{26.1025, 44.4268}},
				"bbox":     []float64{26.1, 44.4, 26.2, 44.5},
				"properties": map[string]interface{}{
					"id":         "123",
					"gid":        "openstreetmap:venue:node/123",
					"layer":      "venue",
					"source":     "openstreetmap",
					"source_id":  "node/123",
					"name":       "Cafe Bucur",
					"label":      "Cafe Bucur, Bucharest, Romania",
					"confidence": 0.95,
					"housenumber": "12",
					"street":      "Strada Lipscani",
					"locality":    "Bucharest",
					"country":     "Romania",
					"category":    []string{"food:cafe"},
				},
			},
		},
	}
	body, _ := json.Marshal(sample)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			capture[r.URL.Path] = r.URL.RawQuery
		}
		switch r.URL.Path {
		case "/v1/autocomplete", "/v1/search", "/v1/reverse", "/v1/place":
			_, _ = w.Write(body)
		case "/empty":
			_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAutocomplete_FlattensFeature(t *testing.T) {
	capture := map[string]string{}
	srv := newFakePelias(t, capture)
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	focusLat, focusLon := 44.4268, 26.1025
	results, err := c.Autocomplete(context.Background(), geocoding.AutocompleteParams{
		Text:     "cafe",
		FocusLat: &focusLat,
		FocusLon: &focusLon,
		Size:     5,
		Lang:     "en",
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Cafe Bucur, Bucharest, Romania", results[0].Label)
	require.Equal(t, "openstreetmap:venue:node/123", results[0].ID)
	require.InDelta(t, 44.4268, results[0].Center.Lat, 1e-9)
	require.InDelta(t, 26.1025, results[0].Center.Lon, 1e-9)
	require.Equal(t, 0.95, results[0].Confidence)
	require.Equal(t, "Bucharest", results[0].Address["locality"])
	require.NotNil(t, results[0].Category)
	require.Equal(t, "food:cafe", *results[0].Category)

	// Forwarded query string must carry focus.point.* and size.
	q := capture["/v1/autocomplete"]
	require.Contains(t, q, "text=cafe")
	require.Contains(t, q, "focus.point.lat=44.4268")
	require.Contains(t, q, "focus.point.lon=26.1025")
	require.Contains(t, q, "size=5")
	require.Contains(t, q, "lang=en")
}

func TestSearch_OmitsFocusWhenAbsent(t *testing.T) {
	capture := map[string]string{}
	srv := newFakePelias(t, capture)
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	_, err := c.Search(context.Background(), geocoding.SearchParams{Text: "bucuresti"})
	require.NoError(t, err)

	q := capture["/v1/search"]
	require.Contains(t, q, "text=bucuresti")
	require.NotContains(t, q, "focus.point")
}

func TestReverse_EmptyFeaturesReturnsNotFound(t *testing.T) {
	// Build a server that responds to /v1/reverse with an empty
	// FeatureCollection so the client translates it to ErrNotFound.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	_, err := c.Reverse(context.Background(), geocoding.ReverseParams{Lat: 1, Lon: 2})
	require.Error(t, err)
	require.True(t, errors.Is(err, geocoding.ErrNotFound))
}

func TestReverse_RequestsAddressVenueStreetLayers(t *testing.T) {
	// Reverse-geocode must prefer real addresses over POIs / streets so
	// "click on map → Strada Lipscani 12" works the way Google does.
	// The forwarded query string must list the layers in priority order.
	capture := map[string]string{}
	srv := newFakePelias(t, capture)
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	_, err := c.Reverse(context.Background(), geocoding.ReverseParams{Lat: 44.4268, Lon: 26.1025})
	require.NoError(t, err)

	q := capture["/v1/reverse"]
	require.Contains(t, q, "layers=address%2Cvenue%2Cstreet")
	require.Contains(t, q, "point.lat=44.4268")
	require.Contains(t, q, "point.lon=26.1025")
}

func TestQueryPois_FiltersToVenue(t *testing.T) {
	// Mix of venue + non-venue features — client must drop the
	// non-venue entry.
	body := `{"type":"FeatureCollection","features":[
		{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},
		 "properties":{"gid":"a","layer":"venue","name":"Venue A","label":"Venue A"}},
		{"type":"Feature","geometry":{"type":"Point","coordinates":[3,4]},
		 "properties":{"gid":"b","layer":"address","name":"B","label":"B"}}
	]}`
	capture := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture[r.URL.Path] = r.URL.RawQuery
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	minLon, minLat, maxLon, maxLat := 25.0, 44.0, 27.0, 45.0
	pois, err := c.QueryPois(context.Background(), geocoding.PoiQueryParams{
		BBoxMinLon: &minLon, BBoxMinLat: &minLat,
		BBoxMaxLon: &maxLon, BBoxMaxLat: &maxLat,
		Category: "restaurant",
		Size:     50,
	})
	require.NoError(t, err)
	require.Len(t, pois, 1)
	require.Equal(t, "a", pois[0].ID)
	require.Equal(t, "osm", pois[0].Source)

	q := capture["/v1/search"]
	require.Contains(t, q, "layers=venue")
	require.Contains(t, q, "categories=restaurant")
	require.Contains(t, q, "boundary.rect.min_lon=25")
	require.Contains(t, q, "boundary.rect.max_lat=45")
	// Pelias rejects `text=*`; the empty-text + category-supplied case
	// must use the category as the text token, not the wildcard.
	require.NotContains(t, q, "text=%2A", "pelias rejects text=* — must not be sent")
	require.NotContains(t, q, "text=*")
	require.Contains(t, q, "text=restaurant")
}

// When text is empty AND category is empty (true bbox-only query) the
// client must NOT hit /v1/search with a wildcard text — pelias 400s.
// Instead it falls back to a direct pelias-es POST.
func TestQueryPois_BBoxOnly_FallsBackToES(t *testing.T) {
	var seenSearchPath, seenESPath string
	var seenESBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pelias/_search":
			seenESPath = r.URL.Path
			b, _ := io.ReadAll(r.Body)
			seenESBody = b
			_, _ = w.Write([]byte(`{"hits":{"hits":[
				{"_source":{"source":"openstreetmap","source_id":"node/1","layer":"venue",
				 "center_point":{"lat":44.5,"lon":26.1},"name":{"default":"X"}}}
			]}}`))
		default:
			seenSearchPath = r.URL.Path
			_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
		}
	}))
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	c.SetESURL(srv.URL)
	minLon, minLat, maxLon, maxLat := 25.0, 44.0, 27.0, 45.0
	pois, err := c.QueryPois(context.Background(), geocoding.PoiQueryParams{
		BBoxMinLon: &minLon, BBoxMinLat: &minLat,
		BBoxMaxLon: &maxLon, BBoxMaxLat: &maxLat,
		Size: 25,
	})
	require.NoError(t, err)
	require.Len(t, pois, 1)
	require.Equal(t, "openstreetmap:venue:node/1", pois[0].ID)
	require.Equal(t, "osm", pois[0].Source)

	require.Equal(t, "", seenSearchPath, "/v1/search must not be hit on bbox-only query")
	require.Equal(t, "/pelias/_search", seenESPath)
	require.Contains(t, string(seenESBody), `"layer":"venue"`)
	require.Contains(t, string(seenESBody), `"geo_bounding_box"`)
	// And the synthetic text=* must never have appeared.
	require.NotContains(t, string(seenESBody), `"text":"*"`)
}

func TestPlace_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer srv.Close()

	c := geocoding.NewClient(srv.URL)
	_, err := c.Place(context.Background(), "gid:missing")
	require.ErrorIs(t, err, geocoding.ErrNotFound)
}

func TestGetRaw_ClassifiesStatusCodes(t *testing.T) {
	cases := []struct {
		status int
		target error
	}{
		{http.StatusBadRequest, geocoding.ErrUpstreamBadRequest},
		{http.StatusNotFound, geocoding.ErrUpstreamBadRequest},
		{http.StatusInternalServerError, geocoding.ErrUpstreamUnavailable},
		{http.StatusBadGateway, geocoding.ErrUpstreamUnavailable},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		c := geocoding.NewClient(srv.URL)
		_, err := c.Search(context.Background(), geocoding.SearchParams{Text: "x"})
		require.Error(t, err)
		require.Truef(t, errors.Is(err, tc.target),
			"status %d wanted %v, got %v", tc.status, tc.target, err)
		srv.Close()
	}
}
