package routing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newFakeValhalla returns an httptest.Server whose /route, /isochrone
// and /sources_to_targets handlers return canned responses shaped like
// the real thing. fn is called with (path, rawBody) for assertions.
func newFakeValhalla(t *testing.T, fn func(path string, body []byte)) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/route", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if fn != nil {
			fn("/route", body)
		}
		// Encoded polyline of three points near Bucharest (see polyline_test).
		pts := []LatLon{
			{Lat: 44.4268, Lon: 26.1025},
			{Lat: 44.4270, Lon: 26.1030},
			{Lat: 44.4272, Lon: 26.1029},
		}
		encoded := encodePolyline6(pts)
		resp := map[string]interface{}{
			"trip": map[string]interface{}{
				"units": "kilometers",
				"locations": []map[string]float64{
					{"lat": 44.4268, "lon": 26.1025},
					{"lat": 44.4272, "lon": 26.1029},
				},
				"summary": map[string]float64{"time": 120, "length": 0.5},
				"legs": []map[string]interface{}{
					{
						"shape":   encoded,
						"summary": map[string]float64{"time": 120, "length": 0.5},
						"maneuvers": []map[string]interface{}{
							{
								"instruction":       "Head east on Main Street",
								"begin_shape_index": 0,
								"end_shape_index":   2,
								"length":            0.5,
								"time":              120,
								"type":              1,
								"street_names":      []string{"Main Street"},
							},
						},
					},
				},
				"status": 0,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/isochrone", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if fn != nil {
			fn("/isochrone", body)
		}
		// Canned GeoJSON FeatureCollection.
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[26.1,44.4],[26.2,44.4],[26.2,44.5],[26.1,44.4]]]},"properties":{"contour":10}}]}`))
	})
	mux.HandleFunc("/sources_to_targets", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if fn != nil {
			fn("/sources_to_targets", body)
		}
		_, _ = w.Write([]byte(`{"units":"kilometers","sources_to_targets":[[{"time":60,"distance":0.5}]]}`))
	})
	return httptest.NewServer(mux)
}

func TestClient_Route_MapsValhallaResponse(t *testing.T) {
	var captured []byte
	srv := newFakeValhalla(t, func(path string, body []byte) {
		if path == "/route" {
			captured = body
		}
	})
	defer srv.Close()
	c := NewClient(srv.URL)

	resp, err := c.Route(context.Background(), RouteRequest{
		Locations: []LatLon{{Lat: 44.4268, Lon: 26.1025}, {Lat: 44.4272, Lon: 26.1029}},
		Mode:      ModeAuto,
	}, "trace-123")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if resp.TraceID != "trace-123" {
		t.Errorf("traceId propagated: want %q got %q", "trace-123", resp.TraceID)
	}
	if len(resp.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(resp.Routes))
	}
	r := resp.Routes[0]
	if r.ID == "" {
		t.Error("route id must be non-empty")
	}
	if r.Summary.DistanceMeters != 500 {
		t.Errorf("distance meters: want 500 got %v", r.Summary.DistanceMeters)
	}
	if len(r.Legs) != 1 || len(r.Legs[0].Maneuvers) != 1 {
		t.Fatalf("leg/maneuver count wrong: %+v", r.Legs)
	}
	if r.Legs[0].Maneuvers[0].Instruction != "Head east on Main Street" {
		t.Errorf("maneuver instruction mapped wrong: %q", r.Legs[0].Maneuvers[0].Instruction)
	}
	// Cached route is retrievable + has decoded shape.
	cr, ok := c.LookupRoute(r.ID)
	if !ok {
		t.Fatal("expected cached route for generated id")
	}
	if len(cr.Shape) != 3 {
		t.Errorf("cached shape should have 3 points, got %d", len(cr.Shape))
	}
	// Check the outbound body translated mode -> costing.
	var sent map[string]interface{}
	if err := json.Unmarshal(captured, &sent); err != nil {
		t.Fatalf("captured body not json: %v", err)
	}
	if sent["costing"] != "auto" {
		t.Errorf("costing: want auto got %v", sent["costing"])
	}
	if _, hasLocations := sent["locations"].([]interface{}); !hasLocations {
		t.Error("outbound body missing locations array")
	}
}

func TestClient_Route_RejectsBadInputWithoutUpstream(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // unreachable; must fail before hitting it
	_, err := c.Route(context.Background(), RouteRequest{
		Locations: []LatLon{{Lat: 1, Lon: 1}},
		Mode:      ModeAuto,
	}, "")
	if err == nil {
		t.Fatal("expected error for <2 locations")
	}
	if !strings.Contains(err.Error(), "need at least 2") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClient_Isochrone_PassesThroughGeoJSON(t *testing.T) {
	srv := newFakeValhalla(t, nil)
	defer srv.Close()
	c := NewClient(srv.URL)

	raw, err := c.Isochrone(context.Background(), IsochroneRequest{
		Origin:          LatLon{Lat: 44.43, Lon: 26.10},
		Mode:            ModePedestrian,
		ContoursSeconds: []int{600},
	})
	if err != nil {
		t.Fatalf("Isochrone: %v", err)
	}
	// Must be valid JSON and a FeatureCollection.
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("isochrone response not json: %v", err)
	}
	if parsed["type"] != "FeatureCollection" {
		t.Errorf("expected FeatureCollection, got %v", parsed["type"])
	}
}

func TestClient_Matrix_ConvertsKmToMeters(t *testing.T) {
	srv := newFakeValhalla(t, nil)
	defer srv.Close()
	c := NewClient(srv.URL)

	resp, err := c.Matrix(context.Background(), MatrixRequest{
		Sources: []LatLon{{Lat: 1, Lon: 2}},
		Targets: []LatLon{{Lat: 3, Lon: 4}},
		Mode:    ModeAuto,
	}, "trace-matrix")
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	if resp.TraceID != "trace-matrix" {
		t.Errorf("traceId: want %q got %q", "trace-matrix", resp.TraceID)
	}
	if len(resp.Matrix) != 1 || len(resp.Matrix[0]) != 1 {
		t.Fatalf("matrix shape: want 1x1, got %dx…", len(resp.Matrix))
	}
	cell := resp.Matrix[0][0]
	if cell.DistanceMeters == nil || *cell.DistanceMeters != 500 {
		t.Errorf("distance meters: want 500 got %+v", cell.DistanceMeters)
	}
	if cell.TimeSeconds == nil || *cell.TimeSeconds != 60 {
		t.Errorf("time seconds: want 60 got %+v", cell.TimeSeconds)
	}
}

func TestClient_StripsTrailingSlash(t *testing.T) {
	c := NewClient("http://valhalla:8002//")
	if c.BaseURL() != "http://valhalla:8002" {
		t.Errorf("baseURL: want %q got %q", "http://valhalla:8002", c.BaseURL())
	}
}

func TestClient_EmptyURLFallsBackToDefault(t *testing.T) {
	c := NewClient("   ")
	if c.BaseURL() != "http://valhalla:8002" {
		t.Errorf("baseURL: want fallback, got %q", c.BaseURL())
	}
}
